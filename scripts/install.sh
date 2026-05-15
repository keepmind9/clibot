#!/bin/bash
# clibot Auto-Installation Script
# Downloads latest release from GitHub and installs to ~/.local/bin

set -e

REPO="keepmind9/clibot"
BINARY="clibot"
INSTALL_DIR="$HOME/.local/bin"

echo "Checking clibot installation..."

# Get latest version info
echo "Fetching latest release..."
RELEASE=$(curl -sf "https://api.github.com/repos/${REPO}/releases/latest")
LATEST_VERSION=$(echo "$RELEASE" | grep -o '"tag_name": "[^"]*"' | head -1 | sed 's/.*: "//;s/"//')

if [ -z "$LATEST_VERSION" ]; then
    echo "Failed to fetch release info. Install manually:"
    echo "  https://github.com/${REPO}/releases"
    exit 1
fi

if command -v "$BINARY" &> /dev/null; then
    CURRENT=$("$BINARY" version 2>/dev/null | grep "Version:" | awk '{print $2}')
    if [ "$CURRENT" = "${LATEST_VERSION#v}" ]; then
        echo "clibot is already up to date ($LATEST_VERSION)."
        exit 0
    fi
    if [ -n "$CURRENT" ]; then
        echo "clibot $CURRENT installed, upgrading to $LATEST_VERSION..."
    else
        echo "clibot installed, upgrading to $LATEST_VERSION..."
    fi
else
    echo "clibot not found. Installing $LATEST_VERSION..."
fi

# Detect platform
OS=""
ARCH=""
case "$(uname -s)" in
    Linux*)  OS="linux";;
    Darwin*) OS="darwin";;
    *)       echo "Unsupported OS: $(uname -s)"; exit 1;;
esac

case "$(uname -m)" in
    x86_64|amd64)   ARCH="amd64";;
    aarch64|arm64)  ARCH="arm64";;
    *)              echo "Unsupported architecture: $(uname -m)"; exit 1;;
esac

PATTERN="${OS}-${ARCH}"

# Find matching asset download URL
DOWNLOAD_URL=$(echo "$RELEASE" | grep -o "\"browser_download_url\": \"[^\"]*${PATTERN}[^\"]*\"" | grep -v '\.sha256' | head -1 | sed 's/.*: "\(.*\)"/\1/')

if [ -z "$DOWNLOAD_URL" ]; then
    echo "No matching release found for ${PATTERN}."
    echo "Available assets:"
    echo "$RELEASE" | grep "browser_download_url" | sed 's/.*: "//;s/"//'
    exit 1
fi

FILENAME=$(basename "$DOWNLOAD_URL")

echo "Downloading clibot ${LATEST_VERSION} for ${OS}/${ARCH}..."

TMPDIR=$(mktemp -d)
trap 'rm -rf "$TMPDIR"' EXIT

if ! curl -fL --progress-bar "$DOWNLOAD_URL" -o "$TMPDIR/$FILENAME"; then
    echo "Download failed. Install manually:"
    echo "  https://github.com/${REPO}/releases"
    exit 1
fi

mkdir -p "$INSTALL_DIR"
mv "$TMPDIR/$FILENAME" "$INSTALL_DIR/$BINARY"
chmod +x "$INSTALL_DIR/$BINARY"

# Add to PATH if needed
if ! echo "$PATH" | grep -q "$INSTALL_DIR"; then
    echo "Adding $INSTALL_DIR to PATH..."
    SHELL_RC=""
    if [ -f "$HOME/.zshrc" ]; then
        SHELL_RC="$HOME/.zshrc"
    elif [ -f "$HOME/.bashrc" ]; then
        SHELL_RC="$HOME/.bashrc"
    fi

    if [ -n "$SHELL_RC" ]; then
        echo "export PATH=\"$INSTALL_DIR:\$PATH\"" >> "$SHELL_RC"
    fi
    export PATH="$INSTALL_DIR:$PATH"
fi

echo ""
echo "clibot ${LATEST_VERSION} installed successfully!"
echo "  Location: $INSTALL_DIR/$BINARY"
echo ""
echo "Verify:"
echo "  clibot version"
echo ""
echo "If 'clibot' not found, restart your shell or run:"
echo "  source ~/.zshrc  # or source ~/.bashrc"
