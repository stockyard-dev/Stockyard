// Package storage manages SQLite persistence for all Stockyard data.
package storage

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
)

// DB wraps a SQLite database connection.
type DB struct {
	conn    *sql.DB
	dataDir string
}

// Open creates or opens a SQLite database in the given data directory.
func Open(dataDir string) (*DB, error) {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}

	dbPath := filepath.Join(dataDir, "stockyard.db")
	conn, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	// Enable WAL mode for better concurrent read/write performance
	if _, err := conn.Exec("PRAGMA journal_mode=WAL"); err != nil {
		return nil, fmt.Errorf("set WAL mode: %w", err)
	}
	// Set busy timeout to avoid "database is locked" errors
	if _, err := conn.Exec("PRAGMA busy_timeout=5000"); err != nil {
		return nil, fmt.Errorf("set busy timeout: %w", err)
	}
	// Enforce foreign key constraints (off by default in SQLite)
	if _, err := conn.Exec("PRAGMA foreign_keys=ON"); err != nil {
		return nil, fmt.Errorf("enable foreign keys: %w", err)
	}

	db := &DB{conn: conn, dataDir: dataDir}

	// SQLite WAL mode: concurrent readers, serialized writers.
	// More than 1 connection allows concurrent reads while writes wait.
	// busy_timeout=5000 handles write contention gracefully.
	conn.SetMaxOpenConns(4)
	conn.SetMaxIdleConns(2)
	conn.SetConnMaxLifetime(0) // Don't close idle connections
	if err := db.migrate(); err != nil {
		return nil, fmt.Errorf("run migrations: %w", err)
	}

	log.Printf("database opened at %s", dbPath)
	return db, nil
}

// Close closes the database connection.
func (db *DB) Close() error {
	return db.conn.Close()
}

// Conn returns the underlying sql.DB for direct queries.
func (db *DB) Conn() *sql.DB {
	return db.conn
}
