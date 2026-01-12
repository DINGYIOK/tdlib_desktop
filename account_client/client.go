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

type TelegramProxy struct {
	Server   string
	Port     int32
	Username string
	Password string
}

// GetProxy 获取代理 禁用
func _GetProxy(db *gorm.DB) (string, *TelegramProxy, error) {
	var setting model.TelegramClientSettings
	err := db.Model(&model.TelegramClientSettings{}).Where("key = ?", "proxy_count").First(&setting).Error
	if err != nil {
		return "", nil, fmt.Errorf("查询代理数量时错误:%w", err)
	}

	proxyCount := cast.ToInt(setting.Value) // 代理数量 默认50

	var clientSettings []model.TelegramClientSettings
	err = db.Model(&model.TelegramClientSettings{}).Where("description = ?", "代理地址").Find(&clientSettings).Error // 查询出所有的代理地址
	if err != nil {
		return "", nil, fmt.Errorf("查询所有代理时错误:%w", err)
	}
	for _, clientSetting := range clientSettings { // 遍历所有设置中的代理
		settingProxyCount := cast.ToInt(clientSetting.Value) //
		if settingProxyCount <= proxyCount {
			// Key 才是链接
			proxyURL := strings.Split(clientSetting.Key, ":")
			// ip:port:acc:password
			if len(proxyURL) != 4 {
				return "", nil, fmt.Errorf("代理链接拆分错误:%w", err)
			}
			server := proxyURL[0]
			port := proxyURL[1]
			username := proxyURL[2]
			password := proxyURL[3]
			// 只要找到代理数量小于设置数量的就返回
			return clientSetting.Key, &TelegramProxy{
				Server:   server,
				Port:     cast.ToInt32(port),
				Username: username,
				Password: password,
			}, nil
		}
	}
	return "", nil, fmt.Errorf("没有可用代理链接，请后台添加")
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
	// 5. 保存 channel 引用（这些字段是公开的！）
	// 通过反射或类型断言获取（因为返回类型虽然是私有结构体，但字段是导出的）
	// 实际上，我们可以直接使用，因为 Go 允许访问未导出类型的导出字段
	s.PhoneChannel = authorizer.PhoneNumber
	s.CodeChannel = authorizer.Code
	s.PasswordChannel = authorizer.Password
	s.StateChannel = authorizer.State

	tools.Go(fmt.Sprintf("Phone:%s 登陆", s.Phone), s.handleAuthStates) // 登陆
	tools.Go(fmt.Sprintf("Phone:%s 创建客户端", s.Phone), s.CreateClient)  // 创建客户端
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
	//addProxy := func(tdlibClient *client.Client) {
	//	tdlibClient.AddProxy(&client.AddProxyRequest{
	//		Server: s.Proxy.Server,
	//		Port:   s.Proxy.Port,
	//		Enable: true,
	//		Type: &client.ProxyTypeSocks5{
	//			Username: s.Proxy.Username,
	//			Password: s.Proxy.Password,
	//		},
	//	})
	//}

	slog.Info("CreateClient: 开始创建客户端")
	// 7. 创建客户端（使用同一个 authorizer！）
	tdlibClient, err := client.NewClient(s.authorizer, logStream, logVerbosity)
	if err != nil {
		slog.Error(fmt.Sprintf("创建客户端 Phone:%s 错误:%s", s.Phone, err.Error()))
		return
	}

	//slog.Debug(fmt.Sprintf("代理Proxy:%s", s.Proxy.Server))
	//slog.Debug(fmt.Sprintf("代理Port:%d", s.Proxy.Port))
	//slog.Debug(fmt.Sprintf("代理Name:%s", s.Proxy.Username))
	//slog.Debug(fmt.Sprintf("代理Paws:%s", s.Proxy.Password))
	// 194.33.62.127:8000:3ZyHfm:P14kns
	// 设置代理 Socks5 IP
	//_, err = tdlibClient.AddProxy(&client.AddProxyRequest{
	//	Server: s.Proxy.Server,
	//	Port:   s.Proxy.Port,
	//	Enable: true,
	//	Type: &client.ProxyTypeSocks5{
	//		Username: s.Proxy.Username,
	//		Password: s.Proxy.Password,
	//	},
	//})
	//if err != nil {
	//	slog.Error(fmt.Sprintf("设置客户端 Phone:%s 代理错误:%s", s.Phone, err.Error()))
	//	return
	//}

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

// serviceListener 协程监听器
//func (s *TelegramService) serviceListener() {
//	slog.Info("开始实时监听 Telegram 事件...")
//	for event := range s.Listener.Updates {
//		switch update := event.(type) {
//		case *client.UpdateNewMessage: // 收到新消息
//			msg := update.Message
//			switch content := msg.Content.(type) {
//			case *client.MessageText: // 讲UserID和文本消息通过SSE传输给前端
//				switch senderUser := msg.SenderId.(type) {
//				case *client.MessageSenderUser:
//					s.MessageChannel <- TelegramServiceMessage{ // 给到
//						UserID: int(senderUser.UserId),
//						Text:   content.Text.Text,
//					}
//				}
//				slog.Info("收到文本消息", "来自", msg.SenderId, "文本", content.Text.Text)
//			case *client.MessagePhoto:
//				// slog.Info("收到图片消息", "图片信息", content.Caption.Text)
//			}
//		case *client.UpdateChatLastMessage: // 会话列表的最后一条消息变了（比如有人撤回消息，或者新消息置顶）
//			// msg := update.LastMessage
//			// switch content := msg.Content.(type) {
//			// case *client.MessageText:
//			// 	slog.Info("会话状态更新 收到文本消息", "来自", msg.SenderId, "文本", content.Text.Text)
//			// case *client.MessagePhoto:
//			// 	slog.Info("会话状态更新 收到图片消息", "图片信息", content.Caption.Text)
//			// }
//			// slog.Info("会话状态更新", "chat_id", update.ChatId)
//		case *client.UpdateUserStatus: // 好友上线/下线
//			// slog.Info("用户状态改变", "user_id", update.UserId, "status", update.Status.UserStatusType())
//		case *client.UpdateConnectionState: // 网络连接状态（连接中、正在更新、就绪等）
//			// slog.Info("网络状态", "state", update.State.ConnectionStateType())
//		default:
//			// 暂时不处理的其他数千种更新类型
//			// slog.Debug("收到其他更新", "type", event.GetType())
//		}
//	}
//}

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
	//if s.Listener == nil {
	//	return fmt.Errorf("%d 关闭监听器错误:listener为nil", s.Phone)
	//}
	//s.Listener.Close()         // 先关闭监听器
	//close(s.MessageChannel)    // 关闭消息通道
	_, err := s.Client.Close() // 再关闭客户端
	if err != nil {
		return fmt.Errorf("%s 关闭客户端错误%w", s.Phone, err)
	}
	return nil // 没有错误表示关闭成功
}
