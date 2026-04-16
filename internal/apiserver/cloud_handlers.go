// Package apiserver — Cloud backend HTTP handlers.
//
// Endpoints wired in server.go:
//   POST /api/cloud/login/request   — { email } → send magic link
//   GET  /api/cloud/login/verify    — ?token=... → set session cookie + redirect
//   POST /api/cloud/logout          — revoke session
//   GET  /api/cloud/me              — return current account info
//   POST /api/cloud/backup          — upload encrypted blob
//   GET  /api/cloud/backup/latest   — download most recent blob
//
// Cookie: HttpOnly, Secure, SameSite=Lax, 30-day expiry. Name
// `sy_cloud_session`. The cookie value is the raw session token; the
// server stores only its hash.
package apiserver

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

// CloudService holds the dependencies needed by Cloud endpoints. Wired
// up in server.go where config is available.
type CloudService struct {
	db           *SqliteDB
	mailer       Mailer
	blobs        BlobStore
	siteBase     string // "https://stockyard.dev" — for magic-link URLs
	cookieSecure bool   // false in local dev, true in prod
}

// NewCloudService wires a Cloud handler bundle.
// blobs may be nil — endpoints that need it will return 503.
func NewCloudService(db *SqliteDB, mailer Mailer, blobs BlobStore, siteBase string, cookieSecure bool) *CloudService {
	if siteBase == "" {
		siteBase = "https://stockyard.dev"
	}
	return &CloudService{
		db:           db,
		mailer:       mailer,
		blobs:        blobs,
		siteBase:     strings.TrimRight(siteBase, "/"),
		cookieSecure: cookieSecure,
	}
}

// --- Auth helpers ----------------------------------------------------

// requireSession returns the CloudAccount the current request is
// authenticated as, or writes a 401 and returns nil.
func (c *CloudService) requireSession(w http.ResponseWriter, r *http.Request) *CloudAccount {
	cookie, err := r.Cookie(sessionCookie)
	if err != nil || cookie.Value == "" {
		writeJSON(w, 401, map[string]string{"error": "not logged in"})
		return nil
	}
	acct, err := c.db.LookupSession(cookie.Value)
	if err != nil {
		writeJSON(w, 401, map[string]string{"error": "session expired"})
		return nil
	}
	return acct
}

// setSessionCookie writes the session cookie with production-safe
// attributes. SameSite=Lax allows the cookie to flow on top-level
// navigations (e.g. returning from the magic-link email) while still
// being blocked on most cross-site requests.
func (c *CloudService) setSessionCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    token,
		Path:     "/",
		Expires:  time.Now().Add(sessionTTL),
		MaxAge:   int(sessionTTL.Seconds()),
		HttpOnly: true,
		Secure:   c.cookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
}

func (c *CloudService) clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   c.cookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
}

// accountCreator returns a function the Stripe webhook invokes when a
// Cloud tier becomes paid (invoice.payment_succeeded with
// billing_reason=subscription_create). Called with the customer email,
// tier, and Stripe IDs. Idempotent via UPSERT in CreateCloudAccount.
func (c *CloudService) accountCreator() func(email, tier, stripeCustomerID, stripeSubID string) error {
	return func(email, tier, stripeCustomerID, stripeSubID string) error {
		if tier != "cloud-single" && tier != "cloud-multi" {
			return nil // not a Cloud tier, nothing to do
		}
		_, err := c.db.CreateCloudAccount(email, tier, stripeCustomerID, stripeSubID)
		return err
	}
}

// writeJSON is a thin helper. Kept local so this file has no other
// dependencies on the rest of apiserver's helpers beyond Mailer and DB.
func writeJSON(w http.ResponseWriter, code int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(body)
}

// --- Login request --------------------------------------------------

type loginRequestBody struct {
	Email string `json:"email"`
}

