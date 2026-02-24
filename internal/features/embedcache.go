package features

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/stockyard-dev/stockyard/internal/config"
)

// EmbeddingRequest is the OpenAI-compatible /v1/embeddings request format.
type EmbeddingRequest struct {
	Input          interface{} `json:"input"`          // string or []string
	Model          string      `json:"model"`
	EncodingFormat string      `json:"encoding_format,omitempty"` // float or base64
	Dimensions     *int        `json:"dimensions,omitempty"`
	User           string      `json:"user,omitempty"`
}

// InputTexts normalizes the Input field to always return a string slice.
func (r *EmbeddingRequest) InputTexts() []string {
	switch v := r.Input.(type) {
	case string:
		return []string{v}
	case []interface{}:
		texts := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				texts = append(texts, s)
			}
		}
		return texts
	case []string:
		return v
	default:
		return nil
	}
}

// EmbeddingResponseData is a single embedding in the response.
type EmbeddingResponseData struct {
	Object    string    `json:"object"`
	Embedding []float64 `json:"embedding"`
	Index     int       `json:"index"`
}

// EmbeddingResponse is the OpenAI-compatible /v1/embeddings response format.
type EmbeddingResponse struct {
	Object string                  `json:"object"`
	Data   []EmbeddingResponseData `json:"data"`
	Model  string                  `json:"model"`
	Usage  struct {
		PromptTokens int `json:"prompt_tokens"`
		TotalTokens  int `json:"total_tokens"`
	} `json:"usage"`
}

// EmbedCache caches embedding responses by content hash.
type EmbedCache struct {
	cfg     config.EmbedCacheConfig
	mu      sync.RWMutex
	cache   map[string]*embedCacheEntry
	hits    atomic.Int64
	misses  atomic.Int64
	evictions atomic.Int64
	bytesSaved atomic.Int64
}

type embedCacheEntry struct {
	response []byte // JSON-encoded single EmbeddingResponseData
	model    string
	tokens   int
	cachedAt time.Time
	size     int
}

// NewEmbedCache creates a new embedding cache.
func NewEmbedCache(cfg config.EmbedCacheConfig) *EmbedCache {
	if cfg.MaxEntries <= 0 {
		cfg.MaxEntries = 100000
	}
	ec := &EmbedCache{
		cfg:   cfg,
		cache: make(map[string]*embedCacheEntry),
	}
	// Background eviction of expired entries
	go ec.evictionLoop()
	return ec
}

// ContentHash produces a deterministic hash for an embedding input.
// Format: sha256(model + "|" + encoding_format + "|" + dimensions + "|" + text)
func ContentHash(model, text string, encodingFormat string, dimensions *int) string {
	h := sha256.New()
	h.Write([]byte(model))
	h.Write([]byte("|"))
	h.Write([]byte(encodingFormat))
	h.Write([]byte("|"))
	if dimensions != nil {
		h.Write([]byte(fmt.Sprintf("%d", *dimensions)))
	}
	h.Write([]byte("|"))
	h.Write([]byte(text))
	return hex.EncodeToString(h.Sum(nil))
}

// Get looks up a cached embedding by content hash.
// Returns the cached data and true if found and not expired.
func (ec *EmbedCache) Get(hash string) ([]byte, bool) {
	ec.mu.RLock()
	entry, ok := ec.cache[hash]
	ec.mu.RUnlock()

	if !ok {
		ec.misses.Add(1)
		return nil, false
	}

	// Check TTL
	if ec.cfg.TTL.Duration > 0 && time.Since(entry.cachedAt) > ec.cfg.TTL.Duration {
		ec.mu.Lock()
		delete(ec.cache, hash)
		ec.mu.Unlock()
		ec.misses.Add(1)
		ec.evictions.Add(1)
		return nil, false
	}

	ec.hits.Add(1)
	ec.bytesSaved.Add(int64(entry.size))
	return entry.response, true
}

// Put stores an embedding response in the cache.
func (ec *EmbedCache) Put(hash string, data []byte, model string, tokens int) {
	ec.mu.Lock()
	defer ec.mu.Unlock()

	// Evict if at capacity (LRU-ish: evict oldest)
	if len(ec.cache) >= ec.cfg.MaxEntries {
		ec.evictOldest()
	}

	ec.cache[hash] = &embedCacheEntry{
		response: data,
		model:    model,
		tokens:   tokens,
		cachedAt: time.Now(),
		size:     len(data),
	}
}

// IsModelCached returns whether the given model should be cached.
func (ec *EmbedCache) IsModelCached(model string) bool {
	if len(ec.cfg.Models) == 0 {
		return true // cache all models
	}
	for _, m := range ec.cfg.Models {
		if strings.EqualFold(m, model) {
			return true
		}
	}
	return false
}

// Stats returns cache statistics.
func (ec *EmbedCache) Stats() map[string]interface{} {
	ec.mu.RLock()
	entries := len(ec.cache)
	ec.mu.RUnlock()

	hits := ec.hits.Load()
	misses := ec.misses.Load()
	total := hits + misses
	var hitRate float64
	if total > 0 {
		hitRate = float64(hits) / float64(total) * 100
	}

	return map[string]interface{}{
		"enabled":     ec.cfg.Enabled,
		"entries":     entries,
		"max_entries": ec.cfg.MaxEntries,
		"hits":        hits,
		"misses":      misses,
		"hit_rate":    fmt.Sprintf("%.1f%%", hitRate),
		"evictions":   ec.evictions.Load(),
		"bytes_saved": ec.bytesSaved.Load(),
	}
}

