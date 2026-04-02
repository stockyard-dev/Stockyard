#!/bin/sh
# Stockyard Multi-Tool Installer
# Usage: curl -fsSL stockyard.dev/install-tools.sh | sh -s -- bounty headcount strongbox
#    or: curl -fsSL stockyard.dev/install-tools.sh | sh -s -- --all-free
set -e

GREEN='\033[0;32m'; GOLD='\033[0;33m'; RED='\033[0;31m'; CYAN='\033[0;36m'; NC='\033[0m'
info() { printf "${GREEN}▸${NC} %s\n" "$1"; }
warn() { printf "${GOLD}▸${NC} %s\n" "$1"; }
fail() { printf "${RED}▸${NC} %s\n" "$1"; exit 1; }

INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
case "$ARCH" in x86_64|amd64) ARCH="amd64" ;; aarch64|arm64) ARCH="arm64" ;; *) fail "Unsupported: $ARCH" ;; esac
case "$OS" in linux) OS="linux" ;; darwin) OS="darwin" ;; *) fail "Unsupported: $OS" ;; esac

# Popular tool bundles
STARTER="bounty headcount strongbox saltlick bellwether"
DEVTOOLS="bounty roundup prairie codex assay pipeline strongbox embargo"
OPS="bellwether inquest sentinel paddock outpost quarry handbook announcements"
FINANCE="billfold dossier prospector steward exchequer books"
ALL_FREE="bounty headcount strongbox saltlick bellwether corral gate trough fence notebook roundup prospector billfold"

TOOLS=""
for arg in "$@"; do
  case "$arg" in
    --starter)    TOOLS="$STARTER" ;;
    --devtools)   TOOLS="$DEVTOOLS" ;;
    --ops)        TOOLS="$OPS" ;;
    --finance)    TOOLS="$FINANCE" ;;
    --all-free)   TOOLS="$ALL_FREE" ;;
    --*)          warn "Unknown flag: $arg" ;;
    *)            TOOLS="$TOOLS $arg" ;;
  esac
done

TOOLS=$(echo "$TOOLS" | xargs)  # trim whitespace
if [ -z "$TOOLS" ]; then
  printf "\n${CYAN}  Stockyard Multi-Tool Installer${NC}\n\n"
  echo "  Usage: curl -fsSL stockyard.dev/install-tools.sh | sh -s -- [tools...]"
  echo ""
  echo "  Examples:"
  echo "    sh -s -- bounty headcount strongbox    # Install specific tools"
  echo "    sh -s -- --starter                     # Bug tracker, analytics, secrets, flags, monitoring"
  echo "    sh -s -- --devtools                    # Full developer toolkit (8 tools)"
  echo "    sh -s -- --ops                         # Operations bundle (8 tools)"
  echo "    sh -s -- --finance                     # Finance bundle (6 tools)"
  echo "    sh -s -- --all-free                    # All popular tools (13 tools)"
  echo ""
  echo "  All tools: stockyard.dev/tools"
  echo ""
  exit 0
fi

info "Detected ${OS}/${ARCH}"
COUNT=$(echo "$TOOLS" | wc -w | tr -d ' ')
info "Installing $COUNT tools..."
echo ""

INSTALLED=0
FAILED=0

for TOOL in $TOOLS; do
  REPO="stockyard-dev/stockyard-${TOOL}"
  BINARY="stockyard-${TOOL}"

  printf "  ${CYAN}%-20s${NC} " "$TOOL"

  TMP=$(mktemp -d)
  URL="https://github.com/${REPO}/releases/latest/download/${BINARY}_${OS}_${ARCH}.tar.gz"

  if curl -fsSL "$URL" -o "${TMP}/archive.tar.gz" 2>/dev/null; then
    tar -xzf "${TMP}/archive.tar.gz" -C "$TMP" 2>/dev/null
    if [ -f "${TMP}/${BINARY}" ]; then
      if [ -w "$INSTALL_DIR" ]; then
        mv "${TMP}/${BINARY}" "${INSTALL_DIR}/${BINARY}"
      else
        sudo mv "${TMP}/${BINARY}" "${INSTALL_DIR}/${BINARY}" 2>/dev/null
      fi
      chmod +x "${INSTALL_DIR}/${BINARY}"
      printf "${GREEN}installed${NC}\n"
      INSTALLED=$((INSTALLED + 1))
    else
      # Try building from source
      if command -v go >/dev/null 2>&1; then
        printf "${GOLD}building...${NC}"
        if CGO_ENABLED=0 GOBIN="$INSTALL_DIR" go install "github.com/${REPO}/cmd/${TOOL}@latest" 2>/dev/null; then
          printf "\r  ${CYAN}%-20s${NC} ${GREEN}built${NC}    \n" "$TOOL"
          INSTALLED=$((INSTALLED + 1))
        else
          printf "\r  ${CYAN}%-20s${NC} ${RED}failed${NC}    \n" "$TOOL"
          FAILED=$((FAILED + 1))
        fi
      else
        printf "${RED}no release found${NC}\n"
        FAILED=$((FAILED + 1))
      fi
    fi
  else
    # No release — try go install
    if command -v go >/dev/null 2>&1; then
      printf "${GOLD}building...${NC}"
      if CGO_ENABLED=0 GOBIN="$INSTALL_DIR" go install "github.com/${REPO}/cmd/${TOOL}@latest" 2>/dev/null; then
        printf "\r  ${CYAN}%-20s${NC} ${GREEN}built${NC}    \n" "$TOOL"
        INSTALLED=$((INSTALLED + 1))
      else
        printf "\r  ${CYAN}%-20s${NC} ${RED}failed${NC}    \n" "$TOOL"
        FAILED=$((FAILED + 1))
      fi
    else
      printf "${RED}failed${NC}\n"
      FAILED=$((FAILED + 1))
    fi
  fi

  rm -rf "$TMP"
done

echo ""
info "${INSTALLED}/${COUNT} tools installed"
if [ "$FAILED" -gt 0 ]; then
  warn "${FAILED} tools failed (may need Go to build from source)"
fi
echo ""
echo "  Start any tool:  stockyard-bounty"
echo "  Dashboard:       http://localhost:PORT/ui"
echo "  All tools:       stockyard.dev/tools"
echo ""
