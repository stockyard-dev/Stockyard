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
	"net/http"
	"strings"
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

// APIKey represents an issued API key (without the secret).
type APIKey struct {
	ID        int64   `json:"id"`
	UserID    int64   `json:"user_id"`
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
type Store struct {
	db *sql.DB
}

// NewStore creates a new auth store and runs migrations.
func NewStore(db *sql.DB) (*Store, error) {
	s := &Store{db: db}
	if _, err := db.Exec(schema); err != nil {
		return nil, fmt.Errorf("auth migration: %w", err)
	}
	log.Println("[auth] migrations applied")
	return s, nil
}

// ─── User Operations ───────────────────────────────────────────────────────

// CreateUser creates a new user and returns it.
func (s *Store) CreateUser(email, name string) (*User, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	if email == "" {
		return nil, errors.New("email is required")
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
			return nil, fmt.Errorf("user with email %s already exists", email)
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
	return users, nil
}

// UpdateUserTier updates a user's tier.
func (s *Store) UpdateUserTier(id int64, tier string) error {
	_, err := s.db.Exec(`UPDATE users SET tier = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, tier, id)
	return err
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
	var ak APIKey
	var uid int64
	var revokedAt sql.NullString
	var lastUsed sql.NullString

	err := s.db.QueryRow(
		`SELECT id, user_id, key_prefix, name, scopes, created_at, last_used, revoked_at
		 FROM api_keys WHERE key_hash = ?`, hash,
	).Scan(&ak.ID, &uid, &ak.KeyPrefix, &ak.Name, &ak.Scopes, &ak.CreatedAt, &lastUsed, &revokedAt)

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

	// Update last_used (fire-and-forget)
	go func() {
		s.db.Exec(`UPDATE api_keys SET last_used = CURRENT_TIMESTAMP WHERE id = ?`, ak.ID)
	}()

	// Look up user
	user, err := s.GetUser(uid)
	if err != nil {
		return nil, nil, err
	}

	return user, &ak, nil
}

// ListKeys returns all (non-revoked) API keys for a user.
func (s *Store) ListKeys(userID int64) ([]APIKey, error) {
	rows, err := s.db.Query(
		`SELECT id, user_id, key_prefix, name, scopes, created_at, last_used, revoked_at
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
		if err := rows.Scan(&ak.ID, &ak.UserID, &ak.KeyPrefix, &ak.Name, &ak.Scopes,
			&ak.CreatedAt, &lastUsed, &revokedAt); err != nil {
			return nil, err
		}
		if lastUsed.Valid {
			ak.LastUsed = &lastUsed.String
		}
		keys = append(keys, ak)
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
	return nil
}

// ─── Provider Key Operations ───────────────────────────────────────────────

// SetProviderKey stores or updates a user's provider API key.
func (s *Store) SetProviderKey(userID int64, providerName, apiKey, baseURL string) error {
	providerName = strings.TrimSpace(strings.ToLower(providerName))
	if providerName == "" || apiKey == "" {
		return errors.New("provider and api_key are required")
	}
	_, err := s.db.Exec(
		`INSERT INTO user_provider_keys (user_id, provider, api_key, base_url)
		 VALUES (?, ?, ?, ?)
		 ON CONFLICT(user_id, provider) DO UPDATE SET api_key = excluded.api_key, base_url = excluded.base_url, updated_at = CURRENT_TIMESTAMP`,
		userID, providerName, apiKey, baseURL,
	)
	return err
}

// GetProviderKey returns a user's API key for a specific provider.
func (s *Store) GetProviderKey(userID int64, providerName string) (apiKey, baseURL string, err error) {
	err = s.db.QueryRow(
		`SELECT api_key, base_url FROM user_provider_keys WHERE user_id = ? AND provider = ?`,
		userID, providerName,
	).Scan(&apiKey, &baseURL)
	if err == sql.ErrNoRows {
		return "", "", nil
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
		var prov string
		if err := rows.Scan(&prov, &pk.APIKey, &pk.BaseURL); err != nil {
			return nil, err
		}
		keys[prov] = pk
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
	store *Store
}

// NewAPI creates a new auth API handler.
func NewAPI(store *Store) *API {
	return &API{store: store}
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

	// Provider key management
	mux.HandleFunc("PUT /api/auth/users/{id}/providers/{provider}", a.handleSetProviderKey)
	mux.HandleFunc("GET /api/auth/users/{id}/providers", a.handleListProviderKeys)
	mux.HandleFunc("DELETE /api/auth/users/{id}/providers/{provider}", a.handleDeleteProviderKey)

	// Self-service: current user info (via API key)
	mux.HandleFunc("GET /api/auth/me", a.handleMe)
	mux.HandleFunc("POST /api/auth/me/keys", a.handleCreateMyKey)
	mux.HandleFunc("GET /api/auth/me/keys", a.handleListMyKeys)
	mux.HandleFunc("DELETE /api/auth/me/keys/{keyId}", a.handleRevokeMyKey)
	mux.HandleFunc("PUT /api/auth/me/providers/{provider}", a.handleSetMyProviderKey)
	mux.HandleFunc("GET /api/auth/me/providers", a.handleListMyProviderKeys)
	mux.HandleFunc("DELETE /api/auth/me/providers/{provider}", a.handleDeleteMyProviderKey)

	log.Println("[auth] API routes registered")
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
	user, err := a.store.CreateUser(body.Email, body.Name)
	if err != nil {
		status := 500
		if strings.Contains(err.Error(), "already exists") || strings.Contains(err.Error(), "required") {
			status = 409
		}
		writeJSON(w, status, map[string]string{"error": err.Error()})
		return
	}

	// Auto-generate first API key
	key, err := a.store.GenerateKey(user.ID, "default")
	if err != nil {
		writeJSON(w, 201, map[string]any{"user": user, "error": "user created but key generation failed: " + err.Error()})
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
	user, err := a.store.CreateUser(body.Email, body.Name)
	if err != nil {
		status := 500
		if strings.Contains(err.Error(), "already exists") || strings.Contains(err.Error(), "required") {
			status = 409
		}
		writeJSON(w, status, map[string]string{"error": err.Error()})
		return
	}

	// Auto-generate first API key
	key, err := a.store.GenerateKey(user.ID, "default")
	if err != nil {
		// User created but key failed — still return user
		writeJSON(w, 201, map[string]any{"user": user, "error": "user created but key generation failed: " + err.Error()})
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
