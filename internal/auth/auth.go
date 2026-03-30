// Package auth provides user authentication, API key management, and provider key storage.
//
// Key format: sk-sy-{44 chars base64} (total ~50 chars)
// Storage: SHA-256 hash for lookup, prefix for display
// Provider keys: stored per-user for bring-your-own-key support
package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// ─── Schema ────────────────────────────────────────────────────────────────

const schema = `
CREATE TABLE IF NOT EXISTS users (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    email      TEXT UNIQUE NOT NULL,
    name       TEXT NOT NULL DEFAULT '',
    tier       TEXT NOT NULL DEFAULT 'free',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS api_keys (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id     INTEGER NOT NULL REFERENCES users(id),
    key_hash    TEXT UNIQUE NOT NULL,
    key_prefix  TEXT NOT NULL,
    name        TEXT NOT NULL DEFAULT 'default',
    scopes      TEXT NOT NULL DEFAULT '*',
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_used   DATETIME,
    revoked_at  DATETIME,
    UNIQUE(user_id, name)
);

CREATE INDEX IF NOT EXISTS idx_api_keys_hash ON api_keys(key_hash);
CREATE INDEX IF NOT EXISTS idx_api_keys_user ON api_keys(user_id);

CREATE TABLE IF NOT EXISTS user_provider_keys (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id    INTEGER NOT NULL REFERENCES users(id),
    provider   TEXT NOT NULL,
    api_key    TEXT NOT NULL,
    base_url   TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(user_id, provider)
);

CREATE INDEX IF NOT EXISTS idx_upk_user ON user_provider_keys(user_id);
`

// ─── Types ─────────────────────────────────────────────────────────────────

// User represents a registered user.
type User struct {
	ID        int64  `json:"id"`
	Email     string `json:"email"`
	Name      string `json:"name"`
	Tier      string `json:"tier"`
	CreatedAt string `json:"created_at"`
}

// Team represents a named group that owns API keys.
type Team struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	Description string `json:"description"`
	CreatedBy   int64  `json:"created_by"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

// APIKey represents an issued API key (without the secret).
type APIKey struct {
	ID        int64   `json:"id"`
	UserID    int64   `json:"user_id"`
	TeamID    *int64  `json:"team_id,omitempty"`
	KeyPrefix string  `json:"key_prefix"`
	Name      string  `json:"name"`
	Scopes    string  `json:"scopes"`
	CreatedAt string  `json:"created_at"`
	LastUsed  *string `json:"last_used,omitempty"`
	RevokedAt *string `json:"revoked_at,omitempty"`
}

// APIKeyWithSecret is returned only at creation time.
type APIKeyWithSecret struct {
	APIKey
	Key string `json:"key"`
}

// ProviderKey represents a user's stored provider credentials.
type ProviderKey struct {
	ID        int64  `json:"id"`
	UserID    int64  `json:"user_id"`
	Provider  string `json:"provider"`
	BaseURL   string `json:"base_url,omitempty"`
	CreatedAt string `json:"created_at"`
	// api_key is never returned in JSON
}

// ─── Store ─────────────────────────────────────────────────────────────────

// Store manages users, API keys, and provider keys.
// cachedKeyEntry stores a validated API key result for the hot path.
type cachedKeyEntry struct {
	user    *User
	apiKey  *APIKey
	expires time.Time
}

type Store struct {
	db       *sql.DB
	encKey   []byte    // AES-256 key for provider key encryption at rest
	keyCache sync.Map  // map[keyHash]cachedKeyEntry — validated key cache
}

// NewStore creates a new auth store and runs migrations.
func NewStore(db *sql.DB) (*Store, error) {
	s := &Store{db: db}
	if _, err := db.Exec(schema); err != nil {
		return nil, fmt.Errorf("auth migration: %w", err)
	}

	// Initialize AES-256-GCM encryption for provider keys at rest
	encKey, err := initEncryptionKey(db)
	if err != nil {
		return nil, fmt.Errorf("encryption init: %w", err)
	}
	s.encKey = encKey

	// Migrate any existing plaintext provider keys to encrypted
	if err := migrateEncryptExistingKeys(db, encKey); err != nil {
		log.Printf("[auth] warning: provider key migration failed: %v", err)
	}

	log.Println("[auth] migrations applied (provider keys encrypted at rest)")
	return s, nil
}

// ─── User Operations ───────────────────────────────────────────────────────

// CreateUser creates a new user and returns it.
func (s *Store) CreateUser(email, name string) (*User, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	if email == "" {
		return nil, errors.New("email is required")
	}
	if !strings.Contains(email, "@") || !strings.Contains(email, ".") {
		return nil, errors.New("invalid email format")
	}
	if len(email) > 254 {
		return nil, errors.New("email too long")
	}
	if name == "" {
		name = strings.Split(email, "@")[0]
	}
	res, err := s.db.Exec(
		`INSERT INTO users (email, name) VALUES (?, ?)`,
		email, name,
	)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return nil, fmt.Errorf("account already exists")
		}
		return nil, err
	}
	id, _ := res.LastInsertId()
	return s.GetUser(id)
}

// GetUser returns a user by ID.
func (s *Store) GetUser(id int64) (*User, error) {
	u := &User{}
	err := s.db.QueryRow(
		`SELECT id, email, name, tier, created_at FROM users WHERE id = ?`, id,
	).Scan(&u.ID, &u.Email, &u.Name, &u.Tier, &u.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, errors.New("user not found")
	}
	return u, err
}

// GetUserByEmail returns a user by email.
func (s *Store) GetUserByEmail(email string) (*User, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	u := &User{}
	err := s.db.QueryRow(
		`SELECT id, email, name, tier, created_at FROM users WHERE email = ?`, email,
	).Scan(&u.ID, &u.Email, &u.Name, &u.Tier, &u.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, errors.New("user not found")
	}
	return u, err
}

// ListUsers returns all users.
// CountUsers returns the total number of users.
func (s *Store) CountUsers() int {
	var count int
	s.db.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
	return count
}

func (s *Store) ListUsers() ([]User, error) {
	rows, err := s.db.Query(`SELECT id, email, name, tier, created_at FROM users ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var users []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Email, &u.Name, &u.Tier, &u.CreatedAt); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	if err := rows.Err(); err != nil {
		log.Printf("[db] rows iteration error: %v", err)
	}
	return users, nil
}

