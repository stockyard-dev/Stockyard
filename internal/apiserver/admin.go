package apiserver

import (
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

// Admin dashboard — read-only views over Cloud accounts, backups, and
// sessions. Gated by HTTP basic auth against STOCKYARD_ADMIN_PASSWORD.
// If the env var is empty, every admin endpoint returns 404 (fail-
// closed: an unset password must never grant access).
//
// Scope is deliberately narrow:
//   - List accounts with summary stats
//   - Drill into one account to see backup history, per-site breakdown,
//     and recent session activity
//
// Explicitly NOT here: granting/revoking tiers, deleting blobs,
// impersonating accounts. Any mutation requires dropping into the
// Railway console and writing SQL. Keeping the admin surface read-only
// is the single biggest protection against a leaked admin cookie.

// adminAuthOK returns true if the incoming request authenticates
// against STOCKYARD_ADMIN_PASSWORD. Uses constant-time comparison to
// avoid leaking password-length info via timing.
//
// If the env var is empty, returns false unconditionally — callers
// should also check adminEnabled() and return 404 in that case.
// The double-check is intentional: we want admin endpoints to be
// indistinguishable from a non-existent route when disabled.
func adminAuthOK(r *http.Request) bool {
	want := os.Getenv("STOCKYARD_ADMIN_PASSWORD")
	if want == "" {
		return false
	}
	_, got, ok := r.BasicAuth()
	if !ok {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(got), []byte(want)) == 1
}

// adminEnabled reports whether the admin dashboard is configured.
// When false, all admin routes return 404 rather than 401. This means
// an attacker probing for /admin/ on a site without STOCKYARD_ADMIN_
// PASSWORD set gets no signal that the endpoint exists.
func adminEnabled() bool {
	return os.Getenv("STOCKYARD_ADMIN_PASSWORD") != ""
}

// adminGuard is middleware: enforces auth + enabled checks, then
// passes through. Handlers behind this don't need to re-check.
func (c *CloudService) adminGuard(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !adminEnabled() {
			http.NotFound(w, r)
			return
		}
		if !adminAuthOK(r) {
			w.Header().Set("WWW-Authenticate", `Basic realm="Stockyard Admin"`)
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte("Unauthorized"))
			return
		}
		// Don't cache any admin page; customer data must not end up
		// in a shared cache.
		w.Header().Set("Cache-Control", "no-store, max-age=0")
		w.Header().Set("X-Robots-Tag", "noindex, nofollow")
		h(w, r)
	}
}

// adminAccountSummary is one row in the accounts table on the index
// page. Aggregate counts come from a LEFT JOIN so accounts with zero
// backups/sites still appear.
type adminAccountSummary struct {
	ID             int64
	Email          string
	Tier           string
	Status         string
	StripeCustomer string
	CreatedAt      string
	UpdatedAt      string
	SiteCount      int64
	BackupCount    int64
	TotalBytes     int64
	LastBackup     string // RFC3339, empty if never backed up
}