// HandleLoginRequest accepts an email and emails a magic link.
//
// Security: we always return 200 regardless of whether the email has
// a Cloud account. This prevents email enumeration.
func (c *CloudService) HandleLoginRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, 405, map[string]string{"error": "POST required"})
		return
	}
	var body loginRequestBody
	if err := json.NewDecoder(io.LimitReader(r.Body, 4096)).Decode(&body); err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid body"})
		return
	}
	email := strings.ToLower(strings.TrimSpace(body.Email))
	if !looksLikeEmail(email) {
		writeJSON(w, 400, map[string]string{"error": "invalid email"})
		return
	}

	// Issue a token regardless. If the email isn't a customer, the
	// token will just never be used — or if it is, the verify
	// endpoint will check account existence.
	token, err := c.db.IssueMagicToken(email)
	if err != nil {
		log.Printf("cloud login: issue magic token: %v", err)
		writeJSON(w, 500, map[string]string{"error": "could not send link"})
		return
	}

	// Only send mail if we have an account on file. Avoids sending
	// "did you try to log in?" emails to strangers.
	acct, _ := c.db.GetCloudAccountByEmail(email)
	if acct != nil && c.mailer != nil {
		link := fmt.Sprintf("%s/api/cloud/login/verify?token=%s", c.siteBase, token)
		if err := c.mailer.SendCloudMagicLink(email, link); err != nil {
			log.Printf("cloud login: send magic link: %v", err)
			// Don't leak to caller — still return generic 200 so
			// enumeration isn't possible via error timing.
		}
	}

	writeJSON(w, 200, map[string]string{
		"status": "ok",
		"message": "If that email is a Cloud customer, a login link is on its way. Check your inbox.",
	})
}

// looksLikeEmail is a minimal syntax check. The "real" validator is
// whether the magic-link email gets delivered.
func looksLikeEmail(s string) bool {
	if len(s) < 3 || len(s) > 254 {
		return false
	}
	at := strings.IndexByte(s, '@')
	if at <= 0 || at == len(s)-1 {
		return false
	}
	if strings.ContainsAny(s, " \t\n\r") {
		return false
	}
	return strings.ContainsRune(s[at+1:], '.')
}

// --- Login verify ---------------------------------------------------

// HandleLoginVerify consumes a magic link and issues a session. On
// success redirects to /cloud/ (or ?next=... if present). On failure
// returns a plain HTML page — this is the one endpoint a human visits
// directly from email, so JSON errors aren't friendly here.
func (c *CloudService) HandleLoginVerify(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		renderLoginError(w, "Missing token.")
		return
	}

	email, err := c.db.ConsumeMagicToken(token)
	if err != nil {
		renderLoginError(w, "That login link is invalid or has expired. Request a new one.")
		return
	}

	acct, err := c.db.GetCloudAccountByEmail(email)
	if err != nil {
		// Rare: token was valid but account doesn't exist. Could
		// happen if account was deleted between request and verify.
		renderLoginError(w, "No Cloud account found for that email.")
		return
	}

	sessionToken, err := c.db.CreateSession(acct.ID, r.UserAgent())
	if err != nil {
		log.Printf("cloud login: create session: %v", err)
		renderLoginError(w, "Could not sign you in. Try again.")
		return
	}
	c.setSessionCookie(w, sessionToken)

	next := r.URL.Query().Get("next")
	if !isSafeRedirectTarget(next) {
		next = "/cloud/"
	}
	http.Redirect(w, r, next, http.StatusFound)
}

// isSafeRedirectTarget prevents open-redirect exploitation of the
// ?next= parameter. Only allows same-origin absolute paths.
func isSafeRedirectTarget(s string) bool {
	if s == "" {
		return false
	}
	if !strings.HasPrefix(s, "/") {
		return false
	}
	if strings.HasPrefix(s, "//") {
		// Protocol-relative URL — could target another origin.
		return false
	}
	return true
}

func renderLoginError(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(400)
	fmt.Fprintf(w, `<!DOCTYPE html><html><head><title>Sign-in failed</title>
<style>body{font-family:system-ui;max-width:40rem;margin:4rem auto;padding:0 1rem;color:#222;line-height:1.6}
.card{background:#fdf6e3;border:1px solid #e4d5a7;padding:1.5rem;border-radius:4px}
a{color:#b34a1e}</style></head><body>
<div class="card"><h2>Sign-in failed</h2><p>%s</p>
<p><a href="/cloud/login/">Request a new link</a></p></div></body></html>`, msg)
}

// --- Logout ---------------------------------------------------------

