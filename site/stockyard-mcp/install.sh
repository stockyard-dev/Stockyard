#!/usr/bin/env bash
set -euo pipefail

REPO="stockyard-dev/Stockyard"
BINARY="stockyard-mcp"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"

OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"
case "$ARCH" in
  x86_64)  ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *) echo "Unsupported architecture: $ARCH"; exit 1 ;;
esac

echo ""
echo "  ┌──────────────────────────────────────────┐"
echo "  │  Stockyard MCP — 384 tools for your      │"
echo "  │  AI editor. One binary. Zero deps.        │"
echo "  │  https://stockyard.dev/mcp/               │"
echo "  └──────────────────────────────────────────┘"
echo ""

# Try GitHub release first
TAG="${VERSION:-}"
if [ -z "$TAG" ]; then
  RELEASE_JSON="$(mktemp)"
  if curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" -o "$RELEASE_JSON" 2>/dev/null; then
    TAG=$(cat "$RELEASE_JSON" | tr ',' '\n' | grep '"tag_name"' | cut -d'"' -f4 || true)
  fi
  rm -f "$RELEASE_JSON"
fi

FILENAME="${BINARY}_${OS}_${ARCH}.tar.gz"
if [ -n "$TAG" ]; then
  URL="https://github.com/${REPO}/releases/download/${TAG}/${FILENAME}"
  TMP="$(mktemp -d)"
  trap 'rm -rf "$TMP"' EXIT

  if curl -fsSL "$URL" -o "${TMP}/${FILENAME}" 2>/dev/null; then
    tar -xzf "${TMP}/${FILENAME}" -C "$TMP"
    install -m755 "${TMP}/${BINARY}" "${INSTALL_DIR}/${BINARY}"
    echo "  Installed ${BINARY} ${TAG} to ${INSTALL_DIR}/${BINARY}"
    echo ""
    echo "  Quick start:"
    echo "    ${BINARY} --tools costcap:4100,llmcache:4200"
    echo "    ${BINARY} --scan 4000-7000"
    echo "    ${BINARY} --list"
    echo ""
    echo "  Docs: https://stockyard.dev/mcp/"
    echo "  Questions? hello@stockyard.dev"
    exit 0
  fi
fi

# Fall back to go install
if command -v go &>/dev/null; then
  echo "  Building from source with go install..."
  CGO_ENABLED=0 GOBIN="${INSTALL_DIR}" go install "github.com/${REPO}/cmd/stockyard-mcp@latest"
  echo "  Installed ${BINARY} to ${INSTALL_DIR}/${BINARY}"
  echo ""
  echo "  Quick start:"
  echo "    ${BINARY} --tools costcap:4100,llmcache:4200"
  echo "    ${BINARY} --scan 4000-7000"
  echo "    ${BINARY} --list"
  echo ""
  echo "  Docs: https://stockyard.dev/mcp/"
  echo "  Questions? hello@stockyard.dev"
  exit 0
fi

echo "  Could not install automatically."
echo "  Options:"
echo "    1. Install Go (https://go.dev) and re-run this script"
echo "    2. Download from https://github.com/${REPO}/releases"
echo "    3. Build from source:"
echo "       git clone https://github.com/${REPO}.git"
echo "       cd Stockyard && go build -o stockyard-mcp ./cmd/stockyard-mcp/"
echo ""
exit 1
