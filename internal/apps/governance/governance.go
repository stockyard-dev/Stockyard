// Package governance implements platform governance, proposals, disputes, and compliance.
package governance

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"time"
)

// App implements the platform.App interface for the governance app.
type App struct {
	conn  *sql.DB
	audit func(string, string, string, string, any)
}

// New creates a new governance App backed by the given database.
func New(conn *sql.DB) *App { return &App{conn: conn} }

// SetAuditor wires the trust audit function for recording governance events.
func (a *App) SetAuditor(fn func(string, string, string, string, any)) {
	a.audit = fn
}

func (a *App) auditEvent(action, resource string, detail any) {
	if a.audit != nil {
		go a.audit("governance_event", "governance", resource, action, detail)
	}
}

func (a *App) Name() string        { return "governance" }
func (a *App) Description() string { return "Platform governance, proposals, disputes, and compliance" }

func (a *App) Migrate(conn *sql.DB) error {
	a.conn = conn
	_, err := conn.Exec(governanceSchema)
	if err != nil {
		return err
	}

	// Seed compliance rules
	a.seedComplianceRules()

	log.Printf("[governance] migrations applied")
	return nil
}

const governanceSchema = `
CREATE TABLE IF NOT EXISTS governance_proposals (
    id TEXT PRIMARY KEY,
    author_id TEXT,
    title TEXT,
    description TEXT,
    proposal_type TEXT,
    status TEXT DEFAULT 'discussion',
    discussion_ends TEXT,
    voting_ends TEXT,
    votes_for INTEGER DEFAULT 0,
    votes_against INTEGER DEFAULT 0,
    created_at TEXT
);

CREATE TABLE IF NOT EXISTS governance_votes (
    proposal_id TEXT,
    voter_id TEXT,
    vote TEXT,
    created_at TEXT,
    PRIMARY KEY(proposal_id, voter_id)
);

CREATE TABLE IF NOT EXISTS disputes (
    id TEXT PRIMARY KEY,
    type TEXT,
    reporter_id TEXT,
    subject_id TEXT,
    description TEXT,
    evidence TEXT DEFAULT '{}',
    status TEXT DEFAULT 'open',
    resolution TEXT,
    resolved_by TEXT,
    created_at TEXT,
    resolved_at TEXT
);

CREATE TABLE IF NOT EXISTS safety_certifications (
    app_id TEXT PRIMARY KEY,
    grade TEXT,
    score REAL,
    details TEXT,
    certified_at TEXT,
    expires_at TEXT
);

CREATE TABLE IF NOT EXISTS compliance_rules (
    id TEXT PRIMARY KEY,
    jurisdiction TEXT,
    regulation TEXT,
    requirement TEXT,
    stockyard_config TEXT,
    effective_date TEXT
);

CREATE TABLE IF NOT EXISTS compliance_status (
    app_id TEXT,
    rule_id TEXT,
    status TEXT,
    checked_at TEXT,
    PRIMARY KEY(app_id, rule_id)
);
`

func (a *App) seedComplianceRules() {
	rules := []struct {
		jurisdiction    string
		regulation      string
		requirement     string
		stockyardConfig string
	}{
		{
			"EU",
			"EU AI Act",
			"AI systems must provide transparency about their capabilities, limitations, and intended use",
			`{"guardrails.transparency": true, "guardrails.model_card": true, "logging.ai_decisions": true}`,
		},
		{
			"EU",
			"GDPR",
			"Personal data must be processed lawfully with explicit consent and data subjects have right to erasure",
			`{"privacy.consent_gate": true, "privacy.data_retention_days": 90, "privacy.right_to_erasure": true}`,
		},
		{
			"US",
			"HIPAA",
			"Protected health information must be encrypted at rest and in transit with access controls",
			`{"security.encryption_at_rest": true, "security.encryption_in_transit": true, "security.access_controls": true, "audit.phi_access": true}`,
		},
		{
			"US",
			"SOX",
			"Financial reporting systems must maintain audit trails and internal controls",
			`{"audit.enabled": true, "audit.retention_days": 2555, "security.change_management": true}`,
		},
		{
			"UK",
			"UK AI Framework",
			"AI systems should be safe, transparent, fair, and accountable with human oversight",
			`{"guardrails.human_oversight": true, "guardrails.fairness_check": true, "guardrails.safety_testing": true, "logging.decisions": true}`,
		},
	}

	for _, rule := range rules {
		id := genID("cr_")
		a.conn.Exec(
			"INSERT OR IGNORE INTO compliance_rules (id, jurisdiction, regulation, requirement, stockyard_config, effective_date) VALUES (?,?,?,?,?,?)",
			id, rule.jurisdiction, rule.regulation, rule.requirement, rule.stockyardConfig, "2024-01-01",
		)
	}
}

