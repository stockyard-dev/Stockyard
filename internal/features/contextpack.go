package features

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/stockyard-dev/stockyard/internal/config"
	"github.com/stockyard-dev/stockyard/internal/provider"
	"github.com/stockyard-dev/stockyard/internal/proxy"
)

// ContextChunk is a piece of indexed content from a source.
type ContextChunk struct {
	Source   string
	Content  string
	Keywords []string
	Score    float64 // relevance score for this request
}

// ContextPacker manages context sources and injection.
type ContextPacker struct {
	mu       sync.RWMutex
	sources  []contextSource
	cfg      config.ContextPackConfig

	totalReqs   atomic.Int64
	injected    atomic.Int64
	tokensAdded atomic.Int64
}

type contextSource struct {
	Name      string
	Type      string // directory | sqlite | url | inline
	Chunks    []ContextChunk
	LoadedAt  time.Time
}

// NewContextPacker creates a new context packer and loads sources.
func NewContextPacker(cfg config.ContextPackConfig) *ContextPacker {
	cp := &ContextPacker{cfg: cfg}
	cp.loadSources()
	return cp
}

// loadSources reads and indexes all configured context sources.
func (cp *ContextPacker) loadSources() {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	for _, src := range cp.cfg.Sources {
		switch src.Type {
		case "directory":
			chunks := cp.loadDirectory(src)
			cp.sources = append(cp.sources, contextSource{
				Name:     src.Name,
				Type:     "directory",
				Chunks:   chunks,
				LoadedAt: time.Now(),
			})
			log.Printf("contextpack: loaded %d chunks from directory source %q", len(chunks), src.Name)

		case "inline":
			// Inline content specified directly in config
			if src.Content != "" {
				chunks := chunkText(src.Content, src.ChunkSize, src.Overlap)
				cp.sources = append(cp.sources, contextSource{
					Name:     src.Name,
					Type:     "inline",
					Chunks:   chunks,
					LoadedAt: time.Now(),
				})
				log.Printf("contextpack: loaded %d inline chunks from %q", len(chunks), src.Name)
			}

		default:
			log.Printf("contextpack: unsupported source type %q for %q", src.Type, src.Name)
		}
	}
}

// loadDirectory reads files matching patterns from a directory.
func (cp *ContextPacker) loadDirectory(src config.ContextSource) []ContextChunk {
	var chunks []ContextChunk

	patterns := src.Patterns
	if len(patterns) == 0 {
		patterns = []string{"*.md", "*.txt"}
	}

	for _, pattern := range patterns {
		matches, err := filepath.Glob(filepath.Join(src.Path, pattern))
		if err != nil {
			log.Printf("contextpack: glob error for %s/%s: %v", src.Path, pattern, err)
			continue
		}

		for _, path := range matches {
			data, err := os.ReadFile(path)
			if err != nil {
				log.Printf("contextpack: read error for %s: %v", path, err)
				continue
			}

			chunkSize := src.ChunkSize
			if chunkSize == 0 {
				chunkSize = 500 // default characters
			}
			overlap := src.Overlap
			if overlap == 0 {
				overlap = 50
			}

			fileChunks := chunkText(string(data), chunkSize, overlap)
			for i := range fileChunks {
				fileChunks[i].Source = filepath.Base(path)
			}
			chunks = append(chunks, fileChunks...)
		}
	}

	return chunks
}

// chunkText splits text into overlapping chunks with keyword extraction.
func chunkText(text string, chunkSize, overlap int) []ContextChunk {
	if chunkSize == 0 {
		chunkSize = 500
	}
	if overlap == 0 {
		overlap = 50
	}

	// Split by paragraphs first for natural boundaries
	paragraphs := strings.Split(text, "\n\n")
	var chunks []ContextChunk
	currentChunk := ""

	for _, para := range paragraphs {
		para = strings.TrimSpace(para)
		if para == "" {
			continue
		}

		if len(currentChunk)+len(para) > chunkSize && currentChunk != "" {
			chunks = append(chunks, ContextChunk{
				Content:  currentChunk,
				Keywords: extractKeywords(currentChunk),
			})
			// Overlap: keep the last `overlap` characters
			if len(currentChunk) > overlap {
				currentChunk = currentChunk[len(currentChunk)-overlap:]
			}
		}
		if currentChunk != "" {
			currentChunk += "\n\n"
		}
		currentChunk += para
	}

	if currentChunk != "" {
		chunks = append(chunks, ContextChunk{
			Content:  currentChunk,
			Keywords: extractKeywords(currentChunk),
		})
	}

	return chunks
}

