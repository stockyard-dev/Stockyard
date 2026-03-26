package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/stockyard-dev/stockyard/internal/breed/store"
)

// tournamentRequest is the JSON body for POST /api/tournaments.
type tournamentRequest struct {
	Name       string   `json:"name"`
	BracketSize int     `json:"bracket_size"` // 4, 8, or 16 (default 8)
	GenomeIDs  []string `json:"genome_ids"`   // explicit genomes, or empty for auto-select top N
	Model      string   `json:"model"`        // judge model (default gpt-4o-mini)
	APIKey     string   `json:"api_key"`
}

// tournamentResult is returned when a tournament completes.
type tournamentResult struct {
	TournamentID   string         `json:"tournament_id"`
	Name           string         `json:"name"`
	Status         string         `json:"status"`
	Rounds         int            `json:"rounds"`
	TotalMatches   int            `json:"total_matches"`
	ChampionID     string         `json:"champion_id"`
	ChampionOutput string         `json:"champion_output"`
	Bracket        []roundSummary `json:"bracket"`
}

type roundSummary struct {
	Round   int            `json:"round"`
	Matches []matchSummary `json:"matches"`
}

type matchSummary struct {
	GenomeA   string  `json:"genome_a"`
	OutputA   string  `json:"output_a"`
	ScoreA    float64 `json:"score_a"`
	GenomeB   string  `json:"genome_b"`
	OutputB   string  `json:"output_b"`
	ScoreB    float64 `json:"score_b"`
	Winner    string  `json:"winner"`
	Judgment  string  `json:"judgment"`
}

func (s *Server) handleCreateTournament(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Store == nil {
		writeJSON(w, 503, map[string]string{"error": "store not available"})
		return
	}

	var req tournamentRequest
	json.NewDecoder(r.Body).Decode(&req)

	if req.APIKey == "" {
		req.APIKey = r.Header.Get("X-Breed-Key")
	}
	if req.APIKey == "" {
		writeJSON(w, 400, map[string]string{"error": "api_key required"})
		return
	}
	if req.Model == "" {
		req.Model = "gpt-4o-mini"
	}
	if req.Name == "" {
		req.Name = fmt.Sprintf("Tournament %s", time.Now().Format("2006-01-02 15:04"))
	}

	// Determine bracket size (must be power of 2)
	bracketSize := req.BracketSize
	if bracketSize <= 0 {
		bracketSize = 8
	}
	bracketSize = nearestPow2(bracketSize)
	if bracketSize < 4 {
		bracketSize = 4
	}
	if bracketSize > 16 {
		bracketSize = 16
	}

	// Select genomes
	var genomes []store.GenomeRecord
	if len(req.GenomeIDs) > 0 {
		// Use explicit genome IDs — look them up
		for _, gid := range req.GenomeIDs {
			gs, _ := s.cfg.Store.ListGenomes("", 0) // can't query by ID directly, use top genomes
			for _, g := range gs {
				if g.ID == gid {
					genomes = append(genomes, g)
					break
				}
			}
		}
	}

	// If not enough explicit genomes, auto-select top across all populations
	if len(genomes) < bracketSize {
		top, err := s.cfg.Store.GetTopGenomesAcrossPopulations(bracketSize)
		if err != nil || len(top) < 2 {
			writeJSON(w, 400, map[string]string{"error": "not enough scored genomes for a tournament — run evolution first"})
			return
		}
		// Merge, dedup
		seen := map[string]bool{}
		for _, g := range genomes {
			seen[g.ID] = true
		}
		for _, g := range top {
			if !seen[g.ID] {
				genomes = append(genomes, g)
				seen[g.ID] = true
			}
		}
	}

	// Trim to bracket size
	if len(genomes) > bracketSize {
		genomes = genomes[:bracketSize]
	}

	// Pad with byes if needed (shouldn't happen with auto-select)
	if len(genomes) < bracketSize {
		bracketSize = nearestPow2(len(genomes))
		if bracketSize < 4 {
			bracketSize = 4
		}
		genomes = genomes[:min(len(genomes), bracketSize)]
	}

	totalRounds := int(math.Log2(float64(bracketSize)))

	// Create tournament
	t := &store.Tournament{
		ID:          genID("trn"),
		Name:        req.Name,
		BracketSize: bracketSize,
		Status:      "running",
		TotalRounds: totalRounds,
		Model:       req.Model,
	}
	if err := s.cfg.Store.CreateTournament(t); err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}

	// Get target endpoint from the first genome's population
	endpoint := ""
	if len(genomes) > 0 {
		pop, err := s.cfg.Store.GetPopulation(genomes[0].PopulationID)
		if err == nil {
			endpoint = pop.TargetEndpoint
		}
	}
	if endpoint == "" {
		endpoint = fmt.Sprintf("http://localhost:%d/v1/chat/completions", 8080)
	}

	// Run the bracket
	result := s.runTournament(t, genomes, req.APIKey, endpoint)
	writeJSON(w, 200, result)
}

