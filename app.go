package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/rand"
	"os"
	"strings"
	"sync/atomic"
	"tdlib_desktop/account_client"
	"tdlib_desktop/db/database"
	"tdlib_desktop/db/model"
	"tdlib_desktop/tools"
	"time"
	"unicode/utf16"

	"github.com/lmittmann/tint"
	"github.com/spf13/cast"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"gorm.io/gorm"
)

type AccountItem struct {
	ID        uint   `json:"id"`
	Phone     string `json:"phone"`
	Name      string `json:"name"`
	IsPremium bool   `json:"is_premium"`
	IsActive  bool   `json:"is_active"`
	CreateAt  string `json:"create_at"`
}

// App struct
type App struct {
	ctx         context.Context
	db          *gorm.DB
	sm          *account_client.TelegramServiceStateManager
	sending     atomic.Bool
	messageChan chan string
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	// TODO writer  os.Stdout
	slog.SetDefault(slog.New(tint.NewHandler(os.Stdout, &tint.Options{
		AddSource:   true, // 记录日志位置
		Level:       slog.LevelDebug,
		ReplaceAttr: nil,
	})))
	// 初始化数据库
	database.InitDB(ctx)

	// 设置数据库
	db := database.GetDB()
	a.db = db
	// 初始化信息
	err := a.db.Model(&model.TelegramClientSettings{}).
		Where("key = ?", "account_private_count").
		Attrs(model.TelegramClientSettings{
			Key:         "account_private_count",
			Value:       "45",
			Description: "账号每日最大私信数量",
		}).FirstOrCreate(&model.TelegramClientSettings{}).Error
	if err != nil {
		panic(fmt.Errorf("设置每日最大私信数量错误:%s", err.Error()))
	}
	// 设置客户端管理
	stateManager := account_client.InitTelegramServiceStateManager()
	a.sm = stateManager

	// 设置消息通道
	messageChan := make(chan string, 100)
	a.messageChan = messageChan

	// 启动实时通知
	go a.AccountMessageSSE()
}

// SetAppInfo 设置App信息
func (a *App) SetAppInfo(appID string, appHash string) error {
	err := a.db.Transaction(func(tx *gorm.DB) error {
		err := tx.Model(&model.TelegramClientSettings{}).Create(&model.TelegramClientSettings{
			Key:         "appID",
			Value:       appID,
			Description: "AppID",
		}).Error
		if err != nil {
			slog.Error("设置APP详情ID错误", "err", err)
			return fmt.Errorf("设置APP详情ID错误:%w", err)
		}

		err = tx.Model(&model.TelegramClientSettings{}).Create(&model.TelegramClientSettings{
			Key:         "appHash",
			Value:       appHash,
			Description: "AppHash",
		}).Error

		if err != nil {
			slog.Error("设置APP详情Hash错误", "err", err)
			return fmt.Errorf("设置APP详情Hash错误:%w", err)
		}
		return nil
	})
	if err != nil {
		return err
	}
	return nil
}

// GetAppInfoStatus 获取App信息状态
func (a *App) GetAppInfoStatus() bool {
	var appIDSetting model.TelegramClientSettings
	err := a.db.Model(&model.TelegramClientSettings{}).Where("key = ?", "appID").First(&appIDSetting).Error
	if err != nil {
		slog.Error("查询APP详情ID错误", "err", err)
		return false
	}

	var appHashSetting model.TelegramClientSettings
	err = a.db.Model(&model.TelegramClientSettings{}).Where("key = ?", "appHash").First(&appHashSetting).Error
	if err != nil {
		slog.Error("查询APP详情Hash错误", "err", err)
		return false
	}
	if appIDSetting != (model.TelegramClientSettings{}) && appHashSetting != (model.TelegramClientSettings{}) {
		return true
	}
	return false
}