// UpdateUserTier updates a user's tier.
func (s *Store) UpdateUserTier(id int64, tier string) error {
	_, err := s.db.Exec(`UPDATE users SET tier = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, tier, id)
	return err
}

// UpdateUserTierByEmail updates a user's tier by email address.
// Satisfies the apiserver.AuthTierUpdater interface.
func (s *Store) UpdateUserTierByEmail(email, tier string) error {
	email = strings.TrimSpace(strings.ToLower(email))
	res, err := s.db.Exec(`UPDATE users SET tier = ?, updated_at = CURRENT_TIMESTAMP WHERE email = ?`, tier, email)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("no user found with email %s", email)
	}
	return nil
}

// ─── API Key Operations ────────────────────────────────────────────────────

// GenerateKey creates a new API key for a user. Returns the full key only once.
func (s *Store) GenerateKey(userID int64, name string) (*APIKeyWithSecret, error) {
	if name == "" {
		name = "default"
	}

	// Generate random key: sk-sy-{32 random bytes base64}
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return nil, fmt.Errorf("generate key: %w", err)
	}
	key := "sk-sy-" + base64.RawURLEncoding.EncodeToString(raw)

	// Hash for storage
	hash := hashKey(key)
	prefix := key[:12] + "..." + key[len(key)-4:]

	res, err := s.db.Exec(
		`INSERT INTO api_keys (user_id, key_hash, key_prefix, name) VALUES (?, ?, ?, ?)`,
		userID, hash, prefix, name,
	)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return nil, fmt.Errorf("key with name %q already exists for this user", name)
		}
		return nil, err
	}
	id, _ := res.LastInsertId()

	return &APIKeyWithSecret{
		APIKey: APIKey{
			ID:        id,
			UserID:    userID,
			KeyPrefix: prefix,
			Name:      name,
			Scopes:    "*",
			CreatedAt: time.Now().UTC().Format(time.RFC3339),
		},
		Key: key,
	}, nil
}

// ValidateKey checks an API key and returns the associated user.
// Returns nil, nil if the key is invalid/revoked.
func (s *Store) ValidateKey(key string) (*User, *APIKey, error) {
	if !strings.HasPrefix(key, "sk-sy-") {
		return nil, nil, nil
	}

	hash := hashKey(key)

	// Cache hit — skip both DB queries on the hot path
	if cached, ok := s.keyCache.Load(hash); ok {
		entry := cached.(cachedKeyEntry)
		if time.Now().Before(entry.expires) {
			return entry.user, entry.apiKey, nil
		}
		s.keyCache.Delete(hash)
	}

	// Cache miss — hit the DB
	var ak APIKey
	var uid int64
	var revokedAt sql.NullString
	var lastUsed sql.NullString
	var teamID sql.NullInt64

	err := s.db.QueryRow(
		`SELECT id, user_id, key_prefix, name, scopes, created_at, last_used, revoked_at, team_id
		 FROM api_keys WHERE key_hash = ?`, hash,
	).Scan(&ak.ID, &uid, &ak.KeyPrefix, &ak.Name, &ak.Scopes, &ak.CreatedAt, &lastUsed, &revokedAt, &teamID)

	if err == sql.ErrNoRows {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}

	// Key is revoked
	if revokedAt.Valid {
		return nil, nil, nil
	}

	ak.UserID = uid
	if lastUsed.Valid {
		ak.LastUsed = &lastUsed.String
	}
	if teamID.Valid {
		ak.TeamID = &teamID.Int64
	}

	// Update last_used (fire-and-forget)
	go func() {
		s.db.Exec(`UPDATE api_keys SET last_used = CURRENT_TIMESTAMP WHERE id = ?`, ak.ID)
	}()

	// Look up user
	user, err := s.GetUser(uid)
	if err != nil {
		return nil, nil, err
	}

	// Cache for 30 seconds — balances hot-path speed vs revocation latency
	s.keyCache.Store(hash, cachedKeyEntry{
		user:    user,
		apiKey:  &ak,
		expires: time.Now().Add(30 * time.Second),
	})

	return user, &ak, nil
}

// InvalidateKeyCache removes a key from the validation cache.
// Called on key revocation to ensure revoked keys aren't served from cache.
func (s *Store) InvalidateKeyCache(key string) {
	if strings.HasPrefix(key, "sk-sy-") {
		s.keyCache.Delete(hashKey(key))
	}
}

// InvalidateAllKeyCache clears the entire key validation cache.
func (s *Store) InvalidateAllKeyCache() {
	s.keyCache.Range(func(k, v any) bool {
		s.keyCache.Delete(k)
		return true
	})
}

// ListKeys returns all (non-revoked) API keys for a user.
func (s *Store) ListKeys(userID int64) ([]APIKey, error) {
	rows, err := s.db.Query(
		`SELECT id, user_id, key_prefix, name, scopes, created_at, last_used, revoked_at, team_id
		 FROM api_keys WHERE user_id = ? AND revoked_at IS NULL ORDER BY id`, userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var keys []APIKey
	for rows.Next() {
		var ak APIKey
		var lastUsed, revokedAt sql.NullString
		var teamID sql.NullInt64
		if err := rows.Scan(&ak.ID, &ak.UserID, &ak.KeyPrefix, &ak.Name, &ak.Scopes,
			&ak.CreatedAt, &lastUsed, &revokedAt, &teamID); err != nil {
			return nil, err
		}
		if lastUsed.Valid {
			ak.LastUsed = &lastUsed.String
		}
		if teamID.Valid {
			ak.TeamID = &teamID.Int64
		}
		keys = append(keys, ak)
	}
	if err := rows.Err(); err != nil {
		log.Printf("[db] rows iteration error: %v", err)
	}
	return keys, nil
}

// RevokeKey marks an API key as revoked.
func (s *Store) RevokeKey(userID int64, keyID int64) error {
	res, err := s.db.Exec(
		`UPDATE api_keys SET revoked_at = CURRENT_TIMESTAMP WHERE id = ? AND user_id = ? AND revoked_at IS NULL`,
		keyID, userID,
	)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return errors.New("key not found or already revoked")
	}
	// Flush validation cache — revoked key must not be served from cache.
	// Full flush is fine since revocations are rare.
	s.InvalidateAllKeyCache()
	return nil
}

// RotateKey atomically revokes the old key and generates a new one with the same name.
// Returns the new key (with secret). The old key is immediately invalid.
func (s *Store) RotateKey(userID int64, keyID int64) (*APIKeyWithSecret, error) {
	// Look up old key name
	var name string
	err := s.db.QueryRow(
		`SELECT name FROM api_keys WHERE id = ? AND user_id = ? AND revoked_at IS NULL`,
		keyID, userID,
	).Scan(&name)
	if err == sql.ErrNoRows {
		return nil, errors.New("key not found or already revoked")
	}
	if err != nil {
		return nil, err
	}

	// Revoke the old key
	_, err = s.db.Exec(
		`UPDATE api_keys SET revoked_at = CURRENT_TIMESTAMP WHERE id = ? AND user_id = ?`,
		keyID, userID,
	)
	if err != nil {
		return nil, fmt.Errorf("revoke old key: %w", err)
	}

	// Generate a new key with the same name + " (rotated)" suffix if name conflict
	newKey, err := s.GenerateKey(userID, name)
	if err != nil {
		// Name conflict — try with suffix
		newKey, err = s.GenerateKey(userID, name+" (rotated)")
		if err != nil {
			return nil, fmt.Errorf("generate new key: %w", err)
		}
	}

	return newKey, nil
}

// ─── Team Operations ───────────────────────────────────────────────────────

// slugify converts a team name to a URL-safe slug.
func slugify(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	// Replace non-alphanumeric with hyphens
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		} else if r == ' ' || r == '-' || r == '_' {
			b.WriteRune('-')
		}
	}
	// Collapse multiple hyphens
	result := b.String()
	for strings.Contains(result, "--") {
		result = strings.ReplaceAll(result, "--", "-")
	}
	return strings.Trim(result, "-")
}

// CreateTeam creates a new team.
func (s *Store) CreateTeam(name, description string, createdBy int64) (*Team, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, errors.New("team name is required")
	}
	if len(name) > 100 {
		return nil, errors.New("team name too long (max 100 chars)")
	}
	slug := slugify(name)
	if slug == "" {
		return nil, errors.New("team name must contain at least one alphanumeric character")
	}

	res, err := s.db.Exec(
		`INSERT INTO teams (name, slug, description, created_by) VALUES (?, ?, ?, ?)`,
		name, slug, description, createdBy,
	)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return nil, fmt.Errorf("team %q already exists", name)
		}
		return nil, err
	}
	id, _ := res.LastInsertId()
	return s.GetTeam(id)
}

// GetTeam returns a team by ID.
func (s *Store) GetTeam(id int64) (*Team, error) {
	t := &Team{}
	err := s.db.QueryRow(
		`SELECT id, name, slug, description, created_by, created_at, updated_at FROM teams WHERE id = ?`, id,
	).Scan(&t.ID, &t.Name, &t.Slug, &t.Description, &t.CreatedBy, &t.CreatedAt, &t.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, errors.New("team not found")
	}
	return t, err
}

// GetTeamBySlug returns a team by slug.
func (s *Store) GetTeamBySlug(slug string) (*Team, error) {
	t := &Team{}
	err := s.db.QueryRow(
		`SELECT id, name, slug, description, created_by, created_at, updated_at FROM teams WHERE slug = ?`, slug,
	).Scan(&t.ID, &t.Name, &t.Slug, &t.Description, &t.CreatedBy, &t.CreatedAt, &t.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, errors.New("team not found")
	}
	return t, err
}

// ListTeams returns all teams.
func (s *Store) ListTeams() ([]Team, error) {
	rows, err := s.db.Query(`SELECT id, name, slug, description, created_by, created_at, updated_at FROM teams ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var teams []Team
	for rows.Next() {
		var t Team
		if err := rows.Scan(&t.ID, &t.Name, &t.Slug, &t.Description, &t.CreatedBy, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		teams = append(teams, t)
	}
	if err := rows.Err(); err != nil {
		log.Printf("[db] rows iteration error: %v", err)
	}
	return teams, nil
}

// UpdateTeam updates a team's name and/or description.
func (s *Store) UpdateTeam(id int64, name, description *string) (*Team, error) {
	if name != nil {
		n := strings.TrimSpace(*name)
		if n == "" {
			return nil, errors.New("team name cannot be empty")
		}
		slug := slugify(n)
		_, err := s.db.Exec(`UPDATE teams SET name = ?, slug = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, n, slug, id)
		if err != nil {
			if strings.Contains(err.Error(), "UNIQUE") {
				return nil, fmt.Errorf("team name %q already exists", n)
			}
			return nil, err
		}
	}
	if description != nil {
		s.db.Exec(`UPDATE teams SET description = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, *description, id)
	}
	return s.GetTeam(id)
}

// DeleteTeam deletes a team after revoking all its keys.
func (s *Store) DeleteTeam(id int64) error {
	// Revoke all team keys first
	s.db.Exec(`UPDATE api_keys SET revoked_at = CURRENT_TIMESTAMP WHERE team_id = ? AND revoked_at IS NULL`, id)
	s.InvalidateAllKeyCache()

	res, err := s.db.Exec(`DELETE FROM teams WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return errors.New("team not found")
	}
	return nil
}

// GenerateTeamKey creates an API key scoped to a team.
func (s *Store) GenerateTeamKey(teamID, userID int64, name string) (*APIKeyWithSecret, error) {
	if name == "" {
		name = "default"
	}

	// Verify team exists
	if _, err := s.GetTeam(teamID); err != nil {
		return nil, err
	}

	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return nil, fmt.Errorf("generate key: %w", err)
	}
	key := "sk-sy-" + base64.RawURLEncoding.EncodeToString(raw)
	hash := hashKey(key)
	prefix := key[:12] + "..." + key[len(key)-4:]

	res, err := s.db.Exec(
		`INSERT INTO api_keys (user_id, key_hash, key_prefix, name, team_id) VALUES (?, ?, ?, ?, ?)`,
		userID, hash, prefix, name, teamID,
	)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return nil, fmt.Errorf("key with name %q already exists", name)
		}
		return nil, err
	}
	id, _ := res.LastInsertId()
	tid := teamID

	return &APIKeyWithSecret{
		APIKey: APIKey{
			ID:        id,
			UserID:    userID,
			TeamID:    &tid,
			KeyPrefix: prefix,
			Name:      name,
			Scopes:    "*",
			CreatedAt: time.Now().UTC().Format(time.RFC3339),
		},
		Key: key,
	}, nil
}

// ListTeamKeys returns all non-revoked API keys for a team.
func (s *Store) ListTeamKeys(teamID int64) ([]APIKey, error) {
	rows, err := s.db.Query(
		`SELECT id, user_id, key_prefix, name, scopes, created_at, last_used, team_id
		 FROM api_keys WHERE team_id = ? AND revoked_at IS NULL ORDER BY id`, teamID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var keys []APIKey
	for rows.Next() {
		var ak APIKey
		var lastUsed sql.NullString
		var tid sql.NullInt64
		if err := rows.Scan(&ak.ID, &ak.UserID, &ak.KeyPrefix, &ak.Name, &ak.Scopes,
			&ak.CreatedAt, &lastUsed, &tid); err != nil {
			return nil, err
		}
		if lastUsed.Valid {
			ak.LastUsed = &lastUsed.String
		}
		if tid.Valid {
			ak.TeamID = &tid.Int64
		}
		keys = append(keys, ak)
	}
	if err := rows.Err(); err != nil {
		log.Printf("[db] rows iteration error: %v", err)
	}
	return keys, nil
}

// RevokeTeamKey revokes a key belonging to a team.
func (s *Store) RevokeTeamKey(teamID, keyID int64) error {
	res, err := s.db.Exec(
		`UPDATE api_keys SET revoked_at = CURRENT_TIMESTAMP WHERE id = ? AND team_id = ? AND revoked_at IS NULL`,
		keyID, teamID,
	)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return errors.New("key not found or already revoked")
	}
	s.InvalidateAllKeyCache()
	return nil
}

// RotateTeamKey revokes an old team key and generates a new one.
func (s *Store) RotateTeamKey(teamID, keyID int64) (*APIKeyWithSecret, error) {
	var name string
	var userID int64
	err := s.db.QueryRow(
		`SELECT name, user_id FROM api_keys WHERE id = ? AND team_id = ? AND revoked_at IS NULL`,
		keyID, teamID,
	).Scan(&name, &userID)
	if err == sql.ErrNoRows {
		return nil, errors.New("key not found or already revoked")
	}
	if err != nil {
		return nil, err
	}

	s.db.Exec(`UPDATE api_keys SET revoked_at = CURRENT_TIMESTAMP WHERE id = ?`, keyID)
	s.InvalidateAllKeyCache()

	newKey, err := s.GenerateTeamKey(teamID, userID, name)
	if err != nil {
		newKey, err = s.GenerateTeamKey(teamID, userID, name+" (rotated)")
		if err != nil {
			return nil, fmt.Errorf("generate new key: %w", err)
		}
	}
	return newKey, nil
}

// ─── Provider Key Operations ───────────────────────────────────────────────

// validateBaseURL checks that a provider base URL is safe (no SSRF to internal services).
func validateBaseURL(baseURL string) error {
	if baseURL == "" {
		return nil
	}
	u, err := url.Parse(baseURL)
	if err != nil {
		return fmt.Errorf("invalid base URL: %w", err)
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return fmt.Errorf("base URL must use http or https scheme")
	}
	host := u.Hostname()
	if host == "" {
		return errors.New("base URL must include a hostname")
	}
	// Block known internal/metadata hostnames
	if host == "metadata.google.internal" {
		return fmt.Errorf("blocked internal hostname: %s", host)
	}
	ip := net.ParseIP(host)
	if ip != nil {
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
			return fmt.Errorf("base URL must not target private/internal IP: %s", ip)
		}
	}
	return nil
}

// SetProviderKey stores or updates a user's provider API key (encrypted at rest).
func (s *Store) SetProviderKey(userID int64, providerName, apiKey, baseURL string) error {
	providerName = strings.TrimSpace(strings.ToLower(providerName))
	if providerName == "" || apiKey == "" {
		return errors.New("provider and api_key are required")
	}
	if err := validateBaseURL(baseURL); err != nil {
		return err
	}
	encrypted, err := encrypt(apiKey, s.encKey)
	if err != nil {
		return fmt.Errorf("encrypt provider key: %w", err)
	}
	_, err = s.db.Exec(
		`INSERT INTO user_provider_keys (user_id, provider, api_key, base_url)
		 VALUES (?, ?, ?, ?)
		 ON CONFLICT(user_id, provider) DO UPDATE SET api_key = excluded.api_key, base_url = excluded.base_url, updated_at = CURRENT_TIMESTAMP`,
		userID, providerName, encrypted, baseURL,
	)
	return err
}

// GetProviderKey returns a user's API key for a specific provider (decrypted).
func (s *Store) GetProviderKey(userID int64, providerName string) (apiKey, baseURL string, err error) {
	var encryptedKey string
	err = s.db.QueryRow(
		`SELECT api_key, base_url FROM user_provider_keys WHERE user_id = ? AND provider = ?`,
		userID, providerName,
	).Scan(&encryptedKey, &baseURL)
	if err == sql.ErrNoRows {
		return "", "", nil
	}
	if err != nil {
		return "", "", err
	}
	apiKey, err = decrypt(encryptedKey, s.encKey)
	if err != nil {
		return "", "", fmt.Errorf("decrypt provider key: %w", err)
	}
	return
}

// ListProviderKeys returns all provider keys for a user (without the actual key values).
func (s *Store) ListProviderKeys(userID int64) ([]ProviderKey, error) {
	rows, err := s.db.Query(
		`SELECT id, user_id, provider, base_url, created_at FROM user_provider_keys WHERE user_id = ? ORDER BY provider`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var keys []ProviderKey
	for rows.Next() {
		var pk ProviderKey
		if err := rows.Scan(&pk.ID, &pk.UserID, &pk.Provider, &pk.BaseURL, &pk.CreatedAt); err != nil {
			return nil, err
		}
		keys = append(keys, pk)
	}
	if err := rows.Err(); err != nil {
		log.Printf("[db] rows iteration error: %v", err)
	}
	return keys, nil
}

// DeleteProviderKey removes a user's provider key.
func (s *Store) DeleteProviderKey(userID int64, providerName string) error {
	res, err := s.db.Exec(
		`DELETE FROM user_provider_keys WHERE user_id = ? AND provider = ?`,
		userID, providerName,
	)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return errors.New("provider key not found")
	}
	return nil
}

// GetAllProviderKeys returns all provider keys for a user (with key values — internal use only).
func (s *Store) GetAllProviderKeys(userID int64) (map[string]ProviderKeyFull, error) {
	rows, err := s.db.Query(
		`SELECT provider, api_key, base_url FROM user_provider_keys WHERE user_id = ?`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	keys := make(map[string]ProviderKeyFull)
	for rows.Next() {
		var pk ProviderKeyFull
		var prov, encryptedKey string
		if err := rows.Scan(&prov, &encryptedKey, &pk.BaseURL); err != nil {
			return nil, err
		}
		decrypted, err := decrypt(encryptedKey, s.encKey)
		if err != nil {
			log.Printf("[auth] warning: failed to decrypt key for provider %s: %v", prov, err)
			continue
		}
		pk.APIKey = decrypted
		keys[prov] = pk
	}
	if err := rows.Err(); err != nil {
		log.Printf("[db] rows iteration error: %v", err)
	}
	return keys, nil
}

// ProviderKeyFull includes the actual API key (internal use only).
type ProviderKeyFull struct {
	APIKey  string
	BaseURL string
}

// ─── Helpers ───────────────────────────────────────────────────────────────

func hashKey(key string) string {
	h := sha256.Sum256([]byte(key))
	return hex.EncodeToString(h[:])
}

// ─── HTTP API ──────────────────────────────────────────────────────────────

// API provides HTTP handlers for user and key management.
type API struct {
	store       *Store
	licEnforcer interface {
		CheckUserLimit(currentUsers int) error
	}
}

// NewAPI creates a new auth API handler.
func NewAPI(store *Store) *API {
	return &API{store: store}
}

// SetLicenseEnforcer sets the license enforcer for user cap checks.
func (a *API) SetLicenseEnforcer(e interface{ CheckUserLimit(int) error }) {
	a.licEnforcer = e
}

// Register mounts auth API routes on the given mux.
func (a *API) Register(mux *http.ServeMux) {
	// Public signup
	mux.HandleFunc("POST /api/auth/signup", a.handleSignup)

	// User management (admin)
	mux.HandleFunc("POST /api/auth/users", a.handleCreateUser)
	mux.HandleFunc("GET /api/auth/users", a.handleListUsers)
	mux.HandleFunc("GET /api/auth/users/{id}", a.handleGetUser)

	// API key management (admin creates keys for users)
	mux.HandleFunc("POST /api/auth/users/{id}/keys", a.handleCreateKey)
	mux.HandleFunc("GET /api/auth/users/{id}/keys", a.handleListKeys)
	mux.HandleFunc("DELETE /api/auth/users/{id}/keys/{keyId}", a.handleRevokeKey)
	mux.HandleFunc("POST /api/auth/users/{id}/keys/{keyId}/rotate", a.handleRotateKey)

	// Provider key management
	mux.HandleFunc("PUT /api/auth/users/{id}/providers/{provider}", a.handleSetProviderKey)
	mux.HandleFunc("GET /api/auth/users/{id}/providers", a.handleListProviderKeys)
	mux.HandleFunc("DELETE /api/auth/users/{id}/providers/{provider}", a.handleDeleteProviderKey)

	// Self-service: current user info (via API key)
	mux.HandleFunc("GET /api/auth/me", a.handleMe)
	mux.HandleFunc("POST /api/auth/me/keys", a.handleCreateMyKey)
	mux.HandleFunc("GET /api/auth/me/keys", a.handleListMyKeys)
	mux.HandleFunc("DELETE /api/auth/me/keys/{keyId}", a.handleRevokeMyKey)
	mux.HandleFunc("POST /api/auth/me/keys/{keyId}/rotate", a.handleRotateMyKey)
	mux.HandleFunc("PUT /api/auth/me/providers/{provider}", a.handleSetMyProviderKey)
	mux.HandleFunc("GET /api/auth/me/providers", a.handleListMyProviderKeys)
	mux.HandleFunc("DELETE /api/auth/me/providers/{provider}", a.handleDeleteMyProviderKey)
	mux.HandleFunc("GET /api/auth/me/usage", a.handleMyUsage)

	// Team management (admin)
	mux.HandleFunc("POST /api/teams", a.handleCreateTeam)
	mux.HandleFunc("GET /api/teams", a.handleListTeams)
	mux.HandleFunc("GET /api/teams/{id}", a.handleGetTeam)
	mux.HandleFunc("PUT /api/teams/{id}", a.handleUpdateTeam)
	mux.HandleFunc("DELETE /api/teams/{id}", a.handleDeleteTeam)

	// Team key management (admin)
	mux.HandleFunc("POST /api/teams/{id}/keys", a.handleCreateTeamKey)
	mux.HandleFunc("GET /api/teams/{id}/keys", a.handleListTeamKeys)
	mux.HandleFunc("DELETE /api/teams/{id}/keys/{keyId}", a.handleRevokeTeamKey)
	mux.HandleFunc("POST /api/teams/{id}/keys/{keyId}/rotate", a.handleRotateTeamKey)

	// Team observability (admin)
	mux.HandleFunc("GET /api/teams/{id}/spend", a.handleTeamSpend)
	mux.HandleFunc("GET /api/teams/{id}/logs", a.handleTeamLogs)

	log.Println("[auth] API routes registered (including teams)")
}

// ─── Public signup ──────────────────────────────────────────────────────────

func (a *API) handleSignup(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email string `json:"email"`
		Name  string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid JSON"})
		return
	}

	// Validate email format
	body.Email = strings.TrimSpace(body.Email)
	if body.Email == "" || !strings.Contains(body.Email, "@") || !strings.Contains(body.Email, ".") || len(body.Email) > 254 {
		writeJSON(w, 400, map[string]string{"error": "invalid email format"})
		return
	}

	// Enforce user cap
	if a.licEnforcer != nil {
		if err := a.licEnforcer.CheckUserLimit(a.store.CountUsers()); err != nil {
			writeJSON(w, 402, map[string]string{
				"error": "user limit reached for current license tier",
				"type":  "license_limit",
			})
			return
		}
	}

	user, err := a.store.CreateUser(body.Email, body.Name)
	if err != nil {
		errMsg := err.Error()
		if strings.Contains(errMsg, "already exists") {
			writeJSON(w, 409, map[string]string{"error": "email already exists"})
		} else if strings.Contains(errMsg, "required") {
			writeJSON(w, 400, map[string]string{"error": "email is required"})
		} else {
			log.Printf("[auth] signup create user error: %v", err)
			writeJSON(w, 500, map[string]string{"error": "failed to create user"})
		}
		return
	}

	// Auto-generate first API key
	key, err := a.store.GenerateKey(user.ID, "default")
	if err != nil {
		log.Printf("[auth] signup key generation error for user %d: %v", user.ID, err)
		writeJSON(w, 201, map[string]any{"user": user, "error": "user created but key generation failed"})
		return
	}

	writeJSON(w, 201, map[string]any{
		"user":    user,
		"api_key": key,
		"usage":   "Set Authorization: Bearer " + key.Key + " on all requests to /v1/*",
	})
}

// ─── Admin handlers ────────────────────────────────────────────────────────

func (a *API) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email string `json:"email"`
		Name  string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid JSON"})
		return
	}

	// Enforce user cap
	if a.licEnforcer != nil {
		if err := a.licEnforcer.CheckUserLimit(a.store.CountUsers()); err != nil {
			writeJSON(w, 402, map[string]string{
				"error": "user limit reached for current license tier",
				"type":  "license_limit",
			})
			return
		}
	}

	user, err := a.store.CreateUser(body.Email, body.Name)
	if err != nil {
		errMsg := err.Error()
		if strings.Contains(errMsg, "already exists") {
			writeJSON(w, 409, map[string]string{"error": "email already exists"})
		} else if strings.Contains(errMsg, "required") {
			writeJSON(w, 400, map[string]string{"error": "email is required"})
		} else {
			log.Printf("[auth] admin create user error: %v", err)
			writeJSON(w, 500, map[string]string{"error": "failed to create user"})
		}
		return
	}

	// Auto-generate first API key
	key, err := a.store.GenerateKey(user.ID, "default")
	if err != nil {
		// User created but key failed — still return user
		log.Printf("[auth] admin key generation error for user %d: %v", user.ID, err)
		writeJSON(w, 201, map[string]any{"user": user, "error": "user created but key generation failed"})
		return
	}

	writeJSON(w, 201, map[string]any{
		"user":    user,
		"api_key": key,
	})
}

func (a *API) handleListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := a.store.ListUsers()
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	if users == nil {
		users = []User{}
	}
	writeJSON(w, 200, map[string]any{"users": users, "count": len(users)})
}

func (a *API) handleGetUser(w http.ResponseWriter, r *http.Request) {
	id := parseID(r.PathValue("id"))
	if id == 0 {
		writeJSON(w, 400, map[string]string{"error": "invalid user id"})
		return
	}
	user, err := a.store.GetUser(id)
	if err != nil {
		writeJSON(w, 404, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, user)
}

func (a *API) handleCreateKey(w http.ResponseWriter, r *http.Request) {
	uid := parseID(r.PathValue("id"))
	if uid == 0 {
		writeJSON(w, 400, map[string]string{"error": "invalid user id"})
		return
	}
	var body struct {
		Name string `json:"name"`
	}
	json.NewDecoder(r.Body).Decode(&body)

	key, err := a.store.GenerateKey(uid, body.Name)
	if err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 201, key)
}

func (a *API) handleListKeys(w http.ResponseWriter, r *http.Request) {
	uid := parseID(r.PathValue("id"))
	if uid == 0 {
		writeJSON(w, 400, map[string]string{"error": "invalid user id"})
		return
	}
	keys, err := a.store.ListKeys(uid)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	if keys == nil {
		keys = []APIKey{}
	}
	writeJSON(w, 200, map[string]any{"keys": keys, "count": len(keys)})
}

func (a *API) handleRevokeKey(w http.ResponseWriter, r *http.Request) {
	uid := parseID(r.PathValue("id"))
	kid := parseID(r.PathValue("keyId"))
	if uid == 0 || kid == 0 {
		writeJSON(w, 400, map[string]string{"error": "invalid ids"})
		return
	}
	if err := a.store.RevokeKey(uid, kid); err != nil {
		writeJSON(w, 404, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]string{"status": "revoked"})
}

func (a *API) handleRotateKey(w http.ResponseWriter, r *http.Request) {
	uid := parseID(r.PathValue("id"))
	kid := parseID(r.PathValue("keyId"))
	if uid == 0 || kid == 0 {
		writeJSON(w, 400, map[string]string{"error": "invalid ids"})
		return
	}
	newKey, err := a.store.RotateKey(uid, kid)
	if err != nil {
		writeJSON(w, 404, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]any{
		"status":  "rotated",
		"old_key": kid,
		"new_key": newKey,
	})
}

func (a *API) handleSetProviderKey(w http.ResponseWriter, r *http.Request) {
	uid := parseID(r.PathValue("id"))
	prov := r.PathValue("provider")
	if uid == 0 || prov == "" {
		writeJSON(w, 400, map[string]string{"error": "invalid parameters"})
		return
	}
	var body struct {
		APIKey  string `json:"api_key"`
		BaseURL string `json:"base_url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid JSON"})
		return
	}
	if err := a.store.SetProviderKey(uid, prov, body.APIKey, body.BaseURL); err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]string{"status": "saved", "provider": prov})
}

