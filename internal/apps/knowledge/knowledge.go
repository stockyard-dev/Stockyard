// Package knowledge implements the Knowledge marketplace and fact base.
package knowledge

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"log"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type App struct {
	conn  *sql.DB
	audit func(string, string, string, string, any)
}

func New(conn *sql.DB) *App { return &App{conn: conn} }

func (a *App) Name() string        { return "knowledge" }
func (a *App) Description() string { return "Knowledge marketplace and fact base" }

func (a *App) SetAuditor(fn func(string, string, string, string, any)) { a.audit = fn }

func (a *App) Migrate(conn *sql.DB) error {
	a.conn = conn
	_, err := conn.Exec(knowledgeSchema)
	if err != nil {
		return err
	}
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

CREATE INDEX IF NOT EXISTS idx_knowledge_entries_kb ON knowledge_entries(kb_id);
CREATE INDEX IF NOT EXISTS idx_knowledge_bases_domain ON knowledge_bases(domain);
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
	mux.HandleFunc("DELETE /api/knowledge/bases/{id}/entries/{eid}", a.handleDeleteEntry)

	// Subscribe
	mux.HandleFunc("POST /api/knowledge/bases/{id}/subscribe", a.handleSubscribe)

	// Query
	mux.HandleFunc("GET /api/knowledge/bases/{id}/query", a.handleQuery)

	// Verify / Dispute
	mux.HandleFunc("POST /api/knowledge/entries/{id}/verify", a.handleVerify)
	mux.HandleFunc("POST /api/knowledge/entries/{id}/dispute", a.handleDispute)

	// Marketplace
	mux.HandleFunc("GET /api/knowledge/marketplace", a.handleMarketplace)

	// Watches
	mux.HandleFunc("POST /api/knowledge/bases/{id}/watches", a.handleCreateWatch)
	mux.HandleFunc("GET /api/knowledge/bases/{id}/watches", a.handleListWatches)

	// Embeddings
	mux.HandleFunc("POST /api/embeddings", a.handleEmbeddings)

	log.Printf("[knowledge] routes registered")
}