func (a *App) RegisterRoutes(mux *http.ServeMux) {
	// Proposals
	mux.HandleFunc("POST /api/governance/proposals", a.handleCreateProposal)
	mux.HandleFunc("GET /api/governance/proposals", a.handleListProposals)
	mux.HandleFunc("GET /api/governance/proposals/{id}", a.handleGetProposal)
	mux.HandleFunc("PUT /api/governance/proposals/{id}", a.handleUpdateProposal)
	mux.HandleFunc("POST /api/governance/proposals/{id}/vote", a.handleVote)

	// Disputes
	mux.HandleFunc("POST /api/disputes", a.handleFileDispute)
	mux.HandleFunc("GET /api/disputes", a.handleListDisputes)
	mux.HandleFunc("GET /api/disputes/{id}", a.handleGetDispute)
	mux.HandleFunc("PUT /api/disputes/{id}/resolve", a.handleResolveDispute)

	// Safety
	mux.HandleFunc("POST /api/safety/certify/{app_id}", a.handleCertifySafety)
	mux.HandleFunc("GET /api/safety/certifications/{app_id}", a.handleGetSafetyCert)

	// Compliance
	mux.HandleFunc("GET /api/compliance/check/{app_id}", a.handleComplianceCheck)
	mux.HandleFunc("POST /api/compliance/auto-fix/{app_id}", a.handleAutoFix)

	log.Printf("[governance] routes registered")
}

// --- Helpers ---