// HandleAdminIndex lists all Cloud accounts with summary stats.
// Single SQL query with aggregates; cheap at any realistic scale.
// Results default to newest-first (by created_at DESC) so recent
// signups are front-and-center.
func (c *CloudService) HandleAdminIndex(w http.ResponseWriter, r *http.Request) {
	rows, err := c.db.conn.Query(`
		SELECT
		  a.id, a.email, a.tier, a.status,
		  COALESCE(a.stripe_customer_id, ''),
		  a.created_at, a.updated_at,
		  COALESCE((SELECT COUNT(*) FROM cloud_sites s WHERE s.account_id = a.id), 0)   AS site_count,
		  COALESCE((SELECT COUNT(*) FROM cloud_backup_blobs b WHERE b.account_id = a.id), 0) AS backup_count,
		  COALESCE((SELECT SUM(size_bytes) FROM cloud_backup_blobs b WHERE b.account_id = a.id), 0) AS total_bytes,
		  COALESCE((SELECT MAX(uploaded_at) FROM cloud_backup_blobs b WHERE b.account_id = a.id), '') AS last_backup
		FROM cloud_accounts a
		ORDER BY a.created_at DESC
	`)
	if err != nil {
		log.Printf("admin index query: %v", err)
		http.Error(w, "query failed", 500)
		return
	}
	defer rows.Close()

	var accounts []adminAccountSummary
	for rows.Next() {
		var a adminAccountSummary
		if err := rows.Scan(&a.ID, &a.Email, &a.Tier, &a.Status,
			&a.StripeCustomer, &a.CreatedAt, &a.UpdatedAt,
			&a.SiteCount, &a.BackupCount, &a.TotalBytes, &a.LastBackup); err != nil {
			log.Printf("admin index scan: %v", err)
			http.Error(w, "scan failed", 500)
			return
		}
		accounts = append(accounts, a)
	}

	// System-wide totals. Cheap single-row query per table.
	var totalAccounts, totalBackups int64
	var totalStorage int64
	c.db.conn.QueryRow(`SELECT COUNT(*) FROM cloud_accounts`).Scan(&totalAccounts)
	c.db.conn.QueryRow(`SELECT COUNT(*) FROM cloud_backup_blobs`).Scan(&totalBackups)
	c.db.conn.QueryRow(`SELECT COALESCE(SUM(size_bytes), 0) FROM cloud_backup_blobs`).Scan(&totalStorage)

	data := struct {
		Accounts      []adminAccountSummary
		TotalAccounts int64
		TotalBackups  int64
		TotalStorage  int64
		Now           string
	}{
		Accounts:      accounts,
		TotalAccounts: totalAccounts,
		TotalBackups:  totalBackups,
		TotalStorage:  totalStorage,
		Now:           time.Now().UTC().Format(time.RFC3339),
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := adminIndexTmpl.Execute(w, data); err != nil {
		log.Printf("admin index render: %v", err)
	}
}

// adminAccountDetail is everything we show on the drill-down page.
// Structured so the template can iterate cleanly.
type adminAccountDetail struct {
	Account  adminAccountSummary
	Sites    []adminSiteRow
	Backups  []adminBackupRow
	Sessions []adminSessionRow
}

type adminSiteRow struct {
	ID          int64
	Slug        string
	DisplayName string
	CreatedAt   string
	BackupCount int64
	TotalBytes  int64
	LastBackup  string
}

type adminBackupRow struct {
	ID            int64
	UploadedAt    string
	SizeBytes     int64
	SiteSlug      string
	ClientVersion string
	Sha256Short   string // first 12 chars for readability
}

type adminSessionRow struct {
	Token     string // short, for readability; full token never shown
	CreatedAt string
	ExpiresAt string
	Expired   bool
}

// HandleAdminAccount shows everything we know about one account:
// summary stats, sites, recent backups, active sessions. ID comes
// from the {id} path parameter.
func (c *CloudService) HandleAdminAccount(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		http.Error(w, "bad account id", 400)
		return
	}

	// Summary row. Reuses the same shape as the index so the template
	// can share.
	var a adminAccountSummary
	err = c.db.conn.QueryRow(`
		SELECT
		  a.id, a.email, a.tier, a.status,
		  COALESCE(a.stripe_customer_id, ''),
		  a.created_at, a.updated_at,
		  COALESCE((SELECT COUNT(*) FROM cloud_sites s WHERE s.account_id = a.id), 0),
		  COALESCE((SELECT COUNT(*) FROM cloud_backup_blobs b WHERE b.account_id = a.id), 0),
		  COALESCE((SELECT SUM(size_bytes) FROM cloud_backup_blobs b WHERE b.account_id = a.id), 0),
		  COALESCE((SELECT MAX(uploaded_at) FROM cloud_backup_blobs b WHERE b.account_id = a.id), '')
		FROM cloud_accounts a
		WHERE a.id = ?
	`, id).Scan(&a.ID, &a.Email, &a.Tier, &a.Status, &a.StripeCustomer,
		&a.CreatedAt, &a.UpdatedAt, &a.SiteCount, &a.BackupCount,
		&a.TotalBytes, &a.LastBackup)
	if err != nil {
		http.Error(w, "account not found", 404)
		return
	}

	detail := adminAccountDetail{Account: a}

	// Sites with per-site aggregates. Empty slice is fine — the
	// template checks length.
	siteRows, err := c.db.conn.Query(`
		SELECT
		  s.id, s.slug, s.display_name, s.created_at,
		  COALESCE((SELECT COUNT(*) FROM cloud_backup_blobs b WHERE b.site_id = s.id), 0),
		  COALESCE((SELECT SUM(size_bytes) FROM cloud_backup_blobs b WHERE b.site_id = s.id), 0),
		  COALESCE((SELECT MAX(uploaded_at) FROM cloud_backup_blobs b WHERE b.site_id = s.id), '')
		FROM cloud_sites s WHERE s.account_id = ?
		ORDER BY s.id ASC
	`, id)
	if err == nil {
		defer siteRows.Close()
		for siteRows.Next() {
			var s adminSiteRow
			if err := siteRows.Scan(&s.ID, &s.Slug, &s.DisplayName, &s.CreatedAt,
				&s.BackupCount, &s.TotalBytes, &s.LastBackup); err == nil {
				detail.Sites = append(detail.Sites, s)
			}
		}
	}

	// Backups — cap at most 100 rows. If an account has more than
	// that we've got bigger problems (retention runs inline on
	// upload and should be keeping counts bounded).
	bRows, err := c.db.conn.Query(`
		SELECT b.id, b.uploaded_at, b.size_bytes,
		       COALESCE(s.slug, ''),
		       COALESCE(b.client_version, ''),
		       COALESCE(b.sha256_hex, '')
		FROM cloud_backup_blobs b
		LEFT JOIN cloud_sites s ON s.id = b.site_id
		WHERE b.account_id = ?
		ORDER BY b.uploaded_at DESC
		LIMIT 100
	`, id)
	if err == nil {
		defer bRows.Close()
		for bRows.Next() {
			var b adminBackupRow
			var fullSha string
			if err := bRows.Scan(&b.ID, &b.UploadedAt, &b.SizeBytes,
				&b.SiteSlug, &b.ClientVersion, &fullSha); err == nil {
				if len(fullSha) > 12 {
					b.Sha256Short = fullSha[:12]
				} else {
					b.Sha256Short = fullSha
				}
				detail.Backups = append(detail.Backups, b)
			}
		}
	}

	// Sessions — recent cookie-based sessions for the /cloud/ web
	// dashboard. License-key bearer sessions are implicit and not
	// stored in cloud_sessions, so this tab only shows web logins.
	now := time.Now().UTC()
	sRows, err := c.db.conn.Query(`
		SELECT session_token, created_at, expires_at
		FROM cloud_sessions
		WHERE account_id = ?
		ORDER BY created_at DESC
		LIMIT 20
	`, id)
	if err == nil {
		defer sRows.Close()
		for sRows.Next() {
			var s adminSessionRow
			var token string
			if err := sRows.Scan(&token, &s.CreatedAt, &s.ExpiresAt); err == nil {
				// Show only first 8 chars so if this page is ever
				// screenshotted/pasted, the full session token isn't
				// leaked. An admin with DB access can always see the
				// full token via SQL if needed.
				if len(token) > 8 {
					s.Token = token[:8] + "…"
				} else {
					s.Token = token
				}
				// Parse expires_at to mark expired sessions. Tolerate
				// both 'YYYY-MM-DD HH:MM:SS' (SQLite default) and
				// RFC3339.
				if t, perr := parseDBTime(s.ExpiresAt); perr == nil {
					s.Expired = t.Before(now)
				}
				detail.Sessions = append(detail.Sessions, s)
			}
		}
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := adminAccountTmpl.Execute(w, detail); err != nil {
		log.Printf("admin account render: %v", err)
	}
}

// parseDBTime parses SQLite's default datetime output OR RFC3339.
func parseDBTime(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, fmt.Errorf("empty")
	}
	// SQLite default: '2006-01-02 15:04:05'
	if t, err := time.Parse("2006-01-02 15:04:05", s); err == nil {
		return t, nil
	}
	return time.Parse(time.RFC3339, s)
}

