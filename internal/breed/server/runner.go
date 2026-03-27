package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/stockyard-dev/stockyard/internal/breed/evolve"
	"github.com/stockyard-dev/stockyard/internal/breed/store"
)

// evolveRequest is the JSON body for POST /api/populations/{id}/evolve.
type evolveRequest struct {
	APIKey      string `json:"api_key"`
	Generations int    `json:"generations"` // how many generations to run (default 3)
	Model       string `json:"model"`       // model for generation + judging (default gpt-4o-mini)
}

// evolveResult is returned when evolution completes.
type evolveResult struct {
	PopulationID string           `json:"population_id"`
	Generations  int              `json:"generations_run"`
	FinalGen     int              `json:"final_generation"`
	BestFitness  float64          `json:"best_fitness"`
	BestTagline  string           `json:"best_tagline"`
	BestPrompt   string           `json:"best_prompt"`
	TopGenomes   []genomeResult   `json:"top_genomes"`
}

type genomeResult struct {
	ID       string  `json:"id"`
	Prompt   string  `json:"prompt"`
	Output   string  `json:"output"`
	Fitness  float64 `json:"fitness"`
}

// seedPrompts v2: structurally diverse, product-aware seeds.
// Each uses different prompt architecture (persona, analogy, constraint, fact-heavy,
// emotion, competitive, minimal) and encodes specific Stockyard differentiators.
var seedPrompts = []string{
	// Persona-based
	`You are the person who wrote "There's a computer science to advertising" for Cloudflare. Write a tagline for Stockyard: a single Go binary that replaces your entire LLM infrastructure stack (proxy, caching, rate limiting, failover, cost tracking, guardrails). One line, under 10 words.`,
	`You are a developer who just replaced 8 separate LLM tools with one 25MB binary called Stockyard. Write the tagline you'd tweet about it. Raw, authentic, under 12 words.`,
	`You are Paul Graham writing about a new kind of infrastructure tool. Stockyard is one binary that sits between apps and LLM providers with 76 middleware modules, 16 providers, 400ns overhead. Distill this into a tagline a YC founder would remember.`,
	// Analogy-based
	`nginx is to HTTP as Stockyard is to LLMs. Write a tagline that captures this analogy. Stockyard proxies every AI request through 70 configurable middleware modules in a single binary. Under 10 words.`,
	`Stockyard is to LLM APIs what a cattle ranch foreman is to a herd: it routes, monitors, protects, and optimizes every request. Write a tagline that uses this western metaphor subtly. One sentence.`,
	`Docker made "build once, run anywhere." Stockyard makes "one binary, every LLM." Write a tagline with that same structural clarity. Under 8 words.`,
	// Constraint-focused
	`Write a tagline for Stockyard. Rules: must be under 6 words, must contain a verb, must communicate that it's a single binary that handles all LLM infrastructure. No buzzwords.`,
	`Write a Stockyard tagline that would fit on a laptop sticker (max 5 words). Stockyard is a Go binary that proxies LLM requests with caching, failover, cost tracking, and 76 middleware modules.`,
	`Write exactly one sentence describing Stockyard for a developer who has never heard of it. It's a single binary LLM proxy with 29 products, 16 provider integrations, and sub-microsecond middleware overhead. Make them want to try it.`,
	// Fact-heavy
	`Stockyard facts: single Go binary (25MB), 76 middleware modules at 400ns per chain, 16 LLM providers, 29 products, source-available, zero dependencies, sub-microsecond overhead. Write a tagline that makes the technical audience on Hacker News stop scrolling. One line.`,
	`Stockyard runs between your app and OpenAI/Anthropic/Groq/etc. Every request gets: cost tracking, caching, rate limiting, guardrails, failover, provenance certificates, confidence scoring. All in one binary you deploy in 30 seconds. Tagline. One line.`,
	`Before Stockyard: 8 SaaS subscriptions for LLM ops. After Stockyard: one binary, self-hosted, source-available. Write a tagline about this transformation. Under 10 words.`,
	// Output-style variants
	`Write a hero section tagline for stockyard.dev. Format: "[Bold claim] -- [supporting detail]." Stockyard is a single Go binary with 76 middleware modules for proxying, caching, securing, and optimizing LLM traffic.`,
	`Write a tagline for Stockyard in the style of a Unix man page one-liner: "stockyard - [description]". It's a reverse proxy for LLM APIs with middleware, failover, cost tracking, and 29 integrated products.`,
	`Complete this sentence as a Stockyard tagline: "One binary to ___." Stockyard replaces your entire LLM infrastructure stack with a single Go executable.`,
	// Emotion-based
	`Write a tagline for Stockyard that makes a platform engineer feel relief. They're juggling 6 different LLM tools, worried about costs, fighting rate limits. Stockyard replaces all of it with one binary. Express that relief in under 10 words.`,
	`Write a tagline that expresses the confidence of knowing every LLM request is logged, cached, rate-limited, validated, and tracked automatically. Stockyard: one binary, total control. Under 10 words.`,
	// Competitive
	`Most LLM proxies do one thing. Stockyard does 70 things in 400 nanoseconds. Write a tagline that positions it as absurdly comprehensive without being arrogant. Under 10 words.`,
	`Everyone is building LLM wrappers. Stockyard built the infrastructure layer: proxy, middleware, 29 products, one binary. Write a tagline that says "we built the real thing." One line.`,
	// Minimal
	`Three words. Tagline. Stockyard is a single-binary LLM infrastructure platform. Go.`,
}

