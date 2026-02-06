# Contributing to clibot

First off, thank you for considering contributing to clibot! It's people like you that make clibot such a great tool.

## Table of Contents

- [Code of Conduct](#code-of-conduct)
- [What Can I Contribute?](#what-can-i-contribute)
- [Getting Started](#getting-started)
- [Development Workflow](#development-workflow)
- [Coding Standards](#coding-standards)
- [Testing Guidelines](#testing-guidelines)
- [Submitting Changes](#submitting-changes)
- [Style Guide](#style-guide)
- [Additional Resources](#additional-resources)

## Code of Conduct

This project and everyone participating in it is governed by our [Code of Conduct](CODE_OF_CONDUCT.md). By participating, you are expected to uphold this code. Please report unacceptable behavior to [security@keepmind9.com](mailto:security@keepmind9.com).

## What Can I Contribute?

We welcome contributions in many forms:

- **Bug reports**: Found a bug? [Report it](../../issues/new?template=bug_report.md)
- **Feature requests**: Have an idea? [Suggest it](../../issues/new?template=feature_request.md)
- **Documentation**: Improve docs, fix typos, add examples
- **Code**: Fix bugs, implement features, improve tests
- **Code review**: Review PRs and provide feedback
- **Testing**: Test on different platforms/IMs/CLIs

## Getting Started

### Prerequisites

- **Go**: Version 1.24 or higher
- **Git**: For cloning the repository
- **Make**: For running build commands (optional but recommended)
- **tmux**: Required for session management
- **golangci-lint**: For code linting (see [Installation](#installation))

### Installation

1. **Fork and clone the repository**:
```bash
git clone https://github.com/YOUR_USERNAME/clibot.git
cd clibot
```

2. **Install dependencies**:
```bash
go mod download
```

3. **Install development tools**:

```bash
# Install golangci-lint
curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin v1.62.0

# Verify installation
golangci-lint --version
```

4. **Build the project**:
```bash
make build
# or
go build -o bin/clibot ./cmd/clibot
```

5. **Run tests**:
```bash
make test
# or
go test -v ./...
```

## Development Workflow

### Setting Up Your Branch

1. **Sync your fork** with the upstream repository:
```bash
git remote add upstream https://github.com/keepmind9/clibot.git
git fetch upstream
git checkout main
git merge upstream/main
```

2. **Create a new branch** for your work:
```bash
git checkout -b feature/your-feature-name
# or
git checkout -b fix/your-bug-fix
```

Branch naming conventions:
- `feature/` - New features
- `fix/` - Bug fixes
- `docs/` - Documentation changes
- `refactor/` - Code refactoring
- `test/` - Test improvements

### Making Changes

1. **Write code** following our [Coding Standards](#coding-standards)
2. **Add tests** for your changes (see [Testing Guidelines](#testing-guidelines))
3. **Format code**:
```bash
make fmt
# or
go fmt ./...
```
4. **Run linters**:
```bash
make lint
# or
golangci-lint run ./...
```
5. **Run tests**:
```bash
make test
```
6. **Check all**:
```bash
make check  # Runs fmt, vet, and test
```

## Coding Standards

### Language

- **ALL code, documentation, and comments must be in English**
- This includes variable/function names, error messages, comments, and documentation

### Architecture Principles

- **Adapter Pattern**: Use `CLIAdapter` and `BotAdapter` interfaces for extensibility
- **Event-driven**: The `Engine` coordinates message flow between bots and CLI tools
- **Two modes**: Support both hook mode (real-time) and polling mode (zero-config)

### Code Style

- Follow standard Go conventions defined in [Effective Go](https://golang.org/doc/effective_go)
- Use `gofmt` for formatting (run `make fmt`)
- Use meaningful variable and function names
- Add comments for exported functions, types, and complex logic
- Keep functions focused and concise

### Error Handling

- Always handle errors explicitly
- Provide context with `fmt.Errorf`:
```go
return fmt.Errorf("failed to load config: %w", err)
```
- Use custom error types for expected errors

### Logging

- Use structured logging with the `logger` package
- Log at appropriate levels (debug, info, warn, error)
- Never log sensitive information (tokens, passwords, user data)

## Testing Guidelines

### Test Coverage

- **Maintain test coverage above 50%** (checked by CI)
- Write tests for new features and bug fixes
- Focus on critical paths and complex logic

### Test Types

1. **Unit Tests**: Test individual functions and methods
```go
func TestLoadConfig_ValidConfig_ReturnsConfigStruct(t *testing.T) {
    // test code
}
```

2. **Table-Driven Tests**: For multiple scenarios
```go
tests := []struct {
    name    string
    input   string
    want    string
    wantErr bool
}{
    // test cases
}
for _, tt := range tests {
    t.Run(tt.name, func(t *testing.T) {
        // test code
    })
}
```

3. **Integration Tests**: Test component interactions

### Running Tests

```bash
# Run all tests
make test

# Run short tests only
make test-short

# Run tests with coverage
make test-coverage
```

### Testing Best Practices

- Use `github.com/stretchr/testify` for assertions
- Mock external dependencies (CLI tools, IM platforms)
- Test error cases, not just success cases
- Make tests fast and independent

## Submitting Changes

### Before Submitting

- [ ] Code compiles successfully: `make build`
- [ ] All tests pass: `make test`
- [ ] Code is formatted: `make fmt`
- [ ] Linter passes: `make lint`
- [ ] Coverage is above 50%: `make test-coverage`
- [ ] Documentation is updated (README, docs/)
- [ ] Commits follow our [commit message convention](#commit-messages)

### Commit Messages

Follow the [Conventional Commits](https://www.conventionalcommits.org/) specification:

```
<type>: <description>

[optional body]

[optional footer]
```

**Types**:
- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation changes
- `refactor`: Code refactoring
- `opt`: Performance optimization
- `security`: Security fixes
- `chore`: Build/tooling changes
- `test`: Test improvements

**Examples**:
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

**Length limit**: Subject line maximum 150 characters

### Creating a Pull Request

1. **Push your changes**:
```bash
git push origin feature/your-feature-name
```

2. **Create a PR**:
   - Go to https://github.com/keepmind9/clibot
   - Click "Pull Requests" → "New Pull Request"
   - Select your branch
   - Fill in the PR template

3. **PR title**: Use the same format as commit messages:
   - `feat: add Telegram bot support`
   - `fix: handle empty config file`

4. **PR description**: Explain:
   - What changes were made and why
   - How you tested the changes
   - Any breaking changes
   - Related issues

### Review Process

- All PRs must be reviewed by maintainers
- Address review feedback promptly
- Keep PRs focused and small (one logical change)
- Update PR description as needed

### After Merge

- Delete your branch (unless it's part of a larger series)
- Celebrate! 🎉 You've contributed to clibot!

## Style Guide

### Naming Conventions

- **Packages**: Lowercase, single word when possible
- **Constants**: `PascalCase` or `UPPER_SNAKE_CASE`
- **Variables**: `camelCase`
- **Functions**: `PascalCase` (exported), `camelCase` (private)
- **Interfaces**: `PascalCase` (usually end with `er`)

### File Organization

```
internal/
├── core/          # Core logic
├── cli/           # CLI adapters
├── bot/           # Bot adapters
├── logger/        # Logging utilities
└── ...
```

### Comments

- **Exported functions**: Must have godoc comments
```go
// LoadConfig loads configuration from file and expands environment variables.
func LoadConfig(configPath string) (*Config, error) {
```

- **Complex logic**: Add inline comments explaining "why", not "what"

### Git Workflow

- **One atomic change per commit**
- Don't combine unrelated changes in a single commit
- Write clear, descriptive commit messages
- Don't commit automatically without user confirmation

## Additional Resources

- [README.md](README.md) - Project overview and usage
- [SECURITY.md](SECURITY.md) - Security policy and best practices
- [AGENTS.md](AGENTS.md) - Development guidelines for AI agents
- [Documentation](./docs) - Detailed documentation
- [Discussions](https://github.com/keepmind9/clibot/discussions) - Ask questions and discuss ideas

## Getting Help

- **GitHub Issues**: For bugs and feature requests
- **GitHub Discussions**: For questions and general discussion
- **Security Issues**: Email [security@keepmind9.com](mailto:security@keepmind9.com)

---

**Thank you for contributing to clibot! 🚀**

Every contribution, no matter how small, is appreciated. Together we can make clibot better for everyone.
