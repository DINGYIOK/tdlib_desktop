package account_client

import (
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"
	"tdlib_desktop/db/model"
	"tdlib_desktop/tools"
	"time"

	"github.com/spf13/cast"
	"github.com/zelenin/go-tdlib/client"
	"gorm.io/gorm"
)

type TelegramServiceMessage struct {
	UserID int    `json:"userId"`
	Text   string `json:"text"`
}

type TelegramService struct {
	Phone         string         // 手机号
	UserID        int64          // 用户 ID
	Path          string         // 存储路径
	DB            *gorm.DB       // 数据库连接
	AuthStatus    bool           // 登陆状态
	Client        *client.Client // 客户端
	AccountStatus bool           // 账户状态 账户是否被封等等
	//Proxy         *TelegramProxy // 代理链接
	//Listener       *client.Listener            // 监听器
	//MessageChannel chan TelegramServiceMessage // 消息通道

	// 关键：保存 authorizer 实例（虽然类型是私有的，但可以通过接口持有）
	authorizer interface {
		Handle(*client.Client, client.AuthorizationState) error
		Close()
	}

	// 直接暴露 channel 给外部使用
	PhoneChannel    chan string                    // 手机号通道
	CodeChannel     chan string                    // 验证码通道
	PasswordChannel chan string                    // 二步密码通道
	StateChannel    chan client.AuthorizationState // 状态通道
	mu              sync.RWMutex                   // 读写锁
	LastAccessTime  time.Time                      // TODO 记录最后一次 API 调用时间
	CreatedAt       time.Time                      // 创建时间
}

// CreateTelegramService 创建客户端
func CreateTelegramService(phone string, db *gorm.DB) *TelegramService {
	return &TelegramService{
		Phone:      phone,
		DB:         db,
		AuthStatus: false,
		CreatedAt:  time.Now(),
	}
}

// InitializeClient 初始化客户端
func (s *TelegramService) InitializeClient() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 1. 设置路径
	path := fmt.Sprintf(".tdlibs/%s", strings.ReplaceAll(s.Phone, "+", ""))
	s.Path = path

	// 1.2 获取appID和appHash
	var appIdSetting model.TelegramClientSettings
	err := s.DB.Model(&model.TelegramClientSettings{}).
		Where("key = ?", "appID").
		First(&appIdSetting).Error
	if err != nil {
		return fmt.Errorf("客户端 Phone:%s 查询AppID错误: %w", s.Phone, err)
	}

	var appHashSetting model.TelegramClientSettings
	err = s.DB.Model(&model.TelegramClientSettings{}).
		Where("key = ?", "appHash").
		First(&appHashSetting).Error
	if err != nil {
		return fmt.Errorf("客户端 Phone:%s 查询AppHash错误: %w", s.Phone, err)
	}

	// 2. 数据库逻辑
	var telegramClientAccount model.TelegramClientAccount
	err = s.DB.Model(&model.TelegramClientAccount{}).
		Where("phone = ?", s.Phone).
		Attrs(model.TelegramClientAccount{
			Phone:        s.Phone,
			AppID:        appIdSetting.Value,
			AppHash:      appHashSetting.Value,
			DatabasePath: path,
		}).
		FirstOrCreate(&telegramClientAccount).Error
	if err != nil {
		return fmt.Errorf("客户端 Phone:%s 创建错误: %w", s.Phone, err)
	}

	// 3. 创建 TDLib 参数
	tdlibParameters := &client.SetTdlibParametersRequest{
		UseTestDc:           false,
		DatabaseDirectory:   filepath.Join(path, "database"),
		FilesDirectory:      filepath.Join(path, "files"),
		UseFileDatabase:     true,
		UseChatInfoDatabase: true,
		UseMessageDatabase:  true,
		UseSecretChats:      false,
		ApiId:               cast.ToInt32(appIdSetting.Value),
		ApiHash:             appHashSetting.Value,
		SystemLanguageCode:  "en",
		DeviceModel:         "CaiCai Client",
		SystemVersion:       "1.0.0",
		ApplicationVersion:  "1.0.0",
	}
	// 4. 使用库提供的 ClientAuthorizer（关键！只创建一次）
	authorizer := client.ClientAuthorizer(tdlibParameters)
	s.authorizer = authorizer

	s.PhoneChannel = authorizer.PhoneNumber
	s.CodeChannel = authorizer.Code
	s.PasswordChannel = authorizer.Password
	s.StateChannel = authorizer.State

	tools.Go(fmt.Sprintf("Phone:%s 登陆", s.Phone), s.handleAuthStates) // 登陆
	time.Sleep(2 * time.Second)
	tools.Go(fmt.Sprintf("Phone:%s 创建客户端", s.Phone), s.CreateClient) // 创建客户端

	// 8. 启动状态监听（类似 CliInteractor 的逻辑）
	return nil
}

