package storage

// Register the pure-Go SQLite driver so sql.Open("sqlite", ...) works at runtime.
// Without this import, storage.Open() panics: "sql: unknown driver 'sqlite'"
//
// This uses modernc.org/sqlite — a pure-Go SQLite implementation.
// No CGO required. Works on all platforms.
//
// After cloning, run: ./bootstrap.sh (or: go get modernc.org/sqlite && go mod tidy)
import _ "modernc.org/sqlite"
