# clibot Makefile
# A CLI bot that bridges chat platforms with CLI tools

# Project metadata
BINARY_NAME=clibot
VERSION?=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME=$(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS=-ldflags "-X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME)"

# Directories
CMD_DIR=./cmd/$(BINARY_NAME)
BUILD_DIR=./bin
CONFIG_DIR=./configs

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
GOVENDOR=$(GOCMD) mod vendor

# Color output
BLUE=\033[0;34m
GREEN=\033[0;32m
RED=\033[0;31m
YELLOW=\033[0;33m
NC=\033[0m # No Color

.PHONY: all
all: build

## build: Build the application
.PHONY: build
build:
	@echo "$(BLUE)Building $(BINARY_NAME)...$(NC)"
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) $(CMD_DIR)
	@echo "$(GREEN)Build complete: $(BUILD_DIR)/$(BINARY_NAME)$(NC)"

## build-linux: Build for Linux
.PHONY: build-linux
build-linux:
	@echo "$(BLUE)Building $(BINARY_NAME) for Linux...$(NC)"
	@mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 $(CMD_DIR)
	@echo "$(GREEN)Build complete: $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64$(NC)"

## build-darwin: Build for macOS
.PHONY: build-darwin
build-darwin:
	@echo "$(BLUE)Building $(BINARY_NAME) for macOS...$(NC)"
	@mkdir -p $(BUILD_DIR)
	GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 $(CMD_DIR)
	@echo "$(GREEN)Build complete: $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64$(NC)"

## build-all: Build for all platforms
.PHONY: build-all
build-all: build-linux build-darwin
	@echo "$(GREEN)All builds complete!$(NC)"

## install: Install the application to $GOPATH/bin
.PHONY: install
install:
	@echo "$(BLUE)Installing $(BINARY_NAME)...$(NC)"
	$(GOBUILD) $(LDFLAGS) -o $(GOPATH)/bin/$(BINARY_NAME) $(CMD_DIR)
	@echo "$(GREEN)Install complete: $(GOPATH)/bin/$(BINARY_NAME)$(NC)"

## run: Run the application (requires config file)
.PHONY: run
run: build
	@echo "$(BLUE)Running $(BINARY_NAME)...$(NC)"
	@if [ ! -f "$(CONFIG_DIR)/config.yaml" ]; then \
		echo "$(RED)Error: Config file not found at $(CONFIG_DIR)/config.yaml$(NC)"; \
		exit 1; \
	fi
	$(BUILD_DIR)/$(BINARY_NAME) server

## test: Run all tests
.PHONY: test
test:
	@echo "$(BLUE)Running tests...$(NC)"
	$(GOTEST) -v -race -cover ./...

## test-short: Run short tests only
.PHONY: test-short
test-short:
	@echo "$(BLUE)Running short tests...$(NC)"
	$(GOTEST) -v -short ./...

## test-coverage: Run tests with coverage report
.PHONY: test-coverage
test-coverage:
	@echo "$(BLUE)Running tests with coverage...$(NC)"
	$(GOTEST) -v -race -coverprofile=coverage.out -covermode=atomic ./...
	@echo "$(GREEN)Coverage report generated: coverage.out$(NC)"
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "$(GREEN)HTML coverage report: coverage.html$(NC)"

## clean: Clean build artifacts
.PHONY: clean
clean:
	@echo "$(BLUE)Cleaning...$(NC)"
	$(GOCLEAN)
	@rm -rf $(BUILD_DIR)
	@rm -f coverage.out coverage.html
	@echo "$(GREEN)Clean complete$(NC)"

## deps: Download dependencies
.PHONY: deps
deps:
	@echo "$(BLUE)Downloading dependencies...$(NC)"
	$(GOMOD) download
	@echo "$(GREEN)Dependencies downloaded$(NC)"

## deps-tidy: Tidy go.mod
.PHONY: deps-tidy
deps-tidy:
	@echo "$(BLUE)Tidying go.mod...$(NC)"
	$(GOMOD) tidy
	@echo "$(GREEN)go.mod tidied$(NC)"

## fmt: Format code
.PHONY: fmt
fmt:
	@echo "$(BLUE)Formatting code...$(NC)"
	$(GOCMD) fmt ./...
	@echo "$(GREEN)Code formatted$(NC)"

## vet: Run go vet
.PHONY: vet
vet:
	@echo "$(BLUE)Running go vet...$(NC)"
	$(GOCMD) vet ./...
	@echo "$(GREEN)go vet complete$(NC)"

## lint: Run linter (requires golangci-lint)
.PHONY: lint
lint:
	@echo "$(BLUE)Running linter...$(NC)"
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run ./...; \
		echo "$(GREEN)Linting complete$(NC)"; \
	else \
		echo "$(YELLOW)golangci-lint not found. Install it from: https://golangci-lint.run/usage/install/$(NC)"; \
	fi

## check: Run all checks (fmt, vet, test)
.PHONY: check
check: fmt vet test
	@echo "$(GREEN)All checks passed!$(NC)"

## help: Show this help message
.PHONY: help
help:
	@echo "$(BLUE)clibot Makefile$(NC)"
	@echo ""
	@echo "$(GREEN)Usage:$(NC)"
	@echo "  make [target]"
	@echo ""
	@echo "$(GREEN)Available targets:$(NC)"
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/## /  /' | column -t -s ':'

## dev: Development build with race detection
.PHONY: dev
dev:
	@echo "$(BLUE)Building $(BINARY_NAME) with race detection...$(NC)"
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) -race $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) $(CMD_DIR)
	@echo "$(GREEN)Dev build complete: $(BUILD_DIR)/$(BINARY_NAME)$(NC)"

## config-example: Print example configuration
.PHONY: config-example
config-example:
	@echo "$(BLUE)Example configuration:$(NC)"
	@cat $(CONFIG_DIR)/config.yaml

## tmux-list: List tmux sessions (useful for debugging)
.PHONY: tmux-list
tmux-list:
	@echo "$(BLUE)Active tmux sessions:$(NC)"
	@tmux list-sessions 2>/dev/null || echo "$(YELLOW)No tmux sessions running$(NC)"

## tmux-attach: Attach to tmux session (use SESSION=name)
.PHONY: tmux-attach
tmux-attach:
	@if [ -z "$(SESSION)" ]; then \
		echo "$(RED)Error: Please specify SESSION=name$(NC)"; \
		exit 1; \
	fi
	@echo "$(BLUE)Attaching to tmux session: $(SESSION)$(NC)"
	@tmux attach-session -t $(SESSION)

## test-hook: Send a test hook request (requires running server)
.PHONY: test-hook
test-hook:
	@echo "$(BLUE)Sending test hook request...$(NC)"
	@echo '{"session_id": "test-session", "transcript_path": "/tmp/test.jsonl"}' | \
		curl -X POST "http://localhost:8080/hook?cli_type=claude" \
		--data-binary @- -v

.DEFAULT_GOAL := help
