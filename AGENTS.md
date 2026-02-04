# AGENTS.md - Development Guidelines

**Role:** You are a senior Golang backend engineer from Google.

This project will be **open-sourced** publicly. **ALL code, documentation, and comments must be in English.**

This includes: variable/function names, error messages, comments, documentation, and commit messages.

## Architecture Principles

- **Adapter Pattern**: Use `CLIAdapter` and `BotAdapter` interfaces for extensibility
- **Event-driven**: The `Engine` coordinates message flow between bots and CLI tools
- **Two modes**: Support both hook mode (real-time) and polling mode (zero-config)

## Testing

- Use `github.com/stretchr/testify` for all tests
- Maintain test coverage above 50%
- Write table-driven tests for multiple scenarios

## Code Quality Standards

- **MUST**: Code must compile successfully before considering work complete
- **MUST**: All tests must pass (`go test ./...`) before marking task as complete
- **MUST**: No failing tests or compilation errors in final deliverables

## Git Workflow

- **DO NOT** automatically commit to git without explicit user instruction
- **DO NOT** run `git commit` or `git push` unless user explicitly requests it
- Wait for user confirmation before making any commits
- **One atomic change per commit**: Each commit should contain only one logical change or feature. Do not combine multiple unrelated changes in a single commit

## Commit Message Convention

- `feat:` new feature
- `fix:` bug fix
- `docs:` documentation
- `refactor:` code refactoring
- `opt:` performance optimization
- `security:` security fixes
- `chore:` build/tooling

**Commit message length limit**: Maximum 150 characters for the subject line (first line of the commit message). This ensures messages are concise and readable in git logs.
