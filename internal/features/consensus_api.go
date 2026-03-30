package features

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"
)

// RegisterConsensusRoutes registers the HTTP endpoints for managing consensus
// configurations and viewing consensus results.
func (ce *ConsensusEngine) RegisterConsensusRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/proxy/consensus/configs", ce.handleCreateConfig)
	mux.HandleFunc("GET /api/proxy/consensus/configs", ce.handleListConfigs)
	mux.HandleFunc("PUT /api/proxy/consensus/configs/{id}", ce.handleUpdateConfig)
	mux.HandleFunc("DELETE /api/proxy/consensus/configs/{id}", ce.handleDeleteConfig)
	mux.HandleFunc("GET /api/proxy/consensus/results", ce.handleListResults)
	mux.HandleFunc("GET /api/proxy/consensus/results/{id}", ce.handleGetResult)
	mux.HandleFunc("GET /api/proxy/consensus/stats", ce.handleStats)
}

// handleCreateConfig creates a new consensus configuration.
func (ce *ConsensusEngine) handleCreateConfig(w http.ResponseWriter, r *http.Request) {
	var cfg ConsensusConfig
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON: " + err.Error()})
		return
	}

	if cfg.Name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name is required"})
		return
	}
	if len(cfg.Models) < 2 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "at least 2 models are required"})
		return
	}
	if cfg.AgreementThreshold <= 0 || cfg.AgreementThreshold > 1 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "agreement_threshold must be between 0 and 1"})
		return
	}
	if cfg.MinAgree < 2 {
		cfg.MinAgree = 2
	}
	if cfg.OnDisagree == "" {
		cfg.OnDisagree = "escalate"
	}

	cfg.ID = genConsensusID("cc_")
	cfg.CreatedAt = time.Now().UTC().Format(time.RFC3339)

	modelsJSON, err := json.Marshal(cfg.Models)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "marshal models: " + err.Error()})
		return
	}

	_, err = ce.conn.Exec(
		`INSERT INTO consensus_configs (id, name, models, agreement_threshold, min_agree, on_disagree, enabled, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		cfg.ID, cfg.Name, string(modelsJSON), cfg.AgreementThreshold, cfg.MinAgree, cfg.OnDisagree, cfg.Enabled, cfg.CreatedAt,
	)
	if err != nil {
		log.Printf("consensus: insert config: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to create config"})
		return
	}

	// Invalidate cache so next request picks up the new config.
	ce.mu.Lock()
	ce.lastLoad = time.Time{}
	ce.mu.Unlock()

	writeJSON(w, http.StatusCreated, cfg)
}

// handleListConfigs returns all consensus configurations.
func (ce *ConsensusEngine) handleListConfigs(w http.ResponseWriter, r *http.Request) {
	rows, err := ce.conn.Query(`SELECT id, name, models, agreement_threshold, min_agree, on_disagree, enabled, created_at FROM consensus_configs ORDER BY created_at DESC`)
	if err != nil {
		log.Printf("consensus: list configs: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list configs"})
		return
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
			log.Printf("consensus: unmarshal models: %v", err)
			continue
		}
		configs = append(configs, cfg)
	}
	if err := rows.Err(); err != nil {
		log.Printf("[db] rows iteration error: %v", err)
	}

	if configs == nil {
		configs = []ConsensusConfig{}
	}
	writeJSON(w, http.StatusOK, configs)
}

// handleUpdateConfig updates an existing consensus configuration.
func (ce *ConsensusEngine) handleUpdateConfig(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "id is required"})
		return
	}

	var update struct {
		Name               *string  `json:"name"`
		Models             []string `json:"models"`
		AgreementThreshold *float64 `json:"agreement_threshold"`
		MinAgree           *int     `json:"min_agree"`
		OnDisagree         *string  `json:"on_disagree"`
		Enabled            *bool    `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON: " + err.Error()})
		return
	}

	// Fetch existing config.
	var cfg ConsensusConfig
	var modelsJSON string
	err := ce.conn.QueryRow(
		`SELECT id, name, models, agreement_threshold, min_agree, on_disagree, enabled, created_at FROM consensus_configs WHERE id = ?`, id,
	).Scan(&cfg.ID, &cfg.Name, &modelsJSON, &cfg.AgreementThreshold, &cfg.MinAgree, &cfg.OnDisagree, &cfg.Enabled, &cfg.CreatedAt)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "config not found"})
		return
	}
	_ = json.Unmarshal([]byte(modelsJSON), &cfg.Models)

	// Apply updates.
	if update.Name != nil {
		cfg.Name = *update.Name
	}
	if update.Models != nil {
		if len(update.Models) < 2 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "at least 2 models are required"})
			return
		}
		cfg.Models = update.Models
	}
	if update.AgreementThreshold != nil {
		if *update.AgreementThreshold <= 0 || *update.AgreementThreshold > 1 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "agreement_threshold must be between 0 and 1"})
			return
		}
		cfg.AgreementThreshold = *update.AgreementThreshold
	}
	if update.MinAgree != nil {
		cfg.MinAgree = *update.MinAgree
	}
	if update.OnDisagree != nil {
		cfg.OnDisagree = *update.OnDisagree
	}
	if update.Enabled != nil {
		cfg.Enabled = *update.Enabled
	}

	newModelsJSON, marshalErr := json.Marshal(cfg.Models)
	if marshalErr != nil {
		newModelsJSON = []byte("{}")
	}

	_, err = ce.conn.Exec(
		`UPDATE consensus_configs SET name=?, models=?, agreement_threshold=?, min_agree=?, on_disagree=?, enabled=? WHERE id=?`,
		cfg.Name, string(newModelsJSON), cfg.AgreementThreshold, cfg.MinAgree, cfg.OnDisagree, cfg.Enabled, id,
	)
	if err != nil {
		log.Printf("consensus: update config %s: %v", id, err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to update config"})
		return
	}

	// Invalidate cache.
	ce.mu.Lock()
	ce.lastLoad = time.Time{}
	ce.mu.Unlock()

	writeJSON(w, http.StatusOK, cfg)
}

