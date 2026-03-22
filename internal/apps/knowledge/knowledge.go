// Package knowledge implements the Knowledge Base marketplace and management app.
package knowledge

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// App implements the platform.App interface for the knowledge app.
type App struct {
	conn  *sql.DB
	audit func(string, string, string, string, any)
}

// New creates a new knowledge App backed by the given database.
func New(conn *sql.DB) *App { return &App{conn: conn} }

// SetAuditor wires the trust audit function for recording knowledge events.
func (a *App) SetAuditor(fn func(string, string, string, string, any)) {
	a.audit = fn
}

func (a *App) auditEvent(action, resource string, detail any) {
	if a.audit != nil {
		go a.audit("knowledge_event", "knowledge", resource, action, detail)
	}
}

func (a *App) Name() string        { return "knowledge" }
func (a *App) Description() string { return "Knowledge base marketplace and management" }

func (a *App) Migrate(conn *sql.DB) error {
	a.conn = conn
	_, err := conn.Exec(knowledgeSchema)
	if err != nil {
		return err
	}
	// Backfill FTS5 index from existing entries (rebuild syncs from content table)
	conn.Exec("INSERT INTO knowledge_entries_fts(knowledge_entries_fts) VALUES('rebuild')")
	log.Printf("[knowledge] migrations applied")
	return nil
}

const knowledgeSchema = `
CREATE TABLE IF NOT EXISTS knowledge_bases (
    id TEXT PRIMARY KEY,
    author_id TEXT DEFAULT 'default',
    title TEXT,
    description TEXT,
    domain TEXT,
    entries_count INTEGER DEFAULT 0,
    rating REAL DEFAULT 0,
    installs INTEGER DEFAULT 0,
    price_per_query_cents INTEGER DEFAULT 0,
    status TEXT DEFAULT 'active',
    created_at TEXT,
    updated_at TEXT
);

CREATE TABLE IF NOT EXISTS knowledge_entries (
    id TEXT PRIMARY KEY,
    kb_id TEXT,
    fact TEXT,
    source TEXT,
    verified INTEGER DEFAULT 0,
    verification_count INTEGER DEFAULT 0,
    disputed INTEGER DEFAULT 0,
    created_at TEXT,
    expires_at TEXT
);

CREATE VIRTUAL TABLE IF NOT EXISTS knowledge_entries_fts USING fts5(
    fact, source, content=knowledge_entries, content_rowid=rowid
);

CREATE TABLE IF NOT EXISTS knowledge_subscriptions (
    app_id TEXT,
    kb_id TEXT,
    created_at TEXT,
    PRIMARY KEY(app_id, kb_id)
);

CREATE TABLE IF NOT EXISTS knowledge_watches (
    id TEXT PRIMARY KEY,
    kb_id TEXT,
    url TEXT,
    check_interval_hours INTEGER DEFAULT 24,
    last_checked TEXT,
    last_hash TEXT,
    created_at TEXT
);

CREATE TABLE IF NOT EXISTS embedding_cache (
    text_hash TEXT PRIMARY KEY,
    trigrams TEXT,
    created_at TEXT
);

CREATE TABLE IF NOT EXISTS knowledge_earnings (
    id TEXT PRIMARY KEY,
    kb_id TEXT NOT NULL,
    author_id TEXT NOT NULL,
    amount_cents INTEGER DEFAULT 0,
    fee_cents INTEGER DEFAULT 0,
    net_cents INTEGER DEFAULT 0,
    query TEXT,
    created_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_kb_earnings_kb ON knowledge_earnings(kb_id);
CREATE INDEX IF NOT EXISTS idx_kb_earnings_author ON knowledge_earnings(author_id);
`