func (a *API) handleListProviderKeys(w http.ResponseWriter, r *http.Request) {
	uid := parseID(r.PathValue("id"))
	if uid == 0 {
		writeJSON(w, 400, map[string]string{"error": "invalid user id"})
		return
	}
	keys, err := a.store.ListProviderKeys(uid)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	if keys == nil {
		keys = []ProviderKey{}
	}
	writeJSON(w, 200, map[string]any{"providers": keys, "count": len(keys)})
}

func (a *API) handleDeleteProviderKey(w http.ResponseWriter, r *http.Request) {
	uid := parseID(r.PathValue("id"))
	prov := r.PathValue("provider")
	if uid == 0 || prov == "" {
		writeJSON(w, 400, map[string]string{"error": "invalid parameters"})
		return
	}
	if err := a.store.DeleteProviderKey(uid, prov); err != nil {
		writeJSON(w, 404, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]string{"status": "deleted"})
}

// ─── Self-service handlers (use API key to identify user) ──────────────────

func (a *API) handleMe(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	if user == nil {
		writeJSON(w, 401, map[string]string{"error": "not authenticated — send Authorization: Bearer sk-sy-..."})
		return
	}
	provKeys, _ := a.store.ListProviderKeys(user.ID)
	if provKeys == nil {
		provKeys = []ProviderKey{}
	}
	writeJSON(w, 200, map[string]any{
		"user":      user,
		"providers": provKeys,
	})
}