// runEvolution executes the genetic evolution loop for a population.
func (s *Server) runEvolution(pop *Population, req evolveRequest) evolveResult {
	popID := pop.ID
	model := req.Model
	if model == "" {
		model = "gpt-4o-mini"
	}
	generations := req.Generations
	if generations <= 0 {
		generations = 3
	}
	if generations > 10 {
		generations = 10
	}

	s.cfg.Store.UpdatePopulation(popID, pop.Generation, pop.BestFitness, "evolving")
	log.Printf("[breed] starting evolution: pop=%s generations=%d model=%s", popID, generations, model)

	currentGen := pop.Generation

	// Seed generation 0 if no genomes exist
	if currentGen == 0 {
		s.seedGeneration(popID, pop.PopulationSize)
	}

	// Run evolution loop
	type evaluated struct {
		genome  store.GenomeRecord
		output  string
		latency time.Duration
		cost    float64
	}
	var lastResults []evaluated // track last evaluated generation for results
	for g := 0; g < generations; g++ {
		currentGen++
		log.Printf("[breed] === generation %d ===", currentGen)

		// Get current population (from previous gen or seed)
		prevGen := currentGen - 1
		genomes, err := s.cfg.Store.ListGenomes(popID, prevGen)
		if err != nil || len(genomes) == 0 {
			log.Printf("[breed] no genomes in gen %d, seeding", prevGen)
			s.seedGeneration(popID, pop.PopulationSize)
			genomes, _ = s.cfg.Store.ListGenomes(popID, 0)
			if len(genomes) == 0 {
				log.Printf("[breed] seed failed, aborting")
				break
			}
		}

		// Phase 1: Evaluate — send each genome's prompt through proxy, get tagline
		var results []evaluated

		for _, g := range genomes {
			output, latency, cost := s.evaluateGenome(g, model, req.APIKey, pop.TargetEndpoint)
			results = append(results, evaluated{genome: g, output: output, latency: latency, cost: cost})
		}

		// Phase 2: Score — use LLM-as-judge to score each tagline
		for i := range results {
			quality := s.scoreTagline(results[i].output, model, req.APIKey, pop.TargetEndpoint)
			results[i].genome.QualityScore = quality
			results[i].genome.LatencyMs = float64(results[i].latency.Milliseconds())
			results[i].genome.Cost = results[i].cost
			// Composite fitness: 70% quality, 20% speed, 10% cost
			latNorm := 1.0 / (1.0 + results[i].genome.LatencyMs/1000.0)
			costNorm := 1.0 / (1.0 + results[i].genome.Cost)
			results[i].genome.Fitness = 0.7*quality + 0.2*latNorm + 0.1*costNorm

			// Persist fitness back to the genome record
			s.cfg.Store.UpdateGenomeFitness(
				results[i].genome.ID,
				results[i].genome.Fitness,
				quality,
				results[i].genome.LatencyMs,
				results[i].genome.Cost,
			)

			log.Printf("[breed] gen%d genome %s: fitness=%.3f quality=%.3f latency=%dms output=%q",
				currentGen, results[i].genome.ID[:16], results[i].genome.Fitness,
				quality, results[i].latency.Milliseconds(), truncate(results[i].output, 60))
		}

		// Save for final results
		lastResults = results

		// Phase 3: Select — pick top performers
		fitnesses := make([]float64, len(results))
		for i, r := range results {
			fitnesses[i] = r.genome.Fitness
		}
		parentCount := max(len(results)/3, 2)
		selectedIdx := evolve.Select(fitnesses, parentCount)

		var parents []evaluated
		for _, idx := range selectedIdx {
			if idx < len(results) {
				parents = append(parents, results[idx])
			}
		}

		// Phase 4: Breed — crossover + mutation to fill next generation
		var nextGen []store.GenomeRecord
		targetSize := pop.PopulationSize
		if targetSize > 20 {
			targetSize = 20 // cap to control costs
		}

		// Elitism: carry top 2 forward unchanged
		for i := 0; i < min(2, len(parents)); i++ {
			elite := parents[i].genome
			elite.ID = genID("bg")
			elite.Generation = currentGen
			elite.ParentA = parents[i].genome.ID
			elite.ParentB = ""
			elite.Mutations = `["elite"]`
			elite.Fitness = 0 // reset for re-evaluation
			nextGen = append(nextGen, elite)
		}

		// Fill rest with crossover + mutation
		for len(nextGen) < targetSize && len(parents) >= 2 {
			pA := parents[rand.Intn(len(parents))]
			pB := parents[rand.Intn(len(parents))]
			for pB.genome.ID == pA.genome.ID && len(parents) > 1 {
				pB = parents[rand.Intn(len(parents))]
			}

			// Crossover: blend system prompts
			childPrompt := crossoverPrompts(pA.genome.SystemPrompt, pB.genome.SystemPrompt)

			// Mutation
			mutatedPrompt, muts := evolve.Mutate(childPrompt, pop.MutationRate)

			// Temperature jitter
			temp := (pA.genome.Temperature + pB.genome.Temperature) / 2
			temp += (rand.Float64() - 0.5) * 0.2
			temp = math.Max(0.1, math.Min(1.5, temp))

			child := store.GenomeRecord{
				ID:           genID("bg"),
				PopulationID: popID,
				Generation:   currentGen,
				ParentA:      pA.genome.ID,
				ParentB:      pB.genome.ID,
				SystemPrompt: mutatedPrompt,
				Temperature:  temp,
				MaxTokens:    50,
			}
			if len(muts) > 0 {
				mutsJSON, _ := json.Marshal(muts)
				child.Mutations = string(mutsJSON)
			} else {
				child.Mutations = `["crossover"]`
			}
			nextGen = append(nextGen, child)
		}

		// Persist next generation
		for i := range nextGen {
			if err := s.cfg.Store.CreateGenome(&nextGen[i]); err != nil {
				log.Printf("[breed] failed to persist genome: %v", err)
			}
		}

		// Update population
		bestFit := 0.0
		for _, r := range results {
			if r.genome.Fitness > bestFit {
				bestFit = r.genome.Fitness
			}
		}
		s.cfg.Store.UpdatePopulation(popID, currentGen, bestFit, "evolving")
	}

	// Final: update population status
	bestFitOverall := 0.0
	for _, r := range lastResults {
		if r.genome.Fitness > bestFitOverall {
			bestFitOverall = r.genome.Fitness
		}
	}
	s.cfg.Store.UpdatePopulation(popID, currentGen, bestFitOverall, "idle")

	result := evolveResult{
		PopulationID: popID,
		Generations:  generations,
		FinalGen:     currentGen,
		BestFitness:  bestFitOverall,
	}

	// Build top genomes from the last evaluated generation (already scored)
	// Sort by fitness descending
	sortedResults := make([]evaluated, len(lastResults))
	copy(sortedResults, lastResults)
	for i := range sortedResults {
		for j := i + 1; j < len(sortedResults); j++ {
			if sortedResults[j].genome.Fitness > sortedResults[i].genome.Fitness {
				sortedResults[i], sortedResults[j] = sortedResults[j], sortedResults[i]
			}
		}
	}

	for i, r := range sortedResults {
		if i >= 5 { break }
		result.TopGenomes = append(result.TopGenomes, genomeResult{
			ID:      r.genome.ID,
			Prompt:  truncate(r.genome.SystemPrompt, 100),
			Output:  r.output,
			Fitness: r.genome.Fitness,
		})
	}

	if len(sortedResults) > 0 {
		result.BestTagline = sortedResults[0].output
		result.BestPrompt = sortedResults[0].genome.SystemPrompt
	}

	log.Printf("[breed] evolution complete: %d generations, best_fitness=%.3f", generations, result.BestFitness)
	return result
}

