#!/usr/bin/env bash
set -euo pipefail

# bootstrap.sh — Run once after cloning to set up the Stockyard repo.
# Fixes dependencies, verifies build, runs tests.

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

info()  { echo -e "${GREEN}[ok]${NC} $1"; }
warn()  { echo -e "${YELLOW}[!!]${NC} $1"; }
fail()  { echo -e "${RED}[FAIL]${NC} $1"; exit 1; }

echo "Stockyard Bootstrap"
echo "==================="
echo ""

# Check Go version
if ! command -v go &>/dev/null; then
    fail "Go is not installed. Install Go 1.22+ from https://go.dev/dl/"
fi
GO_VERSION=$(go version | grep -oP 'go\d+\.\d+')
info "Go version: $GO_VERSION"

# 1. Add SQLite driver dependency
echo ""
echo "1. Fixing dependencies..."

# Ensure the driver registration file exists
DRIVER_FILE="internal/storage/driver_sqlite.go"
if [ ! -f "$DRIVER_FILE" ]; then
    cat > "$DRIVER_FILE" << 'GOEOF'
package storage

// Register the pure-Go SQLite driver.
// This blank import ensures the "sqlite" driver is available to database/sql.
// Used by db.go: sql.Open("sqlite", dbPath)
import _ "modernc.org/sqlite"
GOEOF
    info "Created $DRIVER_FILE"
else
    info "$DRIVER_FILE already exists"
fi

# Add modernc.org/sqlite to go.mod if missing
if ! grep -q 'modernc.org/sqlite' go.mod; then
    go get modernc.org/sqlite@latest
    info "Added modernc.org/sqlite to go.mod"
else
    info "modernc.org/sqlite already in go.mod"
fi

# Tidy
go mod tidy
info "go mod tidy complete"

# 2. Verify build
echo ""
echo "2. Verifying build..."

if CGO_ENABLED=0 go build ./...; then
    info "All packages compile (CGO_ENABLED=0)"
else
    fail "Build failed"
fi

# 3. Run tests
echo ""
echo "3. Running tests..."

if go test ./internal/... -count=1 -timeout 120s; then
    info "All tests pass"
else
    warn "Some tests failed (see output above)"
fi

# 4. Verify SQLite works at runtime
echo ""
echo "4. Verifying SQLite..."

TMPDB=$(mktemp /tmp/stockyard-test-XXXXXX.db)
go run -tags '' -ldflags '' - <<'GOEOF' "$TMPDB"
package main

import (
    "database/sql"
    "fmt"
    "os"
    _ "modernc.org/sqlite"
)

func main() {
    db, err := sql.Open("sqlite", os.Args[1])
    if err != nil { fmt.Println("FAIL: open:", err); os.Exit(1) }
    defer db.Close()
    _, err = db.Exec("CREATE TABLE _bootstrap_test (id INTEGER PRIMARY KEY, msg TEXT)")
    if err != nil { fmt.Println("FAIL: create:", err); os.Exit(1) }
    _, err = db.Exec("INSERT INTO _bootstrap_test (msg) VALUES ('bootstrap ok')")
    if err != nil { fmt.Println("FAIL: insert:", err); os.Exit(1) }
    var msg string
    db.QueryRow("SELECT msg FROM _bootstrap_test WHERE id=1").Scan(&msg)
    if msg != "bootstrap ok" { fmt.Println("FAIL: read:", msg); os.Exit(1) }
    fmt.Println("SQLite: OK")
}
GOEOF
rm -f "$TMPDB"
info "SQLite driver works (pure Go, no CGO)"

echo ""
echo "==================="
info "Bootstrap complete. Run 'make build' to compile all binaries."