// CreateClient 启动创建协程客户端
func (s *TelegramService) CreateClient() {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// 6. 配置日志
	logStream := func(tdlibClient *client.Client) {
		tdlibClient.SetLogStream(&client.SetLogStreamRequest{
			LogStream: &client.LogStreamFile{
				Path:           filepath.Join(s.Path, "tdlib.log"),
				MaxFileSize:    10485760,
				RedirectStderr: true,
			},
		})
	}
	logVerbosity := func(tdlibClient *client.Client) {
		tdlibClient.SetLogVerbosityLevel(&client.SetLogVerbosityLevelRequest{NewVerbosityLevel: 1})
	}

	slog.Info("CreateClient: 开始创建客户端")
	// 7. 创建客户端（使用同一个 authorizer！）
	tdlibClient, err := client.NewClient(s.authorizer, logStream, logVerbosity)
	if err != nil {
		slog.Error(fmt.Sprintf("创建客户端 Phone:%s 错误:%s", s.Phone, err.Error()))
		return
	}

	s.Client = tdlibClient        // 设置客户端
	s.AuthStatus = true           // 设置登陆成功
	s.LastAccessTime = time.Now() // 设置登陆成功时间

	user, err := s.Client.GetMe() // 获取个人信息
	if err != nil {
		slog.Error(fmt.Sprintf("获取 Phone:%s 账户信息错误:%s", s.Phone, err.Error()))
		return
	}
	slog.Debug("获取个人信息成功")
	err = s.DB.Model(&model.TelegramClientAccount{}).
		Where("phone = ?", s.Phone).
		Updates(model.TelegramClientAccount{
			AccountStatus: 3,
			FirstName:     user.FirstName,
			LastName:      user.LastName,
			TGUserId:      user.Id,
			IsPremium:     user.IsPremium,
			IsActive:      true,
		}).Error
	if err != nil {
		slog.Error(fmt.Sprintf("更新 Phone:%s 账户信息错误:%s", s.Phone, err.Error()))
		return
	}
	slog.Info("CreateClient: 创建客户端完成")
}

// handleAuthStates 类似于 CliInteractor，但是通过 Web API 接收输入
func (s *TelegramService) handleAuthStates() {
	slog.Info("开始监听认证状态...")
	for {
		select {
		case state, ok := <-s.StateChannel:
			if !ok {
				slog.Info("⚠️状态 channel 已关闭")
				return
			}
			slog.Info("收到认证状态", "type", state.AuthorizationStateType())
			switch state.AuthorizationStateType() {
			case client.TypeAuthorizationStateWaitPhoneNumber:
				// 自动发送手机号（已经在初始化时设置）
				// slog.Info("需要手机号，自动发送", "phone", s.Phone)
				s.PhoneChannel <- s.Phone
			case client.TypeAuthorizationStateWaitCode:
				// slog.Info("⏳ 等待验证码... 请通过 API 提交")
			// 不做任何事，等待外部调用 SubmitCode
			case client.TypeAuthorizationStateWaitPassword:
				// slog.Info("⏳ 等待二步验证密码... 请通过 API 提交")
			// 不做任何事，等待外部调用 SubmitPassword
			case client.TypeAuthorizationStateReady:
				// slog.Info("✅ 登录成功!")
				// s.mu.Lock()
				// s.AuthStatus = true
				// s.mu.Unlock()
				// // 启动消息监听
				// go s.serviceListener()
				return
			case client.TypeAuthorizationStateClosed:
				slog.Info("❌ 客户端已关闭")
				return
			default:
				//slog.Warn(fmt.Sprintf("⚠️未处理的状态:%s", state.AuthorizationStateType()))
			}
		}
	}
}