// seedGeneration creates the initial population with diverse prompts.
func (s *Server) seedGeneration(popID string, size int) {
	prompts := seedPrompts
	if size < len(prompts) {
		prompts = prompts[:size]
	}
	for _, prompt := range prompts {
		g := store.GenomeRecord{
			ID:           genID("bg"),
			PopulationID: popID,
			Generation:   0,
			SystemPrompt: prompt,
			Temperature:  0.7 + (rand.Float64()-0.5)*0.3,
			MaxTokens:    50,
			Mutations:    `["seed"]`,
		}
		s.cfg.Store.CreateGenome(&g)
	}
	log.Printf("[breed] seeded %d genomes for gen 0", len(prompts))
}

// evaluateGenome sends a genome's prompt through the proxy and gets a tagline.
func (s *Server) evaluateGenome(g store.GenomeRecord, model, apiKey, endpoint string) (string, time.Duration, float64) {
	body, _ := json.Marshal(map[string]any{
		"model": model,
		"messages": []map[string]string{
			{"role": "system", "content": g.SystemPrompt},
			{"role": "user", "content": "Generate a tagline for Stockyard. Output ONLY the tagline text, no quotes, no explanation."},
		},
		"max_tokens":  60,
		"temperature": g.Temperature,
	})

	req, _ := http.NewRequest("POST", endpoint, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("X-Stockyard-Tags", `{"source":"breed","population":"tagline"}`)

	client := &http.Client{Timeout: 30 * time.Second}
	start := time.Now()
	resp, err := client.Do(req)
	latency := time.Since(start)

	if err != nil {
		log.Printf("[breed] evaluate error: %v", err)
		return "", latency, 0
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		log.Printf("[breed] evaluate HTTP %d: %s", resp.StatusCode, truncate(string(respBody), 200))
		return "", latency, 0
	}

	content, _ := parseOAIResponse(respBody)

	// Rough cost estimate
	cost := float64(len(g.SystemPrompt)+30)/4/1000*0.00015 + float64(len(content))/4/1000*0.0006
	return strings.TrimSpace(content), latency, cost
}

// scoreTagline uses LLM-as-judge to score a tagline 0-1.
func (s *Server) scoreTagline(tagline, model, apiKey, endpoint string) float64 {
	if tagline == "" {
		return 0
	}

	judgePrompt := fmt.Sprintf(`Score this tagline for Stockyard, a single-binary LLM proxy platform, on a scale of 0 to 10.

Scoring criteria (weighted):
- Product accuracy (3 pts): Does it communicate what Stockyard does? (LLM proxy/infrastructure, single binary, middleware). Generic slogans that could apply to any product score 0 here.
- Memorability (3 pts): Would a developer remember this tomorrow?
- Brevity (2 pts): Shorter is better. Under 8 words = full marks. Over 15 = 0.
- Developer appeal (2 pts): Would this make a developer on Hacker News click through?

Tagline: "%s"

Reply with ONLY a number between 0 and 10, nothing else.`, tagline)

	body, _ := json.Marshal(map[string]any{
		"model": model,
		"messages": []map[string]string{
			{"role": "user", "content": judgePrompt},
		},
		"max_tokens":  5,
		"temperature": 0.1,
	})

	req, _ := http.NewRequest("POST", endpoint, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("X-Stockyard-Tags", `{"source":"breed","task":"judge"}`)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return 0.5 // default mid-score on error
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	content, _ := parseOAIResponse(respBody)
	content = strings.TrimSpace(content)

	// Parse score
	score, err := strconv.ParseFloat(strings.TrimRight(content, "./ "), 64)
	if err != nil {
		return 0.5
	}
	return math.Max(0, math.Min(1, score/10.0))
}

// crossoverPrompts blends two system prompts by taking sentences from each.
func crossoverPrompts(a, b string) string {
	aSentences := splitSentences(a)
	bSentences := splitSentences(b)

	if len(aSentences) == 0 { return b }
	if len(bSentences) == 0 { return a }

	// Take first half from A, second half from B
	mid := len(aSentences) / 2
	if mid == 0 { mid = 1 }

	var result []string
	result = append(result, aSentences[:mid]...)
	bMid := len(bSentences) / 2
	if bMid < len(bSentences) {
		result = append(result, bSentences[bMid:]...)
	}
	return strings.Join(result, " ")
}

func splitSentences(s string) []string {
	var sentences []string
	current := ""
	for _, r := range s {
		current += string(r)
		if r == '.' || r == '!' || r == '?' {
			trimmed := strings.TrimSpace(current)
			if trimmed != "" {
				sentences = append(sentences, trimmed)
			}
			current = ""
		}
	}
	if trimmed := strings.TrimSpace(current); trimmed != "" {
		sentences = append(sentences, trimmed)
	}
	return sentences
}

func parseOAIResponse(body []byte) (content, model string) {
	var resp struct {
		Model   string `json:"model"`
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", ""
	}
	model = resp.Model
	if len(resp.Choices) > 0 {
		content = resp.Choices[0].Message.Content
	}
	return
}

func truncate(s string, n int) string {
	if len(s) <= n { return s }
	return s[:n] + "..."
}

// Type alias for use in runner (Population from store)
type Population = store.Population