func (s *Server) handleListTournaments(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Store == nil {
		writeJSON(w, 200, []any{})
		return
	}
	tournaments, err := s.cfg.Store.ListTournaments()
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	if tournaments == nil {
		tournaments = []store.Tournament{}
	}
	writeJSON(w, 200, tournaments)
}

func (s *Server) handleGetTournament(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Store == nil {
		writeJSON(w, 503, map[string]string{"error": "store not available"})
		return
	}
	t, err := s.cfg.Store.GetTournament(r.PathValue("id"))
	if err != nil {
		writeJSON(w, 404, map[string]string{"error": "tournament not found"})
		return
	}

	// Include all matches
	matches, _ := s.cfg.Store.AllMatches(t.ID)
	if matches == nil {
		matches = []store.Match{}
	}

	writeJSON(w, 200, map[string]any{
		"tournament": t,
		"matches":    matches,
	})
}

// runTournament executes single-elimination bracket.
func (s *Server) runTournament(t *store.Tournament, genomes []store.GenomeRecord, apiKey, endpoint string) tournamentResult {
	result := tournamentResult{
		TournamentID: t.ID,
		Name:         t.Name,
		Status:       "running",
	}

	// Current contenders (by genome ID → genome)
	contenders := make([]store.GenomeRecord, len(genomes))
	copy(contenders, genomes)

	log.Printf("[breed] tournament %s: %d contenders, %d rounds", t.ID, len(contenders), t.TotalRounds)

	for round := 1; round <= t.TotalRounds; round++ {
		log.Printf("[breed] === round %d (%d contenders) ===", round, len(contenders))

		var roundMatches []matchSummary
		var winners []store.GenomeRecord

		for i := 0; i+1 < len(contenders); i += 2 {
			a := contenders[i]
			b := contenders[i+1]

			// Both generate outputs
			outputA, _, _ := s.evaluateGenome(a, t.Model, apiKey, endpoint)
			outputB, _, _ := s.evaluateGenome(b, t.Model, apiKey, endpoint)

			// Judge picks winner
			winnerID, scoreA, scoreB, judgment := s.judgeMatch(outputA, outputB, a.ID, b.ID, t.Model, apiKey, endpoint)

			// Persist match
			m := &store.Match{
				ID:           genID("mtc"),
				TournamentID: t.ID,
				Round:        round,
				MatchOrder:   i / 2,
				GenomeA:      a.ID,
				GenomeB:      b.ID,
			}
			s.cfg.Store.CreateMatch(m)
			s.cfg.Store.UpdateMatch(m.ID, outputA, outputB, winnerID, judgment, scoreA, scoreB)

			ms := matchSummary{
				GenomeA: a.ID, OutputA: outputA, ScoreA: scoreA,
				GenomeB: b.ID, OutputB: outputB, ScoreB: scoreB,
				Winner: winnerID, Judgment: judgment,
			}
			roundMatches = append(roundMatches, ms)

			// Advance winner
			if winnerID == a.ID {
				winners = append(winners, a)
			} else {
				winners = append(winners, b)
			}

			log.Printf("[breed] match: %s vs %s → winner %s (%.1f vs %.1f)",
				a.ID[:12], b.ID[:12], winnerID[:12], scoreA, scoreB)
		}

		// Handle odd contender (bye to next round)
		if len(contenders)%2 == 1 {
			winners = append(winners, contenders[len(contenders)-1])
			log.Printf("[breed] bye: %s advances", contenders[len(contenders)-1].ID[:12])
		}

		result.Bracket = append(result.Bracket, roundSummary{Round: round, Matches: roundMatches})
		result.TotalMatches += len(roundMatches)

		// Update tournament progress
		s.cfg.Store.UpdateTournament(t.ID, "running", "", "", round)

		contenders = winners
	}

	// Champion
	if len(contenders) > 0 {
		champion := contenders[0]
		// Generate champion's final output
		champOutput, _, _ := s.evaluateGenome(champion, t.Model, apiKey, endpoint)

		result.ChampionID = champion.ID
		result.ChampionOutput = champOutput
		result.Status = "complete"
		result.Rounds = t.TotalRounds

		s.cfg.Store.UpdateTournament(t.ID, "complete", champion.ID, champOutput, t.TotalRounds)

		log.Printf("[breed] tournament %s complete: champion=%s output=%q",
			t.ID, champion.ID[:12], truncate(champOutput, 60))
	}

	return result
}

