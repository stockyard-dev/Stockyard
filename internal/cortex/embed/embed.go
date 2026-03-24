// Package embed generates and manages text embeddings for semantic search.
package embed

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/stockyard-dev/stockyard/internal/cortex/cstore"
	"github.com/stockyard-dev/stockyard/internal/cortex/ingest"
)

// Config holds embedding settings.
type Config struct {
	APIKey     string
	Model      string // default: text-embedding-3-small
	BaseURL    string // default: https://api.openai.com
	Dimensions int    // default: 1536
	BatchSize  int    // number of texts to embed per API call (default: 50)
}

// Generator creates embeddings and stores them in the knowledge base.
type Generator struct {
	cfg    Config
	client *http.Client
}

// New creates an embedding generator.
func New(cfg Config) *Generator {
	if cfg.Model == "" {
		cfg.Model = "text-embedding-3-small"
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.openai.com"
	}
	if cfg.Dimensions == 0 {
		cfg.Dimensions = 1536
	}
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 50
	}
	return &Generator{
		cfg:    cfg,
		client: &http.Client{Timeout: 60 * time.Second},
	}
}

// EmbedAllDocuments generates chunks and embeddings for all documents in the DB.
func (g *Generator) EmbedAllDocuments(ctx context.Context, db *cstore.DB, chunkSize, overlap int, onProgress func(current, total int)) error {
	if chunkSize <= 0 {
		chunkSize = 512
	}
	if overlap <= 0 {
		overlap = 64
	}

	// Get all documents that need chunking + embedding
	// (simple approach: re-chunk everything; skip if chunk already exists)
	existingChunks := db.CountChunks()
	if existingChunks > 0 {
		// Already have chunks — skip for now (incremental update would go here)
		return nil
	}

	// Query all documents
	type docRow struct {
		ID      string
		Content string
		Title   string
	}

	rows, err := db.QueryDocuments()
	if err != nil {
		return fmt.Errorf("querying documents: %w", err)
	}

	// Chunk all documents
	type pendingChunk struct {
		chunk cstore.Chunk
		text  string
	}
	var pending []pendingChunk

	for i, doc := range rows {
		if onProgress != nil {
			onProgress(i+1, len(rows))
		}

		chunks := ingest.ChunkText(doc.Content, chunkSize, overlap)
		for j, text := range chunks {
			c := cstore.Chunk{
				ID:         fmt.Sprintf("%s:chunk:%d", doc.ID, j),
				DocumentID: doc.ID,
				Content:    text,
				ChunkIndex: j,
				TokenCount: len(text) / 4, // rough estimate
			}
			pending = append(pending, pendingChunk{chunk: c, text: text})
		}
	}

	// Generate embeddings in batches
	for i := 0; i < len(pending); i += g.cfg.BatchSize {
		end := i + g.cfg.BatchSize
		if end > len(pending) {
			end = len(pending)
		}
		batch := pending[i:end]

		texts := make([]string, len(batch))
		for j, p := range batch {
			texts[j] = p.text
		}

		embeddings, err := g.embedBatch(ctx, texts)
		if err != nil {
			return fmt.Errorf("embedding batch %d-%d: %w", i, end, err)
		}

		for j, emb := range embeddings {
			batch[j].chunk.Embedding = emb
			if err := db.SaveChunk(batch[j].chunk); err != nil {
				return fmt.Errorf("saving chunk: %w", err)
			}
		}

		if onProgress != nil {
			onProgress(end, len(pending))
		}
	}

	return nil
}

// EmbedQuery generates an embedding for a query string.
func (g *Generator) EmbedQuery(ctx context.Context, query string) ([]float32, error) {
	embeddings, err := g.embedBatch(ctx, []string{query})
	if err != nil {
		return nil, err
	}
	if len(embeddings) == 0 {
		return nil, fmt.Errorf("no embedding returned")
	}
	return embeddings[0], nil
}

// embedBatch calls the OpenAI embedding API with a batch of texts.
func (g *Generator) embedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	reqBody := map[string]any{
		"model": g.cfg.Model,
		"input": texts,
	}
	data, _ := json.Marshal(reqBody)

	req, err := http.NewRequestWithContext(ctx, "POST", g.cfg.BaseURL+"/v1/embeddings", bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+g.cfg.APIKey)

	resp, err := g.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("embedding API request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("embedding API returned %d: %s", resp.StatusCode, truncate(string(body), 200))
	}

	var result struct {
		Data []struct {
			Embedding []float32 `json:"embedding"`
			Index     int       `json:"index"`
		} `json:"data"`
		Usage struct {
			TotalTokens int `json:"total_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parsing embedding response: %w", err)
	}

	// Sort by index to ensure order matches input
	embeddings := make([][]float32, len(texts))
	for _, d := range result.Data {
		if d.Index < len(embeddings) {
			embeddings[d.Index] = d.Embedding
		}
	}

	return embeddings, nil
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n] + "..."
	}
	return s
}