func (ec *EmbedCache) evictOldest() {
	var oldestKey string
	var oldestTime time.Time

	for k, v := range ec.cache {
		if oldestKey == "" || v.cachedAt.Before(oldestTime) {
			oldestKey = k
			oldestTime = v.cachedAt
		}
	}
	if oldestKey != "" {
		delete(ec.cache, oldestKey)
		ec.evictions.Add(1)
	}
}

func (ec *EmbedCache) evictionLoop() {
	if ec.cfg.TTL.Duration <= 0 {
		return
	}
	ticker := time.NewTicker(ec.cfg.TTL.Duration / 10)
	defer ticker.Stop()
	for range ticker.C {
		ec.mu.Lock()
		now := time.Now()
		for k, v := range ec.cache {
			if now.Sub(v.cachedAt) > ec.cfg.TTL.Duration {
				delete(ec.cache, k)
				ec.evictions.Add(1)
			}
		}
		ec.mu.Unlock()
	}
}

// ProcessEmbeddingRequestRaw implements proxy.EmbeddingCacheProcessor.
// It works with raw JSON bytes to avoid circular imports.
func (ec *EmbedCache) ProcessEmbeddingRequestRaw(body []byte, forward func(body []byte) ([]byte, error)) ([]byte, error) {
	if !ec.cfg.Enabled {
		return forward(body)
	}

	var req EmbeddingRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return forward(body)
	}

	resp, err := ec.ProcessEmbeddingRequest(&req, func(fwdReq *EmbeddingRequest) (*EmbeddingResponse, error) {
		fwdBody, err := json.Marshal(fwdReq)
		if err != nil {
			return nil, fmt.Errorf("marshal forward request: %w", err)
		}
		respBody, err := forward(fwdBody)
		if err != nil {
			return nil, err
		}
		var resp EmbeddingResponse
		if err := json.Unmarshal(respBody, &resp); err != nil {
			return nil, fmt.Errorf("parse embedding response: %w", err)
		}
		return &resp, nil
	})
	if err != nil {
		return nil, err
	}

	return json.Marshal(resp)
}

// ProcessEmbeddingRequest handles a full /v1/embeddings request with caching.
// It splits multi-input requests, checks cache per-input, and only forwards cache misses.
// Returns the complete response and whether it was fully cached.
func (ec *EmbedCache) ProcessEmbeddingRequest(
	req *EmbeddingRequest,
	forward func(req *EmbeddingRequest) (*EmbeddingResponse, error),
) (*EmbeddingResponse, error) {
	if !ec.cfg.Enabled {
		return forward(req)
	}

	texts := req.InputTexts()
	if len(texts) == 0 {
		return forward(req)
	}

	if !ec.IsModelCached(req.Model) {
		return forward(req)
	}

	encoding := req.EncodingFormat
	if encoding == "" {
		encoding = "float"
	}

	// Check cache for each input text
	type indexedResult struct {
		index int
		data  EmbeddingResponseData
		hash  string
	}

	cached := make([]indexedResult, 0, len(texts))
	uncached := make([]string, 0)
	uncachedIndices := make([]int, 0)
	hashes := make([]string, len(texts))

	for i, text := range texts {
		hash := ContentHash(req.Model, text, encoding, req.Dimensions)
		hashes[i] = hash

		if data, ok := ec.Get(hash); ok {
			var embData EmbeddingResponseData
			if err := json.Unmarshal(data, &embData); err == nil {
				embData.Index = i
				cached = append(cached, indexedResult{index: i, data: embData, hash: hash})
				continue
			}
		}
		uncached = append(uncached, text)
		uncachedIndices = append(uncachedIndices, i)
	}

	// All cached — build response directly
	if len(uncached) == 0 {
		log.Printf("[embedcache] full cache hit for %d embedding(s)", len(texts))
		resp := &EmbeddingResponse{
			Object: "list",
			Model:  req.Model,
		}
		for _, c := range cached {
			resp.Data = append(resp.Data, c.data)
		}
		sort.Slice(resp.Data, func(i, j int) bool {
			return resp.Data[i].Index < resp.Data[j].Index
		})
		return resp, nil
	}

	// Forward uncached inputs
	forwardReq := &EmbeddingRequest{
		Model:          req.Model,
		EncodingFormat: req.EncodingFormat,
		Dimensions:     req.Dimensions,
		User:           req.User,
	}
	if len(uncached) == 1 {
		forwardReq.Input = uncached[0]
	} else {
		forwardReq.Input = uncached
	}

	resp, err := forward(forwardReq)
	if err != nil {
		return nil, err
	}

	// Cache the new results
	for i, embData := range resp.Data {
		if i < len(uncachedIndices) {
			realIndex := uncachedIndices[i]
			embData.Index = realIndex

			// Cache individual embedding
			data, err := json.Marshal(embData)
			if err == nil {
				ec.Put(hashes[realIndex], data, req.Model, resp.Usage.PromptTokens/len(uncached))
			}
		}
	}

	// Merge cached + fresh
	allData := make([]EmbeddingResponseData, 0, len(texts))
	for _, c := range cached {
		allData = append(allData, c.data)
	}
	for i, embData := range resp.Data {
		if i < len(uncachedIndices) {
			embData.Index = uncachedIndices[i]
			allData = append(allData, embData)
		}
	}
	sort.Slice(allData, func(i, j int) bool {
		return allData[i].Index < allData[j].Index
	})

	resp.Data = allData
	if len(cached) > 0 {
		log.Printf("[embedcache] partial cache hit: %d cached, %d forwarded", len(cached), len(uncached))
	}

	return resp, nil
}