// extractKeywords pulls significant words from text for matching.
func extractKeywords(text string) []string {
	stopWords := map[string]bool{
		"the": true, "a": true, "an": true, "and": true, "or": true, "but": true,
		"in": true, "on": true, "at": true, "to": true, "for": true, "of": true,
		"with": true, "by": true, "from": true, "is": true, "are": true, "was": true,
		"were": true, "be": true, "been": true, "have": true, "has": true, "had": true,
		"do": true, "does": true, "did": true, "will": true, "would": true, "could": true,
		"should": true, "may": true, "might": true, "can": true, "this": true, "that": true,
		"it": true, "not": true, "no": true, "so": true, "if": true, "then": true,
		"than": true, "as": true, "its": true, "my": true, "your": true, "we": true,
		"they": true, "he": true, "she": true, "you": true, "i": true, "me": true,
		"over": true, "into": true, "about": true, "between": true, "through": true,
		"each": true, "all": true, "both": true, "more": true, "some": true, "such": true,
	}

	words := strings.Fields(strings.ToLower(text))
	seen := make(map[string]bool)
	var keywords []string
	for _, w := range words {
		w = strings.Trim(w, ".,;:!?\"'()[]{}#*-_/\\")
		if len(w) < 3 || stopWords[w] || seen[w] {
			continue
		}
		seen[w] = true
		keywords = append(keywords, w)
	}
	return keywords
}

// FindRelevantContext matches user message against indexed chunks.
func (cp *ContextPacker) FindRelevantContext(messages []provider.Message, maxTokens int) []ContextChunk {
	cp.mu.RLock()
	defer cp.mu.RUnlock()

	// Extract query from user messages
	query := ""
	for _, m := range messages {
		if m.Role == "user" {
			query += " " + m.Content
		}
	}
	query = strings.TrimSpace(query)
	if query == "" {
		return nil
	}

	queryKeywords := extractKeywords(query)
	if len(queryKeywords) == 0 {
		return nil
	}

	// Score all chunks
	var scored []ContextChunk
	for _, src := range cp.sources {
		for _, chunk := range src.Chunks {
			score := keywordOverlap(queryKeywords, chunk.Keywords)
			if score > 0 {
				c := chunk
				c.Score = score
				if c.Source == "" {
					c.Source = src.Name
				}
				scored = append(scored, c)
			}
		}
	}

	// Sort by score descending
	for i := 0; i < len(scored)-1; i++ {
		for j := i + 1; j < len(scored); j++ {
			if scored[j].Score > scored[i].Score {
				scored[i], scored[j] = scored[j], scored[i]
			}
		}
	}

	// Fill up to maxTokens (rough estimate: 1 token ≈ 4 chars)
	if maxTokens == 0 {
		maxTokens = 2000
	}
	maxChars := maxTokens * 4
	currentChars := 0
	var result []ContextChunk
	for _, chunk := range scored {
		if currentChars+len(chunk.Content) > maxChars {
			break
		}
		result = append(result, chunk)
		currentChars += len(chunk.Content)
	}

	return result
}

// keywordOverlap computes the fraction of query keywords found in chunk keywords.
func keywordOverlap(queryKW, chunkKW []string) float64 {
	if len(queryKW) == 0 {
		return 0
	}

	chunkSet := make(map[string]bool)
	for _, w := range chunkKW {
		chunkSet[w] = true
	}

	matches := 0
	for _, w := range queryKW {
		if chunkSet[w] {
			matches++
		}
	}

	return float64(matches) / float64(len(queryKW))
}

