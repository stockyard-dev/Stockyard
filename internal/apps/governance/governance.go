// Package governance implements platform governance, safety, and compliance.
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

type App struct {
	conn  *sql.DB
	audit func(string, string, string, string, any)
}

func New(conn *sql.DB) *App { return &App{conn: conn} }

func (a *App) Name() string        { return "governance" }
func (a *App) Description() string { return "Platform governance, safety, and compliance" }

func (a *App) SetAuditor(fn func(string, string, string, string, any)) { a.audit = fn }

func (a *App) Migrate(conn *sql.DB) error {
	a.conn = conn
	_, err := conn.Exec(governanceSchema)
	if err != nil {
		return err
	}
	// Seed compliance rules
	for _, rule := range seedRules {
		conn.Exec(
			"INSERT OR IGNORE INTO compliance_rules (id, jurisdiction, regulation, requirement, stockyard_config, effective_date) VALUES (?,?,?,?,?,?)",
			rule.id, rule.jurisdiction, rule.regulation, rule.requirement, rule.stockyardConfig, rule.effectiveDate,
		)
	}
	log.Printf("[governance] migrations applied")
	return nil
}

const governanceSchema = `
CREATE TABLE IF NOT EXISTS governance_council (
    id TEXT PRIMARY KEY,
    user_id TEXT,
    role_class TEXT,
    term_start TEXT,
    term_end TEXT,
    status TEXT DEFAULT 'active'
);

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

CREATE INDEX IF NOT EXISTS idx_governance_proposals_status ON governance_proposals(status);
CREATE INDEX IF NOT EXISTS idx_disputes_status ON disputes(status);
CREATE INDEX IF NOT EXISTS idx_compliance_rules_jurisdiction ON compliance_rules(jurisdiction);
`

type seedRule struct {
	id              string
	jurisdiction    string
	regulation      string
	requirement     string
	stockyardConfig string
	effectiveDate   string
}

var seedRules = []seedRule{
	{
		id: "eu_ai_act_risk", jurisdiction: "EU", regulation: "EU AI Act",
		requirement:     "High-risk AI systems must include risk assessment and mitigation",
		stockyardConfig: `{"modules":["trust.risk_assessment","observe.monitoring"]}`,
		effectiveDate:   "2025-08-01",
	},
	{
		id: "eu_ai_act_transparency", jurisdiction: "EU", regulation: "EU AI Act",
		requirement:     "AI systems interacting with humans must disclose they are AI",
		stockyardConfig: `{"modules":["trust.transparency","proxy.disclosure_headers"]}`,
		effectiveDate:   "2025-08-01",
	},
	{
		id: "gdpr_data_processing", jurisdiction: "EU", regulation: "GDPR",
		requirement:     "Personal data must be processed lawfully with explicit consent",
		stockyardConfig: `{"modules":["trust.consent_management","memory.data_retention"]}`,
		effectiveDate:   "2018-05-25",
	},
	{
		id: "gdpr_right_to_erasure", jurisdiction: "EU", regulation: "GDPR",
		requirement:     "Users have the right to request deletion of their personal data",
		stockyardConfig: `{"modules":["memory.erasure","trust.data_subject_requests"]}`,
		effectiveDate:   "2018-05-25",
	},
	{
		id: "hipaa_phi_protection", jurisdiction: "US", regulation: "HIPAA",
		requirement:     "Protected health information must be encrypted at rest and in transit",
		stockyardConfig: `{"modules":["trust.encryption","proxy.tls_enforcement"]}`,
		effectiveDate:   "2013-09-23",
	},
	{
		id: "hipaa_audit_trail", jurisdiction: "US", regulation: "HIPAA",
		requirement:     "Access to PHI must be logged with immutable audit trails",
		stockyardConfig: `{"modules":["observe.audit_log","trust.access_controls"]}`,
		effectiveDate:   "2013-09-23",
	},
	{
		id: "sox_financial_controls", jurisdiction: "US", regulation: "SOX",
		requirement:     "Financial reporting systems must maintain internal controls and audit trails",
		stockyardConfig: `{"modules":["observe.audit_log","trust.access_controls","billing.audit"]}`,
		effectiveDate:   "2002-07-30",
	},
	{
		id: "uk_ai_framework_principles", jurisdiction: "UK", regulation: "UK AI Regulation Framework",
		requirement:     "AI systems must adhere to safety, transparency, fairness, accountability, and contestability principles",
		stockyardConfig: `{"modules":["trust.fairness","trust.transparency","observe.monitoring","trust.contestability"]}`,
		effectiveDate:   "2024-02-06",
	},
}