func genID(prefix string) string {
	b := make([]byte, 6)
	rand.Read(b)
	return prefix + hex.EncodeToString(b)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func now() string {
	return time.Now().Format(time.RFC3339)
}

// --- Proposals ---

func (a *App) handleCreateProposal(w http.ResponseWriter, r *http.Request) {
	var req struct {
		AuthorID       string `json:"author_id"`
		Title          string `json:"title"`
		Description    string `json:"description"`
		ProposalType   string `json:"proposal_type"`
		DiscussionEnds string `json:"discussion_ends"`
		VotingEnds     string `json:"voting_ends"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	id := genID("gp_")
	ts := now()
	_, err := a.conn.Exec(
		"INSERT INTO governance_proposals (id, author_id, title, description, proposal_type, discussion_ends, voting_ends, created_at) VALUES (?,?,?,?,?,?,?,?)",
		id, req.AuthorID, req.Title, req.Description, req.ProposalType, req.DiscussionEnds, req.VotingEnds, ts,
	)
	if err != nil {
		w.WriteHeader(500)
		log.Printf("[governance] error: %v", err)
		writeJSON(w, map[string]string{"error": "internal error"})
		return
	}
	a.auditEvent("proposal_created", id, map[string]any{"author_id": req.AuthorID, "title": req.Title})
	writeJSON(w, map[string]any{"status": "created", "id": id})
}

func (a *App) handleListProposals(w http.ResponseWriter, r *http.Request) {
	rows, err := a.conn.Query(
		"SELECT id, author_id, title, description, proposal_type, status, discussion_ends, voting_ends, votes_for, votes_against, created_at FROM governance_proposals ORDER BY created_at DESC",
	)
	if err != nil || rows == nil {
		writeJSON(w, map[string]any{"proposals": []any{}})
		return
	}
	defer rows.Close()
	var proposals []map[string]any
	for rows.Next() {
		var id, authorID, title, desc, ptype, status, discussionEnds, votingEnds, created string
		var votesFor, votesAgainst int
		rows.Scan(&id, &authorID, &title, &desc, &ptype, &status, &discussionEnds, &votingEnds, &votesFor, &votesAgainst, &created)
		proposals = append(proposals, map[string]any{
			"id": id, "author_id": authorID, "title": title, "description": desc,
			"proposal_type": ptype, "status": status, "discussion_ends": discussionEnds,
			"voting_ends": votingEnds, "votes_for": votesFor, "votes_against": votesAgainst,
			"created_at": created,
		})
	}
	writeJSON(w, map[string]any{"proposals": proposals, "count": len(proposals)})
}

func (a *App) handleGetProposal(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var authorID, title, desc, ptype, status, discussionEnds, votingEnds, created string
	var votesFor, votesAgainst int
	err := a.conn.QueryRow(
		"SELECT author_id, title, description, proposal_type, status, discussion_ends, voting_ends, votes_for, votes_against, created_at FROM governance_proposals WHERE id = ?", id,
	).Scan(&authorID, &title, &desc, &ptype, &status, &discussionEnds, &votingEnds, &votesFor, &votesAgainst, &created)
	if err != nil {
		w.WriteHeader(404)
		writeJSON(w, map[string]string{"error": "proposal not found"})
		return
	}

	// Get votes
	voteRows, _ := a.conn.Query("SELECT voter_id, vote, created_at FROM governance_votes WHERE proposal_id = ?", id)
	var votes []map[string]any
	if voteRows != nil {
		defer voteRows.Close()
		for voteRows.Next() {
			var voterID, vote, voteCreated string
			voteRows.Scan(&voterID, &vote, &voteCreated)
			votes = append(votes, map[string]any{
				"voter_id": voterID, "vote": vote, "created_at": voteCreated,
			})
		}
	}

	writeJSON(w, map[string]any{
		"id": id, "author_id": authorID, "title": title, "description": desc,
		"proposal_type": ptype, "status": status, "discussion_ends": discussionEnds,
		"voting_ends": votingEnds, "votes_for": votesFor, "votes_against": votesAgainst,
		"created_at": created, "votes": votes,
	})
}

func (a *App) handleUpdateProposal(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req struct {
		Title          *string `json:"title"`
		Description    *string `json:"description"`
		Status         *string `json:"status"`
		DiscussionEnds *string `json:"discussion_ends"`
		VotingEnds     *string `json:"voting_ends"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	var existing string
	if err := a.conn.QueryRow("SELECT id FROM governance_proposals WHERE id = ?", id).Scan(&existing); err != nil {
		w.WriteHeader(404)
		writeJSON(w, map[string]string{"error": "proposal not found"})
		return
	}

	if req.Title != nil {
		a.conn.Exec("UPDATE governance_proposals SET title = ? WHERE id = ?", *req.Title, id)
	}
	if req.Description != nil {
		a.conn.Exec("UPDATE governance_proposals SET description = ? WHERE id = ?", *req.Description, id)
	}
	if req.Status != nil {
		a.conn.Exec("UPDATE governance_proposals SET status = ? WHERE id = ?", *req.Status, id)
	}
	if req.DiscussionEnds != nil {
		a.conn.Exec("UPDATE governance_proposals SET discussion_ends = ? WHERE id = ?", *req.DiscussionEnds, id)
	}
	if req.VotingEnds != nil {
		a.conn.Exec("UPDATE governance_proposals SET voting_ends = ? WHERE id = ?", *req.VotingEnds, id)
	}

	a.auditEvent("proposal_updated", id, nil)
	writeJSON(w, map[string]any{"status": "updated", "id": id})
}

func (a *App) handleVote(w http.ResponseWriter, r *http.Request) {
	proposalID := r.PathValue("id")
	var req struct {
		VoterID string `json:"voter_id"`
		Vote    string `json:"vote"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	if req.Vote != "for" && req.Vote != "against" {
		w.WriteHeader(400)
		writeJSON(w, map[string]string{"error": "vote must be 'for' or 'against'"})
		return
	}

	// Check proposal exists
	var existing string
	if err := a.conn.QueryRow("SELECT id FROM governance_proposals WHERE id = ?", proposalID).Scan(&existing); err != nil {
		w.WriteHeader(404)
		writeJSON(w, map[string]string{"error": "proposal not found"})
		return
	}

	ts := now()
	_, err := a.conn.Exec(
		"INSERT INTO governance_votes (proposal_id, voter_id, vote, created_at) VALUES (?,?,?,?)",
		proposalID, req.VoterID, req.Vote, ts,
	)
	if err != nil {
		w.WriteHeader(409)
		writeJSON(w, map[string]string{"error": "already voted"})
		return
	}

	// Update vote counts
	if req.Vote == "for" {
		a.conn.Exec("UPDATE governance_proposals SET votes_for = votes_for + 1 WHERE id = ?", proposalID)
	} else {
		a.conn.Exec("UPDATE governance_proposals SET votes_against = votes_against + 1 WHERE id = ?", proposalID)
	}

	// Check if proposal should be auto-resolved
	var votesFor, votesAgainst int
	var votingEnds, status string
	a.conn.QueryRow("SELECT votes_for, votes_against, COALESCE(voting_ends,''), status FROM governance_proposals WHERE id = ?",
		proposalID).Scan(&votesFor, &votesAgainst, &votingEnds, &status)

	autoResolved := ""
	if status == "discussion" || status == "voting" {
		// Move to voting if still in discussion
		if status == "discussion" {
			a.conn.Exec("UPDATE governance_proposals SET status = 'voting' WHERE id = ?", proposalID)
		}

		// Auto-pass: 3+ votes for with majority, or voting deadline passed with majority
		totalVotes := votesFor + votesAgainst
		passed := votesFor > votesAgainst && totalVotes >= 3
		if !passed && votingEnds != "" {
			deadline, err := time.Parse(time.RFC3339, votingEnds)
			if err == nil && time.Now().After(deadline) && votesFor > votesAgainst {
				passed = true
			}
		}

		if passed {
			a.conn.Exec("UPDATE governance_proposals SET status = 'passed' WHERE id = ?", proposalID)
			autoResolved = "passed"
			log.Printf("[governance] proposal %s passed (%d for, %d against)", proposalID, votesFor, votesAgainst)
		} else if totalVotes >= 3 && votesAgainst > votesFor {
			a.conn.Exec("UPDATE governance_proposals SET status = 'rejected' WHERE id = ?", proposalID)
			autoResolved = "rejected"
		}
	}

	a.auditEvent("vote_cast", proposalID, map[string]any{"voter_id": req.VoterID, "vote": req.Vote})
	resp := map[string]any{"status": "voted", "proposal_id": proposalID, "vote": req.Vote,
		"votes_for": votesFor, "votes_against": votesAgainst}
	if autoResolved != "" {
		resp["resolution"] = autoResolved
	}
	writeJSON(w, resp)
}

// --- Disputes ---

func (a *App) handleFileDispute(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Type        string `json:"type"`
		ReporterID  string `json:"reporter_id"`
		SubjectID   string `json:"subject_id"`
		Description string `json:"description"`
		Evidence    any    `json:"evidence"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	id := genID("dp_")
	ts := now()
	evidence, _ := json.Marshal(req.Evidence)
	if req.Evidence == nil {
		evidence = []byte("{}")
	}

	_, err := a.conn.Exec(
		"INSERT INTO disputes (id, type, reporter_id, subject_id, description, evidence, created_at) VALUES (?,?,?,?,?,?,?)",
		id, req.Type, req.ReporterID, req.SubjectID, req.Description, string(evidence), ts,
	)
	if err != nil {
		w.WriteHeader(500)
		log.Printf("[governance] error: %v", err)
		writeJSON(w, map[string]string{"error": "internal error"})
		return
	}
	a.auditEvent("dispute_filed", id, map[string]any{"type": req.Type, "reporter_id": req.ReporterID})
	writeJSON(w, map[string]any{"status": "filed", "id": id})
}

func (a *App) handleListDisputes(w http.ResponseWriter, r *http.Request) {
	rows, err := a.conn.Query(
		"SELECT id, type, reporter_id, subject_id, description, status, created_at FROM disputes ORDER BY created_at DESC",
	)
	if err != nil || rows == nil {
		writeJSON(w, map[string]any{"disputes": []any{}})
		return
	}
	defer rows.Close()
	var disputes []map[string]any
	for rows.Next() {
		var id, dtype, reporterID, subjectID, desc, status, created string
		rows.Scan(&id, &dtype, &reporterID, &subjectID, &desc, &status, &created)
		disputes = append(disputes, map[string]any{
			"id": id, "type": dtype, "reporter_id": reporterID, "subject_id": subjectID,
			"description": desc, "status": status, "created_at": created,
		})
	}
	writeJSON(w, map[string]any{"disputes": disputes, "count": len(disputes)})
}

func (a *App) handleGetDispute(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var dtype, reporterID, subjectID, desc, evidenceJSON, status, resolution, resolvedBy, created, resolvedAt string
	err := a.conn.QueryRow(
		"SELECT type, reporter_id, subject_id, description, evidence, status, resolution, resolved_by, created_at, resolved_at FROM disputes WHERE id = ?", id,
	).Scan(&dtype, &reporterID, &subjectID, &desc, &evidenceJSON, &status, &resolution, &resolvedBy, &created, &resolvedAt)
	if err != nil {
		w.WriteHeader(404)
		writeJSON(w, map[string]string{"error": "dispute not found"})
		return
	}
	var evidence any
	json.Unmarshal([]byte(evidenceJSON), &evidence)
	writeJSON(w, map[string]any{
		"id": id, "type": dtype, "reporter_id": reporterID, "subject_id": subjectID,
		"description": desc, "evidence": evidence, "status": status,
		"resolution": resolution, "resolved_by": resolvedBy,
		"created_at": created, "resolved_at": resolvedAt,
	})
}

func (a *App) handleResolveDispute(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req struct {
		Resolution string `json:"resolution"`
		ResolvedBy string `json:"resolved_by"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	var existing string
	if err := a.conn.QueryRow("SELECT id FROM disputes WHERE id = ?", id).Scan(&existing); err != nil {
		w.WriteHeader(404)
		writeJSON(w, map[string]string{"error": "dispute not found"})
		return
	}

	ts := now()
	a.conn.Exec(
		"UPDATE disputes SET status = 'resolved', resolution = ?, resolved_by = ?, resolved_at = ? WHERE id = ?",
		req.Resolution, req.ResolvedBy, ts, id,
	)
	a.auditEvent("dispute_resolved", id, map[string]any{"resolved_by": req.ResolvedBy})
	writeJSON(w, map[string]any{"status": "resolved", "id": id, "resolved_at": ts})
}

// --- Safety Certifications ---

func (a *App) handleCertifySafety(w http.ResponseWriter, r *http.Request) {
	appID := r.PathValue("app_id")
	var req struct {
		Guardrails []string `json:"guardrails"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	// Simple score based on presence of guardrails
	score := 0.0
	guardrailWeights := map[string]float64{
		"input_validation":    15,
		"output_filtering":    15,
		"rate_limiting":       10,
		"content_moderation":  15,
		"pii_detection":       15,
		"bias_detection":      10,
		"hallucination_check": 10,
		"human_oversight":     10,
	}

	details := make(map[string]any)
	for _, g := range req.Guardrails {
		if weight, ok := guardrailWeights[g]; ok {
			score += weight
			details[g] = true
		}
	}

	grade := "F"
	switch {
	case score >= 90:
		grade = "A"
	case score >= 80:
		grade = "B"
	case score >= 70:
		grade = "C"
	case score >= 60:
		grade = "D"
	}

	detailsJSON, _ := json.Marshal(details)
	ts := now()
	expiresAt := time.Now().Add(90 * 24 * time.Hour).Format(time.RFC3339)

	a.conn.Exec(
		"INSERT OR REPLACE INTO safety_certifications (app_id, grade, score, details, certified_at, expires_at) VALUES (?,?,?,?,?,?)",
		appID, grade, score, string(detailsJSON), ts, expiresAt,
	)

	a.auditEvent("safety_certified", appID, map[string]any{"grade": grade, "score": score})
	writeJSON(w, map[string]any{
		"app_id": appID, "grade": grade, "score": score,
		"details": details, "certified_at": ts, "expires_at": expiresAt,
	})
}

func (a *App) handleGetSafetyCert(w http.ResponseWriter, r *http.Request) {
	appID := r.PathValue("app_id")
	var grade, detailsJSON, certifiedAt, expiresAt string
	var score float64
	err := a.conn.QueryRow(
		"SELECT grade, score, details, certified_at, expires_at FROM safety_certifications WHERE app_id = ?", appID,
	).Scan(&grade, &score, &detailsJSON, &certifiedAt, &expiresAt)
	if err != nil {
		w.WriteHeader(404)
		writeJSON(w, map[string]string{"error": "no safety certification found"})
		return
	}
	var details any
	json.Unmarshal([]byte(detailsJSON), &details)
	writeJSON(w, map[string]any{
		"app_id": appID, "grade": grade, "score": score,
		"details": details, "certified_at": certifiedAt, "expires_at": expiresAt,
	})
}

// --- Compliance ---

func (a *App) handleComplianceCheck(w http.ResponseWriter, r *http.Request) {
	appID := r.PathValue("app_id")

	// Get all rules
	rows, err := a.conn.Query("SELECT id, jurisdiction, regulation, requirement, stockyard_config FROM compliance_rules")
	if err != nil || rows == nil {
		writeJSON(w, map[string]any{"app_id": appID, "results": []any{}})
		return
	}
	defer rows.Close()

	ts := now()
	var results []map[string]any
	for rows.Next() {
		var ruleID, jurisdiction, regulation, requirement, config string
		rows.Scan(&ruleID, &jurisdiction, &regulation, &requirement, &config)

		// Check if there's an existing status
		var existingStatus string
		a.conn.QueryRow("SELECT status FROM compliance_status WHERE app_id = ? AND rule_id = ?", appID, ruleID).Scan(&existingStatus)
		if existingStatus == "" {
			existingStatus = "unchecked"
		}

		// Record the check
		a.conn.Exec(
			"INSERT OR REPLACE INTO compliance_status (app_id, rule_id, status, checked_at) VALUES (?,?,?,?)",
			appID, ruleID, existingStatus, ts,
		)

		results = append(results, map[string]any{
			"rule_id":      ruleID,
			"jurisdiction": jurisdiction,
			"regulation":   regulation,
			"requirement":  requirement,
			"config":       config,
			"status":       existingStatus,
			"checked_at":   ts,
		})
	}

	writeJSON(w, map[string]any{"app_id": appID, "results": results, "count": len(results), "checked_at": ts})
}

func (a *App) handleAutoFix(w http.ResponseWriter, r *http.Request) {
	appID := r.PathValue("app_id")

	// Get all rules and generate fix suggestions
	rows, err := a.conn.Query("SELECT id, regulation, requirement, stockyard_config FROM compliance_rules")
	if err != nil || rows == nil {
		writeJSON(w, map[string]any{"app_id": appID, "suggestions": []any{}})
		return
	}
	defer rows.Close()

	var suggestions []map[string]any
	for rows.Next() {
		var ruleID, regulation, requirement, configJSON string
		rows.Scan(&ruleID, &regulation, &requirement, &configJSON)

		var config any
		json.Unmarshal([]byte(configJSON), &config)

		suggestions = append(suggestions, map[string]any{
			"rule_id":          ruleID,
			"regulation":       regulation,
			"requirement":      requirement,
			"suggested_config": config,
			"auto_fixable":     true,
			"fix_description":  "Apply the suggested Stockyard configuration to meet " + regulation + " requirements",
		})
	}

	writeJSON(w, map[string]any{
		"app_id":      appID,
		"suggestions": suggestions,
		"count":       len(suggestions),
		"note":        "Review suggestions before applying. Auto-fix sets recommended configuration values.",
	})
}