// GetChatID 根据用户名获取ChatID
func (s *TelegramService) GetChatID(username string) (int64, error) {
	s.LastAccessTime = time.Now()
	if username == "SpamBot" {
		// 去 Tdlib里进行查询
		chat, err := s.Client.SearchPublicChat(&client.SearchPublicChatRequest{Username: username})
		if err != nil {
			return 0, fmt.Errorf("搜索SpamBot机器人ID:%s 错误: %w", username, err)
		}
		return chat.Id, nil
	}

	// 所有的 chatID和Username都要存储起来
	var telegramClientChat model.TelegramClientChat
	err := s.DB.Model(&model.TelegramClientChat{}).
		Where("username = ?", username).
		First(&telegramClientChat).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) { // 如果错误不等于nil 并且 不是查询不到的错误
		return 0, fmt.Errorf("客户端Phone:%s 数据库查询用户名:%s 错误:%w", s.Phone, username, err)
	}

	// 如果存在就需要返回
	if telegramClientChat != (model.TelegramClientChat{}) {
		return telegramClientChat.ChatID, nil
	}

	// 去 Tdlib里进行查询
	chat, err := s.Client.SearchPublicChat(&client.SearchPublicChatRequest{Username: username})
	if err != nil {
		return 0, fmt.Errorf("搜索公众聊天:%s 错误: %w", username, err)
	}

	return chat.Id, nil
}

// SendMessage 根据用户名发送消息
func (s *TelegramService) SendMessage(chatID int64, fullText string, offset int32, length int32, linkURL string) error {
	s.LastAccessTime = time.Now()

	for {
		if s.Client == nil {
			time.Sleep(2 * time.Second)
		} else {
			break
		}
	}

	_, err := s.Client.SendMessage(&client.SendMessageRequest{
		ChatId: chatID,
		InputMessageContent: &client.InputMessageText{
			Text: &client.FormattedText{
				Text: fullText,
				Entities: []*client.TextEntity{
					{
						Offset: offset,
						Length: length,
						Type: &client.TextEntityTypeTextUrl{
							Url: linkURL,
						},
					},
				},
			},
			ClearDraft: true,
		},
	})
	if err != nil {
		return fmt.Errorf("%d 发送信息错误: %w", chatID, err)
	}
	return nil
}

// CloneAndLogOut 退出登陆并删除
func (s *TelegramService) CloneAndLogOut() error {
	s.LastAccessTime = time.Now()
	for {
		if s.Client == nil {
			time.Sleep(2 * time.Second)
		} else {
			break
		}
	}

	_, err := s.Client.LogOut() // 退出登陆并删除
	if err != nil {
		return fmt.Errorf("客户端 Phone:%s 登出错误: %w", s.Phone, err)
	}
	return nil
}