func (a *App) RegisterRoutes(mux *http.ServeMux) {
	// Council
	mux.HandleFunc("GET /api/governance/council", a.handleListCouncil)

	// Proposals
	mux.HandleFunc("POST /api/governance/proposals", a.handleCreateProposal)
	mux.HandleFunc("GET /api/governance/proposals", a.handleListProposals)
	mux.HandleFunc("GET /api/governance/proposals/{id}", a.handleGetProposal)
	mux.HandleFunc("POST /api/governance/proposals/{id}/vote", a.handleVote)

	// Disputes
	mux.HandleFunc("POST /api/disputes", a.handleCreateDispute)
	mux.HandleFunc("GET /api/disputes", a.handleListDisputes)
	mux.HandleFunc("PUT /api/disputes/{id}/resolve", a.handleResolveDispute)

	// Safety
	mux.HandleFunc("POST /api/safety/certify/{app_id}", a.handleCertifyApp)
	mux.HandleFunc("GET /api/safety/certify/{app_id}", a.handleGetCert)

	// Compliance
	mux.HandleFunc("GET /api/compliance/check/{app_id}", a.handleComplianceCheck)
	mux.HandleFunc("POST /api/compliance/auto-fix/{app_id}", a.handleAutoFix)
	mux.HandleFunc("GET /api/compliance/rules", a.handleListRules)

	log.Printf("[governance] routes registered")
}

