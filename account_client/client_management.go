package account_client

import (
	"log/slog"
	"sync"
	"tdlib_desktop/tools"
	"time"
)

type TelegramServiceStateManager struct {
	Services map[string]*TelegramService // 运行运行期间登陆的账号
	Mu       sync.RWMutex                // 锁
}

var (
	instance *TelegramServiceStateManager
	once     sync.Once
)

// InitTelegramServiceStateManager 初始化
func InitTelegramServiceStateManager() *TelegramServiceStateManager {
	once.Do(func() {
		instance = &TelegramServiceStateManager{
			Services: make(map[string]*TelegramService),
			//MaxActive: 50, // 同时支持50个设备
		}
		tools.Go("客户端管理 AutoGC", instance.AutoGC) // 初始化 启动清理
	})
	return instance
}

// AutoGC 自动清理逻辑 (建议在管理器启动时开启一个协程)
func (t *TelegramServiceStateManager) AutoGC() {
	ticker := time.NewTicker(5 * time.Minute) // 每隔5分钟就开始清理
	for range ticker.C {
		t.Mu.Lock()
		for phone, svc := range t.Services {
			// 策略 1: 清理掉超过 15 分钟仍未登录成功的“僵尸”实例
			if !svc.AuthStatus && time.Since(svc.CreatedAt) > 15*time.Minute {
				slog.Info("清理超时的未登录实例", "phone", phone)
				err := svc.Close()
				if err != nil {
					slog.Error("清理超时的未登录实例错误", "error", err)
				}
				delete(t.Services, phone)
				continue
			}

			// 策略 2: 清理掉闲置过久的已登录实例
			if time.Since(svc.LastAccessTime) > 30*time.Minute {
				slog.Info("自动回收闲置账号", "phone", phone)
				err := svc.Close()
				if err != nil {
					slog.Error("自动回收闲置账号错误", "error", err)
				}
				delete(t.Services, phone)
			}
		}
		t.Mu.Unlock()
	}
}

// AddService 添加服务 - 增加重复检查
func (t *TelegramServiceStateManager) AddService(phone string, service *TelegramService) {
	t.Mu.Lock()
	defer t.Mu.Unlock()
	// 如果旧的还在，直接放回旧的
	if old, exists := t.Services[phone]; exists {
		go func() {
			err := old.Close()
			if err != nil {

			}
		}()
	}
	t.Services[phone] = service
}

// GetService 获取服务 - 优化为 RLock
func (t *TelegramServiceStateManager) GetService(phone string) (*TelegramService, bool) {
	t.Mu.RLock()
	defer t.Mu.RUnlock()
	service, exists := t.Services[phone]
	return service, exists
}

// DeleteService 删除服务
func (t *TelegramServiceStateManager) DeleteService(phone string) {
	t.Mu.Lock()
	defer t.Mu.Unlock()
	delete(t.Services, phone)
}