func (a *API) handleMyUsage(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	if user == nil {
		writeJSON(w, 401, map[string]string{"error": "not authenticated"})
		return
	}

	// If the caller's key belongs to a team, scope to that team
	teamScope := ""
	if team := TeamFromContext(r.Context()); team != nil {
		teamScope = fmt.Sprintf("%d", team.ID)
	}

	uid := fmt.Sprintf("%d", user.ID)

	// Build WHERE clause: user_id always, team_id if scoped
	where := "user_id = ?"
	wargs := []any{uid}
	if teamScope != "" {
		where += " AND team_id = ?"
		wargs = append(wargs, teamScope)
	}

	// Total usage
	var totalReqs int64
	var totalCost float64
	var totalTokensIn, totalTokensOut int64
	a.store.db.QueryRow(`SELECT COUNT(*), COALESCE(SUM(cost_usd),0), COALESCE(SUM(tokens_in),0), COALESCE(SUM(tokens_out),0) FROM requests WHERE `+where, wargs...).Scan(&totalReqs, &totalCost, &totalTokensIn, &totalTokensOut)

	// This month
	var monthReqs int64
	var monthCost float64
	a.store.db.QueryRow(`SELECT COUNT(*), COALESCE(SUM(cost_usd),0) FROM requests WHERE `+where+` AND timestamp >= date('now','start of month')`, wargs...).Scan(&monthReqs, &monthCost)

	// Today
	var todayReqs int64
	var todayCost float64
	a.store.db.QueryRow(`SELECT COUNT(*), COALESCE(SUM(cost_usd),0) FROM requests WHERE `+where+` AND timestamp >= date('now')`, wargs...).Scan(&todayReqs, &todayCost)

	// Top models
	type modelStat struct {
		Model string  `json:"model"`
		Reqs  int64   `json:"requests"`
		Cost  float64 `json:"cost_usd"`
	}
	var topModels []modelStat
	rows, _ := a.store.db.Query(`SELECT model, COUNT(*) as c, COALESCE(SUM(cost_usd),0) FROM requests WHERE `+where+` GROUP BY model ORDER BY c DESC LIMIT 5`, wargs...)
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var m modelStat
			if err := rows.Scan(&m.Model, &m.Reqs, &m.Cost); err != nil {
				continue
			}
			topModels = append(topModels, m)
		}
		if err := rows.Err(); err != nil {
			log.Printf("[db] rows iteration error: %v", err)
		}
	}
	if topModels == nil {
		topModels = []modelStat{}
	}

	result := map[string]any{
		"user_id": user.ID,
		"total": map[string]any{
			"requests":   totalReqs,
			"cost_usd":   totalCost,
			"tokens_in":  totalTokensIn,
			"tokens_out": totalTokensOut,
		},
		"this_month": map[string]any{
			"requests": monthReqs,
			"cost_usd": monthCost,
		},
		"today": map[string]any{
			"requests": todayReqs,
			"cost_usd": todayCost,
		},
		"top_models": topModels,
	}
	if teamScope != "" {
		result["team_id"] = teamScope
	}
	writeJSON(w, 200, result)
}

