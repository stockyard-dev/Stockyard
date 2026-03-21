package features

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/stockyard-dev/stockyard/internal/provider"
	"github.com/stockyard-dev/stockyard/internal/proxy"
)

// ConsensusEngine fans out requests to multiple models and compares their
// responses.  When the models agree (measured by text similarity) the primary
// response is returned immediately; when they disagree the configured
// on_disagree policy decides what happens.
type ConsensusEngine struct {
	conn      *sql.DB
	providers map[string]provider.Provider
	mu        sync.RWMutex
	configs   []ConsensusConfig
	lastLoad  time.Time
}

// ConsensusConfig stores the settings for a single consensus group.
type ConsensusConfig struct {
	ID                 string   `json:"id"`
	Name               string   `json:"name"`
	Models             []string `json:"models"`
	AgreementThreshold float64  `json:"agreement_threshold"`
	MinAgree           int      `json:"min_agree"`
	OnDisagree         string   `json:"on_disagree"`
	Enabled            bool     `json:"enabled"`
	CreatedAt          string   `json:"created_at"`
}

// ConsensusResult records the outcome of a single consensus check.
type ConsensusResult struct {
	ID               string             `json:"id"`
	TraceID          string             `json:"trace_id"`
	ConfigID         string             `json:"config_id"`
	ModelsQueried    []string           `json:"models_queried"`
	Responses        []ModelResponse    `json:"responses"`
	SimilarityMatrix map[string]float64 `json:"similarity_matrix"`
	ConsensusReached bool               `json:"consensus_reached"`
	FinalModel       string             `json:"final_model"`
	Escalated        bool               `json:"escalated"`
	CreatedAt        string             `json:"created_at"`
}

// ModelResponse captures a single model's reply inside a consensus round.
type ModelResponse struct {
	Model           string `json:"model"`
	ResponseSnippet string `json:"response_snippet"`
	LatencyMs       int64  `json:"latency_ms"`
	CostCents       int    `json:"cost_cents"`
}

// consensusBypassKey is used as a context key to prevent the consensus
// middleware from recursing into itself when it fans out sub-requests.
type consensusBypassKey struct{}

// NewConsensusEngine creates a ConsensusEngine backed by the given database
// connection and provider map.
func NewConsensusEngine(conn *sql.DB, providers map[string]provider.Provider) *ConsensusEngine {
	return &ConsensusEngine{
		conn:      conn,
		providers: providers,
	}
}

// loadConfigs returns the current consensus configurations, refreshing from the
// database at most once every 60 seconds.
func (ce *ConsensusEngine) loadConfigs() []ConsensusConfig {
	ce.mu.RLock()
	if time.Since(ce.lastLoad) < 60*time.Second && ce.configs != nil {
		defer ce.mu.RUnlock()
		return ce.configs
	}
	ce.mu.RUnlock()

	ce.mu.Lock()
	defer ce.mu.Unlock()

	// Double-check after acquiring write lock.
	if time.Since(ce.lastLoad) < 60*time.Second && ce.configs != nil {
		return ce.configs
	}

	rows, err := ce.conn.Query(`SELECT id, name, models, agreement_threshold, min_agree, on_disagree, enabled, created_at FROM consensus_configs ORDER BY created_at DESC`)
	if err != nil {
		log.Printf("consensus: load configs: %v", err)
		return ce.configs // return stale data on error
	}
	defer rows.Close()

	var configs []ConsensusConfig
	for rows.Next() {
		var cfg ConsensusConfig
		var modelsJSON string
		if err := rows.Scan(&cfg.ID, &cfg.Name, &modelsJSON, &cfg.AgreementThreshold, &cfg.MinAgree, &cfg.OnDisagree, &cfg.Enabled, &cfg.CreatedAt); err != nil {
			log.Printf("consensus: scan config: %v", err)
			continue
		}
		if err := json.Unmarshal([]byte(modelsJSON), &cfg.Models); err != nil {
			log.Printf("consensus: unmarshal models for %s: %v", cfg.ID, err)
			continue
		}
		configs = append(configs, cfg)
	}

	ce.configs = configs
	ce.lastLoad = time.Now()
	return configs
}