// HandleAdminJSON returns the same index data as JSON, for scripting
// or a future CLI. Same auth, same data, different format.
func (c *CloudService) HandleAdminJSON(w http.ResponseWriter, r *http.Request) {
	rows, err := c.db.conn.Query(`
		SELECT
		  a.id, a.email, a.tier, a.status,
		  COALESCE(a.stripe_customer_id, ''),
		  a.created_at, a.updated_at,
		  COALESCE((SELECT COUNT(*) FROM cloud_sites s WHERE s.account_id = a.id), 0),
		  COALESCE((SELECT COUNT(*) FROM cloud_backup_blobs b WHERE b.account_id = a.id), 0),
		  COALESCE((SELECT SUM(size_bytes) FROM cloud_backup_blobs b WHERE b.account_id = a.id), 0),
		  COALESCE((SELECT MAX(uploaded_at) FROM cloud_backup_blobs b WHERE b.account_id = a.id), '')
		FROM cloud_accounts a
		ORDER BY a.created_at DESC
	`)
	if err != nil {
		http.Error(w, "query failed", 500)
		return
	}
	defer rows.Close()
	var out []adminAccountSummary
	for rows.Next() {
		var a adminAccountSummary
		if err := rows.Scan(&a.ID, &a.Email, &a.Tier, &a.Status,
			&a.StripeCustomer, &a.CreatedAt, &a.UpdatedAt,
			&a.SiteCount, &a.BackupCount, &a.TotalBytes, &a.LastBackup); err == nil {
			out = append(out, a)
		}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"accounts":  out,
		"rendered":  time.Now().UTC().Format(time.RFC3339),
	})
}

