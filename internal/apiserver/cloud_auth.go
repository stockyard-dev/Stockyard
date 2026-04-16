// Package apiserver — Cloud backend authentication.
//
// Flow:
//   1. Customer purchases Cloud tier. Webhook creates a cloud_accounts
//      row keyed by email (see stripe.go).
//   2. Customer visits https://stockyard.dev/cloud/login/ and enters
//      their email. Backend generates a magic token, hashes it, stores
//      the hash in cloud_magic_tokens with 15-minute TTL, and emails
//      the raw token as a one-click URL.
//   3. Customer clicks URL, backend looks up the token hash, marks it
//      consumed, creates a cloud_sessions row, and sets a signed
//      session cookie.
//   4. All /api/cloud/* endpoints read the cookie, look up the session,
//      and resolve the account.
//
// Security model:
//   - Tokens are 32 bytes random, base64url-encoded. Never logged.
//   - DB stores SHA-256 hashes only. A DB leak does not grant login.
//   - Magic tokens TTL 15 minutes, single-use (consumed_at marker).
//   - Sessions TTL 30 days, stored hashed, revocable.
//   - Cookie is HttpOnly, Secure, SameSite=Lax.
package apiserver

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	magicTokenTTL   = 15 * time.Minute
	sessionTTL      = 30 * 24 * time.Hour
	sessionCookie   = "sy_cloud_session"
	magicTokenBytes = 32
	sessionBytes    = 32
)

// CloudAccount is a paying Cloud customer with an account they can
// log into. Distinct from cloud_tenants (legacy LLM-proxy tenants).
type CloudAccount struct {
	ID                   int64
	Email                string
	Tier                 string // "cloud-single" or "cloud-multi"
	StripeCustomerID     string
	StripeSubscriptionID string
	Status               string // "active", "past_due", "canceled"
	CreatedAt            time.Time
}

// CloudSession represents a browser/desktop login session.
type CloudSession struct {
	AccountID  int64
	ExpiresAt  time.Time
	LastSeenAt time.Time
}

// ErrNoAccount is returned when an email isn't yet a Cloud customer.
var ErrNoAccount = errors.New("no cloud account for that email")

// ErrInvalidToken is returned when a magic token or session token
// doesn't exist, is expired, or was already consumed.
var ErrInvalidToken = errors.New("invalid or expired token")

// hashToken returns the hex SHA-256 hash of a token. The token itself
// is NEVER stored.
func hashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

// newRandomToken returns n random bytes base64url-encoded without
// padding. Suitable for magic links and session cookies — URL-safe
// and the length doesn't leak padding.
func newRandomToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("read random: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// CreateCloudAccount upserts a cloud_accounts row. Called from the
// Stripe webhook when a Cloud tier subscription becomes active.
//
// If an account already exists for this email, updates its tier and
// stripe IDs (handles the case where a customer upgrades or their
// subscription is replaced).
func (db *SqliteDB) CreateCloudAccount(email, tier, stripeCustomerID, stripeSubID string) (*CloudAccount, error) {
	if email == "" || tier == "" {
		return nil, errors.New("email and tier required")
	}
	email = strings.ToLower(strings.TrimSpace(email))

	_, err := db.conn.Exec(`
		INSERT INTO cloud_accounts (email, tier, stripe_customer_id, stripe_subscription_id, status)
		VALUES (?, ?, ?, ?, 'active')
		ON CONFLICT(email) DO UPDATE SET
			tier = excluded.tier,
			stripe_customer_id = excluded.stripe_customer_id,
			stripe_subscription_id = excluded.stripe_subscription_id,
			status = 'active',
			updated_at = datetime('now')
	`, email, tier, stripeCustomerID, stripeSubID)
	if err != nil {
		return nil, fmt.Errorf("upsert cloud account: %w", err)
	}
	return db.GetCloudAccountByEmail(email)
}

// GetCloudAccountByEmail returns the account for an email, or
// ErrNoAccount if no row exists.
func (db *SqliteDB) GetCloudAccountByEmail(email string) (*CloudAccount, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	var a CloudAccount
	var createdAt string
	err := db.conn.QueryRow(`
		SELECT id, email, tier, stripe_customer_id, stripe_subscription_id, status, created_at
		FROM cloud_accounts WHERE email = ?
	`, email).Scan(&a.ID, &a.Email, &a.Tier, &a.StripeCustomerID, &a.StripeSubscriptionID, &a.Status, &createdAt)
	if err == sql.ErrNoRows {
		return nil, ErrNoAccount
	}
	if err != nil {
		return nil, fmt.Errorf("query cloud account: %w", err)
	}
	a.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
	return &a, nil
}

// GetCloudAccountByID is the lookup used after session cookie resolves
// to an account_id.
func (db *SqliteDB) GetCloudAccountByID(id int64) (*CloudAccount, error) {
	var a CloudAccount
	var createdAt string
	err := db.conn.QueryRow(`
		SELECT id, email, tier, stripe_customer_id, stripe_subscription_id, status, created_at
		FROM cloud_accounts WHERE id = ?
	`, id).Scan(&a.ID, &a.Email, &a.Tier, &a.StripeCustomerID, &a.StripeSubscriptionID, &a.Status, &createdAt)
	if err == sql.ErrNoRows {
		return nil, ErrNoAccount
	}
	if err != nil {
		return nil, fmt.Errorf("query cloud account: %w", err)
	}
	a.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
	return &a, nil
}

// UpdateCloudAccountStatus is called from Stripe webhooks when a
// subscription's lifecycle changes. Status values we care about:
//   - "active": normal paying customer
//   - "past_due": payment failed, grace period — keep reads working
//   - "canceled": subscription ended, block backup uploads
func (db *SqliteDB) UpdateCloudAccountStatus(stripeSubID, status string) error {
	if stripeSubID == "" {
		return errors.New("empty subscription id")
	}
	_, err := db.conn.Exec(`
		UPDATE cloud_accounts
		SET status = ?, updated_at = datetime('now')
		WHERE stripe_subscription_id = ?
	`, status, stripeSubID)
	return err
}

// IssueMagicToken generates a new magic-link token, stores its hash,
// and returns the raw token (to embed in an email URL). Caller is
// responsible for emailing it — we never log it.
//
// We do NOT require an account to exist. If a stranger types their
// email, we issue a token silently and the login endpoint will reject
// it later — prevents email enumeration ("this email is/isn't a
// customer") at the /cloud/login/ endpoint.
func (db *SqliteDB) IssueMagicToken(email string) (rawToken string, err error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return "", errors.New("empty email")
	}
	token, err := newRandomToken(magicTokenBytes)
	if err != nil {
		return "", err
	}
	_, err = db.conn.Exec(`
		INSERT INTO cloud_magic_tokens (token_hash, email, expires_at)
		VALUES (?, ?, ?)
	`, hashToken(token), email, time.Now().Add(magicTokenTTL).UTC().Format(time.RFC3339))
	if err != nil {
		return "", fmt.Errorf("store magic token: %w", err)
	}
	return token, nil
}

