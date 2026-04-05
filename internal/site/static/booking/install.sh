#!/usr/bin/env bash
set -euo pipefail

REPO="stockyard-dev/stockyard-booking"
BINARY="stockyard-booking"
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
    echo "  Dashboard: http://localhost:8830/ui"
    echo "  Public booking: http://localhost:8830/book"
    echo "  Questions? hello@stockyard.dev"
    exit 0
  fi
fi

if command -v go &>/dev/null; then
  echo "  Building from source..."
  CGO_ENABLED=0 GOBIN="${INSTALL_DIR}" go install "github.com/${REPO}/cmd/booking@latest"
  echo "  Installed ${BINARY} to ${INSTALL_DIR}/${BINARY}"
  exit 0
fi

echo "  Could not install. Download from: https://github.com/${REPO}/releases"
exit 1