// handleDeleteConfig deletes a consensus configuration.
func (ce *ConsensusEngine) handleDeleteConfig(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "id is required"})
		return
	}

	result, err := ce.conn.Exec(`DELETE FROM consensus_configs WHERE id = ?`, id)
	if err != nil {
		log.Printf("consensus: delete config %s: %v", id, err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to delete config"})
		return
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "config not found"})
		return
	}

	// Invalidate cache.
	ce.mu.Lock()
	ce.lastLoad = time.Time{}
	ce.mu.Unlock()

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted", "id": id})
}

// handleListResults returns recent consensus results.
func (ce *ConsensusEngine) handleListResults(w http.ResponseWriter, r *http.Request) {
	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 500 {
			limit = parsed
		}
	}

	rows, err := ce.conn.Query(
		`SELECT id, trace_id, config_id, models_queried, responses, similarity_matrix, consensus_reached, final_model, escalated, created_at
		 FROM consensus_results ORDER BY created_at DESC LIMIT ?`, limit,
	)
	if err != nil {
		log.Printf("consensus: list results: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list results"})
		return
	}
	defer rows.Close()

	var results []ConsensusResult
	for rows.Next() {
		var cr ConsensusResult
		var modelsJSON, responsesJSON, simJSON string
		if err := rows.Scan(&cr.ID, &cr.TraceID, &cr.ConfigID, &modelsJSON, &responsesJSON, &simJSON, &cr.ConsensusReached, &cr.FinalModel, &cr.Escalated, &cr.CreatedAt); err != nil {
			log.Printf("consensus: scan result: %v", err)
			continue
		}
		_ = json.Unmarshal([]byte(modelsJSON), &cr.ModelsQueried)
		_ = json.Unmarshal([]byte(responsesJSON), &cr.Responses)
		_ = json.Unmarshal([]byte(simJSON), &cr.SimilarityMatrix)
		results = append(results, cr)
	}
	if err := rows.Err(); err != nil {
		log.Printf("[db] rows iteration error: %v", err)
	}

	if results == nil {
		results = []ConsensusResult{}
	}
	writeJSON(w, http.StatusOK, results)
}

