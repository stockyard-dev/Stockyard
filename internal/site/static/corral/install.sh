#!/usr/bin/env bash
set -euo pipefail

REPO="stockyard-dev/stockyard-corral"
BINARY="stockyard-corral"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"

OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"
case "$ARCH" in
  x86_64)  ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *) echo "Unsupported architecture: $ARCH"; exit 1 ;;
esac

TAG="${VERSION:-}"
if [ -z "$TAG" ]; then
  TAG=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | sed 's/.*"tag_name": *"\(.*\)".*/\1/')
fi
if [ -z "$TAG" ]; then
  echo "Could not determine latest version."
  echo "Trying go install..."
  if command -v go &>/dev/null; then
    CGO_ENABLED=0 GOBIN="${INSTALL_DIR}" go install "github.com/${REPO}/cmd/corral@latest"
    echo "  Installed via go install"
    exit 0
  fi
  echo "Install Go from https://go.dev or download from:"
  echo "  https://github.com/${REPO}/releases"
  exit 1
fi

FILENAME="${BINARY}_${OS}_${ARCH}.tar.gz"
URL="https://github.com/${REPO}/releases/download/${TAG}/${FILENAME}"

echo "Installing Stockyard Corral ${TAG} (${OS}/${ARCH})..."

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

if curl -fsSL "$URL" -o "${TMP}/${FILENAME}" 2>/dev/null; then
  tar -xzf "${TMP}/${FILENAME}" -C "$TMP"
  install -m755 "${TMP}/${BINARY}" "${INSTALL_DIR}/${BINARY}"
else
  echo "Release not found. Trying go install..."
  if command -v go &>/dev/null; then
    CGO_ENABLED=0 GOBIN="${INSTALL_DIR}" go install "github.com/${REPO}/cmd/corral@latest"
  else
    echo "Install Go from https://go.dev or download from:"
    echo "  https://github.com/${REPO}/releases"
    exit 1
  fi
fi

echo ""
echo "  Stockyard Corral installed to ${INSTALL_DIR}/${BINARY}"
echo "  Quick start:  DATA_DIR=./data stockyard-corral"
echo "  Dashboard:    http://localhost:9700/ui"
echo ""