func (a *API) handleCreateMyKey(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	if user == nil {
		writeJSON(w, 401, map[string]string{"error": "not authenticated"})
		return
	}
	var body struct {
		Name string `json:"name"`
	}
	json.NewDecoder(r.Body).Decode(&body)

	key, err := a.store.GenerateKey(user.ID, body.Name)
	if err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 201, key)
}

func (a *API) handleListMyKeys(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	if user == nil {
		writeJSON(w, 401, map[string]string{"error": "not authenticated"})
		return
	}
	keys, err := a.store.ListKeys(user.ID)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	if keys == nil {
		keys = []APIKey{}
	}
	writeJSON(w, 200, map[string]any{"keys": keys, "count": len(keys)})
}

func (a *API) handleRevokeMyKey(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	if user == nil {
		writeJSON(w, 401, map[string]string{"error": "not authenticated"})
		return
	}
	kid := parseID(r.PathValue("keyId"))
	if kid == 0 {
		writeJSON(w, 400, map[string]string{"error": "invalid key id"})
		return
	}
	if err := a.store.RevokeKey(user.ID, kid); err != nil {
		writeJSON(w, 404, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]string{"status": "revoked"})
}

func (a *API) handleRotateMyKey(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	if user == nil {
		writeJSON(w, 401, map[string]string{"error": "not authenticated"})
		return
	}
	kid := parseID(r.PathValue("keyId"))
	if kid == 0 {
		writeJSON(w, 400, map[string]string{"error": "invalid key id"})
		return
	}
	newKey, err := a.store.RotateKey(user.ID, kid)
	if err != nil {
		writeJSON(w, 404, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]any{
		"status":  "rotated",
		"old_key": kid,
		"new_key": newKey,
	})
}