// HandleLogout revokes the current session and clears the cookie.
func (c *CloudService) HandleLogout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(sessionCookie); err == nil && cookie.Value != "" {
		if err := c.db.RevokeSession(cookie.Value); err != nil {
			log.Printf("cloud logout: revoke: %v", err)
		}
	}
	c.clearSessionCookie(w)
	writeJSON(w, 200, map[string]string{"status": "ok"})
}

// --- Me -------------------------------------------------------------

// HandleMe returns the current account's public fields. Used by the
// desktop app and by the browser /cloud/ dashboard to check whether
// the user is logged in.
func (c *CloudService) HandleMe(w http.ResponseWriter, r *http.Request) {
	acct := c.requireSession(w, r)
	if acct == nil {
		return
	}
	writeJSON(w, 200, map[string]any{
		"email":  acct.Email,
		"tier":   acct.Tier,
		"status": acct.Status,
	})
}

// --- Backup upload --------------------------------------------------

// maxBackupSize caps individual backup uploads. 100MB is generous for
// per-site SQLite tarballs; revisit if real-world data grows past it.
const maxBackupSize = 100 * 1024 * 1024

// HandleBackupUpload accepts an encrypted blob from the desktop app.
// The body is stored as-is; we never see plaintext.
//
// Required headers:
//   X-Site-Slug      — site this backup belongs to. Must exist and
//                      be owned by the current account. For Cloud
//                      Single accounts we auto-create the default
//                      site on first upload.
//   X-Sha256         — hex SHA-256 of the body, verified server-side.
//   X-Client-Version — optional desktop app version, logged for
//                      debugging.
func (c *CloudService) HandleBackupUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, 405, map[string]string{"error": "POST required"})
		return
	}
	acct := c.requireSession(w, r)
	if acct == nil {
		return
	}
	if acct.Status == "canceled" {
		writeJSON(w, 402, map[string]string{"error": "subscription canceled; backups disabled"})
		return
	}
	if c.blobs == nil {
		writeJSON(w, 503, map[string]string{
			"error": "backup storage not yet configured",
			"hint":  "we'll email you when this is live",
		})
		return
	}

	siteSlug := strings.TrimSpace(r.Header.Get("X-Site-Slug"))
	if siteSlug == "" {
		siteSlug = "default"
	}
	if !isValidSiteSlug(siteSlug) {
		writeJSON(w, 400, map[string]string{"error": "invalid site slug"})
		return
	}
	expectedSha := strings.ToLower(strings.TrimSpace(r.Header.Get("X-Sha256")))

	// Resolve or create the site row.
	siteID, err := c.resolveOrCreateSite(acct, siteSlug)
	if err != nil {
		if errors.Is(err, errSiteLimitExceeded) {
			writeJSON(w, 402, map[string]string{
				"error": "Cloud Single Site is limited to one site",
				"hint":  "upgrade to Cloud Multi-Site for unlimited locations",
			})
			return
		}
		log.Printf("cloud backup: resolve site: %v", err)
		writeJSON(w, 500, map[string]string{"error": "could not resolve site"})
		return
	}

	// Write to blob store while hashing.
	key := fmt.Sprintf("acct-%d-site-%d-%d.blob", acct.ID, siteID, time.Now().UnixNano())
	limited := io.LimitReader(r.Body, maxBackupSize+1)
	hash := sha256.New()
	tee := io.TeeReader(limited, hash)

	n, err := c.blobs.Put(key, tee)
	if err != nil {
		log.Printf("cloud backup: put: %v", err)
		writeJSON(w, 500, map[string]string{"error": "storage write failed"})
		return
	}
	if n > maxBackupSize {
		// Over-size: clean up and reject.
		_ = c.blobs.Delete(key)
		writeJSON(w, 413, map[string]string{"error": "backup exceeds 100MB limit"})
		return
	}
	gotSha := hex.EncodeToString(hash.Sum(nil))
	if expectedSha != "" && gotSha != expectedSha {
		_ = c.blobs.Delete(key)
		writeJSON(w, 400, map[string]string{"error": "sha256 mismatch"})
		return
	}

	// Record metadata.
	res, err := c.db.conn.Exec(`
		INSERT INTO cloud_backup_blobs
		    (account_id, site_id, blob_key, size_bytes, sha256_hex, client_version)
		VALUES (?, ?, ?, ?, ?, ?)
	`, acct.ID, siteID, key, n, gotSha, r.Header.Get("X-Client-Version"))
	if err != nil {
		_ = c.blobs.Delete(key)
		log.Printf("cloud backup: insert metadata: %v", err)
		writeJSON(w, 500, map[string]string{"error": "could not record backup"})
		return
	}
	blobID, _ := res.LastInsertId()

	log.Printf("cloud backup: stored — account=%d site=%d size=%d key=%s",
		acct.ID, siteID, n, key)

	writeJSON(w, 200, map[string]any{
		"status":      "ok",
		"id":          blobID,
		"size_bytes":  n,
		"sha256":      gotSha,
		"uploaded_at": time.Now().UTC().Format(time.RFC3339),
	})
}

