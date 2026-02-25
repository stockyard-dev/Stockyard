#!/bin/sh
# Stockyard installer
# Usage: curl -sSL stockyard.dev/install | sh
set -e

REPO="stockyard-dev/stockyard"
VERSION="${1:-latest}"
BINARY="stockyard"

# Detect platform
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
case "$ARCH" in
  x86_64|amd64) ARCH="amd64" ;;
  arm64|aarch64) ARCH="arm64" ;;
  *) echo "Unsupported architecture: $ARCH"; exit 1 ;;
esac
case "$OS" in
  linux|darwin) ;;
  *) echo "Unsupported OS: $OS"; exit 1 ;;
esac

# Get latest version if not specified
if [ "$VERSION" = "latest" ]; then
  VERSION=$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" 2>/dev/null | grep '"tag_name"' | sed 's/.*"v//' | sed 's/".*//')
  if [ -z "$VERSION" ]; then
    VERSION="0.1.0"
  fi
fi

URL="https://github.com/$REPO/releases/download/v${VERSION}/${BINARY}_${OS}_${ARCH}.tar.gz"
INSTALL_DIR="/usr/local/bin"

echo ""
echo "  Stockyard v${VERSION}"
echo "  Platform: ${OS}/${ARCH}"
echo ""

# Download and extract
TMP=$(mktemp -d)
trap "rm -rf $TMP" EXIT

if ! curl -fsSL "$URL" -o "$TMP/archive.tar.gz" 2>/dev/null; then
  echo "  Download failed. Building from source instead..."
  echo ""
  echo "  git clone https://github.com/$REPO"
  echo "  cd stockyard && go build -o stockyard ./cmd/stockyard"
  exit 1
fi

tar -xzf "$TMP/archive.tar.gz" -C "$TMP"

# Install
if [ -w "$INSTALL_DIR" ]; then
  mv "$TMP/$BINARY" "$INSTALL_DIR/$BINARY"
else
  echo "  Installing to $INSTALL_DIR (requires sudo)..."
  sudo mv "$TMP/$BINARY" "$INSTALL_DIR/$BINARY"
fi
chmod +x "$INSTALL_DIR/$BINARY"

echo "  ✓ Installed $INSTALL_DIR/$BINARY"
echo ""
echo "  Get started:"
echo "    stockyard"
echo "    # Console: http://localhost:4200/ui"
echo "    # Proxy:   http://localhost:4200/v1/chat/completions"
echo ""
