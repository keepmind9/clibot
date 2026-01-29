# AGENTS.md - Development Guidelines for AI Agents

## Project Context

This project (**clibot**) will be **open-sourced** publicly. All code, documentation, and comments must be in **English** to ensure international collaboration and accessibility.

## Language Requirements

### Code
- **ALL code comments** MUST be in English
- **Variable names** MUST be in English
- **Function names** MUST be in English
- **Error messages** MUST be in English (unless user-facing localization is implemented)
- **TODO comments** MUST be in English

### Documentation
- **README.md** MUST be in English
- **API documentation** MUST be in English
- **Code examples** MUST use English comments and explanations
- **Inline documentation** (godoc comments) MUST be in English

### Design Documents
- **Technical design docs** SHOULD be in English
- **Architecture diagrams** SHOULD use English labels
- **Commit messages** SHOULD be in English

## Examples

### ❌ BAD (Chinese in code)
```go
// 用户消息处理
func 处理用户消息(msg BotMessage) {
    // 检查用户是否在白名单中
    if !用户已授权(msg) {
        return 错误("未授权")
    }
}
```

### ✅ GOOD (English in code)
```go
// HandleUserMessage processes incoming messages from users
func HandleUserMessage(msg BotMessage) error {
    // Check if user is authorized
    if !isUserAuthorized(msg) {
        return fmt.Errorf("unauthorized access")
    }
}
```

## Code Quality Standards

### Documentation Requirements
1. **Exported functions** MUST have godoc comments
2. **Complex logic** MUST have explanatory comments
3. **Public APIs** MUST have usage examples
4. **Configuration options** MUST be documented

### Example godoc format:
```go
// SendInput sends user input to the CLI through tmux send-keys.
//
// It simulates keyboard input in the specified tmux session.
// Returns an error if the session doesn't exist or tmux command fails.
//
// Example:
//   err := adapter.SendInput("project-a", "help me analyze this code")
//   if err != nil {
//       log.Fatal(err)
//   }
func (c *ClaudeAdapter) SendInput(sessionName, input string) error {
    // implementation...
}
```

## Testing Requirements

- Test function names MUST be in English
- Test comments MUST be in English
- Test data/example content SHOULD be in English

Example:
```go
func TestSendInput_ValidSession_ReturnsNoError(t *testing.T) {
    // Test implementation...
}
```

## Translation Notes

- User-facing bot messages MAY be localized later
- Error logs for debugging SHOULD be in English
- Configuration file keys MUST be in English

## Compliance Checklist

Before submitting code, verify:
- [ ] All comments are in English
- [ ] All variable/function names are in English
- [ ] All error messages are in English
- [ ] Public functions have godoc comments
- [ ] README and documentation are in English
- [ ] No hardcoded Chinese strings in production code

## Exceptions

The ONLY exception is content inside `docs/` directory that is explicitly marked as Chinese documentation or market analysis specifically for Chinese audience. However, even these documents should have English summaries when possible.

---

**Remember**: This codebase will be read by developers worldwide. English ensures maximum accessibility and collaboration opportunities.
