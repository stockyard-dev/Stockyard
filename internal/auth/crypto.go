// Package auth — provider key encryption at rest using AES-256-GCM.
//
// Provider API keys (OpenAI, Anthropic, etc.) are encrypted before being
// written to SQLite and decrypted only when needed for outbound API calls.
//
// Key derivation:
//   - If STOCKYARD_ENCRYPTION_KEY is set, derive a 256-bit key via PBKDF2-HMAC-SHA256 (100k iterations).
//   - Otherwise, auto-generate a random 32-byte key and persist it in the
//     stockyard_secrets table so it survives restarts.
//
// Ciphertext format: base64(nonce || ciphertext || tag)
// Prefix: "enc:" to distinguish from legacy plaintext values.
package auth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
)

const encPrefix = "enc:"

// initEncryptionKey returns a 32-byte AES-256 key.
// Priority: STOCKYARD_ENCRYPTION_KEY env var → stored key in DB → auto-generate.
func initEncryptionKey(db *sql.DB) ([]byte, error) {
	// Ensure secrets table exists
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS stockyard_secrets (
		name  TEXT PRIMARY KEY,
		value TEXT NOT NULL
	)`)
	if err != nil {
		return nil, fmt.Errorf("create secrets table: %w", err)
	}

	// 1. Check env var
	if envKey := strings.TrimSpace(os.Getenv("STOCKYARD_ENCRYPTION_KEY")); envKey != "" {
		if len(envKey) < 32 {
			return nil, fmt.Errorf("STOCKYARD_ENCRYPTION_KEY must be at least 32 characters (got %d)", len(envKey))
		}
		// Derive a 256-bit key using PBKDF2-HMAC-SHA256 (100k iterations).
		salt := []byte("stockyard-encryption-key-v2")
		key := deriveKey([]byte(envKey), salt, 100000)
		log.Println("[auth] encryption key derived from STOCKYARD_ENCRYPTION_KEY (PBKDF2, 100k iterations)")
		return key, nil
	}

	// 2. Check stored key
	var stored string
	err = db.QueryRow(`SELECT value FROM stockyard_secrets WHERE name = 'encryption_key'`).Scan(&stored)
	if err == nil && stored != "" {
		key, decErr := hex.DecodeString(stored)
		if decErr == nil && len(key) == 32 {
			log.Println("[auth] encryption key loaded from database")
			return key, nil
		}
	}

	// 3. Auto-generate and persist
	key := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, fmt.Errorf("generate encryption key: %w", err)
	}
	hexKey := hex.EncodeToString(key)
	_, err = db.Exec(
		`INSERT INTO stockyard_secrets (name, value) VALUES ('encryption_key', ?)
		 ON CONFLICT(name) DO UPDATE SET value = excluded.value`, hexKey)
	if err != nil {
		return nil, fmt.Errorf("persist encryption key: %w", err)
	}
	log.Println("[auth] encryption key auto-generated and stored")
	return key, nil
}

// deriveKey implements PBKDF2-HMAC-SHA256 using only Go stdlib.
// Returns a 32-byte key suitable for AES-256.
func deriveKey(password, salt []byte, iterations int) []byte {
	mac := hmac.New(sha256.New, password)
	mac.Write(salt)
	mac.Write([]byte{0, 0, 0, 1}) // block index 1
	u := mac.Sum(nil)
	result := make([]byte, sha256.Size)
	copy(result, u)
	for i := 1; i < iterations; i++ {
		mac.Reset()
		mac.Write(u)
		u = mac.Sum(nil)
		for j := range result {
			result[j] ^= u[j]
		}
	}
	return result[:32]
}

// encrypt encrypts plaintext using AES-256-GCM and returns "enc:" + base64.
func encrypt(plaintext string, key []byte) (string, error) {
	if plaintext == "" {
		return "", nil
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return encPrefix + base64.StdEncoding.EncodeToString(ciphertext), nil
}

// decrypt decrypts an "enc:"-prefixed ciphertext. If the value has no prefix
// (legacy plaintext), it is returned as-is.
func decrypt(ciphertext string, key []byte) (string, error) {
	if ciphertext == "" {
		return "", nil
	}
	if !strings.HasPrefix(ciphertext, encPrefix) {
		// Legacy plaintext — return as-is but log a warning.
		// These will be encrypted on next write or startup migration.
		log.Println("[auth] WARNING: plaintext provider key detected — will be encrypted on next write")
		return ciphertext, nil
	}
	data, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(ciphertext, encPrefix))
	if err != nil {
		return "", fmt.Errorf("decode ciphertext: %w", err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", errors.New("ciphertext too short")
	}
	plaintext, err := gcm.Open(nil, data[:nonceSize], data[nonceSize:], nil)
	if err != nil {
		return "", fmt.Errorf("decrypt: %w", err)
	}
	return string(plaintext), nil
}

// migrateEncryptExistingKeys encrypts any plaintext provider keys in the database.
func migrateEncryptExistingKeys(db *sql.DB, key []byte) error {
	rows, err := db.Query(`SELECT id, api_key FROM user_provider_keys`)
	if err != nil {
		return nil // table might not exist yet
	}
	defer rows.Close()

	type row struct {
		id     int64
		apiKey string
	}
	var toMigrate []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.id, &r.apiKey); err != nil {
			return err
		}
		if r.apiKey != "" && !strings.HasPrefix(r.apiKey, encPrefix) {
			toMigrate = append(toMigrate, r)
		}
	}
	if len(toMigrate) == 0 {
		return nil
	}

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, r := range toMigrate {
		encrypted, err := encrypt(r.apiKey, key)
		if err != nil {
			return fmt.Errorf("encrypt key id=%d: %w", r.id, err)
		}
		if _, err := tx.Exec(`UPDATE user_provider_keys SET api_key = ? WHERE id = ?`, encrypted, r.id); err != nil {
			return fmt.Errorf("update key id=%d: %w", r.id, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	log.Printf("[auth] migrated %d provider keys to AES-256-GCM encryption", len(toMigrate))
	return nil
}
