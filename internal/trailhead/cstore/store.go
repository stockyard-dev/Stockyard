// Package cstore provides SQLite persistence for Cortex's knowledge base.
package cstore

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"time"

	_ "modernc.org/sqlite"
)

// DB wraps a SQLite connection for the Cortex knowledge base.
type DB struct {
	db *sql.DB
}

// Open opens or creates a Cortex database.
func Open(path string) (*DB, error) {
	db, err := sql.Open("sqlite", path+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}
	s := &DB{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func (s *DB) Close() error { return s.db.Close() }

func (s *DB) migrate() error {
	_, err := s.db.Exec(`
		-- Sources (GitHub repos, Slack workspaces, etc.)
		CREATE TABLE IF NOT EXISTS sources (
			id TEXT PRIMARY KEY,
			type TEXT NOT NULL,
			name TEXT NOT NULL,
			config_json TEXT,
			last_synced_at DATETIME,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);

		-- Documents (individual pieces of indexed content)
		CREATE TABLE IF NOT EXISTS documents (
			id TEXT PRIMARY KEY,
			source_id TEXT REFERENCES sources(id),
			type TEXT NOT NULL,
			title TEXT,
			content TEXT NOT NULL,
			url TEXT,
			author TEXT,
			created_date DATETIME,
			metadata_json TEXT,
			content_hash TEXT,
			indexed_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
		CREATE INDEX IF NOT EXISTS idx_docs_source ON documents(source_id);
		CREATE INDEX IF NOT EXISTS idx_docs_type ON documents(type);
		CREATE INDEX IF NOT EXISTS idx_docs_hash ON documents(content_hash);

		-- Chunks (text segments with embeddings)
		CREATE TABLE IF NOT EXISTS chunks (
			id TEXT PRIMARY KEY,
			document_id TEXT REFERENCES documents(id),
			content TEXT NOT NULL,
			chunk_index INTEGER,
			token_count INTEGER,
			embedding BLOB,
			metadata_json TEXT
		);
		CREATE INDEX IF NOT EXISTS idx_chunks_doc ON chunks(document_id);

		-- Entities (people, files, concepts extracted from content)
		CREATE TABLE IF NOT EXISTS entities (
			id TEXT PRIMARY KEY,
			type TEXT NOT NULL,
			name TEXT NOT NULL,
			properties_json TEXT,
			mention_count INTEGER DEFAULT 1,
			first_seen DATETIME DEFAULT CURRENT_TIMESTAMP,
			last_seen DATETIME DEFAULT CURRENT_TIMESTAMP
		);
		CREATE INDEX IF NOT EXISTS idx_entities_type ON entities(type);
		CREATE INDEX IF NOT EXISTS idx_entities_name ON entities(name);

		-- Relationships (edges in the knowledge graph)
		CREATE TABLE IF NOT EXISTS relationships (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			from_entity TEXT REFERENCES entities(id),
			to_entity TEXT REFERENCES entities(id),
			type TEXT NOT NULL,
			weight REAL DEFAULT 1.0,
			document_id TEXT,
			context TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
		CREATE INDEX IF NOT EXISTS idx_rel_from ON relationships(from_entity);
		CREATE INDEX IF NOT EXISTS idx_rel_to ON relationships(to_entity);
		CREATE INDEX IF NOT EXISTS idx_rel_type ON relationships(type);

		-- Query history
		CREATE TABLE IF NOT EXISTS queries (
			id TEXT PRIMARY KEY,
			question TEXT NOT NULL,
			answer TEXT,
			sources_json TEXT,
			chunks_used INTEGER,
			latency_ms INTEGER,
			cost_cents REAL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
	`)
	return err
}

// =========================================================================
// Source operations
// =========================================================================

// Source represents an indexed data source.
type Source struct {
	ID           string    `json:"id"`
	Type         string    `json:"type"`
	Name         string    `json:"name"`
	Config       string    `json:"config,omitempty"`
	LastSyncedAt *time.Time `json:"last_synced_at,omitempty"`
}

func (s *DB) SaveSource(src Source) error {
	_, err := s.db.Exec(
		"INSERT OR REPLACE INTO sources (id, type, name, config_json) VALUES (?, ?, ?, ?)",
		src.ID, src.Type, src.Name, src.Config,
	)
	return err
}

func (s *DB) UpdateSourceSync(id string) error {
	_, err := s.db.Exec("UPDATE sources SET last_synced_at = ? WHERE id = ?", time.Now().UTC(), id)
	return err
}

func (s *DB) ListSources() ([]Source, error) {
	rows, err := s.db.Query("SELECT id, type, name, COALESCE(config_json,''), last_synced_at FROM sources ORDER BY name")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Source
	for rows.Next() {
		var src Source
		var synced sql.NullString
		if err := rows.Scan(&src.ID, &src.Type, &src.Name, &src.Config, &synced); err != nil {
			continue
		}
		if synced.Valid {
			t, _ := time.Parse(time.RFC3339, synced.String)
			src.LastSyncedAt = &t
		}
		out = append(out, src)
	}
	return out, nil
}

// =========================================================================
// Document operations
// =========================================================================

// Document represents an indexed piece of content.
type Document struct {
	ID          string            `json:"id"`
	SourceID    string            `json:"source_id"`
	Type        string            `json:"type"` // code_file, commit, pull_request, issue, comment, etc.
	Title       string            `json:"title"`
	Content     string            `json:"content"`
	URL         string            `json:"url"`
	Author      string            `json:"author"`
	CreatedDate time.Time         `json:"created_date"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	ContentHash string            `json:"content_hash"`
}

func (s *DB) SaveDocument(doc Document) error {
	metaJSON, _ := json.Marshal(doc.Metadata)
	_, err := s.db.Exec(
		`INSERT OR REPLACE INTO documents (id, source_id, type, title, content, url, author, created_date, metadata_json, content_hash)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		doc.ID, doc.SourceID, doc.Type, doc.Title, doc.Content, doc.URL, doc.Author, doc.CreatedDate, string(metaJSON), doc.ContentHash,
	)
	return err
}

func (s *DB) DocumentExists(contentHash string) bool {
	var count int
	s.db.QueryRow("SELECT COUNT(*) FROM documents WHERE content_hash = ?", contentHash).Scan(&count)
	return count > 0
}

func (s *DB) CountDocuments(sourceID string) int {
	var count int
	if sourceID == "" {
		s.db.QueryRow("SELECT COUNT(*) FROM documents").Scan(&count)
	} else {
		s.db.QueryRow("SELECT COUNT(*) FROM documents WHERE source_id = ?", sourceID).Scan(&count)
	}
	return count
}

func (s *DB) CountChunks() int {
	var count int
	s.db.QueryRow("SELECT COUNT(*) FROM chunks").Scan(&count)
	return count
}

// ChunkExists checks if a chunk with the given ID exists.
func (s *DB) ChunkExists(chunkID string) bool {
	var count int
	s.db.QueryRow("SELECT COUNT(*) FROM chunks WHERE id = ?", chunkID).Scan(&count)
	return count > 0
}

// =========================================================================
// Chunk operations
// =========================================================================

// Chunk represents a text segment with its embedding vector.
type Chunk struct {
	ID          string    `json:"id"`
	DocumentID  string    `json:"document_id"`
	Content     string    `json:"content"`
	ChunkIndex  int       `json:"chunk_index"`
	TokenCount  int       `json:"token_count"`
	Embedding   []float32 `json:"-"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

func (s *DB) SaveChunk(c Chunk) error {
	embBytes := float32sToBytes(c.Embedding)
	metaJSON, _ := json.Marshal(c.Metadata)
	_, err := s.db.Exec(
		"INSERT OR REPLACE INTO chunks (id, document_id, content, chunk_index, token_count, embedding, metadata_json) VALUES (?, ?, ?, ?, ?, ?, ?)",
		c.ID, c.DocumentID, c.Content, c.ChunkIndex, c.TokenCount, embBytes, string(metaJSON),
	)
	return err
}

// SearchResult holds a chunk with its similarity score and parent document info.
type SearchResult struct {
	Chunk      Chunk    `json:"chunk"`
	Score      float64  `json:"score"`
	DocTitle   string   `json:"doc_title"`
	DocType    string   `json:"doc_type"`
	DocURL     string   `json:"doc_url"`
	DocAuthor  string   `json:"doc_author"`
}

// SearchChunks performs brute-force cosine similarity search across all chunks.
func (s *DB) SearchChunks(queryEmbedding []float32, topK int) ([]SearchResult, error) {
	rows, err := s.db.Query(
		`SELECT c.id, c.document_id, c.content, c.chunk_index, c.token_count, c.embedding, c.metadata_json,
		        d.title, d.type, d.url, d.author
		 FROM chunks c JOIN documents d ON c.document_id = d.id
		 WHERE c.embedding IS NOT NULL`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type scored struct {
		result SearchResult
		score  float64
	}
	var all []scored

	for rows.Next() {
		var c Chunk
		var embBytes []byte
		var metaJSON sql.NullString
		var r SearchResult

		if err := rows.Scan(&c.ID, &c.DocumentID, &c.Content, &c.ChunkIndex, &c.TokenCount, &embBytes, &metaJSON,
			&r.DocTitle, &r.DocType, &r.DocURL, &r.DocAuthor); err != nil {
			continue
		}

		c.Embedding = bytesToFloat32s(embBytes)
		if len(c.Embedding) == 0 {
			continue
		}

		sim := cosineSimilarity(queryEmbedding, c.Embedding)
		r.Chunk = c
		r.Score = sim
		all = append(all, scored{result: r, score: sim})
	}

	// Sort by score descending
	for i := 0; i < len(all); i++ {
		for j := i + 1; j < len(all); j++ {
			if all[j].score > all[i].score {
				all[i], all[j] = all[j], all[i]
			}
		}
	}

	if topK > len(all) {
		topK = len(all)
	}
	results := make([]SearchResult, topK)
	for i := 0; i < topK; i++ {
		results[i] = all[i].result
	}
	return results, nil
}

// =========================================================================
// Entity and relationship operations
// =========================================================================

// Entity represents a node in the knowledge graph.
type Entity struct {
	ID           string            `json:"id"`
	Type         string            `json:"type"` // person, file, concept, decision
	Name         string            `json:"name"`
	Properties   map[string]string `json:"properties,omitempty"`
	MentionCount int               `json:"mention_count"`
}

// Relationship represents an edge in the knowledge graph.
type Relationship struct {
	FromEntity string  `json:"from_entity"`
	ToEntity   string  `json:"to_entity"`
	Type       string  `json:"type"` // authored, modified, reviewed, decided, mentioned_in
	Weight     float64 `json:"weight"`
	Context    string  `json:"context"`
}

func (s *DB) UpsertEntity(e Entity) error {
	propsJSON, _ := json.Marshal(e.Properties)
	_, err := s.db.Exec(`
		INSERT INTO entities (id, type, name, properties_json, mention_count, last_seen)
		VALUES (?, ?, ?, ?, 1, CURRENT_TIMESTAMP)
		ON CONFLICT(id) DO UPDATE SET
			mention_count = mention_count + 1,
			last_seen = CURRENT_TIMESTAMP,
			properties_json = CASE WHEN excluded.properties_json != '{}' THEN excluded.properties_json ELSE properties_json END
	`, e.ID, e.Type, e.Name, string(propsJSON))
	return err
}

func (s *DB) AddRelationship(r Relationship, docID string) error {
	_, err := s.db.Exec(
		"INSERT INTO relationships (from_entity, to_entity, type, weight, document_id, context) VALUES (?, ?, ?, ?, ?, ?)",
		r.FromEntity, r.ToEntity, r.Type, r.Weight, docID, r.Context,
	)
	return err
}

// GetRelatedEntities finds entities connected to a given entity.
func (s *DB) GetRelatedEntities(entityID string, limit int) ([]Entity, []Relationship, error) {
	rows, err := s.db.Query(`
		SELECT e.id, e.type, e.name, COALESCE(e.properties_json,'{}'), e.mention_count,
		       r.type, r.weight, COALESCE(r.context,'')
		FROM relationships r
		JOIN entities e ON (e.id = r.to_entity AND r.from_entity = ?) OR (e.id = r.from_entity AND r.to_entity = ?)
		ORDER BY r.weight DESC
		LIMIT ?
	`, entityID, entityID, limit)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	var entities []Entity
	var rels []Relationship
	for rows.Next() {
		var e Entity
		var r Relationship
		var propsJSON string
		if err := rows.Scan(&e.ID, &e.Type, &e.Name, &propsJSON, &e.MentionCount, &r.Type, &r.Weight, &r.Context); err != nil {
			continue
		}
		if err := json.Unmarshal([]byte(propsJSON), &e.Properties); err != nil {
			log.Printf("[cstore] json parse error: %v", err)
		}
		entities = append(entities, e)
		rels = append(rels, r)
	}
	return entities, rels, nil
}

// FindEntitiesByName searches for entities matching a name pattern.
func (s *DB) FindEntitiesByName(query string, limit int) ([]Entity, error) {
	rows, err := s.db.Query(
		"SELECT id, type, name, COALESCE(properties_json,'{}'), mention_count FROM entities WHERE name LIKE ? ORDER BY mention_count DESC LIMIT ?",
		"%"+query+"%", limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Entity
	for rows.Next() {
		var e Entity
		var propsJSON string
		if err := rows.Scan(&e.ID, &e.Type, &e.Name, &propsJSON, &e.MentionCount); err != nil {
			continue
		}
		if err := json.Unmarshal([]byte(propsJSON), &e.Properties); err != nil {
			log.Printf("[cstore] json parse error: %v", err)
		}
		out = append(out, e)
	}
	return out, nil
}

// =========================================================================
// Query history
// =========================================================================

func (s *DB) SaveQuery(id, question, answer, sourcesJSON string, chunksUsed int, latencyMs int64, costCents float64) error {
	_, err := s.db.Exec(
		"INSERT INTO queries (id, question, answer, sources_json, chunks_used, latency_ms, cost_cents) VALUES (?, ?, ?, ?, ?, ?, ?)",
		id, question, answer, sourcesJSON, chunksUsed, latencyMs, costCents,
	)
	return err
}

// Stats returns aggregate statistics about the knowledge base.
type Stats struct {
	Sources       int `json:"sources"`
	Documents     int `json:"documents"`
	Chunks        int `json:"chunks"`
	Entities      int `json:"entities"`
	Relationships int `json:"relationships"`
	Queries       int `json:"queries"`
}

func (s *DB) Stats() Stats {
	var st Stats
	s.db.QueryRow("SELECT COUNT(*) FROM sources").Scan(&st.Sources)
	s.db.QueryRow("SELECT COUNT(*) FROM documents").Scan(&st.Documents)
	s.db.QueryRow("SELECT COUNT(*) FROM chunks").Scan(&st.Chunks)
	s.db.QueryRow("SELECT COUNT(*) FROM entities").Scan(&st.Entities)
	s.db.QueryRow("SELECT COUNT(*) FROM relationships").Scan(&st.Relationships)
	s.db.QueryRow("SELECT COUNT(*) FROM queries").Scan(&st.Queries)
	return st
}

// DocRow is a minimal document for batch processing.
type DocRow struct {
	ID      string
	Content string
	Title   string
}

// QueryDocuments returns all documents for chunking/embedding.
func (s *DB) QueryDocuments() ([]DocRow, error) {
	rows, err := s.db.Query("SELECT id, content, COALESCE(title,'') FROM documents ORDER BY created_date")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []DocRow
	for rows.Next() {
		var d DocRow
		if err := rows.Scan(&d.ID, &d.Content, &d.Title); err != nil {
			continue
		}
		out = append(out, d)
	}
	return out, nil
}

// QueryHistoryRow holds a saved query for display.
type QueryHistoryRow struct {
	ID         string  `json:"id"`
	Question   string  `json:"question"`
	Answer     string  `json:"answer"`
	ChunksUsed int     `json:"chunks_used"`
	LatencyMs  int64   `json:"latency_ms"`
	CostCents  float64 `json:"cost_cents"`
	CreatedAt  string  `json:"created_at"`
}

// QueryHistory returns recent queries.
func (s *DB) QueryHistory(limit int) ([]QueryHistoryRow, error) {
	rows, err := s.db.Query(
		"SELECT id, question, COALESCE(answer,''), COALESCE(chunks_used,0), COALESCE(latency_ms,0), COALESCE(cost_cents,0), COALESCE(created_at,'') FROM queries ORDER BY created_at DESC LIMIT ?",
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []QueryHistoryRow
	for rows.Next() {
		var q QueryHistoryRow
		if err := rows.Scan(&q.ID, &q.Question, &q.Answer, &q.ChunksUsed, &q.LatencyMs, &q.CostCents, &q.CreatedAt); err != nil {
			continue
		}
		out = append(out, q)
	}
	return out, nil
}

// =========================================================================
// Vector math helpers
// =========================================================================

func cosineSimilarity(a, b []float32) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, normA, normB float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dot / (sqrt(normA) * sqrt(normB))
}

func sqrt(x float64) float64 {
	return math.Sqrt(x)
}

func float32sToBytes(fs []float32) []byte {
	if len(fs) == 0 {
		return nil
	}
	buf := make([]byte, len(fs)*4)
	for i, f := range fs {
		u := math.Float32bits(f)
		buf[i*4+0] = byte(u)
		buf[i*4+1] = byte(u >> 8)
		buf[i*4+2] = byte(u >> 16)
		buf[i*4+3] = byte(u >> 24)
	}
	return buf
}

func bytesToFloat32s(b []byte) []float32 {
	if len(b) == 0 || len(b)%4 != 0 {
		return nil
	}
	fs := make([]float32, len(b)/4)
	for i := range fs {
		u := uint32(b[i*4]) | uint32(b[i*4+1])<<8 | uint32(b[i*4+2])<<16 | uint32(b[i*4+3])<<24
		fs[i] = math.Float32frombits(u)
	}
	return fs
}