func (a *API) handleSetMyProviderKey(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	if user == nil {
		writeJSON(w, 401, map[string]string{"error": "not authenticated"})
		return
	}
	prov := r.PathValue("provider")
	var body struct {
		APIKey  string `json:"api_key"`
		BaseURL string `json:"base_url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid JSON"})
		return
	}
	if err := a.store.SetProviderKey(user.ID, prov, body.APIKey, body.BaseURL); err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]string{"status": "saved", "provider": prov})
}

func (a *API) handleListMyProviderKeys(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	if user == nil {
		writeJSON(w, 401, map[string]string{"error": "not authenticated"})
		return
	}
	keys, err := a.store.ListProviderKeys(user.ID)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	if keys == nil {
		keys = []ProviderKey{}
	}
	writeJSON(w, 200, map[string]any{"providers": keys, "count": len(keys)})
}

func (a *API) handleDeleteMyProviderKey(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	if user == nil {
		writeJSON(w, 401, map[string]string{"error": "not authenticated"})
		return
	}
	prov := r.PathValue("provider")
	if err := a.store.DeleteProviderKey(user.ID, prov); err != nil {
		writeJSON(w, 404, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]string{"status": "deleted"})
}

// ─── Team handlers ─────────────────────────────────────────────────────────

func (a *API) handleCreateTeam(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid JSON"})
		return
	}
	// Use user from context if authenticated, else 0
	var createdBy int64
	if user := UserFromContext(r.Context()); user != nil {
		createdBy = user.ID
	}
	team, err := a.store.CreateTeam(body.Name, body.Description, createdBy)
	if err != nil {
		if strings.Contains(err.Error(), "already exists") {
			writeJSON(w, 409, map[string]string{"error": err.Error()})
		} else {
			writeJSON(w, 400, map[string]string{"error": err.Error()})
		}
		return
	}
	writeJSON(w, 201, team)
}

func (a *API) handleListTeams(w http.ResponseWriter, r *http.Request) {
	teams, err := a.store.ListTeams()
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	if teams == nil {
		teams = []Team{}
	}
	writeJSON(w, 200, map[string]any{"teams": teams, "count": len(teams)})
}

func (a *API) handleGetTeam(w http.ResponseWriter, r *http.Request) {
	id := parseID(r.PathValue("id"))
	if id == 0 {
		writeJSON(w, 400, map[string]string{"error": "invalid team id"})
		return
	}
	team, err := a.store.GetTeam(id)
	if err != nil {
		writeJSON(w, 404, map[string]string{"error": err.Error()})
		return
	}
	keys, _ := a.store.ListTeamKeys(id)
	if keys == nil {
		keys = []APIKey{}
	}
	writeJSON(w, 200, map[string]any{"team": team, "keys": keys})
}

func (a *API) handleUpdateTeam(w http.ResponseWriter, r *http.Request) {
	id := parseID(r.PathValue("id"))
	if id == 0 {
		writeJSON(w, 400, map[string]string{"error": "invalid team id"})
		return
	}
	var body struct {
		Name        *string `json:"name"`
		Description *string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid JSON"})
		return
	}
	team, err := a.store.UpdateTeam(id, body.Name, body.Description)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeJSON(w, 404, map[string]string{"error": err.Error()})
		} else {
			writeJSON(w, 400, map[string]string{"error": err.Error()})
		}
		return
	}
	writeJSON(w, 200, team)
}

func (a *API) handleDeleteTeam(w http.ResponseWriter, r *http.Request) {
	id := parseID(r.PathValue("id"))
	if id == 0 {
		writeJSON(w, 400, map[string]string{"error": "invalid team id"})
		return
	}
	if err := a.store.DeleteTeam(id); err != nil {
		writeJSON(w, 404, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]string{"status": "deleted"})
}

func (a *API) handleCreateTeamKey(w http.ResponseWriter, r *http.Request) {
	teamID := parseID(r.PathValue("id"))
	if teamID == 0 {
		writeJSON(w, 400, map[string]string{"error": "invalid team id"})
		return
	}
	var body struct {
		Name   string `json:"name"`
		UserID int64  `json:"user_id"`
	}
	json.NewDecoder(r.Body).Decode(&body)

	// Default to context user if no user_id specified
	uid := body.UserID
	if uid == 0 {
		if user := UserFromContext(r.Context()); user != nil {
			uid = user.ID
		}
	}
	if uid == 0 {
		// Create a system user for team keys if no user specified
		uid = 1 // fallback to first user
	}

	key, err := a.store.GenerateTeamKey(teamID, uid, body.Name)
	if err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 201, key)
}

func (a *API) handleListTeamKeys(w http.ResponseWriter, r *http.Request) {
	teamID := parseID(r.PathValue("id"))
	if teamID == 0 {
		writeJSON(w, 400, map[string]string{"error": "invalid team id"})
		return
	}
	keys, err := a.store.ListTeamKeys(teamID)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	if keys == nil {
		keys = []APIKey{}
	}
	writeJSON(w, 200, map[string]any{"keys": keys, "count": len(keys)})
}

func (a *API) handleRevokeTeamKey(w http.ResponseWriter, r *http.Request) {
	teamID := parseID(r.PathValue("id"))
	keyID := parseID(r.PathValue("keyId"))
	if teamID == 0 || keyID == 0 {
		writeJSON(w, 400, map[string]string{"error": "invalid ids"})
		return
	}
	if err := a.store.RevokeTeamKey(teamID, keyID); err != nil {
		writeJSON(w, 404, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]string{"status": "revoked"})
}

func (a *API) handleRotateTeamKey(w http.ResponseWriter, r *http.Request) {
	teamID := parseID(r.PathValue("id"))
	keyID := parseID(r.PathValue("keyId"))
	if teamID == 0 || keyID == 0 {
		writeJSON(w, 400, map[string]string{"error": "invalid ids"})
		return
	}
	newKey, err := a.store.RotateTeamKey(teamID, keyID)
	if err != nil {
		writeJSON(w, 404, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]any{
		"status":  "rotated",
		"old_key": keyID,
		"new_key": newKey,
	})
}

func (a *API) handleTeamSpend(w http.ResponseWriter, r *http.Request) {
	teamID := r.PathValue("id")
	if teamID == "" {
		writeJSON(w, 400, map[string]string{"error": "invalid team id"})
		return
	}

	var totalReqs int64
	var totalCost float64
	var totalTokensIn, totalTokensOut int64
	a.store.db.QueryRow(`SELECT COUNT(*), COALESCE(SUM(cost_usd),0), COALESCE(SUM(tokens_in),0), COALESCE(SUM(tokens_out),0) FROM requests WHERE team_id = ?`, teamID).Scan(&totalReqs, &totalCost, &totalTokensIn, &totalTokensOut)

	var monthReqs int64
	var monthCost float64
	a.store.db.QueryRow(`SELECT COUNT(*), COALESCE(SUM(cost_usd),0) FROM requests WHERE team_id = ? AND timestamp >= date('now','start of month')`, teamID).Scan(&monthReqs, &monthCost)

	var todayReqs int64
	var todayCost float64
	a.store.db.QueryRow(`SELECT COUNT(*), COALESCE(SUM(cost_usd),0) FROM requests WHERE team_id = ? AND timestamp >= date('now')`, teamID).Scan(&todayReqs, &todayCost)

	writeJSON(w, 200, map[string]any{
		"team_id": teamID,
		"total": map[string]any{
			"requests":   totalReqs,
			"cost_usd":   totalCost,
			"tokens_in":  totalTokensIn,
			"tokens_out": totalTokensOut,
		},
		"this_month": map[string]any{
			"requests": monthReqs,
			"cost_usd": monthCost,
		},
		"today": map[string]any{
			"requests": todayReqs,
			"cost_usd": todayCost,
		},
	})
}

func (a *API) handleTeamLogs(w http.ResponseWriter, r *http.Request) {
	teamID := r.PathValue("id")
	if teamID == "" {
		writeJSON(w, 400, map[string]string{"error": "invalid team id"})
		return
	}

	limit := 50
	offset := 0
	if l := r.URL.Query().Get("limit"); l != "" {
		fmt.Sscanf(l, "%d", &limit)
	}
	if o := r.URL.Query().Get("offset"); o != "" {
		fmt.Sscanf(o, "%d", &offset)
	}
	if limit > 200 {
		limit = 200
	}

	var total int
	a.store.db.QueryRow(`SELECT COUNT(*) FROM requests WHERE team_id = ?`, teamID).Scan(&total)

	rows, err := a.store.db.Query(`
		SELECT id, timestamp, project, COALESCE(user_id, ''), provider, model,
			tokens_in, tokens_out, cost_usd, latency_ms, status, cache_hit,
			failover_used, COALESCE(error, '')
		FROM requests WHERE team_id = ?
		ORDER BY timestamp DESC LIMIT ? OFFSET ?`, teamID, limit, offset)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	defer rows.Close()

	var logs []map[string]any
	for rows.Next() {
		var id, ts, project, userID, prov, model, errStr string
		var tokIn, tokOut int
		var costUSD float64
		var latencyMs int64
		var status int
		var cacheHit, failoverUsed bool
		if err := rows.Scan(&id, &ts, &project, &userID, &prov, &model,
			&tokIn, &tokOut, &costUSD, &latencyMs, &status, &cacheHit,
			&failoverUsed, &errStr); err != nil {
			continue
		}
		logs = append(logs, map[string]any{
			"id": id, "timestamp": ts, "project": project, "user_id": userID,
			"provider": prov, "model": model, "tokens_in": tokIn, "tokens_out": tokOut,
			"cost_usd": costUSD, "latency_ms": latencyMs, "status": status,
			"cache_hit": cacheHit, "failover_used": failoverUsed, "error": errStr,
		})
	}
	if err := rows.Err(); err != nil {
		log.Printf("[db] rows iteration error: %v", err)
	}
	if logs == nil {
		logs = []map[string]any{}
	}
	writeJSON(w, 200, map[string]any{"logs": logs, "total": total, "team_id": teamID})
}

// ─── Helpers ───────────────────────────────────────────────────────────────

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func parseID(s string) int64 {
	var id int64
	fmt.Sscanf(s, "%d", &id)
	return id
}