// --- Templates ------------------------------------------------------

// Shared template helpers. 'bytes' renders 123456 as '120.6 KB' etc.,
// 'ago' renders an RFC3339/SQLite timestamp as a relative duration,
// 'shortTime' renders just the date + HH:MM for compact tables.
var adminTmplFuncs = template.FuncMap{
	"bytes": func(n int64) string {
		if n == 0 {
			return "—"
		}
		mb := float64(n) / (1024 * 1024)
		if mb >= 1024 {
			return fmt.Sprintf("%.1f GB", mb/1024)
		}
		if mb >= 1 {
			return fmt.Sprintf("%.1f MB", mb)
		}
		return fmt.Sprintf("%.1f KB", float64(n)/1024)
	},
	"ago": func(s string) string {
		if s == "" {
			return "—"
		}
		t, err := parseDBTime(s)
		if err != nil {
			return s
		}
		d := time.Since(t)
		switch {
		case d < time.Minute:
			return "just now"
		case d < time.Hour:
			return fmt.Sprintf("%dm ago", int(d.Minutes()))
		case d < 24*time.Hour:
			return fmt.Sprintf("%dh ago", int(d.Hours()))
		case d < 365*24*time.Hour:
			return fmt.Sprintf("%dd ago", int(d.Hours()/24))
		default:
			return t.Format("2006-01-02")
		}
	},
	"shortTime": func(s string) string {
		if s == "" {
			return "—"
		}
		t, err := parseDBTime(s)
		if err != nil {
			return s
		}
		return t.Format("2006-01-02 15:04")
	},
	"truncEmail": func(s string) string {
		// Keep full email but truncate very long ones for table layout.
		if len(s) > 36 {
			return s[:33] + "…"
		}
		return s
	},
}