// AccountSendCode 根据手机号发送验证码
func (a *App) AccountSendCode(phone string) error {
	// 先去数据库里查询号码是否存在
	var telegramClientAccount model.TelegramClientAccount
	err := a.db.Model(&model.TelegramClientAccount{}).Where("phone = ?", phone).First(&telegramClientAccount).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) { // 如果错误不是 nil并且错误不是记录未找到则返回错误
		return fmt.Errorf("DB查询Phone %s 错误:%w", phone, err)
	}

	if telegramClientAccount != (model.TelegramClientAccount{}) && telegramClientAccount.AccountStatus == 3 { // 反之存在直接返回请勿重复登陆
		return fmt.Errorf("Phone:%s 请勿重复登陆", phone)
	}

	go func() {
		service := account_client.CreateTelegramService(phone, a.db) // 创建
		err = service.InitializeClient()                             // 初始化客户端
		if err != nil {
			slog.Error(fmt.Sprintf("Phone:%s 初始化失败", phone))
		}
		a.sm.AddService(phone, service)
	}()
	time.Sleep(2 * time.Second) // 等等发送验证码的状态
	return nil
}

// AccountConfirm 根据手机号、验证码、二步密码进行登录
func (a *App) AccountConfirm(phone string, code string, password string) error {
	service, exists := a.sm.GetService(phone) // 在内存中根据号码读取客户端
	if !exists {                              // 如果没有返回错误
		return fmt.Errorf("读取 TelegramService 不存在")
	}

	err := service.SubmitVerificationCode(code) // 检查验证码
	if err != nil {
		return fmt.Errorf("设置验证码错误:%w", err)
	}

	// 等待状态更新
	time.Sleep(1 * time.Second)
	err = service.SubmitPassword(password) // 检查二步密码
	if err != nil {
		return fmt.Errorf("设置二步密码错误:%w", err)
	}
	time.Sleep(2 * time.Second)
	return nil
}

// AccountPageItems 获取已经登录的所有账户列表  分页查询
func (a *App) AccountPageItems(page int, pageSize int) ([]AccountItem, error) {
	offset := (page - 1) * pageSize

	var telegramClientAccountList []model.TelegramClientAccount
	err := a.db.Model(&model.TelegramClientAccount{}).
		Order("created_at DESC").
		Offset(offset).
		Limit(pageSize).
		Find(&telegramClientAccountList).Error
	if err != nil {
		return nil, fmt.Errorf("查询错误:%w", err)
	}

	var responseData []AccountItem
	for _, telegramClientAccount := range telegramClientAccountList {
		responseData = append(responseData, AccountItem{
			ID:        telegramClientAccount.ID,
			Phone:     telegramClientAccount.Phone,
			Name:      fmt.Sprintf("%s %s", telegramClientAccount.FirstName, telegramClientAccount.LastName),
			IsPremium: telegramClientAccount.IsPremium,
			IsActive:  telegramClientAccount.IsActive,
			CreateAt:  telegramClientAccount.CreatedAt.Format("2006-01-02 15:04:05"),
		})
	}
	return responseData, nil
}

// AccountDelete 删除某一账号
func (a *App) AccountDelete(id uint) error {
	var telegramClientAccount model.TelegramClientAccount
	err := a.db.Where("id = ?", id).First(&telegramClientAccount).Error
	if err != nil {
		return fmt.Errorf("数据库查询客户端ID:%d 错误:%w", id, err)
	}

	// 如果存在就删除
	service, exists := a.sm.GetService(telegramClientAccount.Phone) // 在内存中根据号码读取客户端
	if exists {                                                     // 如果没有返回错误
		// 退出客户端
		err = service.CloneAndLogOut()
		if err != nil {
			return fmt.Errorf("删除客户端Phone:%s 错误:%w", telegramClientAccount.Phone, err)
		}
		// 在总管理中清理
		a.sm.DeleteService(service.Phone)
	}

	// 在数据库里删除 软删除
	err = a.db.Unscoped().Model(&model.TelegramClientAccount{}).Where("id = ?", id).Delete(&id).Error
	if err != nil {
		return fmt.Errorf("数据库客户端Phone:%s 错误:%w", telegramClientAccount.Phone, err)
	}

	return nil
}