func genID(prefix string) string {
	b := make([]byte, 6)
	rand.Read(b)
	return prefix + hex.EncodeToString(b)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

// --- Council ---

func (a *App) handleListCouncil(w http.ResponseWriter, r *http.Request) {
	rows, err := a.conn.Query(
		"SELECT id, user_id, role_class, term_start, term_end, status FROM governance_council WHERE status = 'active'",
	)
	if err != nil {
		writeJSON(w, map[string]any{"members": []any{}})
		return
	}
	defer rows.Close()

	var members []map[string]any
	for rows.Next() {
		var id, userID, roleClass, termStart, termEnd, status string
		rows.Scan(&id, &userID, &roleClass, &termStart, &termEnd, &status)
		members = append(members, map[string]any{
			"id": id, "user_id": userID, "role_class": roleClass,
			"term_start": termStart, "term_end": termEnd, "status": status,
		})
	}
	if members == nil {
		members = []map[string]any{}
	}
	writeJSON(w, map[string]any{"members": members, "count": len(members)})
}

// --- Proposals ---

func (a *App) handleCreateProposal(w http.ResponseWriter, r *http.Request) {
	var req struct {
		AuthorID     string `json:"author_id"`
		Title        string `json:"title"`
		Description  string `json:"description"`
		ProposalType string `json:"proposal_type"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	if req.AuthorID == "" || req.Title == "" {
		w.WriteHeader(400)
		writeJSON(w, map[string]string{"error": "author_id and title required"})
		return
	}

	id := genID("prop_")
	now := time.Now().UTC().Format(time.RFC3339)
	discussionEnds := time.Now().UTC().AddDate(0, 0, 7).Format(time.RFC3339)
	votingEnds := time.Now().UTC().AddDate(0, 0, 14).Format(time.RFC3339)

	_, err := a.conn.Exec(
		"INSERT INTO governance_proposals (id, author_id, title, description, proposal_type, discussion_ends, voting_ends, created_at) VALUES (?,?,?,?,?,?,?,?)",
		id, req.AuthorID, req.Title, req.Description, req.ProposalType, discussionEnds, votingEnds, now,
	)
	if err != nil {
		w.WriteHeader(500)
		writeJSON(w, map[string]string{"error": "failed to create proposal"})
		return
	}

	if a.audit != nil {
		a.audit("governance", "proposal.created", req.AuthorID, id, req)
	}

	writeJSON(w, map[string]any{
		"id": id, "author_id": req.AuthorID, "title": req.Title,
		"status": "discussion", "discussion_ends": discussionEnds, "voting_ends": votingEnds,
	})
}

func (a *App) handleListProposals(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	query := "SELECT id, author_id, title, description, proposal_type, status, discussion_ends, voting_ends, votes_for, votes_against, created_at FROM governance_proposals"
	var args []any
	if status != "" {
		query += " WHERE status = ?"
		args = append(args, status)
	}
	query += " ORDER BY created_at DESC"

	rows, err := a.conn.Query(query, args...)
	if err != nil {
		writeJSON(w, map[string]any{"proposals": []any{}})
		return
	}
	defer rows.Close()

	var proposals []map[string]any
	for rows.Next() {
		var id, authorID, title, description, pType, st, discEnds, voteEnds, createdAt string
		var votesFor, votesAgainst int
		rows.Scan(&id, &authorID, &title, &description, &pType, &st, &discEnds, &voteEnds, &votesFor, &votesAgainst, &createdAt)
		proposals = append(proposals, map[string]any{
			"id": id, "author_id": authorID, "title": title, "description": description,
			"proposal_type": pType, "status": st, "discussion_ends": discEnds,
			"voting_ends": voteEnds, "votes_for": votesFor, "votes_against": votesAgainst,
			"created_at": createdAt,
		})
	}
	if proposals == nil {
		proposals = []map[string]any{}
	}
	writeJSON(w, map[string]any{"proposals": proposals, "count": len(proposals)})
}

func (a *App) handleGetProposal(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var authorID, title, description, pType, status, discEnds, voteEnds, createdAt string
	var votesFor, votesAgainst int
	err := a.conn.QueryRow(
		"SELECT author_id, title, description, proposal_type, status, discussion_ends, voting_ends, votes_for, votes_against, created_at FROM governance_proposals WHERE id = ?", id,
	).Scan(&authorID, &title, &description, &pType, &status, &discEnds, &voteEnds, &votesFor, &votesAgainst, &createdAt)
	if err != nil {
		w.WriteHeader(404)
		writeJSON(w, map[string]string{"error": "proposal not found"})
		return
	}

	// Get votes
	rows, _ := a.conn.Query("SELECT voter_id, vote, created_at FROM governance_votes WHERE proposal_id = ?", id)
	var votes []map[string]any
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var voterID, vote, voteAt string
			rows.Scan(&voterID, &vote, &voteAt)
			votes = append(votes, map[string]any{
				"voter_id": voterID, "vote": vote, "created_at": voteAt,
			})
		}
	}
	if votes == nil {
		votes = []map[string]any{}
	}

	writeJSON(w, map[string]any{
		"id": id, "author_id": authorID, "title": title, "description": description,
		"proposal_type": pType, "status": status, "discussion_ends": discEnds,
		"voting_ends": voteEnds, "votes_for": votesFor, "votes_against": votesAgainst,
		"created_at": createdAt, "votes": votes,
	})
}

func (a *App) handleVote(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req struct {
		VoterID string `json:"voter_id"`
		Vote    string `json:"vote"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	if req.VoterID == "" || (req.Vote != "for" && req.Vote != "against") {
		w.WriteHeader(400)
		writeJSON(w, map[string]string{"error": "voter_id and vote (for|against) required"})
		return
	}

	// Check proposal exists
	var status string
	err := a.conn.QueryRow("SELECT status FROM governance_proposals WHERE id = ?", id).Scan(&status)
	if err != nil {
		w.WriteHeader(404)
		writeJSON(w, map[string]string{"error": "proposal not found"})
		return
	}

	now := time.Now().UTC().Format(time.RFC3339)
	_, err = a.conn.Exec(
		"INSERT INTO governance_votes (proposal_id, voter_id, vote, created_at) VALUES (?,?,?,?)",
		id, req.VoterID, req.Vote, now,
	)
	if err != nil {
		w.WriteHeader(409)
		writeJSON(w, map[string]string{"error": "already voted"})
		return
	}

	// Update tally
	col := "votes_for"
	if req.Vote == "against" {
		col = "votes_against"
	}
	a.conn.Exec("UPDATE governance_proposals SET "+col+" = "+col+" + 1 WHERE id = ?", id)

	if a.audit != nil {
		a.audit("governance", "proposal.voted", req.VoterID, id, req)
	}

	writeJSON(w, map[string]any{"proposal_id": id, "voter_id": req.VoterID, "vote": req.Vote, "status": "recorded"})
}

// --- Disputes ---

func (a *App) handleCreateDispute(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Type        string `json:"type"`
		ReporterID  string `json:"reporter_id"`
		SubjectID   string `json:"subject_id"`
		Description string `json:"description"`
		Evidence    any    `json:"evidence"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	if req.ReporterID == "" || req.Description == "" {
		w.WriteHeader(400)
		writeJSON(w, map[string]string{"error": "reporter_id and description required"})
		return
	}

	id := genID("disp_")
	now := time.Now().UTC().Format(time.RFC3339)
	evidence := "{}"
	if req.Evidence != nil {
		if b, err := json.Marshal(req.Evidence); err == nil {
			evidence = string(b)
		}
	}

	_, err := a.conn.Exec(
		"INSERT INTO disputes (id, type, reporter_id, subject_id, description, evidence, created_at) VALUES (?,?,?,?,?,?,?)",
		id, req.Type, req.ReporterID, req.SubjectID, req.Description, evidence, now,
	)
	if err != nil {
		w.WriteHeader(500)
		writeJSON(w, map[string]string{"error": "failed to create dispute"})
		return
	}

	if a.audit != nil {
		a.audit("governance", "dispute.created", req.ReporterID, id, req)
	}

	writeJSON(w, map[string]any{"id": id, "status": "open", "created_at": now})
}

func (a *App) handleListDisputes(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	query := "SELECT id, type, reporter_id, subject_id, description, evidence, status, resolution, resolved_by, created_at, resolved_at FROM disputes"
	var args []any
	if status != "" {
		query += " WHERE status = ?"
		args = append(args, status)
	}
	query += " ORDER BY created_at DESC"

	rows, err := a.conn.Query(query, args...)
	if err != nil {
		writeJSON(w, map[string]any{"disputes": []any{}})
		return
	}
	defer rows.Close()

	var disputes []map[string]any
	for rows.Next() {
		var id, dtype, reporterID, subjectID, description, evidence, st, createdAt string
		var resolution, resolvedBy, resolvedAt sql.NullString
		rows.Scan(&id, &dtype, &reporterID, &subjectID, &description, &evidence, &st, &resolution, &resolvedBy, &createdAt, &resolvedAt)
		var ev any
		json.Unmarshal([]byte(evidence), &ev)
		disputes = append(disputes, map[string]any{
			"id": id, "type": dtype, "reporter_id": reporterID, "subject_id": subjectID,
			"description": description, "evidence": ev, "status": st,
			"resolution": resolution.String, "resolved_by": resolvedBy.String,
			"created_at": createdAt, "resolved_at": resolvedAt.String,
		})
	}
	if disputes == nil {
		disputes = []map[string]any{}
	}
	writeJSON(w, map[string]any{"disputes": disputes, "count": len(disputes)})
}

func (a *App) handleResolveDispute(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req struct {
		Resolution string `json:"resolution"`
		ResolvedBy string `json:"resolved_by"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	if req.Resolution == "" || req.ResolvedBy == "" {
		w.WriteHeader(400)
		writeJSON(w, map[string]string{"error": "resolution and resolved_by required"})
		return
	}

	now := time.Now().UTC().Format(time.RFC3339)
	result, _ := a.conn.Exec(
		"UPDATE disputes SET status = 'resolved', resolution = ?, resolved_by = ?, resolved_at = ? WHERE id = ? AND status = 'open'",
		req.Resolution, req.ResolvedBy, now, id,
	)
	n, _ := result.RowsAffected()
	if n == 0 {
		w.WriteHeader(404)
		writeJSON(w, map[string]string{"error": "dispute not found or already resolved"})
		return
	}

	if a.audit != nil {
		a.audit("governance", "dispute.resolved", req.ResolvedBy, id, req)
	}

	writeJSON(w, map[string]any{"id": id, "status": "resolved", "resolved_at": now})
}

// --- Safety Certification ---

func (a *App) handleCertifyApp(w http.ResponseWriter, r *http.Request) {
	appID := r.PathValue("app_id")

	// Simple heuristic scoring
	score := 0.0
	details := map[string]any{}

	// Check if app has compliance rules checked
	var complianceCount int
	a.conn.QueryRow("SELECT COUNT(*) FROM compliance_status WHERE app_id = ? AND status = 'compliant'", appID).Scan(&complianceCount)
	hasCompliance := complianceCount > 0
	details["has_compliance"] = hasCompliance
	if hasCompliance {
		score += 30
	}

	// Check if app has guardrails (trust module usage proxy)
	var guardrailCount int
	a.conn.QueryRow("SELECT COUNT(*) FROM compliance_status WHERE app_id = ? AND rule_id LIKE '%transparency%'", appID).Scan(&guardrailCount)
	hasGuardrails := guardrailCount > 0
	details["has_guardrails"] = hasGuardrails
	if hasGuardrails {
		score += 25
	}

	// Check for audit trail
	var auditCount int
	a.conn.QueryRow("SELECT COUNT(*) FROM compliance_status WHERE app_id = ? AND rule_id LIKE '%audit%'", appID).Scan(&auditCount)
	hasAudit := auditCount > 0
	details["has_audit_trail"] = hasAudit
	if hasAudit {
		score += 25
	}

	// Check for data protection
	var dataProtCount int
	a.conn.QueryRow("SELECT COUNT(*) FROM compliance_status WHERE app_id = ? AND rule_id LIKE '%gdpr%'", appID).Scan(&dataProtCount)
	hasDataProt := dataProtCount > 0
	details["has_data_protection"] = hasDataProt
	if hasDataProt {
		score += 20
	}

	// Grade based on score
	grade := "F"
	switch {
	case score >= 90:
		grade = "A"
	case score >= 75:
		grade = "B"
	case score >= 60:
		grade = "C"
	case score >= 40:
		grade = "D"
	}

	now := time.Now().UTC().Format(time.RFC3339)
	expiresAt := time.Now().UTC().AddDate(0, 6, 0).Format(time.RFC3339)
	detailsJSON, _ := json.Marshal(details)

	a.conn.Exec(
		"INSERT INTO safety_certifications (app_id, grade, score, details, certified_at, expires_at) VALUES (?,?,?,?,?,?) ON CONFLICT(app_id) DO UPDATE SET grade=?, score=?, details=?, certified_at=?, expires_at=?",
		appID, grade, score, string(detailsJSON), now, expiresAt, grade, score, string(detailsJSON), now, expiresAt,
	)

	if a.audit != nil {
		a.audit("governance", "safety.certified", "", appID, map[string]any{"grade": grade, "score": score})
	}

	writeJSON(w, map[string]any{
		"app_id": appID, "grade": grade, "score": score,
		"details": details, "certified_at": now, "expires_at": expiresAt,
	})
}

func (a *App) handleGetCert(w http.ResponseWriter, r *http.Request) {
	appID := r.PathValue("app_id")
	var grade, detailsStr, certifiedAt, expiresAt string
	var score float64
	err := a.conn.QueryRow(
		"SELECT grade, score, details, certified_at, expires_at FROM safety_certifications WHERE app_id = ?", appID,
	).Scan(&grade, &score, &detailsStr, &certifiedAt, &expiresAt)
	if err != nil {
		w.WriteHeader(404)
		writeJSON(w, map[string]string{"error": "certification not found"})
		return
	}
	var details any
	json.Unmarshal([]byte(detailsStr), &details)
	writeJSON(w, map[string]any{
		"app_id": appID, "grade": grade, "score": score,
		"details": details, "certified_at": certifiedAt, "expires_at": expiresAt,
	})
}

// --- Compliance ---

func (a *App) handleComplianceCheck(w http.ResponseWriter, r *http.Request) {
	appID := r.PathValue("app_id")
	now := time.Now().UTC().Format(time.RFC3339)

	rows, err := a.conn.Query("SELECT id, jurisdiction, regulation, requirement, stockyard_config FROM compliance_rules")
	if err != nil {
		writeJSON(w, map[string]any{"app_id": appID, "results": []any{}})
		return
	}
	defer rows.Close()

	var results []map[string]any
	for rows.Next() {
		var ruleID, jurisdiction, regulation, requirement, config string
		rows.Scan(&ruleID, &jurisdiction, &regulation, &requirement, &config)

		// Check if already compliant
		var existingStatus string
		err := a.conn.QueryRow("SELECT status FROM compliance_status WHERE app_id = ? AND rule_id = ?", appID, ruleID).Scan(&existingStatus)
		status := "non_compliant"
		if err == nil {
			status = existingStatus
		}

		// Upsert status
		a.conn.Exec(
			"INSERT INTO compliance_status (app_id, rule_id, status, checked_at) VALUES (?,?,?,?) ON CONFLICT(app_id, rule_id) DO UPDATE SET checked_at = ?",
			appID, ruleID, status, now, now,
		)

		var cfg any
		json.Unmarshal([]byte(config), &cfg)
		results = append(results, map[string]any{
			"rule_id": ruleID, "jurisdiction": jurisdiction, "regulation": regulation,
			"requirement": requirement, "stockyard_config": cfg, "status": status,
		})
	}
	if results == nil {
		results = []map[string]any{}
	}
	writeJSON(w, map[string]any{"app_id": appID, "results": results, "checked_at": now})
}

func (a *App) handleAutoFix(w http.ResponseWriter, r *http.Request) {
	appID := r.PathValue("app_id")
	now := time.Now().UTC().Format(time.RFC3339)

	rows, err := a.conn.Query("SELECT id, stockyard_config FROM compliance_rules")
	if err != nil {
		w.WriteHeader(500)
		writeJSON(w, map[string]string{"error": "failed to load rules"})
		return
	}
	defer rows.Close()

	var enabled []string
	for rows.Next() {
		var ruleID, config string
		rows.Scan(&ruleID, &config)

		var cfg struct {
			Modules []string `json:"modules"`
		}
		json.Unmarshal([]byte(config), &cfg)

		enabled = append(enabled, cfg.Modules...)

		// Mark compliant
		a.conn.Exec(
			"INSERT INTO compliance_status (app_id, rule_id, status, checked_at) VALUES (?,?,?,?) ON CONFLICT(app_id, rule_id) DO UPDATE SET status = 'compliant', checked_at = ?",
			appID, ruleID, "compliant", now, now,
		)
	}

	if a.audit != nil {
		a.audit("governance", "compliance.autofix", "", appID, map[string]any{"modules_enabled": enabled})
	}

	writeJSON(w, map[string]any{
		"app_id": appID, "modules_enabled": enabled, "status": "all_compliant", "fixed_at": now,
	})
}

func (a *App) handleListRules(w http.ResponseWriter, r *http.Request) {
	jurisdiction := r.URL.Query().Get("jurisdiction")
	query := "SELECT id, jurisdiction, regulation, requirement, stockyard_config, effective_date FROM compliance_rules"
	var args []any
	if jurisdiction != "" {
		query += " WHERE jurisdiction = ?"
		args = append(args, jurisdiction)
	}
	query += " ORDER BY jurisdiction, regulation"

	rows, err := a.conn.Query(query, args...)
	if err != nil {
		writeJSON(w, map[string]any{"rules": []any{}})
		return
	}
	defer rows.Close()

	var rules []map[string]any
	for rows.Next() {
		var id, juris, regulation, requirement, config, effectiveDate string
		rows.Scan(&id, &juris, &regulation, &requirement, &config, &effectiveDate)
		var cfg any
		json.Unmarshal([]byte(config), &cfg)
		rules = append(rules, map[string]any{
			"id": id, "jurisdiction": juris, "regulation": regulation,
			"requirement": requirement, "stockyard_config": cfg, "effective_date": effectiveDate,
		})
	}
	if rules == nil {
		rules = []map[string]any{}
	}
	writeJSON(w, map[string]any{"rules": rules, "count": len(rules)})
}