func (a *App) RegisterRoutes(mux *http.ServeMux) {
	// Knowledge bases
	mux.HandleFunc("POST /api/knowledge/bases", a.handleCreateBase)
	mux.HandleFunc("GET /api/knowledge/bases", a.handleListBases)
	mux.HandleFunc("GET /api/knowledge/bases/{id}", a.handleGetBase)
	mux.HandleFunc("PUT /api/knowledge/bases/{id}", a.handleUpdateBase)
	mux.HandleFunc("DELETE /api/knowledge/bases/{id}", a.handleDeleteBase)

	// Entries
	mux.HandleFunc("POST /api/knowledge/bases/{id}/entries", a.handleAddEntry)
	mux.HandleFunc("GET /api/knowledge/bases/{id}/entries", a.handleListEntries)
	mux.HandleFunc("PUT /api/knowledge/entries/{id}", a.handleUpdateEntry)
	mux.HandleFunc("DELETE /api/knowledge/entries/{id}", a.handleDeleteEntry)

	// Subscribe and query
	mux.HandleFunc("POST /api/knowledge/bases/{id}/subscribe", a.handleSubscribe)
	mux.HandleFunc("GET /api/knowledge/bases/{id}/query", a.handleQuery)

	// Verification and disputes
	mux.HandleFunc("POST /api/knowledge/entries/{id}/verify", a.handleVerifyEntry)
	mux.HandleFunc("POST /api/knowledge/entries/{id}/dispute", a.handleDisputeEntry)

	// Marketplace
	mux.HandleFunc("GET /api/knowledge/marketplace", a.handleMarketplace)

	// Watches
	mux.HandleFunc("POST /api/knowledge/bases/{id}/watches", a.handleAddWatch)
	mux.HandleFunc("GET /api/knowledge/bases/{id}/watches", a.handleListWatches)
	mux.HandleFunc("DELETE /api/knowledge/watches/{id}", a.handleDeleteWatch)

	// Embeddings
	mux.HandleFunc("POST /api/knowledge/embeddings", a.handleEmbeddings)

	log.Printf("[knowledge] routes registered")
}

// --- Helpers ---

