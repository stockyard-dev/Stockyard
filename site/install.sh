#!/bin/sh
# Stockyard installer — https://stockyard.dev
# Usage: curl -sSL stockyard.dev/install | sh
set -e

REPO="stockyard-dev/stockyard"
BINARY="stockyard"
INSTALL_DIR="/usr/local/bin"

# Colors (if terminal supports them)
RED='\033[0;31m'
GREEN='\033[0;32m'
GOLD='\033[0;33m'
NC='\033[0m' # No Color

info() { printf "${GREEN}▸${NC} %s\n" "$1"; }
warn() { printf "${GOLD}▸${NC} %s\n" "$1"; }
fail() { printf "${RED}▸${NC} %s\n" "$1"; exit 1; }

# Detect OS and architecture
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

case "$ARCH" in
  x86_64|amd64) ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  armv7l)        ARCH="arm"   ;;
  *)             fail "Unsupported architecture: $ARCH" ;;
esac

case "$OS" in
  linux)  OS="linux"  ;;
  darwin) OS="darwin" ;;
  *)      fail "Unsupported OS: $OS" ;;
esac

info "Detected ${OS}/${ARCH}"

# Get version (from query param or latest release)
VERSION="${STOCKYARD_VERSION:-latest}"
if [ "$VERSION" = "latest" ]; then
  VERSION=$(curl -sSL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | head -1 | sed 's/.*"tag_name": *"//;s/".*//')
  if [ -z "$VERSION" ]; then
    # Fallback: try tags
    VERSION=$(curl -sSL "https://api.github.com/repos/${REPO}/tags" | grep '"name"' | head -1 | sed 's/.*"name": *"//;s/".*//')
  fi
fi

if [ -z "$VERSION" ]; then
  warn "Could not determine latest version, building from source..."

  # Check for Go
  if ! command -v go >/dev/null 2>&1; then
    fail "Go is required to build from source. Install Go from https://go.dev/dl/"
  fi

  TMPDIR=$(mktemp -d)
  info "Cloning repository..."
  git clone --depth 1 "https://github.com/${REPO}.git" "$TMPDIR/stockyard" 2>/dev/null
  cd "$TMPDIR/stockyard"
  info "Building..."
  CGO_ENABLED=0 go build -ldflags="-s -w" -o stockyard ./cmd/stockyard/

  # Install
  if [ -w "$INSTALL_DIR" ]; then
    mv stockyard "$INSTALL_DIR/"
  else
    info "Installing to ${INSTALL_DIR} (requires sudo)..."
    sudo mv stockyard "$INSTALL_DIR/"
  fi

  rm -rf "$TMPDIR"
  info "Installed stockyard (built from source)"
  stockyard --version
  exit 0
fi

info "Installing stockyard ${VERSION}"

# Construct download URL
TARBALL="${BINARY}_${VERSION#v}_${OS}_${ARCH}.tar.gz"
URL="https://github.com/${REPO}/releases/download/${VERSION}/${TARBALL}"

# Download and extract
TMPDIR=$(mktemp -d)
info "Downloading ${URL}..."
if command -v curl >/dev/null 2>&1; then
  curl -sSL "$URL" -o "$TMPDIR/stockyard.tar.gz" || fail "Download failed. Check https://github.com/${REPO}/releases for available versions."
elif command -v wget >/dev/null 2>&1; then
  wget -q "$URL" -O "$TMPDIR/stockyard.tar.gz" || fail "Download failed."
else
  fail "curl or wget required"
fi

info "Extracting..."
tar -xzf "$TMPDIR/stockyard.tar.gz" -C "$TMPDIR" 2>/dev/null || {
  # Might be a plain binary
  mv "$TMPDIR/stockyard.tar.gz" "$TMPDIR/stockyard"
  chmod +x "$TMPDIR/stockyard"
}

# Find the binary
BIN=$(find "$TMPDIR" -name "stockyard" -type f | head -1)
if [ -z "$BIN" ]; then
  fail "Binary not found in archive"
fi
chmod +x "$BIN"

# Install
if [ -w "$INSTALL_DIR" ]; then
  mv "$BIN" "$INSTALL_DIR/stockyard"
else
  info "Installing to ${INSTALL_DIR} (requires sudo)..."
  sudo mv "$BIN" "$INSTALL_DIR/stockyard"
fi

rm -rf "$TMPDIR"

# Verify
info "Installed successfully!"
echo ""
echo "  $(stockyard --version)"
echo ""
echo "  Quick start:"
echo "    stockyard doctor    # Check your environment"
echo "    stockyard           # Start the platform"
echo ""
echo "  Dashboard: http://localhost:4200/ui"
echo "  Docs:      https://stockyard.dev/docs"
echo ""
