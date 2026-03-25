package cstore

import (
	"path/filepath"
	"testing"
	"time"
)

func newTestDB(t *testing.T) *DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "trailhead_test.db")
	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestOpen_MigrationIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "trailhead.db")
	db1, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	db1.Close()

	db2, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	db2.Close()
}

// ── Sources ───────────────────────────────────────────────────────

func TestSaveAndListSources(t *testing.T) {
	db := newTestDB(t)

	src := Source{ID: "s-1", Type: "github", Name: "my-repo", Config: `{"token":"x"}`}
	if err := db.SaveSource(src); err != nil {
		t.Fatal(err)
	}

	sources, err := db.ListSources()
	if err != nil {
		t.Fatal(err)
	}
	if len(sources) != 1 {
		t.Fatalf("expected 1, got %d", len(sources))
	}
	if sources[0].Name != "my-repo" {
		t.Errorf("expected my-repo, got %s", sources[0].Name)
	}
}

func TestSaveSource_Upsert(t *testing.T) {
	db := newTestDB(t)
	db.SaveSource(Source{ID: "s-1", Type: "github", Name: "v1"})
	db.SaveSource(Source{ID: "s-1", Type: "github", Name: "v2"})

	sources, _ := db.ListSources()
	if len(sources) != 1 {
		t.Fatalf("expected 1, got %d", len(sources))
	}
	if sources[0].Name != "v2" {
		t.Errorf("expected v2, got %s", sources[0].Name)
	}
}

func TestUpdateSourceSync(t *testing.T) {
	db := newTestDB(t)
	db.SaveSource(Source{ID: "s-1", Type: "github", Name: "repo"})

	if err := db.UpdateSourceSync("s-1"); err != nil {
		t.Fatal(err)
	}
	sources, _ := db.ListSources()
	if sources[0].LastSyncedAt == nil {
		t.Error("expected non-nil LastSyncedAt after sync")
	}
}

func TestListSources_Empty(t *testing.T) {
	db := newTestDB(t)
	sources, err := db.ListSources()
	if err != nil {
		t.Fatal(err)
	}
	if len(sources) != 0 {
		t.Fatalf("expected 0, got %d", len(sources))
	}
}

// ── Documents ─────────────────────────────────────────────────────

func TestSaveDocumentAndExists(t *testing.T) {
	db := newTestDB(t)

	doc := Document{
		ID: "d-1", SourceID: "s-1", Type: "code_file", Title: "main.go",
		Content: "package main", URL: "http://example.com/main.go",
		Author: "dev", CreatedDate: time.Now().UTC(), ContentHash: "abc123",
		Metadata: map[string]string{"lang": "go"},
	}
	if err := db.SaveDocument(doc); err != nil {
		t.Fatal(err)
	}

	if !db.DocumentExists("abc123") {
		t.Error("expected document to exist")
	}
	if db.DocumentExists("nonexistent") {
		t.Error("expected document to not exist")
	}
}

func TestCountDocuments(t *testing.T) {
	db := newTestDB(t)

	db.SaveDocument(Document{ID: "d-1", SourceID: "s-1", Type: "file", Content: "a", ContentHash: "h1"})
	db.SaveDocument(Document{ID: "d-2", SourceID: "s-1", Type: "file", Content: "b", ContentHash: "h2"})
	db.SaveDocument(Document{ID: "d-3", SourceID: "s-2", Type: "file", Content: "c", ContentHash: "h3"})

	if got := db.CountDocuments(""); got != 3 {
		t.Errorf("expected 3 total, got %d", got)
	}
	if got := db.CountDocuments("s-1"); got != 2 {
		t.Errorf("expected 2 for s-1, got %d", got)
	}
	if got := db.CountDocuments("s-2"); got != 1 {
		t.Errorf("expected 1 for s-2, got %d", got)
	}
	if got := db.CountDocuments("none"); got != 0 {
		t.Errorf("expected 0, got %d", got)
	}
}

func TestSaveDocument_Upsert(t *testing.T) {
	db := newTestDB(t)
	db.SaveDocument(Document{ID: "d-1", SourceID: "s-1", Type: "file", Title: "v1", Content: "a", ContentHash: "h1"})
	db.SaveDocument(Document{ID: "d-1", SourceID: "s-1", Type: "file", Title: "v2", Content: "b", ContentHash: "h2"})

	if got := db.CountDocuments(""); got != 1 {
		t.Errorf("expected 1 after upsert, got %d", got)
	}
}