func genID(prefix string) string {
	b := make([]byte, 6)
	rand.Read(b)
	return prefix + hex.EncodeToString(b)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func now() string {
	return time.Now().Format(time.RFC3339)
}

// --- Knowledge Bases ---

func (a *App) handleCreateBase(w http.ResponseWriter, r *http.Request) {
	var req struct {
		AuthorID         string `json:"author_id"`
		Title            string `json:"title"`
		Description      string `json:"description"`
		Domain           string `json:"domain"`
		PricePerQueryCts int    `json:"price_per_query_cents"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	id := genID("kb_")
	ts := now()
	if req.AuthorID == "" {
		req.AuthorID = "default"
	}

	_, err := a.conn.Exec(
		"INSERT INTO knowledge_bases (id, author_id, title, description, domain, price_per_query_cents, created_at, updated_at) VALUES (?,?,?,?,?,?,?,?)",
		id, req.AuthorID, req.Title, req.Description, req.Domain, req.PricePerQueryCts, ts, ts,
	)
	if err != nil {
		w.WriteHeader(500)
		writeJSON(w, map[string]string{"error": err.Error()})
		return
	}
	a.auditEvent("base_created", id, map[string]any{"title": req.Title})
	writeJSON(w, map[string]any{"status": "created", "id": id})
}

func (a *App) handleListBases(w http.ResponseWriter, r *http.Request) {
	rows, err := a.conn.Query("SELECT id, author_id, title, description, domain, entries_count, rating, installs, price_per_query_cents, status, created_at, updated_at FROM knowledge_bases ORDER BY created_at DESC")
	if err != nil || rows == nil {
		writeJSON(w, map[string]any{"bases": []any{}})
		return
	}
	defer rows.Close()
	var bases []map[string]any
	for rows.Next() {
		var id, authorID, title, desc, domain, status, created, updated string
		var entriesCount, installs, price int
		var rating float64
		rows.Scan(&id, &authorID, &title, &desc, &domain, &entriesCount, &rating, &installs, &price, &status, &created, &updated)
		bases = append(bases, map[string]any{
			"id": id, "author_id": authorID, "title": title, "description": desc,
			"domain": domain, "entries_count": entriesCount, "rating": rating,
			"installs": installs, "price_per_query_cents": price, "status": status,
			"created_at": created, "updated_at": updated,
		})
	}
	writeJSON(w, map[string]any{"bases": bases, "count": len(bases)})
}

func (a *App) handleGetBase(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var authorID, title, desc, domain, status, created, updated string
	var entriesCount, installs, price int
	var rating float64
	err := a.conn.QueryRow(
		"SELECT author_id, title, description, domain, entries_count, rating, installs, price_per_query_cents, status, created_at, updated_at FROM knowledge_bases WHERE id = ?", id,
	).Scan(&authorID, &title, &desc, &domain, &entriesCount, &rating, &installs, &price, &status, &created, &updated)
	if err != nil {
		w.WriteHeader(404)
		writeJSON(w, map[string]string{"error": "knowledge base not found"})
		return
	}
	writeJSON(w, map[string]any{
		"id": id, "author_id": authorID, "title": title, "description": desc,
		"domain": domain, "entries_count": entriesCount, "rating": rating,
		"installs": installs, "price_per_query_cents": price, "status": status,
		"created_at": created, "updated_at": updated,
	})
}

func (a *App) handleUpdateBase(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req struct {
		Title       *string `json:"title"`
		Description *string `json:"description"`
		Domain      *string `json:"domain"`
		Price       *int    `json:"price_per_query_cents"`
		Status      *string `json:"status"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	// Check exists
	var existing string
	if err := a.conn.QueryRow("SELECT id FROM knowledge_bases WHERE id = ?", id).Scan(&existing); err != nil {
		w.WriteHeader(404)
		writeJSON(w, map[string]string{"error": "knowledge base not found"})
		return
	}

	if req.Title != nil {
		a.conn.Exec("UPDATE knowledge_bases SET title = ? WHERE id = ?", *req.Title, id)
	}
	if req.Description != nil {
		a.conn.Exec("UPDATE knowledge_bases SET description = ? WHERE id = ?", *req.Description, id)
	}
	if req.Domain != nil {
		a.conn.Exec("UPDATE knowledge_bases SET domain = ? WHERE id = ?", *req.Domain, id)
	}
	if req.Price != nil {
		a.conn.Exec("UPDATE knowledge_bases SET price_per_query_cents = ? WHERE id = ?", *req.Price, id)
	}
	if req.Status != nil {
		a.conn.Exec("UPDATE knowledge_bases SET status = ? WHERE id = ?", *req.Status, id)
	}
	a.conn.Exec("UPDATE knowledge_bases SET updated_at = ? WHERE id = ?", now(), id)

	a.auditEvent("base_updated", id, nil)
	writeJSON(w, map[string]any{"status": "updated", "id": id})
}

func (a *App) handleDeleteBase(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	res, _ := a.conn.Exec("DELETE FROM knowledge_bases WHERE id = ?", id)
	affected, _ := res.RowsAffected()
	if affected == 0 {
		w.WriteHeader(404)
		writeJSON(w, map[string]string{"error": "knowledge base not found"})
		return
	}
	a.conn.Exec("DELETE FROM knowledge_entries WHERE kb_id = ?", id)
	a.conn.Exec("DELETE FROM knowledge_subscriptions WHERE kb_id = ?", id)
	a.conn.Exec("DELETE FROM knowledge_watches WHERE kb_id = ?", id)
	a.auditEvent("base_deleted", id, nil)
	writeJSON(w, map[string]any{"status": "deleted", "id": id})
}

// --- Entries ---

func (a *App) handleAddEntry(w http.ResponseWriter, r *http.Request) {
	kbID := r.PathValue("id")

	// Verify KB exists
	var existing string
	if err := a.conn.QueryRow("SELECT id FROM knowledge_bases WHERE id = ?", kbID).Scan(&existing); err != nil {
		w.WriteHeader(404)
		writeJSON(w, map[string]string{"error": "knowledge base not found"})
		return
	}

	var req struct {
		Fact      string `json:"fact"`
		Source    string `json:"source"`
		ExpiresAt string `json:"expires_at"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	id := genID("ke_")
	ts := now()
	_, err := a.conn.Exec(
		"INSERT INTO knowledge_entries (id, kb_id, fact, source, created_at, expires_at) VALUES (?,?,?,?,?,?)",
		id, kbID, req.Fact, req.Source, ts, req.ExpiresAt,
	)
	if err != nil {
		w.WriteHeader(500)
		writeJSON(w, map[string]string{"error": err.Error()})
		return
	}
	// Populate FTS5 index
	a.conn.Exec("INSERT INTO knowledge_entries_fts(rowid, fact, source) SELECT rowid, fact, source FROM knowledge_entries WHERE id = ?", id)
	a.conn.Exec("UPDATE knowledge_bases SET entries_count = entries_count + 1, updated_at = ? WHERE id = ?", ts, kbID)
	writeJSON(w, map[string]any{"status": "created", "id": id, "kb_id": kbID})
}

func (a *App) handleListEntries(w http.ResponseWriter, r *http.Request) {
	kbID := r.PathValue("id")
	rows, err := a.conn.Query(
		"SELECT id, fact, source, verified, verification_count, disputed, created_at, expires_at FROM knowledge_entries WHERE kb_id = ? ORDER BY created_at DESC", kbID,
	)
	if err != nil || rows == nil {
		writeJSON(w, map[string]any{"entries": []any{}})
		return
	}
	defer rows.Close()
	var entries []map[string]any
	for rows.Next() {
		var id, fact, source, created, expires string
		var verified, verificationCount, disputed int
		rows.Scan(&id, &fact, &source, &verified, &verificationCount, &disputed, &created, &expires)
		entries = append(entries, map[string]any{
			"id": id, "kb_id": kbID, "fact": fact, "source": source,
			"verified": verified, "verification_count": verificationCount,
			"disputed": disputed, "created_at": created, "expires_at": expires,
		})
	}
	writeJSON(w, map[string]any{"entries": entries, "count": len(entries)})
}

func (a *App) handleUpdateEntry(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req struct {
		Fact      *string `json:"fact"`
		Source    *string `json:"source"`
		ExpiresAt *string `json:"expires_at"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	var existing string
	if err := a.conn.QueryRow("SELECT id FROM knowledge_entries WHERE id = ?", id).Scan(&existing); err != nil {
		w.WriteHeader(404)
		writeJSON(w, map[string]string{"error": "entry not found"})
		return
	}

	if req.Fact != nil {
		a.conn.Exec("UPDATE knowledge_entries SET fact = ? WHERE id = ?", *req.Fact, id)
	}
	if req.Source != nil {
		a.conn.Exec("UPDATE knowledge_entries SET source = ? WHERE id = ?", *req.Source, id)
	}
	if req.ExpiresAt != nil {
		a.conn.Exec("UPDATE knowledge_entries SET expires_at = ? WHERE id = ?", *req.ExpiresAt, id)
	}
	writeJSON(w, map[string]any{"status": "updated", "id": id})
}

func (a *App) handleDeleteEntry(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var kbID string
	a.conn.QueryRow("SELECT kb_id FROM knowledge_entries WHERE id = ?", id).Scan(&kbID)

	// Remove from FTS5 index before deleting the row
	a.conn.Exec("DELETE FROM knowledge_entries_fts WHERE rowid = (SELECT rowid FROM knowledge_entries WHERE id = ?)", id)

	res, _ := a.conn.Exec("DELETE FROM knowledge_entries WHERE id = ?", id)
	affected, _ := res.RowsAffected()
	if affected == 0 {
		w.WriteHeader(404)
		writeJSON(w, map[string]string{"error": "entry not found"})
		return
	}
	if kbID != "" {
		a.conn.Exec("UPDATE knowledge_bases SET entries_count = entries_count - 1, updated_at = ? WHERE id = ?", now(), kbID)
	}
	writeJSON(w, map[string]any{"status": "deleted", "id": id})
}

// --- Subscribe ---

func (a *App) handleSubscribe(w http.ResponseWriter, r *http.Request) {
	kbID := r.PathValue("id")
	var req struct {
		AppID string `json:"app_id"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	ts := now()
	_, err := a.conn.Exec(
		"INSERT OR REPLACE INTO knowledge_subscriptions (app_id, kb_id, created_at) VALUES (?,?,?)",
		req.AppID, kbID, ts,
	)
	if err != nil {
		w.WriteHeader(500)
		writeJSON(w, map[string]string{"error": err.Error()})
		return
	}
	a.conn.Exec("UPDATE knowledge_bases SET installs = installs + 1, updated_at = ? WHERE id = ?", ts, kbID)
	a.auditEvent("subscribed", kbID, map[string]any{"app_id": req.AppID})
	writeJSON(w, map[string]any{"status": "subscribed", "app_id": req.AppID, "kb_id": kbID})
}

// --- Query (keyword retrieval) ---

func (a *App) handleQuery(w http.ResponseWriter, r *http.Request) {
	kbID := r.PathValue("id")
	q := r.URL.Query().Get("q")
	limitStr := r.URL.Query().Get("limit")
	limit := 10
	if limitStr != "" {
		if v, err := strconv.Atoi(limitStr); err == nil && v > 0 {
			limit = v
		}
	}

	if q == "" {
		w.WriteHeader(400)
		writeJSON(w, map[string]string{"error": "q parameter required"})
		return
	}

	// Try FTS5 full-text search first, fall back to LIKE
	rows, err := a.conn.Query(
		`SELECT e.id, e.fact, e.source, e.verified, e.verification_count, e.disputed
		FROM knowledge_entries e
		JOIN knowledge_entries_fts fts ON fts.rowid = e.rowid
		WHERE e.kb_id = ? AND knowledge_entries_fts MATCH ?
		ORDER BY rank LIMIT ?`,
		kbID, q, limit,
	)
	if err != nil || rows == nil {
		// FTS5 may not be populated for old entries — fall back to LIKE
		pattern := "%" + q + "%"
		rows, err = a.conn.Query(
			"SELECT id, fact, source, verified, verification_count, disputed FROM knowledge_entries WHERE kb_id = ? AND (fact LIKE ? OR source LIKE ?) LIMIT ?",
			kbID, pattern, pattern, limit,
		)
	}
	if err != nil || rows == nil {
		writeJSON(w, map[string]any{"results": []any{}, "query": q})
		return
	}
	defer rows.Close()
	var results []map[string]any
	for rows.Next() {
		var id, fact, source string
		var verified, verificationCount, disputed int
		rows.Scan(&id, &fact, &source, &verified, &verificationCount, &disputed)
		results = append(results, map[string]any{
			"id": id, "fact": fact, "source": source,
			"verified": verified, "verification_count": verificationCount, "disputed": disputed,
		})
	}
	writeJSON(w, map[string]any{"results": results, "count": len(results), "query": q, "kb_id": kbID})
}

// --- Verify and Dispute ---

func (a *App) handleVerifyEntry(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	res, _ := a.conn.Exec("UPDATE knowledge_entries SET verification_count = verification_count + 1, verified = 1 WHERE id = ?", id)
	affected, _ := res.RowsAffected()
	if affected == 0 {
		w.WriteHeader(404)
		writeJSON(w, map[string]string{"error": "entry not found"})
		return
	}
	writeJSON(w, map[string]any{"status": "verified", "id": id})
}

func (a *App) handleDisputeEntry(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	res, _ := a.conn.Exec("UPDATE knowledge_entries SET disputed = 1 WHERE id = ?", id)
	affected, _ := res.RowsAffected()
	if affected == 0 {
		w.WriteHeader(404)
		writeJSON(w, map[string]string{"error": "entry not found"})
		return
	}
	writeJSON(w, map[string]any{"status": "disputed", "id": id})
}

// --- Marketplace ---

func (a *App) handleMarketplace(w http.ResponseWriter, r *http.Request) {
	domain := r.URL.Query().Get("domain")
	var rows *sql.Rows
	var err error
	if domain != "" {
		rows, err = a.conn.Query(
			"SELECT id, author_id, title, description, domain, entries_count, rating, installs, price_per_query_cents, status FROM knowledge_bases WHERE status = 'active' AND domain = ? ORDER BY installs DESC",
			domain,
		)
	} else {
		rows, err = a.conn.Query(
			"SELECT id, author_id, title, description, domain, entries_count, rating, installs, price_per_query_cents, status FROM knowledge_bases WHERE status = 'active' ORDER BY installs DESC",
		)
	}
	if err != nil || rows == nil {
		writeJSON(w, map[string]any{"bases": []any{}})
		return
	}
	defer rows.Close()
	var bases []map[string]any
	for rows.Next() {
		var id, authorID, title, desc, dom, status string
		var entriesCount, installs, price int
		var rating float64
		rows.Scan(&id, &authorID, &title, &desc, &dom, &entriesCount, &rating, &installs, &price, &status)
		bases = append(bases, map[string]any{
			"id": id, "author_id": authorID, "title": title, "description": desc,
			"domain": dom, "entries_count": entriesCount, "rating": rating,
			"installs": installs, "price_per_query_cents": price, "status": status,
		})
	}
	writeJSON(w, map[string]any{"bases": bases, "count": len(bases), "domain": domain})
}

// --- Watches ---

func (a *App) handleAddWatch(w http.ResponseWriter, r *http.Request) {
	kbID := r.PathValue("id")
	var req struct {
		URL                string `json:"url"`
		CheckIntervalHours int    `json:"check_interval_hours"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	id := genID("kw_")
	ts := now()
	interval := req.CheckIntervalHours
	if interval <= 0 {
		interval = 24
	}
	_, err := a.conn.Exec(
		"INSERT INTO knowledge_watches (id, kb_id, url, check_interval_hours, created_at) VALUES (?,?,?,?,?)",
		id, kbID, req.URL, interval, ts,
	)
	if err != nil {
		w.WriteHeader(500)
		writeJSON(w, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, map[string]any{"status": "created", "id": id, "kb_id": kbID})
}

func (a *App) handleListWatches(w http.ResponseWriter, r *http.Request) {
	kbID := r.PathValue("id")
	rows, err := a.conn.Query(
		"SELECT id, url, check_interval_hours, last_checked, last_hash, created_at FROM knowledge_watches WHERE kb_id = ? ORDER BY created_at DESC", kbID,
	)
	if err != nil || rows == nil {
		writeJSON(w, map[string]any{"watches": []any{}})
		return
	}
	defer rows.Close()
	var watches []map[string]any
	for rows.Next() {
		var id, url, lastChecked, lastHash, created string
		var interval int
		rows.Scan(&id, &url, &interval, &lastChecked, &lastHash, &created)
		watches = append(watches, map[string]any{
			"id": id, "kb_id": kbID, "url": url, "check_interval_hours": interval,
			"last_checked": lastChecked, "last_hash": lastHash, "created_at": created,
		})
	}
	writeJSON(w, map[string]any{"watches": watches, "count": len(watches)})
}

func (a *App) handleDeleteWatch(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	res, _ := a.conn.Exec("DELETE FROM knowledge_watches WHERE id = ?", id)
	affected, _ := res.RowsAffected()
	if affected == 0 {
		w.WriteHeader(404)
		writeJSON(w, map[string]string{"error": "watch not found"})
		return
	}
	writeJSON(w, map[string]any{"status": "deleted", "id": id})
}

// --- Embeddings (trigram-based) ---

func (a *App) handleEmbeddings(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Text string `json:"text"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	if req.Text == "" {
		w.WriteHeader(400)
		writeJSON(w, map[string]string{"error": "text required"})
		return
	}

	// Generate text hash
	hash := sha256.Sum256([]byte(req.Text))
	textHash := hex.EncodeToString(hash[:])

	// Check cache
	var cached string
	if err := a.conn.QueryRow("SELECT trigrams FROM embedding_cache WHERE text_hash = ?", textHash).Scan(&cached); err == nil {
		writeJSON(w, map[string]any{"text_hash": textHash, "trigrams": cached, "cached": true})
		return
	}

	// Generate trigrams
	trigrams := generateTrigrams(strings.ToLower(req.Text))
	trigramJSON, _ := json.Marshal(trigrams)

	a.conn.Exec(
		"INSERT INTO embedding_cache (text_hash, trigrams, created_at) VALUES (?,?,?)",
		textHash, string(trigramJSON), now(),
	)

	writeJSON(w, map[string]any{"text_hash": textHash, "trigrams": string(trigramJSON), "cached": false})
}

// generateTrigrams produces character-level trigrams from text.
func generateTrigrams(text string) []string {
	text = strings.TrimSpace(text)
	if len(text) < 3 {
		return []string{text}
	}
	seen := make(map[string]bool)
	var trigrams []string
	runes := []rune(text)
	for i := 0; i <= len(runes)-3; i++ {
		tri := string(runes[i : i+3])
		if !seen[tri] {
			seen[tri] = true
			trigrams = append(trigrams, tri)
		}
	}
	return trigrams
}