func pickAndInitAccount(
	accounts *[]model.TelegramClientAccount,
	db *gorm.DB,
	sm *account_client.TelegramServiceStateManager,
) (*model.TelegramClientAccount, *account_client.TelegramService, error) {
	if len(*accounts) == 0 {
		return nil, nil, errors.New("no available accounts")
	}

	// 随机选一个
	index := rand.Intn(len(*accounts))
	acc := (*accounts)[index]
	// ⚠️ 先不要立刻删
	service := account_client.CreateTelegramService(acc.Phone, db)
	if err := service.InitializeClient(); err != nil {
		slog.Error("初始化客户端失败", "phone", acc.Phone, "err", err)
		return nil, nil, err
	}

	time.Sleep(3 * time.Second)
	sm.AddService(acc.Phone, service)

	// 初始化成功后，再从池中移除
	*accounts = append((*accounts)[:index], (*accounts)[index+1:]...)
	return &acc, service, nil
}

// AccountPrivateMessage 一键发送私信
func (a *App) AccountPrivateMessage(fullText string, keyword string, linkURL string, usernames []string) error {
	if !a.sending.CompareAndSwap(false, true) {
		return fmt.Errorf("正在发送，请等待本次发送结束")
	}

	startIdx := strings.Index(fullText, keyword)
	beforeText := fullText[:startIdx]
	offset := int32(len(utf16.Encode([]rune(beforeText))))
	length := int32(len(utf16.Encode([]rune(keyword))))

	// 读取设置中的账号当日发送数量；按24小时来算
	var settings model.TelegramClientSettings
	err := a.db.Model(&model.TelegramClientSettings{}).
		Where("key = ?", "account_private_count").
		First(&settings).Error
	if err != nil {
		return fmt.Errorf("数据库查询每日私信数量错误:%w", err)
	}
	accountPrivateCount := cast.ToInt(settings.Value)

	// 获取数据库中的所有私信次数不足45次的账号
	var accounts []model.TelegramClientAccount
	err = a.db.Model(&model.TelegramClientAccount{}).
		Where(
			"is_active = ? AND private_count < ?",
			true,
			accountPrivateCount,
		).
		Find(&accounts).Error
	if err != nil {
		return fmt.Errorf("数据库查询账户错误:%w", err)
	}

	if len(accounts) == 0 {
		return fmt.Errorf("没有可私信的账号，请新增账号或等待次数刷新")
	}

	// 已经发过的用户名
	var existed []string
	err = a.db.Model(&model.TelegramClientChat{}).
		Where("username IN ?", usernames).
		Pluck("username", &existed).Error
	if err != nil {
		return fmt.Errorf("查询已发送用户名时错误:%w", err)
	}
	existedMap := make(map[string]struct{}, len(existed))
	for _, u := range existed {
		existedMap[u] = struct{}{}
	}

	tools.Go("私信接口", func() {
		defer a.sending.Store(false)

		// 创建限流器
		limiter := tools.NewJitterLimiter(
			30,
			1*time.Second,
			2*time.Second,
		)

		// 已经使用完次数的 account
		// 当前正在使用的 account
		var currentAccount *model.TelegramClientAccount
		// 当前正在使用的 账户
		var currentService *account_client.TelegramService
		// 当前账户发送的次数
		var currentAccountPrivateCount int
		// 当前账户是否触发过 SpamBot机器人
		var currentIsSpamBot bool
		// 是否切换账号
		var switchAccount bool

		for _, username := range usernames {
			// 如果用户名发过了就跳过
			if _, ok := existedMap[username]; ok {
				a.messageChan <- fmt.Sprintf("用户名:%s 已被私信过，跳过", username)
				continue
			}

			// 下面就取出可发送的账户
			if currentAccount == nil { // 开始的时候会执行
				acc, svc, err := pickAndInitAccount(&accounts, a.db, a.sm)
				if err != nil {
					a.messageChan <- "没有可用账号，退出"
					break
				}
				currentAccount = acc
				currentService = svc
				currentAccountPrivateCount = currentAccount.PrivateCount
				currentIsSpamBot = false
			}

			if !currentIsSpamBot { // 如果没有触发过 SpamBot机器人则去触发一下
				botChatID, err := currentService.GetChatID("SpamBot") // 固定用户名 SpamBot
				if err != nil {
					a.messageChan <- fmt.Sprintf("Phone:%s 获取SpamBot机器人ChatID失败❌", currentService.Phone)
					slog.Error(fmt.Sprintf("客户端Phone:%s SpamBot机器人的ChatID错误:%s", currentService.Phone, err.Error()))
					continue
				}
				// 先去触发机器人验证
				err = currentService.TriggerRefreshBot(botChatID)
				if err != nil {
					a.messageChan <- fmt.Sprintf("Phone:%s 触发SpamBot机器人失败❌", currentService.Phone)
					slog.Error(fmt.Sprintf("客户端Phone:%s 验证SpamBot机器人错误:%s", currentService.Phone, err.Error()))
					continue
				}
				currentIsSpamBot = true
			}

			_ = limiter.Wait(context.Background()) // 限流

			// 获取用户名的 chatID
			AccChatID, err := currentService.GetChatID(username)
			if err != nil {
				a.messageChan <- fmt.Sprintf("Phone:%s 获取:%s ChatID 失败❌", currentService.Phone, username)
				slog.Error(fmt.Sprintf("客户端Phone:%s 获取Username:%s 的ChatID错误:%s", currentService.Phone, username, err.Error()))
				continue
			}

			// 开始发送私信
			err = currentService.SendMessage(AccChatID, fullText, offset, length, linkURL)
			if err != nil {
				a.messageChan <- fmt.Sprintf("Phone:%s 私信用户名:%s 失败❌", currentService.Phone, username)
				slog.Error(fmt.Sprintf("客户端Phone:%s 向Username:%s/ChatID:%d 发送消息错误:%s", currentService.Phone, username, AccChatID, err.Error()))
				continue
			}

			// 通知通道
			a.messageChan <- fmt.Sprintf("Phone:%s 私信用户名:%s 成功✅", currentService.Phone, username)

			// 发送成功一个就+1
			currentAccountPrivateCount += 1
			// 立刻落库
			err = a.db.Transaction(func(tx *gorm.DB) error {
				res := tx.Model(&model.TelegramClientAccount{}).
					Where("phone = ? AND private_count < ?", currentAccount.Phone, accountPrivateCount).
					Update("private_count", gorm.Expr("private_count + 1"))
				if res.RowsAffected == 0 { // 并发下被别人先写满
					return gorm.ErrRecordNotFound
				}
				return tx.Create(&model.TelegramClientChat{
					AccountID: currentAccount.ID,
					Username:  username,
					ChatID:    AccChatID,
				}).Error
			})

			if err != nil {
				// 计数器已满或被并发抢光，立即换号
				switchAccount = true
				continue
			}

			if switchAccount {
				switchAccount = false
				if currentService != nil {
					_ = currentService.Close()
				}

				acc, svc, err := pickAndInitAccount(&accounts, a.db, a.sm)
				if err != nil {
					a.messageChan <- "账号已用尽⚠️"
					break
				}
				currentAccount = acc
				currentService = svc
				currentAccountPrivateCount = currentAccount.PrivateCount
				currentIsSpamBot = false
				continue // ⭐⭐⭐ 非常关键
			}

			// 如果次数超过设置次数则替换为其他用户
			if currentAccountPrivateCount >= accountPrivateCount {
				if currentService != nil {
					_ = currentService.Close()
				}
				acc, svc, err := pickAndInitAccount(&accounts, a.db, a.sm)
				if err != nil {
					a.messageChan <- "账号已用尽⚠️"
					break
				}
				currentAccount = acc
				currentService = svc
				currentAccountPrivateCount = currentAccount.PrivateCount
				currentIsSpamBot = false
			}
		}
		a.messageChan <- fmt.Sprintf("私信完毕")
	})
	return nil
}