const adminBaseStyle = `
<style>
*{margin:0;padding:0;box-sizing:border-box}
body{background:#1a1410;color:#f0e6d3;font-family:'JetBrains Mono',ui-monospace,Menlo,monospace;line-height:1.5;padding:2rem;font-size:13px}
h1{font-size:1.3rem;color:#e8753a;letter-spacing:1px;margin-bottom:.3rem}
h2{font-size:.9rem;color:#c4a87a;letter-spacing:2px;text-transform:uppercase;margin:1.8rem 0 .8rem}
.stats{display:grid;grid-template-columns:repeat(4,1fr);gap:.8rem;margin:1rem 0 1.5rem;max-width:780px}
.stat{background:#241e18;border:1px solid #2e261e;padding:.8rem 1rem}
.stat .n{font-size:1.4rem;color:#d4a843;font-weight:600;line-height:1}
.stat .l{font-size:.65rem;color:#7a7060;letter-spacing:1.5px;text-transform:uppercase;margin-top:.4rem}
table{width:100%;border-collapse:collapse;margin-bottom:1rem;font-size:12px}
th{text-align:left;padding:.5rem .6rem;background:#2e261e;color:#c4a87a;font-weight:600;font-size:.65rem;letter-spacing:1.5px;text-transform:uppercase;border-bottom:1px solid #3e362e}
td{padding:.5rem .6rem;border-bottom:1px solid #241e18;color:#bfb5a3}
tr:hover td{background:#201a15}
tr.empty td{color:#7a7060;font-style:italic;text-align:center;padding:1rem}
td.num{text-align:right;font-variant-numeric:tabular-nums}
td.email{color:#f0e6d3}
td.tier{color:#d4a843}
a{color:#e8753a;text-decoration:none}
a:hover{color:#d4a843;text-decoration:underline}
.crumb{font-size:.75rem;color:#7a7060;margin-bottom:1rem}
.pill{display:inline-block;padding:.08rem .4rem;border-radius:2px;font-size:.65rem;letter-spacing:1px;text-transform:uppercase;font-weight:600}
.pill-ok{background:rgba(74,158,92,.2);color:#6bb87f}
.pill-warn{background:rgba(212,168,67,.2);color:#d4a843}
.pill-err{background:rgba(196,93,44,.2);color:#e8753a}
.pill-muted{background:#241e18;color:#7a7060}
.footer{margin-top:2rem;padding-top:1rem;border-top:1px solid #2e261e;color:#7a7060;font-size:.7rem}
</style>
`

var adminIndexTmpl = template.Must(template.New("idx").Funcs(adminTmplFuncs).Parse(`<!DOCTYPE html>
<html><head><meta charset="utf-8"><title>Stockyard Admin</title>` + adminBaseStyle + `</head><body>
<h1>Stockyard Admin</h1>
<div class="crumb">Cloud accounts · {{.Now}}</div>

<div class="stats">
  <div class="stat"><div class="n">{{.TotalAccounts}}</div><div class="l">Accounts</div></div>
  <div class="stat"><div class="n">{{.TotalBackups}}</div><div class="l">Backups</div></div>
  <div class="stat"><div class="n">{{bytes .TotalStorage}}</div><div class="l">Storage</div></div>
  <div class="stat"><div class="n">{{len .Accounts}}</div><div class="l">Listed</div></div>
</div>

<h2>Accounts</h2>
<table>
  <thead><tr>
    <th>ID</th><th>Email</th><th>Tier</th><th>Status</th>
    <th class="num">Sites</th><th class="num">Backups</th><th class="num">Storage</th>
    <th>Last backup</th><th>Joined</th>
  </tr></thead>
  <tbody>
  {{if not .Accounts}}
    <tr class="empty"><td colspan="9">No accounts yet.</td></tr>
  {{end}}
  {{range .Accounts}}
    <tr>
      <td><a href="/admin/account/{{.ID}}">#{{.ID}}</a></td>
      <td class="email">{{truncEmail .Email}}</td>
      <td class="tier">{{.Tier}}</td>
      <td>
        {{if eq .Status "active"}}<span class="pill pill-ok">active</span>
        {{else if eq .Status "past_due"}}<span class="pill pill-warn">past due</span>
        {{else if eq .Status "canceled"}}<span class="pill pill-err">canceled</span>
        {{else}}<span class="pill pill-muted">{{.Status}}</span>{{end}}
      </td>
      <td class="num">{{.SiteCount}}</td>
      <td class="num">{{.BackupCount}}</td>
      <td class="num">{{bytes .TotalBytes}}</td>
      <td>{{ago .LastBackup}}</td>
      <td>{{ago .CreatedAt}}</td>
    </tr>
  {{end}}
  </tbody>
</table>

<div class="footer">Read-only · No customer actions from this interface · <a href="/admin/json">json</a></div>
</body></html>`))

