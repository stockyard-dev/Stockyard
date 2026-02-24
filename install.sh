#!/bin/sh
# Stockyard installer
# Usage:
#   curl -fsSL https://stockyard.dev/install.sh | sh                    # installs stockyard
#   curl -fsSL https://stockyard.dev/install.sh | sh -s -- costcap      # installs costcap
#   curl -fsSL https://stockyard.dev/install.sh | sh -s -- stockyard 0.1.0 # specific version
set -e

PRODUCT="${1:-stockyard}"
VERSION="${2:-latest}"
REPO="stockyard-dev/stockyard"

# Validate product name
case "$PRODUCT" in
  costcap|llmcache|jsonguard|routefall|rateshield|promptreplay|keypool|promptguard|modelswitch|evalgate|usagepulse|promptpad|tokentrim|batchqueue|multicall|streamsnap|llmtap|contextpack|retrypilot|stockyard) ;;
  *) echo "Unknown product: $PRODUCT"; echo "Available: costcap llmcache jsonguard routefall rateshield promptreplay keypool promptguard modelswitch evalgate usagepulse promptpad tokentrim batchqueue multicall streamsnap llmtap contextpack retrypilot stockyard"; exit 1 ;;
esac

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
  *) echo "Unsupported OS: $OS (use npx or Docker on Windows)"; exit 1 ;;
esac

# Get latest version if not specified
if [ "$VERSION" = "latest" ]; then
  VERSION=$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" | grep '"tag_name"' | sed 's/.*"v//' | sed 's/".*//')
  if [ -z "$VERSION" ]; then
    echo "Failed to fetch latest version. Specify a version: sh -s -- $PRODUCT 0.1.0"
    exit 1
  fi
fi

URL="https://github.com/$REPO/releases/download/v${VERSION}/${PRODUCT}_${OS}_${ARCH}.tar.gz"
INSTALL_DIR="/usr/local/bin"

echo ""
echo "  ╔══════════════════════════════════════╗"
echo "  ║  Installing $PRODUCT v$VERSION"
echo "  ║  Platform: $OS/$ARCH"
echo "  ╚══════════════════════════════════════╝"
echo ""

# Download and extract
TMP=$(mktemp -d)
trap "rm -rf $TMP" EXIT

HTTP_CODE=$(curl -fsSL -w "%{http_code}" "$URL" -o "$TMP/archive.tar.gz" 2>/dev/null) || true
if [ "$HTTP_CODE" != "200" ] && [ ! -s "$TMP/archive.tar.gz" ]; then
  echo "Download failed (HTTP $HTTP_CODE)."
  echo "URL: $URL"
  echo ""
  echo "Try: npx @stockyard/$PRODUCT"
  exit 1
fi

tar -xzf "$TMP/archive.tar.gz" -C "$TMP"

# Install
if [ -w "$INSTALL_DIR" ]; then
  mv "$TMP/$PRODUCT" "$INSTALL_DIR/$PRODUCT"
else
  echo "  Installing to $INSTALL_DIR (requires sudo)..."
  sudo mv "$TMP/$PRODUCT" "$INSTALL_DIR/$PRODUCT"
fi
chmod +x "$INSTALL_DIR/$PRODUCT"

echo "  ✓ Installed $INSTALL_DIR/$PRODUCT"
echo ""
echo "  Get started:"
echo "    export OPENAI_API_KEY=sk-..."
echo "    $PRODUCT"
echo ""

# Product-specific port hint
case "$PRODUCT" in
  costcap)     echo "  Dashboard: http://localhost:4100/ui" ;;
  llmcache)    echo "  Dashboard: http://localhost:4101/ui" ;;
  jsonguard)   echo "  Dashboard: http://localhost:4102/ui" ;;
  routefall)   echo "  Dashboard: http://localhost:4103/ui" ;;
  rateshield)  echo "  Dashboard: http://localhost:4104/ui" ;;
  promptreplay) echo "  Dashboard: http://localhost:4105/ui" ;;
  stockyard)      echo "  Dashboard: http://localhost:4200/ui" ;;
esac
echo ""