// AccountSwitch 切换登录其他账号
func (a *App) AccountSwitch(phone string) error {
	// 如果存在则不用重新启动
	service, exists := a.sm.GetService(phone)
	if exists {
		return nil
	}

	// 进行登陆
	service = account_client.CreateTelegramService(phone, a.db) // 创建
	err := service.InitializeClient()                           // 初始化客户端
	if err != nil {
		return fmt.Errorf("Phone:%s 初始化错误:%w", phone, err)
	}
	a.sm.AddService(phone, service)
	return nil
}

// AccountMessageSSE 实时更新当前登录账户的所有消息
func (a *App) AccountMessageSSE() {
	for {
		select {
		case <-a.ctx.Done():
			return
		case msg, ok := <-a.messageChan:
			if !ok {
				return
			}
			runtime.EventsEmit(a.ctx, "private_message", msg)
		}
	}
}

// AccountPrivateMessageCount  获取当前最大可私信的数量
func (a *App) AccountPrivateMessageCount() (int, error) {
	var settings model.TelegramClientSettings
	err := a.db.Model(&model.TelegramClientSettings{}).Where("key = ?", "account_private_count").First(&settings).Error
	if err != nil {
		return 0, fmt.Errorf("数据库查询每日私信数量错误:%s", err)
	}

	accountPrivateCount := cast.ToInt(settings.Value)
	// 获取数据库中的所有私信次数不足45次的账号
	var accounts []model.TelegramClientAccount
	err = a.db.Model(&model.TelegramClientAccount{}).
		Where(
			"is_active = ? AND private_count < ?",
			true,
			accountPrivateCount,
		).
		Find(&accounts).Error
	if err != nil {
		return 0, fmt.Errorf("数据库查询账户错误:%w", err)
	}
	var privateCount int
	for _, account := range accounts {
		//slog.Info(fmt.Sprintf("ID:%d 次数:%d", account.ID, account.PrivateCount))
		poorCount := accountPrivateCount - account.PrivateCount
		privateCount += poorCount
	}
	return privateCount, nil
}