// InjectContext adds relevant context to the request messages.
func (cp *ContextPacker) InjectContext(req *provider.Request) bool {
	maxTokens := cp.cfg.Injection.MaxTokens
	if maxTokens == 0 {
		maxTokens = 2000
	}

	chunks := cp.FindRelevantContext(req.Messages, maxTokens)
	if len(chunks) == 0 {
		return false
	}

	// Build context block
	var parts []string
	for _, c := range chunks {
		if c.Source != "" {
			parts = append(parts, fmt.Sprintf("[%s]\n%s", c.Source, c.Content))
		} else {
			parts = append(parts, c.Content)
		}
	}
	contextBlock := strings.Join(parts, "\n---\n")

	// Apply template
	tmpl := cp.cfg.Injection.Template
	if tmpl == "" {
		tmpl = "Relevant context:\n---\n{{context}}\n---"
	}
	injected := strings.Replace(tmpl, "{{context}}", contextBlock, 1)

	// Inject based on position
	position := cp.cfg.Injection.Position
	if position == "" {
		position = "before_user"
	}

	switch position {
	case "system_append":
		// Append to existing system message or create one
		found := false
		for i, m := range req.Messages {
			if m.Role == "system" {
				req.Messages[i].Content += "\n\n" + injected
				found = true
				break
			}
		}
		if !found {
			req.Messages = append([]provider.Message{{Role: "system", Content: injected}}, req.Messages...)
		}

	case "separate_message":
		// Add as a separate system message before the last user message
		idx := len(req.Messages) - 1
		for i := len(req.Messages) - 1; i >= 0; i-- {
			if req.Messages[i].Role == "user" {
				idx = i
				break
			}
		}
		contextMsg := provider.Message{Role: "system", Content: injected}
		newMsgs := make([]provider.Message, 0, len(req.Messages)+1)
		newMsgs = append(newMsgs, req.Messages[:idx]...)
		newMsgs = append(newMsgs, contextMsg)
		newMsgs = append(newMsgs, req.Messages[idx:]...)
		req.Messages = newMsgs

	default: // "before_user"
		// Insert context message before the last user message
		idx := len(req.Messages) - 1
		for i := len(req.Messages) - 1; i >= 0; i-- {
			if req.Messages[i].Role == "user" {
				idx = i
				break
			}
		}
		contextMsg := provider.Message{Role: "user", Content: injected}
		newMsgs := make([]provider.Message, 0, len(req.Messages)+1)
		newMsgs = append(newMsgs, req.Messages[:idx]...)
		newMsgs = append(newMsgs, contextMsg)
		newMsgs = append(newMsgs, req.Messages[idx:]...)
		req.Messages = newMsgs
	}

	// Rough token count of injected context
	tokensInjected := len(contextBlock) / 4
	cp.tokensAdded.Add(int64(tokensInjected))

	return true
}

// Stats returns context pack statistics.
func (cp *ContextPacker) Stats() map[string]any {
	cp.mu.RLock()
	totalChunks := 0
	sources := make([]string, 0)
	for _, s := range cp.sources {
		totalChunks += len(s.Chunks)
		sources = append(sources, fmt.Sprintf("%s(%d)", s.Name, len(s.Chunks)))
	}
	cp.mu.RUnlock()

	return map[string]any{
		"total_requests":  cp.totalReqs.Load(),
		"injected":        cp.injected.Load(),
		"tokens_added":    cp.tokensAdded.Load(),
		"total_chunks":    totalChunks,
		"sources":         sources,
	}
}

// ContextPackMiddleware returns middleware that injects relevant context into requests.
func ContextPackMiddleware(cp *ContextPacker) proxy.Middleware {
	return func(next proxy.Handler) proxy.Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
			cp.totalReqs.Add(1)

			if cp.InjectContext(req) {
				cp.injected.Add(1)
				log.Printf("contextpack: injected context for %s (messages now: %d)",
					req.Project, len(req.Messages))
			}

			return next(ctx, req)
		}
	}
}
