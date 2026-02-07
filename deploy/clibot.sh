#!/bin/bash
# clibot Management Script
# For development and testing environments only
# For production deployment, use systemd or supervisor
# See: deploy/DEPLOYMENT.md

set -e

# ==============================================================================
# Configuration (YOU CAN CUSTOMIZE)
# ==============================================================================

# Default paths (override with environment variables)
CLIBOT_BIN="${CLIBOT_BIN:-clibot}"
CONFIG_FILE="${CONFIG_FILE:-~/.config/clibot/config.yaml}"
PID_FILE="${PID_FILE:-/tmp/clibot.pid}"
LOG_FILE="${LOG_FILE:-/tmp/clibot.log}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# ==============================================================================
# Helper Functions
# ==============================================================================

info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# ==============================================================================
# Core Functions
# ==============================================================================

# Check if clibot binary exists
check_binary() {
    if ! command -v "$CLIBOT_BIN" &> /dev/null; then
        error "clibot binary not found: $CLIBOT_BIN"
        echo ""
        echo "Install clibot:"
        echo "  go install github.com/keepmind9/clibot@latest"
        echo ""
        echo "Or set custom binary path:"
        echo "  export CLIBOT_BIN=/path/to/clibot"
        echo "  $0 $@"
        exit 1
    fi
}

# Check if config file exists
check_config() {
    if [ ! -f "$CONFIG_FILE" ]; then
        error "Config file not found: $CONFIG_FILE"
        echo ""
        echo "Create config file:"
        echo "  mkdir -p ~/.config/clibot"
        echo "  cp configs/config.yaml ~/.config/clibot/config.yaml"
        echo ""
        echo "Or set custom config path:"
        echo "  export CONFIG_FILE=/path/to/config.yaml"
        echo "  $0 $@"
        exit 1
    fi
}

# Get PID from PID file
get_pid() {
    if [ -f "$PID_FILE" ]; then
        cat "$PID_FILE"
    fi
}

# Check if clibot is running
is_running() {
    local pid=$(get_pid)
    if [ -n "$pid" ]; then
        if ps -p "$pid" &> /dev/null; then
            return 0
        fi
    fi
    return 1
}

# ==============================================================================
# Commands
# ==============================================================================

# Start clibot
start() {
    check_binary "$@"
    check_config "$@"

    if is_running; then
        warning "clibot is already running (PID: $(get_pid))"
        return 0
    fi

    info "Starting clibot..."
    info "Binary: $CLIBOT_BIN"
    info "Config: $CONFIG_FILE"
    info "Log: $LOG_FILE"

    # Start in background
    nohup "$CLIBOT_BIN" serve --config "$CONFIG_FILE" >> "$LOG_FILE" 2>&1 &
    local pid=$!

    # Save PID
    echo $pid > "$PID_FILE"

    # Wait a moment and check if it's still running
    sleep 2
    if ps -p $pid &> /dev/null; then
        success "clibot started successfully (PID: $pid)"
        info "View logs: $0 logs"
    else
        error "Failed to start clibot. Check log: $LOG_FILE"
        rm -f "$PID_FILE"
        exit 1
    fi
}

# Stop clibot
stop() {
    if ! is_running; then
        warning "clibot is not running"
        rm -f "$PID_FILE"
        return 0
    fi

    local pid=$(get_pid)
    info "Stopping clibot (PID: $pid)..."

    # Try graceful shutdown first
    kill $pid 2>/dev/null || true

    # Wait for process to terminate
    local count=0
    while ps -p $pid &> /dev/null && [ $count -lt 10 ]; do
        sleep 1
        count=$((count + 1))
    done

    # Force kill if still running
    if ps -p $pid &> /dev/null; then
        warning "Force killing clibot..."
        kill -9 $pid 2>/dev/null || true
        sleep 1
    fi

    # Clean up PID file
    rm -f "$PID_FILE"

    if ps -p $pid &> /dev/null; then
        error "Failed to stop clibot"
        exit 1
    else
        success "clibot stopped successfully"
    fi
}

# Restart clibot
restart() {
    info "Restarting clibot..."
    stop "$@"
    sleep 1
    start "$@"
}

# Show status
status() {
    check_binary "$@"

    echo "=== clibot Status ==="
    echo ""
    echo "Binary: $CLIBOT_BIN"
    echo "Config: $CONFIG_FILE"
    echo "PID File: $PID_FILE"
    echo "Log File: $LOG_FILE"
    echo ""

    if is_running; then
        local pid=$(get_pid)
        success "clibot is running (PID: $pid)"
        echo ""
        echo "Process info:"
        ps -p "$pid" -o pid,ppid,cmd,etime,stat
        echo ""
        echo "Tmux sessions:"
        tmux list-sessions 2>/dev/null || echo "  No tmux sessions found"
    else
        warning "clibot is not running"
        rm -f "$PID_FILE"
    fi
}

# Show logs
logs() {
    if [ ! -f "$LOG_FILE" ]; then
        warning "Log file not found: $LOG_FILE"
        echo ""
        echo "clibot might not be running yet, or logs are in journal/system logger"
        return 0
    fi

    if [ -t 1 ]; then
        # Terminal output: use tail -f
        tail -f "$LOG_FILE"
    else
        # Non-terminal: show last 100 lines
        tail -n 100 "$LOG_FILE"
    fi
}

# Show help
show_help() {
    cat << EOF
clibot Management Script (Development/Testing)

Usage: $0 [command] [options]

Commands:
  start           Start clibot service
  stop            Stop clibot service
  restart         Restart clibot service
  status          Show clibot status
  logs            View clibot logs (tail -f)
  help            Show this help message

Environment Variables:
  CLIBOT_BIN      Path to clibot binary (default: clibot)
  CONFIG_FILE     Path to config file (default: ~/.config/clibot/config.yaml)
  PID_FILE        Path to PID file (default: /tmp/clibot.pid)
  LOG_FILE        Path to log file (default: /tmp/clibot.log)

Examples:
  # Start with default settings
  $0 start

  # Use custom config
  CONFIG_FILE=/etc/clibot/config.yaml $0 start

  # Use custom binary
  CLIBOT_BIN=/usr/local/bin/clibot $0 start

  # View logs
  $0 logs

For production deployment, use systemd or supervisor:
  See: deploy/DEPLOYMENT.md
EOF
}

# ==============================================================================
# Main
# ==============================================================================

main() {
    local command="${1:-help}"

    case "$command" in
        start)
            start "$@"
            ;;
        stop)
            stop "$@"
            ;;
        restart)
            restart "$@"
            ;;
        status)
            status "$@"
            ;;
        logs)
            logs
            ;;
        help|--help|-h)
            show_help
            ;;
        *)
            error "Unknown command: $command"
            echo ""
            show_help
            exit 1
            ;;
    esac
}

main "$@"