// TriggerRefreshBot 触发SpamBot机器人刷新账户状态 每次发之前都需要触发
func (s *TelegramService) TriggerRefreshBot(chatID int64) error {
	for i := 0; i < 2; i++ { // 触发两次 /start
		_, err := s.Client.SendMessage(&client.SendMessageRequest{
			ChatId: chatID,
			InputMessageContent: &client.InputMessageText{
				Text: &client.FormattedText{
					Text: "/start",
				},
			},
		})
		if err != nil {
			return fmt.Errorf("客户端 Phone:%s 向机器人发送信息错误:%w", s.Phone, err)
		}
		time.Sleep(1 * time.Second) // 阻塞一秒
	}

	time.Sleep(1 * time.Second) // 阻塞一秒
	history, err := s.Client.GetChatHistory(&client.GetChatHistoryRequest{
		ChatId:    chatID,
		Limit:     5, // 获取5条消息 稳妥
		OnlyLocal: false,
	})
	if err != nil {
		return fmt.Errorf("客户端 Phone:%s 获取机器人历史消息错误:%w", s.Phone, err)
	}

	for _, message := range history.Messages {
		switch replyMarkupShowKeyboard := message.ReplyMarkup.(type) {
		case *client.ReplyMarkupShowKeyboard:
			if len(replyMarkupShowKeyboard.Rows) == 4 {
				// 将状态更改为不正常
				var account model.TelegramClientAccount
				err = s.DB.Model(&model.TelegramClientAccount{}).
					Where("phone = ?", s.Phone).First(&account).Error
				if err != nil {
					return fmt.Errorf("查找:客户端 Phone:%s 时错误:%w", s.Phone, err)
				}
				account.IsActive = false
				err = s.DB.Save(&account).Error
				if err != nil {
					return fmt.Errorf("客户端 Phone:%s 账户已被封禁/更新账户状态时错误:%w", s.Phone, err)
				}
				return fmt.Errorf("客户端 Phone:%s 账户已被封禁", s.Phone)
			}
		default:
		}
	}
	return nil
}

// Update2Password 更改二步密码
func (s *TelegramService) Update2Password(oldPassword string, newPassword string) error {
	_, err := s.Client.SetPassword(&client.SetPasswordRequest{
		OldPassword: oldPassword,
		NewPassword: newPassword,
	})
	if err != nil {
		return fmt.Errorf("客户端 Phone:%s 账户更改二步密码错误:%w", s.Phone, err)
	}
	return nil
}

// SubmitVerificationCode 提交验证码
func (s *TelegramService) SubmitVerificationCode(code string) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	slog.Debug("SubmitVerificationCode:11")

	if s.CodeChannel == nil {
		return fmt.Errorf("客户端未初始化")
	}
	slog.Debug("SubmitVerificationCode:22")

	slog.Info("提交验证码", "code", code)

	// 直接向 authorizer 的 Code channel 发送
	select {
	case s.CodeChannel <- code:
		return nil
	case <-time.After(10 * time.Second):
		return fmt.Errorf("提交验证码超时")
	}
}

// SubmitPassword 提交二步验证密码
func (s *TelegramService) SubmitPassword(password string) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.PasswordChannel == nil {
		return fmt.Errorf("客户端未初始化")
	}

	// 直接向 authorizer 的 Password channel 发送
	select {
	case s.PasswordChannel <- password:
		return nil
	case <-time.After(10 * time.Second):
		return fmt.Errorf("提交密码超时")
	}
}

// GetCurrentAuthState 获取当前认证状态
func (s *TelegramService) GetCurrentAuthState() (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for {
		if s.Client == nil {
			time.Sleep(2 * time.Second)
		} else {
			break
		}
	}

	state, err := s.Client.GetAuthorizationState()
	if err != nil {
		return "", err
	}

	return state.AuthorizationStateType(), nil
}

// IsReady 检查是否登录成功
func (s *TelegramService) IsReady() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.AuthStatus
}

// Close 关闭客户端和监听器
func (s *TelegramService) Close() error {
	if s.Client == nil {
		return nil
	}
	_, err := s.Client.Close() // 再关闭客户端
	if err != nil {
		return fmt.Errorf("%s 关闭客户端错误%w", s.Phone, err)
	}
	return nil // 没有错误表示关闭成功
}
