#!/bin/sh
set -e

# Stockyard CLI installer
# Usage: curl -fsSL https://stockyard.dev/cli/install.sh | sh

REPO="stockyard-dev/stockyard-cli"
INSTALL_DIR="/usr/local/bin"
BINARY_NAME="stockyard"

# Detect OS
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$OS" in
    linux)  OS="linux" ;;
    darwin) OS="darwin" ;;
    *)      echo "Unsupported OS: $OS"; exit 1 ;;
esac

# Detect architecture
ARCH=$(uname -m)
case "$ARCH" in
    x86_64|amd64)  ARCH="amd64" ;;
    aarch64|arm64) ARCH="arm64" ;;
    *)             echo "Unsupported architecture: $ARCH"; exit 1 ;;
esac

ASSET="stockyard-${OS}-${ARCH}"

echo ""
echo "  ┌─────────────────────────────────────┐"
echo "  │  Stockyard CLI Installer             │"
echo "  │  Wrangle your stack.                │"
echo "  └─────────────────────────────────────┘"
echo ""
echo "  Detected: ${OS}/${ARCH}"

# Get latest release URL
LATEST_URL="https://api.github.com/repos/${REPO}/releases/latest"
echo "  Fetching latest release..."

DOWNLOAD_URL=$(curl -fsSL "$LATEST_URL" | grep "browser_download_url.*${ASSET}" | head -1 | cut -d '"' -f 4)

if [ -z "$DOWNLOAD_URL" ]; then
    echo "  Error: Could not find binary for ${OS}/${ARCH}"
    echo "  Check https://github.com/${REPO}/releases for available binaries."
    exit 1
fi

VERSION=$(echo "$DOWNLOAD_URL" | grep -o 'v[0-9]*\.[0-9]*\.[0-9]*' | head -1)
echo "  Downloading ${BINARY_NAME} ${VERSION}..."

# Download to temp file
TMP=$(mktemp)
curl -fsSL "$DOWNLOAD_URL" -o "$TMP"
chmod +x "$TMP"

# Install
if [ -w "$INSTALL_DIR" ]; then
    mv "$TMP" "${INSTALL_DIR}/${BINARY_NAME}"
else
    echo "  Installing to ${INSTALL_DIR} (requires sudo)..."
    sudo mv "$TMP" "${INSTALL_DIR}/${BINARY_NAME}"
fi

echo ""
echo "  ✓ Installed ${BINARY_NAME} ${VERSION} to ${INSTALL_DIR}/${BINARY_NAME}"
echo ""
echo "  Get started:"
echo "    stockyard list                    # Browse 150 tools"
echo "    stockyard install corral          # Install a tool"
echo "    stockyard start corral            # Start it"
echo "    stockyard status                  # Check status"
echo ""
echo "  Set your license for Pro features:"
echo "    stockyard license set SY-xxxxx"
echo ""
echo "  Docs: https://stockyard.dev/docs/"
echo ""

# Track install
curl -fsSL "https://stockyard.dev/cli/install.sh?event=install&os=${OS}&arch=${ARCH}" > /dev/null 2>&1 || true