// AccountSearchPhone 根据手机号码查询
func (a *App) AccountSearchPhone(phone string) ([]AccountItem, error) {
	var telegramClientAccountList []model.TelegramClientAccount
	err := a.db.Model(&model.TelegramClientAccount{}).Where("phone = ?", phone).Find(&telegramClientAccountList).Error
	if err != nil {
		return nil, fmt.Errorf("查询Phone:%s 错误:%w", phone, err)
	}

	var responseData []AccountItem
	for _, telegramClientAccount := range telegramClientAccountList {
		responseData = append(responseData, AccountItem{
			ID:        telegramClientAccount.ID,
			Phone:     telegramClientAccount.Phone,
			Name:      fmt.Sprintf("%s %s", telegramClientAccount.FirstName, telegramClientAccount.LastName),
			IsPremium: telegramClientAccount.IsPremium,
			IsActive:  telegramClientAccount.IsActive,
			CreateAt:  telegramClientAccount.CreatedAt.Format("2006-01-02 15:04:05"),
		})
	}
	return responseData, nil
}

// AccountLogin 触发登陆
func (a *App) AccountLogin(phone string) error {
	// 如果存在则不用重新启动
	service, exists := a.sm.GetService(phone)
	if exists {
		return nil
	}
	// 进行登陆
	service = account_client.CreateTelegramService(phone, a.db) // 创建
	err := service.InitializeClient()                           // 初始化客户端
	if err != nil {
		return fmt.Errorf("Phone:%s 初始化错误:%w", phone, err)
	}
	a.sm.AddService(phone, service)
	return nil
}
