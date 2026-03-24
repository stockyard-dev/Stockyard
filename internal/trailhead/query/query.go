// Package query provides the intelligence layer: semantic search + graph enrichment + LLM synthesis.
package query

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/stockyard-dev/stockyard/internal/trailhead/cstore"
	"github.com/stockyard-dev/stockyard/internal/trailhead/embed"
	"github.com/stockyard-dev/stockyard/internal/provider"
)

// Engine handles natural language queries against the knowledge base.
type Engine struct {
	db       *cstore.DB
	embedder *embed.Generator
	llm      provider.Provider
	model    string
}

// New creates a query engine.
func New(db *cstore.DB, embedder *embed.Generator, llm provider.Provider, model string) *Engine {
	return &Engine{db: db, embedder: embedder, llm: llm, model: model}
}

// Answer holds the structured response to a query.
type Answer struct {
	Question     string       `json:"question"`
	Response     string       `json:"response"`
	Sources      []SourceRef  `json:"sources"`
	RelatedEntities []string  `json:"related_entities,omitempty"`
	ChunksUsed   int          `json:"chunks_used"`
	Latency      time.Duration `json:"latency_ms"`
	Cost         float64      `json:"cost_cents"`
}

// SourceRef identifies a source document that contributed to the answer.
type SourceRef struct {
	DocID    string  `json:"doc_id"`
	Title    string  `json:"title"`
	Type     string  `json:"type"`
	URL      string  `json:"url"`
	Author   string  `json:"author"`
	Score    float64 `json:"relevance_score"`
	Excerpt  string  `json:"excerpt"`
}

// Ask processes a natural language question and returns a synthesized answer.
func (e *Engine) Ask(ctx context.Context, question string, topK int) (*Answer, error) {
	start := time.Now()

	if topK <= 0 {
		topK = 8
	}

	// Step 1: Generate query embedding
	queryEmb, err := e.embedder.EmbedQuery(ctx, question)
	if err != nil {
		return nil, fmt.Errorf("embedding query: %w", err)
	}

	// Step 2: Semantic search for relevant chunks
	results, err := e.db.SearchChunks(queryEmb, topK)
	if err != nil {
		return nil, fmt.Errorf("searching chunks: %w", err)
	}

	if len(results) == 0 {
		return &Answer{
			Question: question,
			Response: "I couldn't find any relevant information in the knowledge base. Try indexing more sources or rephrasing your question.",
			Latency:  time.Since(start),
		}, nil
	}

	// Step 3: Deduplicate by document and build context
	seen := map[string]bool{}
	var sources []SourceRef
	var contextChunks []string

	for _, r := range results {
		if seen[r.Chunk.DocumentID] {
			continue
		}
		seen[r.Chunk.DocumentID] = true

		sources = append(sources, SourceRef{
			DocID:  r.Chunk.DocumentID,
			Title:  r.DocTitle,
			Type:   r.DocType,
			URL:    r.DocURL,
			Author: r.DocAuthor,
			Score:  r.Score,
			Excerpt: truncate(r.Chunk.Content, 200),
		})

		contextChunks = append(contextChunks, fmt.Sprintf("[Source: %s (%s) by %s]\n%s",
			r.DocTitle, r.DocType, r.DocAuthor, r.Chunk.Content))
	}

	// Step 4: Enrich with knowledge graph context
	graphContext := e.enrichWithGraph(question)

	// Step 5: Synthesize answer with LLM
	synthesisPrompt := buildSynthesisPrompt(question, contextChunks, graphContext)

	temp := 0.3
	maxTokens := 1000
	resp, err := e.llm.Send(ctx, &provider.Request{
		Model:       e.model,
		Messages:    []provider.Message{{Role: "user", Content: synthesisPrompt}},
		Temperature: &temp,
		MaxTokens:   &maxTokens,
	})
	if err != nil {
		return nil, fmt.Errorf("LLM synthesis: %w", err)
	}

	response := ""
	if len(resp.Choices) > 0 {
		response = resp.Choices[0].Message.Content
	}

	cost := float64(resp.Usage.TotalTokens) * 0.00015
	latency := time.Since(start)

	answer := &Answer{
		Question:   question,
		Response:   response,
		Sources:    sources,
		ChunksUsed: len(results),
		Latency:    latency,
		Cost:       cost,
	}

	// Save query to history
	sourcesJSON, _ := json.Marshal(sources)
	queryID := fmt.Sprintf("q-%d", time.Now().UnixNano())
	e.db.SaveQuery(queryID, question, response, string(sourcesJSON), len(results), latency.Milliseconds(), cost)

	return answer, nil
}

// enrichWithGraph looks for entities mentioned in the question and adds graph context.
func (e *Engine) enrichWithGraph(question string) string {
	// Extract potential entity names from the question (simple word matching)
	words := strings.Fields(question)
	var graphInfo []string

	for _, word := range words {
		if len(word) < 3 {
			continue
		}
		clean := strings.Trim(word, "?.!,;:'\"")

		entities, err := e.db.FindEntitiesByName(clean, 3)
		if err != nil || len(entities) == 0 {
			continue
		}

		for _, ent := range entities {
			related, rels, err := e.db.GetRelatedEntities(ent.ID, 5)
			if err != nil || len(related) == 0 {
				continue
			}

			var relDesc []string
			for i, r := range related {
				if i < len(rels) {
					relDesc = append(relDesc, fmt.Sprintf("%s -[%s]-> %s", ent.Name, rels[i].Type, r.Name))
				}
			}

			if len(relDesc) > 0 {
				graphInfo = append(graphInfo, fmt.Sprintf("Entity '%s' (%s): %s",
					ent.Name, ent.Type, strings.Join(relDesc, "; ")))
			}
		}
	}

	if len(graphInfo) == 0 {
		return ""
	}
	return strings.Join(graphInfo, "\n")
}

func buildSynthesisPrompt(question string, chunks []string, graphContext string) string {
	var sb strings.Builder

	sb.WriteString("You are a knowledge assistant that answers questions about a codebase and its history.\n")
	sb.WriteString("Answer based ONLY on the provided context. If the context doesn't contain enough information, say so.\n")
	sb.WriteString("Cite your sources by referencing the [Source: ...] tags.\n")
	sb.WriteString("Be specific and concise. Include file paths, commit SHAs, PR numbers, and names when relevant.\n\n")

	sb.WriteString("CONTEXT FROM KNOWLEDGE BASE:\n")
	sb.WriteString("───────────────────────────\n")
	for _, chunk := range chunks {
		sb.WriteString(chunk)
		sb.WriteString("\n\n")
	}

	if graphContext != "" {
		sb.WriteString("KNOWLEDGE GRAPH RELATIONSHIPS:\n")
		sb.WriteString("─────────────────────────────\n")
		sb.WriteString(graphContext)
		sb.WriteString("\n\n")
	}

	sb.WriteString("───────────────────────────\n\n")
	sb.WriteString("QUESTION: ")
	sb.WriteString(question)
	sb.WriteString("\n\nProvide a clear, specific answer with source citations:")

	return sb.String()
}

func truncate(s string, n int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) > n {
		return s[:n] + "..."
	}
	return s
}
