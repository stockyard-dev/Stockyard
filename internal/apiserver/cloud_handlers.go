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
	"strconv"
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
//
// Two auth paths, tried in order:
//
//  1. Authorization: Bearer <license-key> — used by the desktop app,
//     which has no cookie jar. The license key was issued by the same
//     backend, so we look it up in the licenses table and resolve to
//     the linked cloud_accounts row by email.
//  2. Cookie session — used by the browser /cloud/ dashboard.
//
// Either path must resolve to an active cloud_accounts row. Accounts
// with status="canceled" are treated as authenticated for read paths
// but blocked on write paths at the handler level.
func (c *CloudService) requireSession(w http.ResponseWriter, r *http.Request) *CloudAccount {
	if acct := c.authByBearer(r); acct != nil {
		return acct
	}
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

// authByBearer resolves a license-key bearer token to a cloud account.
// Returns nil (not an error) when no bearer header is present — so the
// caller can fall through to cookie auth without logging a spurious
// miss. A present-but-invalid bearer also returns nil; the caller will
// 401 via the cookie path.
//
// Chain: bearer license key → licenses row → email → cloud_accounts.
// Only "stockyard-desktop" product with tier cloud-single or
// cloud-multi counts; Local-tier license keys cannot authenticate to
// Cloud because Local customers don't have a cloud account.
func (c *CloudService) authByBearer(r *http.Request) *CloudAccount {
	header := r.Header.Get("Authorization")
	if !strings.HasPrefix(header, "Bearer ") {
		return nil
	}
	key := strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
	if key == "" {
		return nil
	}
	lic, err := c.db.GetLicenseByKey(key)
	if err != nil || lic == nil {
		return nil
	}
	if lic.Product != "stockyard-desktop" {
		return nil
	}
	if lic.Tier != "cloud-single" && lic.Tier != "cloud-multi" {
		return nil
	}
	if lic.Status != "active" {
		// "canceled", "trial" licenses don't grant cloud access.
		return nil
	}
	acct, err := c.db.GetCloudAccountByEmail(lic.Email)
	if err != nil {
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

// --- Backup history -------------------------------------------------

// backupListDefaultLimit and backupListMaxLimit bound the page size.
// Default is 20 (matches the UI's initial load); max is 100 so a
// malicious or confused client can't ask for "all 10,000" in one hit.
const (
	backupListDefaultLimit = 20
	backupListMaxLimit     = 100
)

// backupListItem is the shape returned by HandleBackupList for each
// row. Deliberately does NOT include blob_key (that's an internal
// storage detail, not something the desktop needs to see).
type backupListItem struct {
	ID            int64  `json:"id"`
	UploadedAt    string `json:"uploaded_at"` // RFC3339
	SizeBytes     int64  `json:"size_bytes"`
	Sha256        string `json:"sha256"`
	ClientVersion string `json:"client_version,omitempty"`
	SiteSlug      string `json:"site_slug"`
}

// HandleBackupList returns the N most recent backups for the account,
// newest first. Supports ?site=<slug> to scope to one site (default
// "default"), ?limit=N (capped at backupListMaxLimit), and ?before=ID
// for cursor-based pagination ("load more" sends the oldest ID it
// currently has; server returns the next page older than that).
//
// Why cursor pagination instead of offset: uploads are append-only and
// timestamp-ordered, so ID order == upload order. Cursor pagination
// avoids the classic offset-shifts-when-new-items-arrive problem,
// which matters if a backup completes while the UI is paginating.
func (c *CloudService) HandleBackupList(w http.ResponseWriter, r *http.Request) {
	acct := c.requireSession(w, r)
	if acct == nil {
		return
	}
	siteSlug := strings.TrimSpace(r.URL.Query().Get("site"))
	if siteSlug == "" {
		siteSlug = "default"
	}
	limit := backupListDefaultLimit
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	if limit > backupListMaxLimit {
		limit = backupListMaxLimit
	}
	beforeID := int64(0)
	if v := r.URL.Query().Get("before"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil && n > 0 {
			beforeID = n
		}
	}

	// Build the query. SQL with parameterized inputs, no string-
	// concatenation — beforeID is always bound as a parameter.
	query := `
		SELECT b.id, b.uploaded_at, b.size_bytes, b.sha256_hex,
		       COALESCE(b.client_version, ''), s.slug
		FROM cloud_backup_blobs b
		JOIN cloud_sites s ON s.id = b.site_id
		WHERE b.account_id = ? AND s.slug = ?
	`
	args := []any{acct.ID, siteSlug}
	if beforeID > 0 {
		query += ` AND b.id < ?`
		args = append(args, beforeID)
	}
	query += ` ORDER BY b.id DESC LIMIT ?`
	args = append(args, limit+1) // +1 so we can tell if there's more

	rows, err := c.db.conn.Query(query, args...)
	if err != nil {
		log.Printf("cloud backup list: query: %v", err)
		writeJSON(w, 500, map[string]string{"error": "could not list backups"})
		return
	}
	defer rows.Close()

	items := make([]backupListItem, 0, limit)
	for rows.Next() {
		var it backupListItem
		if err := rows.Scan(&it.ID, &it.UploadedAt, &it.SizeBytes,
			&it.Sha256, &it.ClientVersion, &it.SiteSlug); err != nil {
			log.Printf("cloud backup list: scan: %v", err)
			writeJSON(w, 500, map[string]string{"error": "could not list backups"})
			return
		}
		items = append(items, it)
	}

	// Detect "has more" by whether we got limit+1 rows; trim the extra.
	hasMore := false
	if len(items) > limit {
		items = items[:limit]
		hasMore = true
	}

	// next_before is the ID the client should send for the next page,
	// or 0 if there are no more pages. The desktop just plumbs this
	// through — it doesn't need to know the semantics.
	nextBefore := int64(0)
	if hasMore && len(items) > 0 {
		nextBefore = items[len(items)-1].ID
	}

	writeJSON(w, 200, map[string]any{
		"backups":     items,
		"has_more":    hasMore,
		"next_before": nextBefore,
	})
}

// HandleBackupByID returns the encrypted blob for a specific backup ID.
// Enforces ownership: the blob's account_id must match the authenticated
// account. A non-owned or missing ID returns 404 (not 403) so enumerating
// valid IDs across accounts doesn't leak existence information.
func (c *CloudService) HandleBackupByID(w http.ResponseWriter, r *http.Request) {
	acct := c.requireSession(w, r)
	if acct == nil {
		return
	}
	if c.blobs == nil {
		writeJSON(w, 503, map[string]string{"error": "backup storage not yet configured"})
		return
	}

	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		writeJSON(w, 400, map[string]string{"error": "invalid backup id"})
		return
	}

	var (
		blobKey string
		shaHex  string
		size    int64
	)
	err = c.db.conn.QueryRow(`
		SELECT blob_key, sha256_hex, size_bytes
		FROM cloud_backup_blobs
		WHERE id = ? AND account_id = ?
	`, id, acct.ID).Scan(&blobKey, &shaHex, &size)
	if err != nil {
		// 404 regardless of whether it doesn't exist or isn't ours —
		// no enumeration leakage.
		writeJSON(w, 404, map[string]string{"error": "backup not found"})
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

// --- Site management endpoints --------------------------------------

// siteListItem is the JSON shape for GET /sites. Deliberately omits
// internal row IDs — slug is the stable identifier the desktop uses
// on all subsequent API calls, IDs are an implementation detail.
type siteListItem struct {
	Slug        string `json:"slug"`
	DisplayName string `json:"display_name"`
	CreatedAt   string `json:"created_at"`
	BackupCount int64  `json:"backup_count"`
	LastBackup  string `json:"last_backup,omitempty"` // RFC3339, empty if never backed up
}

// HandleSitesList returns the authenticated account's sites, oldest
// first (so the first-created site — usually "default" — is at the
// top). Includes a backup count and last-backup timestamp per site so
// the UI can show activity indicators without a second round trip.
//
// For a brand-new Cloud account with no uploads yet, the "default"
// site exists only if the desktop has already made at least one
// backup attempt (resolveOrCreateSite is lazy). Fresh accounts
// return an empty list, and the desktop handles that case by
// falling back to the legacy behavior of passing X-Site-Slug=default
// on next upload.
func (c *CloudService) HandleSitesList(w http.ResponseWriter, r *http.Request) {
	acct := c.requireSession(w, r)
	if acct == nil {
		return
	}

	// LEFT JOIN so sites with zero backups still appear. The
	// aggregate filters site_id via the inner cloud_backup_blobs
	// alias only; the outer site row is unconditional.
	rows, err := c.db.conn.Query(`
		SELECT s.slug, s.display_name, s.created_at,
		       COUNT(b.id) AS backup_count,
		       COALESCE(MAX(b.uploaded_at), '') AS last_backup
		FROM cloud_sites s
		LEFT JOIN cloud_backup_blobs b ON b.site_id = s.id
		WHERE s.account_id = ?
		GROUP BY s.id
		ORDER BY s.id ASC
	`, acct.ID)
	if err != nil {
		log.Printf("cloud sites list: %v", err)
		writeJSON(w, 500, map[string]string{"error": "could not list sites"})
		return
	}
	defer rows.Close()

	sites := make([]siteListItem, 0)
	for rows.Next() {
		var it siteListItem
		if err := rows.Scan(&it.Slug, &it.DisplayName, &it.CreatedAt,
			&it.BackupCount, &it.LastBackup); err != nil {
			log.Printf("cloud sites list scan: %v", err)
			writeJSON(w, 500, map[string]string{"error": "could not list sites"})
			return
		}
		sites = append(sites, it)
	}

	writeJSON(w, 200, map[string]any{
		"sites": sites,
		"tier":  acct.Tier,
	})
}

// HandleSitesCreate adds a new site to the account. Cloud Single tier
// is limited to 1 site (enforced here + by resolveOrCreateSite on
// upload); Cloud Multi has no ceiling.
//
// Body: {"slug": "uptown", "display_name": "Uptown Studio"}
// Slug is required, must match isValidSiteSlug; display_name is
// optional and defaults to the slug.
//
// Idempotent: creating a site with an existing slug for the same
// account returns 200 with the existing row, not an error. Makes
// retry-friendly for flaky networks without needing PUT semantics.
func (c *CloudService) HandleSitesCreate(w http.ResponseWriter, r *http.Request) {
	acct := c.requireSession(w, r)
	if acct == nil {
		return
	}

	var body struct {
		Slug        string `json:"slug"`
		DisplayName string `json:"display_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid JSON body"})
		return
	}
	slug := strings.ToLower(strings.TrimSpace(body.Slug))
	if !isValidSiteSlug(slug) {
		writeJSON(w, 400, map[string]string{
			"error": "slug must be 1-40 characters, lowercase a-z, 0-9, and - only",
		})
		return
	}
	displayName := strings.TrimSpace(body.DisplayName)
	if displayName == "" {
		displayName = slug
	}
	if len(displayName) > 80 {
		writeJSON(w, 400, map[string]string{"error": "display_name must be 80 characters or less"})
		return
	}

	// Idempotency: if the slug already exists for this account,
	// return it rather than a conflict. The display_name of an
	// existing row is NOT updated — rename is a separate (future)
	// endpoint.
	var existingID int64
	var existingDisplay, existingCreated string
	err := c.db.conn.QueryRow(
		`SELECT id, display_name, created_at FROM cloud_sites WHERE account_id = ? AND slug = ?`,
		acct.ID, slug,
	).Scan(&existingID, &existingDisplay, &existingCreated)
	if err == nil {
		writeJSON(w, 200, map[string]any{
			"slug":         slug,
			"display_name": existingDisplay,
			"created_at":   existingCreated,
			"created":      false,
		})
		return
	}

	// Tier enforcement for new sites. Cloud Single = 1 site max.
	if acct.Tier == "cloud-single" {
		var existing int64
		c.db.conn.QueryRow(
			`SELECT COUNT(*) FROM cloud_sites WHERE account_id = ?`, acct.ID,
		).Scan(&existing)
		if existing >= 1 {
			writeJSON(w, 403, map[string]string{
				"error": "Cloud Single supports one site. Upgrade to Cloud Multi for additional sites.",
			})
			return
		}
	}

	res, err := c.db.conn.Exec(
		`INSERT INTO cloud_sites (account_id, slug, display_name) VALUES (?, ?, ?)`,
		acct.ID, slug, displayName,
	)
	if err != nil {
		log.Printf("cloud sites create: %v", err)
		writeJSON(w, 500, map[string]string{"error": "could not create site"})
		return
	}
	_ = res
	writeJSON(w, 200, map[string]any{
		"slug":         slug,
		"display_name": displayName,
		"created":      true,
	})
}

// --- Site management internals --------------------------------------

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