// ConsensusMiddleware returns proxy middleware that fans a request out to every
// model listed in the first matching ConsensusConfig, compares responses, and
// acts on the result.
func ConsensusMiddleware(ce *ConsensusEngine) proxy.Middleware {
	return func(next proxy.Handler) proxy.Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
			// Skip if bypass flag is set (prevents recursion).
			if ctx.Value(consensusBypassKey{}) != nil {
				return next(ctx, req)
			}

			// Load configs (cached 60s).
			configs := ce.loadConfigs()

			// Find matching config for this model.
			var cfg *ConsensusConfig
			for i := range configs {
				if !configs[i].Enabled {
					continue
				}
				for _, m := range configs[i].Models {
					if m == req.Model || m == "*" {
						cfg = &configs[i]
						break
					}
				}
				if cfg != nil {
					break
				}
			}

			if cfg == nil {
				return next(ctx, req) // No consensus config — pass through.
			}

			// Set bypass flag to prevent recursion.
			bypassCtx := context.WithValue(ctx, consensusBypassKey{}, true)

			// Fan out to all models concurrently.
			type result struct {
				model   string
				resp    *provider.Response
				err     error
				latency time.Duration
			}

			results := make([]result, len(cfg.Models))
			var wg sync.WaitGroup

			for i, model := range cfg.Models {
				wg.Add(1)
				go func(idx int, mdl string) {
					defer wg.Done()
					start := time.Now()

					// Create a copy of the request with the target model.
					subReq := *req
					subReq.Model = mdl

					// Use provider.Send directly (not the middleware chain).
					p, ok := ce.providers[providerForModel(mdl)]
					if !ok {
						// Fallback: use next handler with bypass.
						resp, err := next(bypassCtx, &subReq)
						results[idx] = result{mdl, resp, err, time.Since(start)}
						return
					}

					timeoutCtx, cancel := context.WithTimeout(bypassCtx, 30*time.Second)
					defer cancel()

					resp, err := p.Send(timeoutCtx, &subReq)
					results[idx] = result{mdl, resp, err, time.Since(start)}
				}(i, model)
			}
			wg.Wait()

			// Collect successful responses.
			var successResults []result
			for _, r := range results {
				if r.err == nil && r.resp != nil {
					successResults = append(successResults, r)
				}
			}

			if len(successResults) == 0 {
				return next(ctx, req) // All failed — fall through.
			}

			// Compare responses pairwise.
			texts := make([]string, len(successResults))
			for i, r := range successResults {
				if len(r.resp.Choices) > 0 {
					texts[i] = r.resp.Choices[0].Message.Content
				}
			}

			simMatrix := make(map[string]float64)
			agreementCount := 0
			totalPairs := 0
			totalSim := 0.0

			for i := 0; i < len(texts); i++ {
				for j := i + 1; j < len(texts); j++ {
					sim := computeSimilarity(texts[i], texts[j])
					key := fmt.Sprintf("%s:%s", successResults[i].model, successResults[j].model)
					simMatrix[key] = sim
					totalSim += sim
					totalPairs++
					if sim >= cfg.AgreementThreshold {
						agreementCount++
					}
				}
			}

			avgSim := 0.0
			if totalPairs > 0 {
				avgSim = totalSim / float64(totalPairs)
			}

			consensusReached := agreementCount >= cfg.MinAgree-1 // pairs needed

			// Build result for storage.
			modelResponses := make([]ModelResponse, len(successResults))
			for i, r := range successResults {
				snippet := ""
				if len(r.resp.Choices) > 0 {
					snippet = r.resp.Choices[0].Message.Content
					if len(snippet) > 500 {
						snippet = snippet[:500]
					}
				}
				modelResponses[i] = ModelResponse{
					Model:           r.model,
					ResponseSnippet: snippet,
					LatencyMs:       r.latency.Milliseconds(),
				}
			}

			// Record result asynchronously.
			go ce.recordResult(ConsensusResult{
				ID:               genConsensusID("cr_"),
				ConfigID:         cfg.ID,
				ModelsQueried:    cfg.Models,
				Responses:        modelResponses,
				SimilarityMatrix: simMatrix,
				ConsensusReached: consensusReached,
				FinalModel:       successResults[0].model,
				Escalated:        !consensusReached && cfg.OnDisagree == "escalate",
				CreatedAt:        time.Now().UTC().Format(time.RFC3339),
			})

			// Set consensus headers on the response we'll return.
			primaryResp := successResults[0].resp
			if primaryResp.Meta == nil {
				primaryResp.Meta = make(map[string]string)
			}
			primaryResp.Meta["X-Stockyard-Consensus"] = fmt.Sprintf("%v", consensusReached)
			models := make([]string, len(successResults))
			for i, r := range successResults {
				models[i] = r.model
			}
			primaryResp.Meta["X-Stockyard-Consensus-Models"] = strings.Join(models, ",")
			primaryResp.Meta["X-Stockyard-Consensus-Agreement"] = fmt.Sprintf("%.2f", avgSim)

			if consensusReached {
				return primaryResp, nil
			}

			// Handle disagreement.
			switch cfg.OnDisagree {
			case "return_majority":
				// Find response with highest average similarity.
				best := 0
				bestAvg := 0.0
				for i := range texts {
					sum := 0.0
					count := 0
					for j := range texts {
						if i == j {
							continue
						}
						key1 := fmt.Sprintf("%s:%s", successResults[i].model, successResults[j].model)
						key2 := fmt.Sprintf("%s:%s", successResults[j].model, successResults[i].model)
						if s, ok := simMatrix[key1]; ok {
							sum += s
							count++
						}
						if s, ok := simMatrix[key2]; ok {
							sum += s
							count++
						}
					}
					if count > 0 && sum/float64(count) > bestAvg {
						bestAvg = sum / float64(count)
						best = i
					}
				}
				return successResults[best].resp, nil
			case "return_primary":
				primaryResp.Meta["X-Stockyard-Consensus-Warning"] = "consensus_not_reached"
				return primaryResp, nil
			case "block":
				return nil, fmt.Errorf("consensus not reached: request blocked for safety (agreement: %.2f, threshold: %.2f)", avgSim, cfg.AgreementThreshold)
			default: // escalate
				return nil, fmt.Errorf("consensus not reached: %d models queried, agreement %.2f below threshold %.2f", len(successResults), avgSim, cfg.AgreementThreshold)
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Helper functions
// ---------------------------------------------------------------------------

// computeSimilarity returns the average of Jaccard and cosine similarity
// between two text strings.  Both texts are normalised (lowercased, punctuation
// stripped) before comparison.
func computeSimilarity(a, b string) float64 {
	if a == "" && b == "" {
		return 1.0
	}
	if a == "" || b == "" {
		return 0.0
	}

	wordsA := normalizeTokens(a)
	wordsB := normalizeTokens(b)

	jaccard := consensusJaccard(wordsA, wordsB)
	cosine := consensusCosine(wordsA, wordsB)

	return (jaccard + cosine) / 2.0
}

// normalizeTokens lowercases the text, strips punctuation and splits on
// whitespace.
func normalizeTokens(text string) []string {
	text = strings.ToLower(text)
	var b strings.Builder
	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.IsSpace(r) {
			b.WriteRune(r)
		} else {
			b.WriteRune(' ')
		}
	}
	return strings.Fields(b.String())
}

// jaccardSimilarity computes |intersection| / |union| over two word slices.
func consensusJaccard(a, b []string) float64 {
	setA := make(map[string]struct{}, len(a))
	for _, w := range a {
		setA[w] = struct{}{}
	}
	setB := make(map[string]struct{}, len(b))
	for _, w := range b {
		setB[w] = struct{}{}
	}

	intersection := 0
	for w := range setA {
		if _, ok := setB[w]; ok {
			intersection++
		}
	}

	union := len(setA) + len(setB) - intersection
	if union == 0 {
		return 1.0
	}
	return float64(intersection) / float64(union)
}

// cosineSimilarity computes the cosine similarity of word-frequency vectors.
func consensusCosine(a, b []string) float64 {
	freqA := wordFreq(a)
	freqB := wordFreq(b)

	dot := 0.0
	magA := 0.0
	magB := 0.0

	for w, cA := range freqA {
		magA += float64(cA * cA)
		if cB, ok := freqB[w]; ok {
			dot += float64(cA * cB)
		}
	}
	for _, cB := range freqB {
		magB += float64(cB * cB)
	}

	if magA == 0 || magB == 0 {
		return 0.0
	}
	return dot / (math.Sqrt(magA) * math.Sqrt(magB))
}

// wordFreq builds a word → count map.
func wordFreq(words []string) map[string]int {
	freq := make(map[string]int, len(words))
	for _, w := range words {
		freq[w]++
	}
	return freq
}

// providerForModel returns a provider key based on simple model-name
// heuristics.
func providerForModel(model string) string {
	m := strings.ToLower(model)
	switch {
	case strings.Contains(m, "gpt") || strings.Contains(m, "o1") || strings.Contains(m, "o3"):
		return "openai"
	case strings.Contains(m, "claude"):
		return "anthropic"
	case strings.Contains(m, "gemini"):
		return "google"
	case strings.Contains(m, "llama") || strings.Contains(m, "mixtral"):
		return "groq"
	case strings.Contains(m, "mistral"):
		return "mistral"
	case strings.Contains(m, "command"):
		return "cohere"
	default:
		return "openai"
	}
}

// recordResult persists a ConsensusResult to the database.
func (ce *ConsensusEngine) recordResult(r ConsensusResult) {
	modelsJSON, err := json.Marshal(r.ModelsQueried)
	if err != nil {
		log.Printf("consensus: marshal models_queried: %v", err)
		return
	}
	responsesJSON, err := json.Marshal(r.Responses)
	if err != nil {
		log.Printf("consensus: marshal responses: %v", err)
		return
	}
	simJSON, err := json.Marshal(r.SimilarityMatrix)
	if err != nil {
		log.Printf("consensus: marshal similarity_matrix: %v", err)
		return
	}

	_, err = ce.conn.Exec(
		`INSERT INTO consensus_results (id, trace_id, config_id, models_queried, responses, similarity_matrix, consensus_reached, final_model, escalated, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		r.ID, r.TraceID, r.ConfigID, string(modelsJSON), string(responsesJSON), string(simJSON), r.ConsensusReached, r.FinalModel, r.Escalated, r.CreatedAt,
	)
	if err != nil {
		log.Printf("consensus: insert result: %v", err)
	}
}

// genConsensusID generates a random hex ID with the given prefix.
func genConsensusID(prefix string) string {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		return prefix + fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return prefix + hex.EncodeToString(b)
}

// Ensure unused imports are referenced.
var _ = http.StatusOK
