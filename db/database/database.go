package database

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"tdlib_desktop/db/model"
	"tdlib_desktop/tools"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

var DB *gorm.DB

// InitDB 初始化数据库连接
func InitDB(ctx context.Context) {
	os.Getwd()

	db, err := gorm.Open(sqlite.Open("db/account.db"), &gorm.Config{})
	if err != nil {
		panic(fmt.Sprintf("[DB] ❌初始化数据库错误: %v", err))
	}
	sqlDB, _ := db.DB()
	// 设置连接池
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetConnMaxLifetime(time.Hour)

	// 设置时区为上海
	loc, _ := time.LoadLocation("Asia/Shanghai")
	time.Local = loc

	// 自动迁移模型
	err = db.AutoMigrate(
		&model.TelegramClientAccount{},
		&model.TelegramClientChat{},
		&model.TelegramClientSettings{},
	)

	if err != nil {
		panic(fmt.Sprintf("[DB] ❌初始化数据库迁移模型错误: %v", err))
	}

	tools.Go("定时任务:24小时重置私信次数", func() {
		startResetTask(ctx, db)
	})

	DB = db
	slog.Info("[DB] ✅数据库初始化成功")
}

// GetDB 提供全局获取入口
func GetDB() *gorm.DB {
	return DB
}

func startResetTask(ctx context.Context, db *gorm.DB) {
	// 每小时检查一次（可以调整为更合适的周期）
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			now := time.Now()
			err := db.Transaction(func(tx *gorm.DB) error {
				err := tx.Model(&model.TelegramClientAccount{}).
					Where("last_reset_at IS NULL OR last_reset_at < ?", now.Add(-24*time.Hour)).
					Updates(map[string]interface{}{
						"private_count": 0,
						"last_reset_at": now,
					}).Error
				if err != nil {
					return err
				}
				return nil
			})
			if err != nil {
				slog.Error("重置用户私信次数失败", "error", err)
			}
		case <-ctx.Done():
			slog.Info("定时任务退出：24小时重置私信次数")
			return
		}
	}
}
