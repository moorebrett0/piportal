#!/bin/bash
# PiPortal Installer
#
# Usage:
#   curl -fsSL https://your-server.com/install.sh | bash
#
# Or with custom server:
#   curl -fsSL https://your-server.com/install.sh | PIPORTAL_SERVER=https://your-server.com bash

set -e

VERSION="0.1.4"
# Use PIPORTAL_SERVER env var, or derive from script URL, or fail
if [ -n "$PIPORTAL_SERVER" ]; then
    BASE_URL="${PIPORTAL_SERVER}/downloads"
else
    echo "Error: PIPORTAL_SERVER environment variable not set"
    echo ""
    echo "Usage: curl -fsSL https://your-server.com/install.sh | PIPORTAL_SERVER=https://your-server.com bash"
    exit 1
fi

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

info() { echo -e "${GREEN}✓${NC} $1"; }
warn() { echo -e "${YELLOW}!${NC} $1"; }
error() { echo -e "${RED}✗${NC} $1"; exit 1; }

# Detect architecture
detect_arch() {
    local arch=$(uname -m)
    case $arch in
        aarch64|arm64)
            echo "arm64"
            ;;
        armv7l|armv6l)
            echo "arm"
            ;;
        x86_64)
            echo "amd64"
            ;;
        *)
            error "Unsupported architecture: $arch"
            ;;
    esac
}

echo ""
echo "  ╔═══════════════════════════════════════╗"
echo "  ║        PiPortal Installer             ║"
echo "  ╚═══════════════════════════════════════╝"
echo ""

# Detect system
ARCH=$(detect_arch)
info "Detected architecture: $ARCH"

# Determine install location
if [[ $EUID -eq 0 ]]; then
    INSTALL_DIR="/usr/local/bin"
else
    INSTALL_DIR="$HOME/.local/bin"
    mkdir -p "$INSTALL_DIR"
fi

# Download binary
DOWNLOAD_URL="${BASE_URL}/piportal-linux-${ARCH}"
info "Downloading piportal..."

if command -v curl &> /dev/null; then
    curl -fsSL "$DOWNLOAD_URL" -o "${INSTALL_DIR}/piportal"
elif command -v wget &> /dev/null; then
    wget -q "$DOWNLOAD_URL" -O "${INSTALL_DIR}/piportal"
else
    error "Neither curl nor wget found. Please install one."
fi

chmod +x "${INSTALL_DIR}/piportal"
info "Installed to ${INSTALL_DIR}/piportal"

# Check if in PATH
if ! command -v piportal &> /dev/null; then
    if [[ ":$PATH:" != *":$INSTALL_DIR:"* ]]; then
        warn "Add to your PATH: export PATH=\"${INSTALL_DIR}:\$PATH\""
        echo ""
        echo "  Add this line to ~/.bashrc or ~/.profile:"
        echo "    export PATH=\"${INSTALL_DIR}:\$PATH\""
        echo ""
    fi
fi

# Verify
"${INSTALL_DIR}/piportal" --version 2>/dev/null || true

echo ""
echo "  ─────────────────────────────────────────"
echo ""
echo "  Installation complete!"
echo ""
echo "  Next, run the setup wizard:"
echo ""
echo "    piportal setup"
echo ""
echo "  This will register your device and configure"
echo "  your tunnel automatically."
echo ""
