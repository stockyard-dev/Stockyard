#!/bin/sh
# Stockyard Lasso installer
# Downloads the latest release from GitHub and installs to ~/.local/bin
#
# Usage:  curl -fsSL https://stockyard.dev/lasso/install.sh | sh
#         INSTALL_DIR=/usr/local/bin curl -fsSL https://stockyard.dev/lasso/install.sh | sh
set -e

SLUG="lasso"
REPO="stockyard-dev/stockyard-$SLUG"
INSTALL_DIR="${INSTALL_DIR:-$HOME/.local/bin}"

# Detect OS
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
case "$OS" in
  darwin|linux) : ;;
  mingw*|msys*|cygwin*) OS="windows" ;;
  *) echo "Unsupported OS: $OS" >&2; exit 1 ;;
esac

# Detect architecture
ARCH="$(uname -m)"
case "$ARCH" in
  x86_64|amd64) ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *) echo "Unsupported architecture: $ARCH" >&2; exit 1 ;;
esac

ASSET="stockyard-${SLUG}_${OS}_${ARCH}.tar.gz"
URL="https://github.com/$REPO/releases/latest/download/$ASSET"

mkdir -p "$INSTALL_DIR"
TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT

printf "  Downloading %s...\n" "$SLUG"
if ! curl -fsSL "$URL" -o "$TMP/$SLUG.tar.gz"; then
  echo "  Download failed: $URL" >&2
  echo "  If this tool does not have a release yet, check https://github.com/$REPO/releases" >&2
  exit 1
fi

tar -xzf "$TMP/$SLUG.tar.gz" -C "$TMP"

# Locate the extracted binary. Release tarballs contain a single file
# named stockyard-{slug}_{os}_{arch} (GoReleaser convention) with no
# subdirectory. Fall back to a looser match for older release shapes.
BIN="$(find "$TMP" -type f -name "stockyard-${SLUG}_*" ! -name "*.tar.gz" | head -1)"
if [ -z "$BIN" ]; then
  BIN="$(find "$TMP" -type f \( -name "$SLUG" -o -name "$SLUG.exe" -o -name "stockyard-$SLUG" \) | head -1)"
fi
if [ -z "$BIN" ] || [ ! -f "$BIN" ]; then
  echo "  Could not locate $SLUG binary inside archive" >&2
  find "$TMP" -type f >&2
  exit 1
fi

install -m 755 "$BIN" "$INSTALL_DIR/$SLUG"
printf "  \033[32m✓\033[0m Installed %s to %s\n" "$SLUG" "$INSTALL_DIR/$SLUG"

# PATH hint
case ":$PATH:" in
  *":$INSTALL_DIR:"*) ;;
  *)
    printf "\n  Add %s to your PATH:\n" "$INSTALL_DIR"
    printf "    export PATH=\"%s:\$PATH\"\n" "$INSTALL_DIR"
    ;;
esac

printf "\n  Run it:  %s\n" "$SLUG"
printf "  Dashboard will open at http://localhost:9700/ui\n\n"
