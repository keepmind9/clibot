package bot

import "time"

// BotAdapter Bot 适配器接口
type BotAdapter interface {
	// Start 启动 Bot，建立连接并开始监听消息
	Start(messageHandler func(BotMessage)) error

	// SendMessage 发送消息到 IM 平台
	SendMessage(channel, message string) error

	// Stop 停止 Bot，清理资源
	Stop() error
}

// BotMessage Bot 消息结构
type BotMessage struct {
	Platform  string    // feishu/discord/telegram
	UserID    string    // 用户唯一标识（用于权限控制）
	Channel   string    // 频道/会话 ID
	Content   string    // 消息内容
	Timestamp time.Time
}
