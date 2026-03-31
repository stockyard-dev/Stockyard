#!/usr/bin/env bash
set -euo pipefail

REPO="stockyard-dev/stockyard-trough"
BINARY="trough"
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
  echo "Could not determine latest version. Set VERSION=vX.Y.Z to specify one."
  exit 1
fi

FILENAME="${BINARY}_${OS}_${ARCH}.tar.gz"
URL="https://github.com/${REPO}/releases/download/${TAG}/${FILENAME}"

echo "Installing Stockyard Trough ${TAG} (${OS}/${ARCH})..."

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

curl -fsSL "$URL" -o "${TMP}/${FILENAME}"
tar -xzf "${TMP}/${FILENAME}" -C "$TMP"
install -m755 "${TMP}/${BINARY}" "${INSTALL_DIR}/${BINARY}"

echo ""
echo "  Stockyard Trough ${TAG} installed to ${INSTALL_DIR}/${BINARY}"
echo ""
echo "  Quick start:"
echo "    TROUGH_ADMIN_KEY=secret trough"
echo ""
