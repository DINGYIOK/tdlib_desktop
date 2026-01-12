package model

import (
	"time"

	"gorm.io/gorm"
)

type TelegramClientAccount struct {
	gorm.Model
	Phone        string `gorm:"column:phone;uniqueIndex;not null;comment:登录手机号"` // 电报登录手机号/索引
	AppID        string `gorm:"column:app_id;comment:电报API_ID"`
	AppHash      string `gorm:"column:app_hash;comment:电报API_HASH"`
	DatabasePath string `gorm:"column:database_path;comment:数据库存储路径"`                              // 数据库存储路径
	ProxyUrl     string `gorm:"column:proxy_url;index;comment:代理地址(socks5://user:pass@host:port)"` // 代理IP(后期可能使用/例如50或者100个号分配一个静态IP)可以为空
	// 扩展字段
	AccountStatus    int    `gorm:"column:account_status;default:0;comment:状态:0未登录,1验证码待输入,2二步密码待输入,3在线(登陆成功),4被封号"`
	IsPremium        bool   `gorm:"column:is_premium;comment:是否为会员"`
	FirstName        string `gorm:"column:first_name;comment:电报名"`
	LastName         string `gorm:"column:last_name;comment:电报姓"`
	Username         string `gorm:"column:username;comment:电报用户名"`
	TGUserId         int64  `gorm:"column:tg_user_id;index;comment:电报唯一UID"`
	IsActive         bool   `gorm:"column:is_active;default:true;comment:是否启用"`
	IsUpdatePassword bool   `gorm:"column:is_update_password;index;default:false;comment:是否更改过密码"` // 增加索引
	//IsSpamBot        bool       `gorm:"column:is_spambot;default:false;comment:是否触发过SpamBot机器人"`
	PrivateCount  int        `gorm:"column:private_count;index;default:0;comment:私信次数/索引"`
	LastResetAt   *time.Time `gorm:"column:last_reset_at;comment:重置私信次数的时间"`
	LastLoginTime *time.Time
}

func (TelegramClientAccount) TableName() string {
	return "telegram_client_account"
}

type TelegramClientChat struct { //每个私信用户名只需要发一次
	gorm.Model
	AccountID uint   `gorm:"column:account_id;comment:表示属于哪个用户/引用TelegramClientAccount表ID"`
	Username  string `gorm:"column:username;uniqueIndex;comment:用户名"` // 电报用户名唯一/索引
	ChatID    int64  `gorm:"column:chat_id;comment:对话ID"`
}

func (TelegramClientChat) TableName() string {
	return "telegram_client_chat"
}

// TelegramClientSettings 客户端通用设置
type TelegramClientSettings struct {
	gorm.Model
	Key         string `gorm:"column:key;unique;index;comment:配置key"` // 可以直接存代理地址
	Value       string `gorm:"column:value;not null;comment:配置value"` // 可以存储代理地址使用的次数
	Description string `gorm:"column:description;index;comment:配置描述"` // 统一标注：代理地址(通过搜索代理地址查询出所有的代理链接)  ｜代理地址->使用次数
}

func (TelegramClientSettings) TableName() string {
	return "telegram_client_settings"
}
