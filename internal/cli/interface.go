package cli

// CLIAdapter CLI 适配器接口
type CLIAdapter interface {
	// SendInput 发送输入到 CLI（通过 tmux send-keys）
	SendInput(sessionName, input string) error

	// GetLastResponse 获取最新的完整回复（读取 CLI 历史文件）
	GetLastResponse(sessionName string) (string, error)

	// IsSessionAlive 检查 session 是否存活
	IsSessionAlive(sessionName string) bool

	// CreateSession 创建新 session（可选）
	CreateSession(sessionName, cliType, workDir string) error

	// CheckInteractive 检查 CLI 是否在等待用户输入
	// 返回: (是否在等待, 提示文本, 错误)
	// 用于处理中间交互场景，如确认执行命令、澄清歧义等
	CheckInteractive(sessionName string) (bool, string, error)
}