// --- Backup download ------------------------------------------------

// HandleBackupLatest returns the most recent blob for the account's
// site. Use the default site if X-Site-Slug is absent.
func (c *CloudService) HandleBackupLatest(w http.ResponseWriter, r *http.Request) {
	acct := c.requireSession(w, r)
	if acct == nil {
		return
	}
	if c.blobs == nil {
		writeJSON(w, 503, map[string]string{"error": "backup storage not yet configured"})
		return
	}
	siteSlug := strings.TrimSpace(r.URL.Query().Get("site"))
	if siteSlug == "" {
		siteSlug = "default"
	}

	var (
		blobKey string
		shaHex  string
		size    int64
	)
	err := c.db.conn.QueryRow(`
		SELECT b.blob_key, b.sha256_hex, b.size_bytes
		FROM cloud_backup_blobs b
		JOIN cloud_sites s ON s.id = b.site_id
		WHERE b.account_id = ? AND s.slug = ?
		ORDER BY b.uploaded_at DESC
		LIMIT 1
	`, acct.ID, siteSlug).Scan(&blobKey, &shaHex, &size)
	if err != nil {
		writeJSON(w, 404, map[string]string{"error": "no backup found for that site"})
		return
	}

	body, err := c.blobs.Get(blobKey)
	if err != nil {
		log.Printf("cloud backup: fetch %s: %v", blobKey, err)
		writeJSON(w, 500, map[string]string{"error": "could not read backup"})
		return
	}
	defer body.Close()

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", size))
	w.Header().Set("X-Sha256", shaHex)
	w.Header().Set("Cache-Control", "no-store")
	if _, err := io.Copy(w, body); err != nil {
		log.Printf("cloud backup: stream %s: %v", blobKey, err)
	}
}

// --- Site management ------------------------------------------------

var errSiteLimitExceeded = errors.New("site limit exceeded for tier")

// resolveOrCreateSite looks up or lazily creates a site row. Enforces
// the Cloud-Single-one-site-only rule.
func (c *CloudService) resolveOrCreateSite(acct *CloudAccount, slug string) (int64, error) {
	var id int64
	err := c.db.conn.QueryRow(
		`SELECT id FROM cloud_sites WHERE account_id = ? AND slug = ?`,
		acct.ID, slug,
	).Scan(&id)
	if err == nil {
		return id, nil
	}
	// Need to create. Check tier constraints first.
	if acct.Tier == "cloud-single" {
		var existing int64
		c.db.conn.QueryRow(
			`SELECT COUNT(*) FROM cloud_sites WHERE account_id = ?`, acct.ID,
		).Scan(&existing)
		if existing >= 1 {
			return 0, errSiteLimitExceeded
		}
	}
	display := slug
	if display == "default" {
		display = "My Business"
	}
	res, err := c.db.conn.Exec(
		`INSERT INTO cloud_sites (account_id, slug, display_name) VALUES (?, ?, ?)`,
		acct.ID, slug, display,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// isValidSiteSlug permits [a-z0-9-], length 1-40. Keeps URLs and blob
// key derivation clean.
func isValidSiteSlug(s string) bool {
	if len(s) == 0 || len(s) > 40 {
		return false
	}
	for _, r := range s {
		if !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-') {
			return false
		}
	}
	return true
}
