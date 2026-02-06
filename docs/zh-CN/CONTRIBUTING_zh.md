# 贡献指南

首先，感谢您考虑为 clibot 做出贡献！正是有了像您这样的人，clibot 才能成为如此优秀的工具。

## 目录

- [行为准则](#行为准则)
- [我可以贡献什么？](#我可以贡献什么)
- [入门指南](#入门指南)
- [开发流程](#开发流程)
- [代码规范](#代码规范)
- [测试指南](#测试指南)
- [提交更改](#提交更改)
- [风格指南](#风格指南)
- [其他资源](#其他资源)

## 行为准则

本项目和所有参与者都受我们的[行为准则](CODE_OF_CONDUCT_zh.md)约束。参与本项目即表示您同意遵守该准则。如发现不当行为，请向 [security@keepmind9.com](mailto:security@keepmind9.com) 报告。

## 我可以贡献什么？

我们欢迎多种形式的贡献：

- **Bug 报告**：发现了 Bug？[报告它](../../issues/new?template=bug_report_zh.md)
- **功能建议**：有好主意？[提出建议](../../issues/new?template=feature_request_zh.md)
- **文档**：改进文档、修正错别字、添加示例
- **代码**：修复 Bug、实现功能、改进测试
- **代码审查**：审查 PR 并提供反馈
- **测试**：在不同平台/IM/CLI 上测试

## 入门指南

### 前置要求

- **Go**：1.24 或更高版本
- **Git**：用于克隆仓库
- **Make**：运行构建命令（推荐但非必需）
- **tmux**：会话管理所需
- **golangci-lint**：代码检查（参见[安装](#安装)）

### 安装

1. **Fork 并克隆仓库**：
```bash
git clone https://github.com/YOUR_USERNAME/clibot.git
cd clibot
```

2. **安装依赖**：
```bash
go mod download
```

3. **安装开发工具**：

```bash
# 安装 golangci-lint
curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin v1.62.0

# 验证安装
golangci-lint --version
```

4. **构建项目**：
```bash
make build
# 或
go build -o bin/clibot ./cmd/clibot
```

5. **运行测试**：
```bash
make test
# 或
go test -v ./...
```

## 开发流程

### 设置分支

1. **将您的 fork 与上游仓库同步**：
```bash
git remote add upstream https://github.com/keepmind9/clibot.git
git fetch upstream
git checkout main
git merge upstream/main
```

2. **创建新分支**进行工作：
```bash
git checkout -b feature/your-feature-name
# 或
git checkout -b fix/your-bug-fix
```

分支命名约定：
- `feature/` - 新功能
- `fix/` - Bug 修复
- `docs/` - 文档更改
- `refactor/` - 代码重构
- `test/` - 测试改进

### 进行更改

1. **编写代码**，遵循我们的[代码规范](#代码规范)
2. **添加测试**（参见[测试指南](#测试指南)）
3. **格式化代码**：
```bash
make fmt
# 或
go fmt ./...
```
4. **运行检查器**：
```bash
make lint
# 或
golangci-lint run ./...
```
5. **运行测试**：
```bash
make test
```
6. **全面检查**：
```bash
make check  # 运行 fmt、vet 和 test
```

## 代码规范

### 语言

- **所有代码、文档和注释必须使用英文**
- 包括变量/函数名、错误消息、注释和文档

### 架构原则

- **适配器模式**：使用 `CLIAdapter` 和 `BotAdapter` 接口实现可扩展性
- **事件驱动**：`Engine` 协调 Bot 和 CLI 工具之间的消息流
- **两种模式**：同时支持 hook 模式（实时）和轮询模式（零配置）

### 代码风格

- 遵循 [Effective Go](https://golang.org/doc/effective_go) 中定义的标准 Go 约定
- 使用 `gofmt` 格式化（运行 `make fmt`）
- 使用有意义的变量和函数名
- 为导出的函数、类型和复杂逻辑添加注释
- 保持函数简洁专注

### 错误处理

- 始终显式处理错误
- 使用 `fmt.Errorf` 提供上下文：
```go
return fmt.Errorf("failed to load config: %w", err)
```
- 为预期的错误使用自定义错误类型

### 日志记录

- 使用 `logger` 包进行结构化日志记录
- 使用适当的日志级别（debug、info、warn、error）
- 永不记录敏感信息（token、密码、用户数据）

## 测试指南

### 测试覆盖率

- **保持测试覆盖率在 50% 以上**（由 CI 检查）
- 为新功能和 Bug 修复编写测试
- 重点关注关键路径和复杂逻辑

### 测试类型

1. **单元测试**：测试单个函数和方法
```go
func TestLoadConfig_ValidConfig_ReturnsConfigStruct(t *testing.T) {
    // 测试代码
}
```

2. **表驱动测试**：用于多种场景
```go
tests := []struct {
    name    string
    input   string
    want    string
    wantErr bool
}{
    // 测试用例
}
for _, tt := range tests {
    t.Run(tt.name, func(t *testing.T) {
        // 测试代码
    })
}
```

3. **集成测试**：测试组件交互

### 运行测试

```bash
# 运行所有测试
make test

# 仅运行短测试
make test-short

# 运行带覆盖率的测试
make test-coverage
```

### 测试最佳实践

- 使用 `github.com/stretchr/testify` 进行断言
- 模拟外部依赖（CLI 工具、IM 平台）
- 不仅测试成功情况，也要测试错误情况
- 保持测试快速且独立

## 提交更改

### 提交前

- [ ] 代码编译成功：`make build`
- [ ] 所有测试通过：`make test`
- [ ] 代码已格式化：`make fmt`
- [ ] 检查器通过：`make lint`
- [ ] 覆盖率超过 50%：`make test-coverage`
- [ ] 文档已更新（README、docs/）
- [ ] 提交信息遵循我们的[提交消息约定](#提交消息)

### 提交消息

遵循 [Conventional Commits](https://www.conventionalcommits.org/) 规范：

```
<type>: <description>

[可选正文]

[可选脚注]
```

**类型**：
- `feat`：新功能
- `fix`：Bug 修复
- `docs`：文档更改
- `refactor`：代码重构
- `opt`：性能优化
- `security`：安全修复
- `chore`：构建/工具更改
- `test`：测试改进

**示例**：
```
feat: add support for Telegram bot

Implement Telegram bot adapter using telegram-bot-api.
Added tests for message handling and command parsing.

Closes #123
```

```
fix: handle empty config file gracefully

Return descriptive error instead of panic when config file is empty.

Fixes #456
```

**长度限制**：主题行最多 150 个字符

### 创建 Pull Request

1. **推送更改**：
```bash
git push origin feature/your-feature-name
```

2. **创建 PR**：
   - 访问 https://github.com/keepmind9/clibot
   - 点击"Pull Requests" → "New Pull Request"
   - 选择您的分支
   - 填写 PR 模板

3. **PR 标题**：使用与提交消息相同的格式：
   - `feat: add Telegram bot support`
   - `fix: handle empty config file`

4. **PR 描述**：说明：
   - 做了哪些更改以及为什么
   - 如何测试这些更改
   - 任何破坏性更改
   - 相关 Issue

### 审查流程

- 所有 PR 必须由维护者审查
- 及时响应审查反馈
- 保持 PR 专注且小（一个逻辑更改）
- 根据需要更新 PR 描述

### 合并后

- 删除您的分支（除非它是更大系列的一部分）
- 庆祝！🎉 您已为 clibot 做出贡献！

## 风格指南

### 命名约定

- **包**：小写，尽可能单个单词
- **常量**：`PascalCase` 或 `UPPER_SNAKE_CASE`
- **变量**：`camelCase`
- **函数**：`PascalCase`（导出）、`camelCase`（私有）
- **接口**：`PascalCase`（通常以 `er` 结尾）

### 文件组织

```
internal/
├── core/          # 核心逻辑
├── cli/           # CLI 适配器
├── bot/           # Bot 适配器
├── logger/        # 日志工具
└── ...
```

### 注释

- **导出函数**：必须有 godoc 注释
```go
// LoadConfig loads configuration from file and expands environment variables.
func LoadConfig(configPath string) (*Config, error) {
```

- **复杂逻辑**：添加内联注释解释"为什么"，而不是"是什么"

### Git 工作流

- **每次提交一个原子更改**
- 不要在单个提交中组合无关的更改
- 编写清晰、描述性的提交消息
- 不要在没有用户确认的情况下自动提交

## 其他资源

- [README.md](../README.md) - 项目概述和用法
- [SECURITY.md](../SECURITY.md) - 安全政策和最佳实践
- [AGENTS.md](../AGENTS.md) - AI 代理的开发指南
- [文档](../docs) - 详细文档
- [讨论](https://github.com/keepmind9/clibot/discussions) - 提问和讨论想法

## 获取帮助

- **GitHub Issues**：Bug 报告和功能建议
- **GitHub Discussions**：提问和一般讨论
- **安全问题**：发送邮件至 [security@keepmind9.com](mailto:security@keepmind9.com)

---

**感谢您为 clibot 做出贡献！🚀**

每一个贡献，无论大小，都值得感激。让我们一起让 clibot 变得更好。