// judgeMatch has an LLM compare two outputs head-to-head and pick a winner.
func (s *Server) judgeMatch(outputA, outputB, idA, idB, model, apiKey, endpoint string) (winnerID string, scoreA, scoreB float64, judgment string) {
	if outputA == "" && outputB == "" {
		return idA, 5, 5, "both empty — coin flip"
	}
	if outputA == "" {
		return idB, 0, 10, "A produced no output"
	}
	if outputB == "" {
		return idA, 10, 0, "B produced no output"
	}

	prompt := fmt.Sprintf(`You are judging a head-to-head tagline competition for Stockyard, a single-binary LLM proxy platform with 70 middleware modules.

Compare these two taglines:

TAGLINE A: "%s"
TAGLINE B: "%s"

Score each 0-10 on: product accuracy (does it describe an LLM proxy?), memorability, brevity, developer appeal.

Respond in EXACTLY this format (3 lines, nothing else):
SCORE_A: [number]
SCORE_B: [number]
REASON: [one sentence explaining why the winner is better]`, outputA, outputB)

	body, _ := json.Marshal(map[string]any{
		"model": model,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
		"max_tokens":  80,
		"temperature": 0.2,
	})

	req, _ := http.NewRequest("POST", endpoint, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("X-Stockyard-Tags", `{"source":"breed","task":"tournament_judge"}`)

	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		// Fallback: use string length as tiebreaker (shorter = better for taglines)
		if len(outputA) <= len(outputB) {
			return idA, 6, 5, "judge unavailable — shorter tagline wins"
		}
		return idB, 5, 6, "judge unavailable — shorter tagline wins"
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	content, _ := parseOAIResponse(respBody)
	content = strings.TrimSpace(content)

	// Parse structured response
	scoreA = 5
	scoreB = 5
	judgment = "judge could not determine"

	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "SCORE_A:") {
			if v, err := strconv.ParseFloat(strings.TrimSpace(strings.TrimPrefix(line, "SCORE_A:")), 64); err == nil {
				scoreA = v
			}
		} else if strings.HasPrefix(line, "SCORE_B:") {
			if v, err := strconv.ParseFloat(strings.TrimSpace(strings.TrimPrefix(line, "SCORE_B:")), 64); err == nil {
				scoreB = v
			}
		} else if strings.HasPrefix(line, "REASON:") {
			judgment = strings.TrimSpace(strings.TrimPrefix(line, "REASON:"))
		}
	}

	if scoreA >= scoreB {
		return idA, scoreA, scoreB, judgment
	}
	return idB, scoreA, scoreB, judgment
}

func nearestPow2(n int) int {
	if n <= 4 { return 4 }
	p := 1
	for p < n {
		p *= 2
	}
	// Return the nearest (could be p or p/2)
	if p-n > n-p/2 {
		return p / 2
	}
	return p
}