func TestQueryDocuments(t *testing.T) {
	db := newTestDB(t)
	db.SaveDocument(Document{ID: "d-1", SourceID: "s-1", Type: "file", Title: "first", Content: "hello", ContentHash: "h1"})
	db.SaveDocument(Document{ID: "d-2", SourceID: "s-1", Type: "file", Title: "second", Content: "world", ContentHash: "h2"})

	docs, err := db.QueryDocuments()
	if err != nil {
		t.Fatal(err)
	}
	if len(docs) != 2 {
		t.Fatalf("expected 2, got %d", len(docs))
	}
}

// ── Chunks ────────────────────────────────────────────────────────

func TestSaveChunkAndCount(t *testing.T) {
	db := newTestDB(t)

	c := Chunk{ID: "c-1", DocumentID: "d-1", Content: "some text", ChunkIndex: 0, TokenCount: 10, Embedding: []float32{0.1, 0.2, 0.3}}
	if err := db.SaveChunk(c); err != nil {
		t.Fatal(err)
	}

	if got := db.CountChunks(); got != 1 {
		t.Errorf("expected 1, got %d", got)
	}
	if !db.ChunkExists("c-1") {
		t.Error("expected chunk to exist")
	}
	if db.ChunkExists("nonexistent") {
		t.Error("expected chunk to not exist")
	}
}

func TestSearchChunks(t *testing.T) {
	db := newTestDB(t)

	// Save a document first so the JOIN works.
	db.SaveDocument(Document{ID: "d-1", SourceID: "s-1", Type: "file", Title: "doc", Content: "content", ContentHash: "h", Author: "a", URL: "http://x"})

	db.SaveChunk(Chunk{ID: "c-1", DocumentID: "d-1", Content: "hello", ChunkIndex: 0, TokenCount: 5, Embedding: []float32{1, 0, 0}})
	db.SaveChunk(Chunk{ID: "c-2", DocumentID: "d-1", Content: "world", ChunkIndex: 1, TokenCount: 5, Embedding: []float32{0, 1, 0}})

	results, err := db.SearchChunks([]float32{1, 0, 0}, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2, got %d", len(results))
	}
	// First result should be c-1 (exact match)
	if results[0].Chunk.ID != "c-1" {
		t.Errorf("expected c-1 first, got %s", results[0].Chunk.ID)
	}
	if results[0].Score < 0.99 {
		t.Errorf("expected score ~1.0, got %f", results[0].Score)
	}
}