// ConsumeMagicToken validates a token, marks it consumed, and returns
// the associated email. Returns ErrInvalidToken if the token is
// unknown, expired, or already consumed.
func (db *SqliteDB) ConsumeMagicToken(rawToken string) (email string, err error) {
	if rawToken == "" {
		return "", ErrInvalidToken
	}
	h := hashToken(rawToken)

	var (
		rowEmail  string
		expiresAt string
		consumed  sql.NullString
	)
	err = db.conn.QueryRow(`
		SELECT email, expires_at, consumed_at FROM cloud_magic_tokens WHERE token_hash = ?
	`, h).Scan(&rowEmail, &expiresAt, &consumed)
	if err == sql.ErrNoRows {
		return "", ErrInvalidToken
	}
	if err != nil {
		return "", fmt.Errorf("lookup magic token: %w", err)
	}
	if consumed.Valid {
		return "", ErrInvalidToken
	}
	exp, perr := time.Parse(time.RFC3339, expiresAt)
	if perr != nil || time.Now().After(exp) {
		return "", ErrInvalidToken
	}
	// Mark consumed. We do a conditional UPDATE to prevent race
	// conditions where two simultaneous consumes both see consumed=NULL.
	res, err := db.conn.Exec(`
		UPDATE cloud_magic_tokens
		SET consumed_at = datetime('now')
		WHERE token_hash = ? AND consumed_at IS NULL
	`, h)
	if err != nil {
		return "", fmt.Errorf("consume magic token: %w", err)
	}
	n, _ := res.RowsAffected()
	if n != 1 {
		// Someone else consumed between our SELECT and UPDATE.
		return "", ErrInvalidToken
	}
	return rowEmail, nil
}

// CreateSession issues a new long-lived session token for an account
// and stores its hash. Returns the raw token to set as a cookie.
func (db *SqliteDB) CreateSession(accountID int64, userAgent string) (rawToken string, err error) {
	token, err := newRandomToken(sessionBytes)
	if err != nil {
		return "", err
	}
	_, err = db.conn.Exec(`
		INSERT INTO cloud_sessions (token_hash, account_id, expires_at, user_agent)
		VALUES (?, ?, ?, ?)
	`, hashToken(token), accountID, time.Now().Add(sessionTTL).UTC().Format(time.RFC3339), userAgent)
	if err != nil {
		return "", fmt.Errorf("create session: %w", err)
	}
	return token, nil
}

// LookupSession resolves a raw session cookie value to an account.
// Updates last_seen_at as a side effect. Returns ErrInvalidToken if
// the session is unknown, expired, or revoked.
func (db *SqliteDB) LookupSession(rawToken string) (*CloudAccount, error) {
	if rawToken == "" {
		return nil, ErrInvalidToken
	}
	h := hashToken(rawToken)
	var (
		accountID int64
		expiresAt string
		revoked   sql.NullString
	)
	err := db.conn.QueryRow(`
		SELECT account_id, expires_at, revoked_at FROM cloud_sessions WHERE token_hash = ?
	`, h).Scan(&accountID, &expiresAt, &revoked)
	if err == sql.ErrNoRows {
		return nil, ErrInvalidToken
	}
	if err != nil {
		return nil, fmt.Errorf("lookup session: %w", err)
	}
	if revoked.Valid {
		return nil, ErrInvalidToken
	}
	exp, perr := time.Parse(time.RFC3339, expiresAt)
	if perr != nil || time.Now().After(exp) {
		return nil, ErrInvalidToken
	}
	// Best-effort last-seen update. Failure here shouldn't block login.
	_, _ = db.conn.Exec(`UPDATE cloud_sessions SET last_seen_at = datetime('now') WHERE token_hash = ?`, h)

	return db.GetCloudAccountByID(accountID)
}

// RevokeSession marks a session as revoked (for logout).
func (db *SqliteDB) RevokeSession(rawToken string) error {
	_, err := db.conn.Exec(
		`UPDATE cloud_sessions SET revoked_at = datetime('now') WHERE token_hash = ?`,
		hashToken(rawToken),
	)
	return err
}