var adminAccountTmpl = template.Must(template.New("acc").Funcs(adminTmplFuncs).Parse(`<!DOCTYPE html>
<html><head><meta charset="utf-8"><title>Account #{{.Account.ID}} · Stockyard Admin</title>` + adminBaseStyle + `</head><body>
<div class="crumb"><a href="/admin/">← accounts</a> · account #{{.Account.ID}}</div>
<h1>{{.Account.Email}}</h1>
<div class="crumb">
  <span class="pill pill-muted">{{.Account.Tier}}</span>
  {{if eq .Account.Status "active"}}<span class="pill pill-ok">active</span>
  {{else if eq .Account.Status "past_due"}}<span class="pill pill-warn">past due</span>
  {{else if eq .Account.Status "canceled"}}<span class="pill pill-err">canceled</span>
  {{else}}<span class="pill pill-muted">{{.Account.Status}}</span>{{end}}
  · joined {{ago .Account.CreatedAt}} · updated {{ago .Account.UpdatedAt}}
  {{if .Account.StripeCustomer}} · stripe: <code>{{.Account.StripeCustomer}}</code>{{end}}
</div>

<div class="stats">
  <div class="stat"><div class="n">{{.Account.SiteCount}}</div><div class="l">Sites</div></div>
  <div class="stat"><div class="n">{{.Account.BackupCount}}</div><div class="l">Backups</div></div>
  <div class="stat"><div class="n">{{bytes .Account.TotalBytes}}</div><div class="l">Storage</div></div>
  <div class="stat"><div class="n">{{if .Account.LastBackup}}{{ago .Account.LastBackup}}{{else}}—{{end}}</div><div class="l">Last backup</div></div>
</div>

<h2>Sites</h2>
<table>
  <thead><tr>
    <th>ID</th><th>Slug</th><th>Name</th>
    <th class="num">Backups</th><th class="num">Storage</th>
    <th>Last backup</th><th>Created</th>
  </tr></thead>
  <tbody>
  {{if not .Sites}}
    <tr class="empty"><td colspan="7">No sites. Backend falls through to 'default' on upload.</td></tr>
  {{end}}
  {{range .Sites}}
    <tr>
      <td>#{{.ID}}</td>
      <td><code>{{.Slug}}</code></td>
      <td>{{.DisplayName}}</td>
      <td class="num">{{.BackupCount}}</td>
      <td class="num">{{bytes .TotalBytes}}</td>
      <td>{{ago .LastBackup}}</td>
      <td>{{ago .CreatedAt}}</td>
    </tr>
  {{end}}
  </tbody>
</table>

<h2>Backups <span style="font-size:.6rem;color:#7a7060;letter-spacing:.5px;font-weight:400">(most recent 100)</span></h2>
<table>
  <thead><tr>
    <th>ID</th><th>Uploaded</th><th>Site</th>
    <th class="num">Size</th><th>Client</th><th>SHA-256</th>
  </tr></thead>
  <tbody>
  {{if not .Backups}}
    <tr class="empty"><td colspan="6">No backups yet.</td></tr>
  {{end}}
  {{range .Backups}}
    <tr>
      <td>#{{.ID}}</td>
      <td>{{shortTime .UploadedAt}}</td>
      <td><code>{{if .SiteSlug}}{{.SiteSlug}}{{else}}?{{end}}</code></td>
      <td class="num">{{bytes .SizeBytes}}</td>
      <td>{{if .ClientVersion}}{{.ClientVersion}}{{else}}—{{end}}</td>
      <td><code style="font-size:.65rem;color:#7a7060">{{.Sha256Short}}</code></td>
    </tr>
  {{end}}
  </tbody>
</table>

<h2>Web sessions <span style="font-size:.6rem;color:#7a7060;letter-spacing:.5px;font-weight:400">(most recent 20)</span></h2>
<table>
  <thead><tr><th>Token</th><th>Created</th><th>Expires</th><th>State</th></tr></thead>
  <tbody>
  {{if not .Sessions}}
    <tr class="empty"><td colspan="4">No web sessions. (Bearer-auth from the desktop app does not create session rows.)</td></tr>
  {{end}}
  {{range .Sessions}}
    <tr>
      <td><code>{{.Token}}</code></td>
      <td>{{shortTime .CreatedAt}}</td>
      <td>{{shortTime .ExpiresAt}}</td>
      <td>{{if .Expired}}<span class="pill pill-muted">expired</span>{{else}}<span class="pill pill-ok">active</span>{{end}}</td>
    </tr>
  {{end}}
  </tbody>
</table>

<div class="footer">Read-only · To mutate anything, SSH Railway + SQL</div>
</body></html>`))

// Silence unused-import warnings during incremental development.
// strings is used via template funcs which the linter can't see in
// some go versions.
var _ = strings.TrimSpace