func TestSearchChunks_Empty(t *testing.T) {
	db := newTestDB(t)
	results, err := db.SearchChunks([]float32{1, 0, 0}, 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 0 {
		t.Fatalf("expected 0, got %d", len(results))
	}
}

func TestSearchChunks_TopK(t *testing.T) {
	db := newTestDB(t)
	db.SaveDocument(Document{ID: "d-1", SourceID: "s-1", Type: "file", Title: "doc", Content: "c", ContentHash: "h"})
	for i := 0; i < 5; i++ {
		emb := make([]float32, 3)
		emb[i%3] = 1
		db.SaveChunk(Chunk{ID: "c-" + string(rune('a'+i)), DocumentID: "d-1", Content: "x", ChunkIndex: i, Embedding: emb})
	}
	results, _ := db.SearchChunks([]float32{1, 0, 0}, 2)
	if len(results) != 2 {
		t.Fatalf("expected 2, got %d", len(results))
	}
}

// ── Entities ──────────────────────────────────────────────────────

func TestUpsertEntity(t *testing.T) {
	db := newTestDB(t)

	e := Entity{ID: "e-1", Type: "person", Name: "Alice", Properties: map[string]string{"role": "dev"}}
	if err := db.UpsertEntity(e); err != nil {
		t.Fatal(err)
	}

	// Upsert again increments mention_count
	if err := db.UpsertEntity(e); err != nil {
		t.Fatal(err)
	}

	entities, err := db.FindEntitiesByName("Alice", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(entities) != 1 {
		t.Fatalf("expected 1, got %d", len(entities))
	}
	if entities[0].MentionCount != 2 {
		t.Errorf("expected mention_count 2, got %d", entities[0].MentionCount)
	}
}

func TestFindEntitiesByName(t *testing.T) {
	db := newTestDB(t)
	db.UpsertEntity(Entity{ID: "e-1", Type: "person", Name: "Alice Smith"})
	db.UpsertEntity(Entity{ID: "e-2", Type: "person", Name: "Bob Jones"})
	db.UpsertEntity(Entity{ID: "e-3", Type: "file", Name: "alice.go"})

	results, err := db.FindEntitiesByName("alice", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 matching 'alice', got %d", len(results))
	}
}

func TestFindEntitiesByName_NoMatch(t *testing.T) {
	db := newTestDB(t)
	results, err := db.FindEntitiesByName("nobody", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 0 {
		t.Fatalf("expected 0, got %d", len(results))
	}
}

// ── Relationships ─────────────────────────────────────────────────

func TestAddAndGetRelatedEntities(t *testing.T) {
	db := newTestDB(t)
	db.UpsertEntity(Entity{ID: "e-1", Type: "person", Name: "Alice"})
	db.UpsertEntity(Entity{ID: "e-2", Type: "file", Name: "main.go"})

	r := Relationship{FromEntity: "e-1", ToEntity: "e-2", Type: "authored", Weight: 1.0, Context: "initial commit"}
	if err := db.AddRelationship(r, "d-1"); err != nil {
		t.Fatal(err)
	}

	entities, rels, err := db.GetRelatedEntities("e-1", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(entities) != 1 {
		t.Fatalf("expected 1, got %d", len(entities))
	}
	if entities[0].Name != "main.go" {
		t.Errorf("expected main.go, got %s", entities[0].Name)
	}
	if len(rels) != 1 || rels[0].Type != "authored" {
		t.Errorf("unexpected rels: %+v", rels)
	}
}

func TestGetRelatedEntities_Empty(t *testing.T) {
	db := newTestDB(t)
	entities, rels, err := db.GetRelatedEntities("nobody", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(entities) != 0 || len(rels) != 0 {
		t.Fatalf("expected empty results")
	}
}

// ── Queries ───────────────────────────────────────────────────────

func TestSaveQueryAndHistory(t *testing.T) {
	db := newTestDB(t)

	if err := db.SaveQuery("q-1", "What is X?", "X is Y", `["d-1"]`, 3, 200, 0.5); err != nil {
		t.Fatal(err)
	}

	history, err := db.QueryHistory(10)
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 1 {
		t.Fatalf("expected 1, got %d", len(history))
	}
	if history[0].Question != "What is X?" {
		t.Errorf("expected 'What is X?', got %s", history[0].Question)
	}
	if history[0].ChunksUsed != 3 {
		t.Errorf("expected 3, got %d", history[0].ChunksUsed)
	}
}

func TestQueryHistory_Empty(t *testing.T) {
	db := newTestDB(t)
	history, err := db.QueryHistory(10)
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 0 {
		t.Fatalf("expected 0, got %d", len(history))
	}
}

func TestQueryHistory_Limit(t *testing.T) {
	db := newTestDB(t)
	for i := 0; i < 5; i++ {
		db.SaveQuery("q-"+string(rune('a'+i)), "q", "a", "[]", 1, 100, 0.1)
	}
	history, _ := db.QueryHistory(2)
	if len(history) != 2 {
		t.Fatalf("expected 2, got %d", len(history))
	}
}

// ── Stats ─────────────────────────────────────────────────────────

func TestStats(t *testing.T) {
	db := newTestDB(t)

	// Empty
	st := db.Stats()
	if st.Sources != 0 || st.Documents != 0 || st.Chunks != 0 || st.Entities != 0 || st.Relationships != 0 || st.Queries != 0 {
		t.Errorf("expected all zeros, got %+v", st)
	}

	// Add some data
	db.SaveSource(Source{ID: "s-1", Type: "github", Name: "repo"})
	db.SaveDocument(Document{ID: "d-1", SourceID: "s-1", Type: "file", Content: "x", ContentHash: "h"})
	db.SaveChunk(Chunk{ID: "c-1", DocumentID: "d-1", Content: "x"})
	db.UpsertEntity(Entity{ID: "e-1", Type: "person", Name: "Alice"})
	db.UpsertEntity(Entity{ID: "e-2", Type: "file", Name: "main.go"})
	db.AddRelationship(Relationship{FromEntity: "e-1", ToEntity: "e-2", Type: "authored", Weight: 1.0}, "d-1")
	db.SaveQuery("q-1", "q", "a", "[]", 1, 100, 0.1)

	st = db.Stats()
	if st.Sources != 1 {
		t.Errorf("expected 1 source, got %d", st.Sources)
	}
	if st.Documents != 1 {
		t.Errorf("expected 1 doc, got %d", st.Documents)
	}
	if st.Chunks != 1 {
		t.Errorf("expected 1 chunk, got %d", st.Chunks)
	}
	if st.Entities != 2 {
		t.Errorf("expected 2 entities, got %d", st.Entities)
	}
	if st.Relationships != 1 {
		t.Errorf("expected 1 relationship, got %d", st.Relationships)
	}
	if st.Queries != 1 {
		t.Errorf("expected 1 query, got %d", st.Queries)
	}
}