func genID(prefix string) string {
	b := make([]byte, 6)
	rand.Read(b)
	return prefix + hex.EncodeToString(b)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

// --- Knowledge Bases ---

func (a *App) handleCreateBase(w http.ResponseWriter, r *http.Request) {
	var req struct {
		AuthorID          string `json:"author_id"`
		Title             string `json:"title"`
		Description       string `json:"description"`
		Domain            string `json:"domain"`
		PricePerQueryCents int   `json:"price_per_query_cents"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	if req.AuthorID == "" {
		req.AuthorID = "default"
	}

	id := genID("kb_")
	now := time.Now().UTC().Format(time.RFC3339)

	_, err := a.conn.Exec(
		"INSERT INTO knowledge_bases (id, author_id, title, description, domain, price_per_query_cents, created_at, updated_at) VALUES (?,?,?,?,?,?,?,?)",
		id, req.AuthorID, req.Title, req.Description, req.Domain, req.PricePerQueryCents, now, now,
	)
	if err != nil {
		w.WriteHeader(500)
		writeJSON(w, map[string]string{"error": "failed to create knowledge base"})
		return
	}

	writeJSON(w, map[string]any{"id": id, "status": "created"})
}

func (a *App) handleListBases(w http.ResponseWriter, r *http.Request) {
	rows, err := a.conn.Query("SELECT id, author_id, title, description, domain, entries_count, rating, installs, price_per_query_cents, status, created_at, updated_at FROM knowledge_bases ORDER BY created_at DESC")
	if err != nil {
		writeJSON(w, map[string]any{"bases": []any{}})
		return
	}
	defer rows.Close()

	var bases []map[string]any
	for rows.Next() {
		var id, authorID, title, desc, domain, status, createdAt, updatedAt string
		var entriesCount, installs, price int
		var rating float64
		rows.Scan(&id, &authorID, &title, &desc, &domain, &entriesCount, &rating, &installs, &price, &status, &createdAt, &updatedAt)
		bases = append(bases, map[string]any{
			"id": id, "author_id": authorID, "title": title, "description": desc,
			"domain": domain, "entries_count": entriesCount, "rating": rating,
			"installs": installs, "price_per_query_cents": price, "status": status,
			"created_at": createdAt, "updated_at": updatedAt,
		})
	}
	if bases == nil {
		bases = []map[string]any{}
	}
	writeJSON(w, map[string]any{"bases": bases, "count": len(bases)})
}

func (a *App) handleGetBase(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var authorID, title, desc, domain, status, createdAt, updatedAt string
	var entriesCount, installs, price int
	var rating float64
	err := a.conn.QueryRow(
		"SELECT author_id, title, description, domain, entries_count, rating, installs, price_per_query_cents, status, created_at, updated_at FROM knowledge_bases WHERE id = ?", id,
	).Scan(&authorID, &title, &desc, &domain, &entriesCount, &rating, &installs, &price, &status, &createdAt, &updatedAt)
	if err != nil {
		w.WriteHeader(404)
		writeJSON(w, map[string]string{"error": "knowledge base not found"})
		return
	}
	writeJSON(w, map[string]any{
		"id": id, "author_id": authorID, "title": title, "description": desc,
		"domain": domain, "entries_count": entriesCount, "rating": rating,
		"installs": installs, "price_per_query_cents": price, "status": status,
		"created_at": createdAt, "updated_at": updatedAt,
	})
}

func (a *App) handleUpdateBase(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req struct {
		Title             *string  `json:"title"`
		Description       *string  `json:"description"`
		Domain            *string  `json:"domain"`
		PricePerQueryCents *int    `json:"price_per_query_cents"`
		Status            *string  `json:"status"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	now := time.Now().UTC().Format(time.RFC3339)
	if req.Title != nil {
		a.conn.Exec("UPDATE knowledge_bases SET title = ?, updated_at = ? WHERE id = ?", *req.Title, now, id)
	}
	if req.Description != nil {
		a.conn.Exec("UPDATE knowledge_bases SET description = ?, updated_at = ? WHERE id = ?", *req.Description, now, id)
	}
	if req.Domain != nil {
		a.conn.Exec("UPDATE knowledge_bases SET domain = ?, updated_at = ? WHERE id = ?", *req.Domain, now, id)
	}
	if req.PricePerQueryCents != nil {
		a.conn.Exec("UPDATE knowledge_bases SET price_per_query_cents = ?, updated_at = ? WHERE id = ?", *req.PricePerQueryCents, now, id)
	}
	if req.Status != nil {
		a.conn.Exec("UPDATE knowledge_bases SET status = ?, updated_at = ? WHERE id = ?", *req.Status, now, id)
	}

	writeJSON(w, map[string]any{"id": id, "status": "updated"})
}

func (a *App) handleDeleteBase(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	result, _ := a.conn.Exec("DELETE FROM knowledge_bases WHERE id = ?", id)
	n, _ := result.RowsAffected()
	if n == 0 {
		w.WriteHeader(404)
		writeJSON(w, map[string]string{"error": "knowledge base not found"})
		return
	}
	a.conn.Exec("DELETE FROM knowledge_entries WHERE kb_id = ?", id)
	a.conn.Exec("DELETE FROM knowledge_subscriptions WHERE kb_id = ?", id)
	a.conn.Exec("DELETE FROM knowledge_watches WHERE kb_id = ?", id)
	writeJSON(w, map[string]any{"status": "deleted"})
}

// --- Entries ---

func (a *App) handleAddEntry(w http.ResponseWriter, r *http.Request) {
	kbID := r.PathValue("id")
	var req struct {
		Fact      string `json:"fact"`
		Source    string `json:"source"`
		ExpiresAt string `json:"expires_at"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	id := genID("ke_")
	now := time.Now().UTC().Format(time.RFC3339)

	var expiresAt *string
	if req.ExpiresAt != "" {
		expiresAt = &req.ExpiresAt
	}

	_, err := a.conn.Exec(
		"INSERT INTO knowledge_entries (id, kb_id, fact, source, created_at, expires_at) VALUES (?,?,?,?,?,?)",
		id, kbID, req.Fact, req.Source, now, expiresAt,
	)
	if err != nil {
		w.WriteHeader(500)
		writeJSON(w, map[string]string{"error": "failed to add entry"})
		return
	}

	a.conn.Exec("UPDATE knowledge_bases SET entries_count = entries_count + 1, updated_at = ? WHERE id = ?", now, kbID)

	writeJSON(w, map[string]any{"id": id, "kb_id": kbID, "status": "created"})
}

func (a *App) handleListEntries(w http.ResponseWriter, r *http.Request) {
	kbID := r.PathValue("id")
	rows, err := a.conn.Query(
		"SELECT id, fact, source, verified, verification_count, disputed, created_at, expires_at FROM knowledge_entries WHERE kb_id = ? ORDER BY created_at DESC", kbID,
	)
	if err != nil {
		writeJSON(w, map[string]any{"entries": []any{}})
		return
	}
	defer rows.Close()

	var entries []map[string]any
	for rows.Next() {
		var id, fact, source, createdAt string
		var expiresAt sql.NullString
		var verified, verificationCount, disputed int
		rows.Scan(&id, &fact, &source, &verified, &verificationCount, &disputed, &createdAt, &expiresAt)
		entries = append(entries, map[string]any{
			"id": id, "fact": fact, "source": source,
			"verified": verified, "verification_count": verificationCount,
			"disputed": disputed, "created_at": createdAt, "expires_at": expiresAt.String,
		})
	}
	if entries == nil {
		entries = []map[string]any{}
	}
	writeJSON(w, map[string]any{"entries": entries, "count": len(entries)})
}

func (a *App) handleDeleteEntry(w http.ResponseWriter, r *http.Request) {
	kbID := r.PathValue("id")
	eid := r.PathValue("eid")
	result, _ := a.conn.Exec("DELETE FROM knowledge_entries WHERE id = ? AND kb_id = ?", eid, kbID)
	n, _ := result.RowsAffected()
	if n == 0 {
		w.WriteHeader(404)
		writeJSON(w, map[string]string{"error": "entry not found"})
		return
	}
	now := time.Now().UTC().Format(time.RFC3339)
	a.conn.Exec("UPDATE knowledge_bases SET entries_count = entries_count - 1, updated_at = ? WHERE id = ?", now, kbID)
	writeJSON(w, map[string]any{"status": "deleted"})
}

// --- Subscribe ---

func (a *App) handleSubscribe(w http.ResponseWriter, r *http.Request) {
	kbID := r.PathValue("id")
	var req struct {
		AppID string `json:"app_id"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	if req.AppID == "" {
		w.WriteHeader(400)
		writeJSON(w, map[string]string{"error": "app_id required"})
		return
	}

	now := time.Now().UTC().Format(time.RFC3339)
	_, err := a.conn.Exec(
		"INSERT OR IGNORE INTO knowledge_subscriptions (app_id, kb_id, created_at) VALUES (?,?,?)",
		req.AppID, kbID, now,
	)
	if err != nil {
		w.WriteHeader(500)
		writeJSON(w, map[string]string{"error": "failed to subscribe"})
		return
	}
	a.conn.Exec("UPDATE knowledge_bases SET installs = installs + 1, updated_at = ? WHERE id = ?", now, kbID)
	writeJSON(w, map[string]any{"status": "subscribed", "app_id": req.AppID, "kb_id": kbID})
}

// --- Query ---

func (a *App) handleQuery(w http.ResponseWriter, r *http.Request) {
	kbID := r.PathValue("id")
	q := r.URL.Query().Get("q")
	limitStr := r.URL.Query().Get("limit")
	limit := 5
	if limitStr != "" {
		if n, err := strconv.Atoi(limitStr); err == nil && n > 0 && n <= 50 {
			limit = n
		}
	}

	if q == "" {
		w.WriteHeader(400)
		writeJSON(w, map[string]string{"error": "q parameter required"})
		return
	}

	rows, err := a.conn.Query(
		"SELECT id, fact, source, verified, verification_count, disputed, created_at FROM knowledge_entries WHERE kb_id = ? ORDER BY created_at DESC LIMIT 200", kbID,
	)
	if err != nil {
		writeJSON(w, map[string]any{"results": []any{}})
		return
	}
	defer rows.Close()

	type scored struct {
		ID                string  `json:"id"`
		Fact              string  `json:"fact"`
		Source            string  `json:"source"`
		Verified          int     `json:"verified"`
		VerificationCount int     `json:"verification_count"`
		Disputed          int     `json:"disputed"`
		Score             float64 `json:"relevance_score"`
		CreatedAt         string  `json:"created_at"`
	}

	queryVec := trigramVec(q)
	var results []scored

	for rows.Next() {
		var id, fact, source, createdAt string
		var verified, verificationCount, disputed int
		rows.Scan(&id, &fact, &source, &verified, &verificationCount, &disputed, &createdAt)

		factVec := trigramVec(fact)
		sim := cosSim(queryVec, factVec)

		if sim > 0.05 {
			results = append(results, scored{
				ID: id, Fact: fact, Source: source,
				Verified: verified, VerificationCount: verificationCount,
				Disputed: disputed, Score: sim, CreatedAt: createdAt,
			})
		}
	}

	// Sort by relevance descending
	for i := 0; i < len(results); i++ {
		for j := i + 1; j < len(results); j++ {
			if results[j].Score > results[i].Score {
				results[i], results[j] = results[j], results[i]
			}
		}
	}

	if len(results) > limit {
		results = results[:limit]
	}
	if results == nil {
		results = []scored{}
	}

	writeJSON(w, map[string]any{"results": results, "count": len(results), "query": q})
}

// --- Verify / Dispute ---

func (a *App) handleVerify(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	result, _ := a.conn.Exec("UPDATE knowledge_entries SET verification_count = verification_count + 1, verified = 1 WHERE id = ?", id)
	n, _ := result.RowsAffected()
	if n == 0 {
		w.WriteHeader(404)
		writeJSON(w, map[string]string{"error": "entry not found"})
		return
	}
	writeJSON(w, map[string]any{"id": id, "status": "verified"})
}

func (a *App) handleDispute(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	result, _ := a.conn.Exec("UPDATE knowledge_entries SET disputed = 1 WHERE id = ?", id)
	n, _ := result.RowsAffected()
	if n == 0 {
		w.WriteHeader(404)
		writeJSON(w, map[string]string{"error": "entry not found"})
		return
	}
	writeJSON(w, map[string]any{"id": id, "status": "disputed"})
}

// --- Marketplace ---

func (a *App) handleMarketplace(w http.ResponseWriter, r *http.Request) {
	domain := r.URL.Query().Get("domain")

	var rows *sql.Rows
	var err error
	if domain != "" {
		rows, err = a.conn.Query(
			"SELECT id, author_id, title, description, domain, entries_count, rating, installs, price_per_query_cents, status, created_at FROM knowledge_bases WHERE status = 'active' AND domain = ? ORDER BY installs DESC", domain,
		)
	} else {
		rows, err = a.conn.Query(
			"SELECT id, author_id, title, description, domain, entries_count, rating, installs, price_per_query_cents, status, created_at FROM knowledge_bases WHERE status = 'active' ORDER BY installs DESC",
		)
	}
	if err != nil {
		writeJSON(w, map[string]any{"bases": []any{}})
		return
	}
	defer rows.Close()

	var bases []map[string]any
	for rows.Next() {
		var id, authorID, title, desc, dom, status, createdAt string
		var entriesCount, installs, price int
		var rating float64
		rows.Scan(&id, &authorID, &title, &desc, &dom, &entriesCount, &rating, &installs, &price, &status, &createdAt)
		bases = append(bases, map[string]any{
			"id": id, "author_id": authorID, "title": title, "description": desc,
			"domain": dom, "entries_count": entriesCount, "rating": rating,
			"installs": installs, "price_per_query_cents": price, "status": status,
			"created_at": createdAt,
		})
	}
	if bases == nil {
		bases = []map[string]any{}
	}
	writeJSON(w, map[string]any{"bases": bases, "count": len(bases)})
}

// --- Watches ---

func (a *App) handleCreateWatch(w http.ResponseWriter, r *http.Request) {
	kbID := r.PathValue("id")
	var req struct {
		URL                string `json:"url"`
		CheckIntervalHours int    `json:"check_interval_hours"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	if req.URL == "" {
		w.WriteHeader(400)
		writeJSON(w, map[string]string{"error": "url required"})
		return
	}
	if req.CheckIntervalHours <= 0 {
		req.CheckIntervalHours = 24
	}

	id := genID("kw_")
	now := time.Now().UTC().Format(time.RFC3339)

	_, err := a.conn.Exec(
		"INSERT INTO knowledge_watches (id, kb_id, url, check_interval_hours, created_at) VALUES (?,?,?,?,?)",
		id, kbID, req.URL, req.CheckIntervalHours, now,
	)
	if err != nil {
		w.WriteHeader(500)
		writeJSON(w, map[string]string{"error": "failed to create watch"})
		return
	}

	writeJSON(w, map[string]any{"id": id, "kb_id": kbID, "status": "created"})
}

func (a *App) handleListWatches(w http.ResponseWriter, r *http.Request) {
	kbID := r.PathValue("id")
	rows, err := a.conn.Query(
		"SELECT id, url, check_interval_hours, last_checked, last_hash, created_at FROM knowledge_watches WHERE kb_id = ? ORDER BY created_at DESC", kbID,
	)
	if err != nil {
		writeJSON(w, map[string]any{"watches": []any{}})
		return
	}
	defer rows.Close()

	var watches []map[string]any
	for rows.Next() {
		var id, url, createdAt string
		var lastChecked, lastHash sql.NullString
		var interval int
		rows.Scan(&id, &url, &interval, &lastChecked, &lastHash, &createdAt)
		watches = append(watches, map[string]any{
			"id": id, "url": url, "check_interval_hours": interval,
			"last_checked": lastChecked.String, "last_hash": lastHash.String,
			"created_at": createdAt,
		})
	}
	if watches == nil {
		watches = []map[string]any{}
	}
	writeJSON(w, map[string]any{"watches": watches, "count": len(watches)})
}

// --- Embeddings ---

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

	h := sha256.Sum256([]byte(req.Text))
	textHash := hex.EncodeToString(h[:])

	// Check cache
	var cached string
	err := a.conn.QueryRow("SELECT trigrams FROM embedding_cache WHERE text_hash = ?", textHash).Scan(&cached)
	if err == nil {
		var tg any
		json.Unmarshal([]byte(cached), &tg)
		writeJSON(w, map[string]any{"hash": textHash, "trigrams": tg})
		return
	}

	// Compute trigrams
	vec := trigramVec(req.Text)
	trigramsJSON, _ := json.Marshal(vec)
	now := time.Now().UTC().Format(time.RFC3339)
	a.conn.Exec("INSERT OR REPLACE INTO embedding_cache (text_hash, trigrams, created_at) VALUES (?,?,?)", textHash, string(trigramsJSON), now)

	writeJSON(w, map[string]any{"hash": textHash, "trigrams": vec})
}

// --- Trigram similarity ---

func trigramVec(text string) map[string]float64 {
	text = strings.ToLower(strings.TrimSpace(text))
	counts := make(map[string]int)
	total := 0
	runes := []rune(text)
	for i := 0; i <= len(runes)-3; i++ {
		tri := string(runes[i : i+3])
		counts[tri]++
		total++
	}
	for _, w := range strings.Fields(text) {
		if len(w) >= 2 {
			counts["w:"+w]++
			total++
		}
	}
	vec := make(map[string]float64, len(counts))
	for k, c := range counts {
		if total > 0 {
			vec[k] = float64(c) / float64(total)
		}
	}
	return vec
}

func cosSim(a, b map[string]float64) float64 {
	dot := 0.0
	magA := 0.0
	magB := 0.0
	for k, v := range a {
		magA += v * v
		if bv, ok := b[k]; ok {
			dot += v * bv
		}
	}
	for _, v := range b {
		magB += v * v
	}
	if magA == 0 || magB == 0 {
		return 0
	}
	return dot / (math.Sqrt(magA) * math.Sqrt(magB))
}
