#!/usr/bin/env bash
set -euo pipefail

REPO="stockyard-dev/stockyard-curriculum"
BINARY="stockyard-curriculum"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"

OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"
case "$ARCH" in
  x86_64)  ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *) echo "Unsupported architecture: $ARCH"; exit 1 ;;
esac

echo "Installing Stockyard Curriculum (${OS}/${ARCH})..."

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

URL="https://github.com/${REPO}/releases/latest/download/${BINARY}_${OS}_${ARCH}.tar.gz"
if curl -fsSL "$URL" -o "${TMP}/archive.tar.gz" 2>/dev/null; then
  tar -xzf "${TMP}/archive.tar.gz" -C "$TMP"
  install -m755 "${TMP}/${BINARY}" "${INSTALL_DIR}/${BINARY}"
else
  echo "Release not found. Trying go install..."
  if command -v go &>/dev/null; then
    CGO_ENABLED=0 GOBIN="${INSTALL_DIR}" go install "github.com/stockyard-dev/stockyard-curriculum/cmd/curriculum@latest"
  else
    echo "Install Go from https://go.dev or download from:"
    echo "  https://github.com/${REPO}/releases"
    exit 1
  fi
fi

echo ""
echo "  Stockyard Curriculum installed to ${INSTALL_DIR}/${BINARY}"
echo "  Quick start:  stockyard-curriculum"
echo "  Dashboard:    http://localhost:9813/ui"
echo ""
