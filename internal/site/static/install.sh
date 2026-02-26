#!/bin/sh
# Stockyard installer
# Usage: curl -sSL https://stockyard-production.up.railway.app/install.sh | sh
set -e

REPO="stockyard-dev/stockyard"
BINARY="stockyard"
INSTALL_DIR="/usr/local/bin"

C='\033[0;36m' G='\033[0;32m' Y='\033[1;33m' R='\033[0;31m' N='\033[0m'
info() { printf "${C}▸${N} %s\n" "$1"; }
ok()   { printf "${G}✓${N} %s\n" "$1"; }
warn() { printf "${Y}!${N} %s\n" "$1"; }
fail() { printf "${R}✗${N} %s\n" "$1"; exit 1; }

echo ""
echo "  ┌─────────────────────────────┐"
echo "  │  S T O C K Y A R D          │"
echo "  │  Where LLM traffic          │"
echo "  │  gets sorted.               │"
echo "  └─────────────────────────────┘"
echo ""

# Detect platform
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
case "$ARCH" in x86_64|amd64) ARCH="amd64" ;; arm64|aarch64) ARCH="arm64" ;; *) fail "Unsupported architecture: $ARCH" ;; esac
case "$OS" in linux|darwin) ;; *) fail "Unsupported OS: $OS" ;; esac
info "Platform: ${OS}/${ARCH}"

# Try GitHub release first
info "Checking for releases..."
LATEST=$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" 2>/dev/null | grep '"tag_name"' | head -1 | sed 's/.*"v//;s/".*//' || echo "")

if [ -n "$LATEST" ]; then
  VERSION="$LATEST"
  info "Latest release: v${VERSION}"
  URL="https://github.com/$REPO/releases/download/v${VERSION}/${BINARY}_${VERSION}_${OS}_${ARCH}.tar.gz"

  TMP=$(mktemp -d)
  trap "rm -rf $TMP" EXIT

  info "Downloading..."
  if curl -fsSL "$URL" -o "$TMP/archive.tar.gz" 2>/dev/null; then
    tar -xzf "$TMP/archive.tar.gz" -C "$TMP"
    info "Installing to ${INSTALL_DIR}..."
    if [ -w "$INSTALL_DIR" ]; then
      mv "$TMP/$BINARY" "$INSTALL_DIR/$BINARY"
    else
      sudo mv "$TMP/$BINARY" "$INSTALL_DIR/$BINARY"
    fi
    chmod +x "$INSTALL_DIR/$BINARY"
    ok "Stockyard v${VERSION} installed"
    echo ""
    printf "  ${C}Quick start:${N}\n"
    echo "    export STOCKYARD_ADMIN_KEY=my-secret-key"
    echo "    export OPENAI_API_KEY=sk-..."
    echo "    stockyard"
    echo ""
    echo "  Console:    http://localhost:8080/ui"
    echo "  Playground: http://localhost:8080/playground"
    echo "  Docs:       https://stockyard-production.up.railway.app/docs/"
    echo ""
    exit 0
  fi
  warn "Release download failed, falling back to source build..."
fi

# Build from source
info "Building from source..."

if ! command -v go >/dev/null 2>&1; then
  fail "Go 1.22+ required. Install from https://go.dev/dl/ and try again."
fi

if ! command -v git >/dev/null 2>&1; then
  fail "git is required. Install git and try again."
fi

TMP=$(mktemp -d)
trap "rm -rf $TMP" EXIT

info "Cloning repository..."
git clone --depth 1 "https://github.com/${REPO}.git" "$TMP/stockyard" 2>/dev/null || fail "Clone failed"

info "Compiling (this takes ~30s)..."
cd "$TMP/stockyard"
CGO_ENABLED=1 go build -ldflags="-s -w" -o "$TMP/${BINARY}" ./cmd/stockyard/ || fail "Build failed"

info "Installing to ${INSTALL_DIR}..."
if [ -w "$INSTALL_DIR" ]; then
  mv "$TMP/${BINARY}" "$INSTALL_DIR/${BINARY}"
else
  sudo mv "$TMP/${BINARY}" "$INSTALL_DIR/${BINARY}"
fi
chmod +x "$INSTALL_DIR/$BINARY"

ok "Stockyard installed (built from source)"
echo ""
printf "  ${C}Quick start:${N}\n"
echo "    export STOCKYARD_ADMIN_KEY=my-secret-key"
echo "    export OPENAI_API_KEY=sk-..."
echo "    stockyard"
echo ""
echo "  Console:    http://localhost:8080/ui"
echo "  Playground: http://localhost:8080/playground"
echo "  Docs:       https://stockyard-production.up.railway.app/docs/"
echo ""