// handleGetResult returns a single consensus result by ID.
func (ce *ConsensusEngine) handleGetResult(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "id is required"})
		return
	}

	var cr ConsensusResult
	var modelsJSON, responsesJSON, simJSON string
	err := ce.conn.QueryRow(
		`SELECT id, trace_id, config_id, models_queried, responses, similarity_matrix, consensus_reached, final_model, escalated, created_at
		 FROM consensus_results WHERE id = ?`, id,
	).Scan(&cr.ID, &cr.TraceID, &cr.ConfigID, &modelsJSON, &responsesJSON, &simJSON, &cr.ConsensusReached, &cr.FinalModel, &cr.Escalated, &cr.CreatedAt)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "result not found"})
		return
	}

	_ = json.Unmarshal([]byte(modelsJSON), &cr.ModelsQueried)
	_ = json.Unmarshal([]byte(responsesJSON), &cr.Responses)
	_ = json.Unmarshal([]byte(simJSON), &cr.SimilarityMatrix)

	writeJSON(w, http.StatusOK, cr)
}

// handleStats returns aggregate consensus statistics.
func (ce *ConsensusEngine) handleStats(w http.ResponseWriter, r *http.Request) {
	type stats struct {
		TotalChecks    int     `json:"total_checks"`
		ConsensusRate  float64 `json:"consensus_rate"`
		AvgAgreement   float64 `json:"avg_agreement"`
		EscalationRate float64 `json:"escalation_rate"`
		Last24h        int     `json:"last_24h"`
	}

	var s stats

	// Total checks.
	_ = ce.conn.QueryRow(`SELECT COUNT(*) FROM consensus_results`).Scan(&s.TotalChecks)

	if s.TotalChecks == 0 {
		writeJSON(w, http.StatusOK, s)
		return
	}

	// Consensus rate.
	var consensusCount int
	_ = ce.conn.QueryRow(`SELECT COUNT(*) FROM consensus_results WHERE consensus_reached = true`).Scan(&consensusCount)
	s.ConsensusRate = float64(consensusCount) / float64(s.TotalChecks)

	// Average agreement: we compute this from stored similarity matrices.
	// As a pragmatic approach, count consensus_reached as 1.0 agreement proxy.
	// For a more accurate number we'd parse every similarity_matrix JSON.
	rows, err := ce.conn.Query(`SELECT similarity_matrix FROM consensus_results`)
	if err == nil {
		defer rows.Close()
		totalAvg := 0.0
		count := 0
		for rows.Next() {
			var simJSON string
			if err := rows.Scan(&simJSON); err != nil {
				continue
			}
			var matrix map[string]float64
			if err := json.Unmarshal([]byte(simJSON), &matrix); err != nil {
				continue
			}
			if len(matrix) == 0 {
				continue
			}
			sum := 0.0
			for _, v := range matrix {
				sum += v
			}
			totalAvg += sum / float64(len(matrix))
			count++
		}
		if err := rows.Err(); err != nil {
			log.Printf("[db] rows iteration error: %v", err)
		}
		if count > 0 {
			s.AvgAgreement = totalAvg / float64(count)
		}
	}

	// Escalation rate.
	var escalatedCount int
	_ = ce.conn.QueryRow(`SELECT COUNT(*) FROM consensus_results WHERE escalated = true`).Scan(&escalatedCount)
	s.EscalationRate = float64(escalatedCount) / float64(s.TotalChecks)

	// Last 24 hours.
	cutoff := time.Now().UTC().Add(-24 * time.Hour).Format(time.RFC3339)
	_ = ce.conn.QueryRow(`SELECT COUNT(*) FROM consensus_results WHERE created_at >= ?`, cutoff).Scan(&s.Last24h)

	writeJSON(w, http.StatusOK, s)
}

// writeJSON is a helper that encodes v as JSON and writes it to w with the
// given HTTP status code.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("consensus: write json: %v", err)
	}
}

// Ensure fmt is referenced (used transitively via consensus.go but keeping
// the import explicit avoids issues if consensus_api.go is compiled alone in
// tests).
var _ = fmt.Sprintf
