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

## Commit Message Convention

- `feat:` new feature
- `fix:` bug fix
- `docs:` documentation
- `refactor:` code refactoring
- `opt:` performance optimization
- `security:` security fixes
- `chore:` build/tooling
