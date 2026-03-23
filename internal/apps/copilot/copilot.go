// Package copilot implements the Stockyard Copilot — AI-assisted platform configuration via natural language.
package copilot

import (
	"bytes"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"time"
)

// App is the Copilot application.
type App struct {
	conn      *sql.DB
	audit     func(string, string, string, string, any)
	mux       *http.ServeMux
	proxyPort int

	// Rate limiting: max 20 requests per minute, in-memory.
	rateMu       sync.Mutex
	rateRequests []time.Time
}

// New creates a new Copilot app.
func New(conn *sql.DB) *App { return &App{conn: conn} }

// Name returns the app identifier.
func (a *App) Name() string { return "copilot" }

// Description returns a human-readable description.
func (a *App) Description() string {
	return "AI assistant for natural language platform configuration"
}

// Migrate creates the copilot schema and stores the DB connection.
func (a *App) Migrate(conn *sql.DB) error {
	a.conn = conn
	conn.Exec(`CREATE TABLE IF NOT EXISTS copilot_sessions (
		id TEXT PRIMARY KEY,
		messages TEXT DEFAULT '[]',
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL
	)`)
	log.Printf("[copilot] migrations applied")
	return nil
}

// SetAuditor wires the trust audit function.
func (a *App) SetAuditor(fn func(string, string, string, string, any)) { a.audit = fn }

// SetMux provides the shared HTTP mux for executing internal actions.
func (a *App) SetMux(mux *http.ServeMux) { a.mux = mux }

// SetProxyPort sets the local proxy port for LLM calls.
func (a *App) SetProxyPort(port int) { a.proxyPort = port }

// RegisterRoutes registers all copilot HTTP routes.
func (a *App) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/copilot/chat", a.handleChat)
	mux.HandleFunc("GET /api/copilot/sessions", a.handleListSessions)
	mux.HandleFunc("GET /api/copilot/sessions/{id}", a.handleGetSession)
	mux.HandleFunc("DELETE /api/copilot/sessions/{id}", a.handleDeleteSession)
	log.Printf("[copilot] routes registered")
}

// ---------------------------------------------------------------------------
// Security allowlist
// ---------------------------------------------------------------------------

var allowedPrefixes = []struct {
	prefix  string
	methods []string // empty = all methods allowed
}{
	{"/api/proxy/modules", nil},
	{"/api/proxy/routing", nil},
	{"/api/proxy/providers", []string{"GET"}},
	{"/api/proxy/consensus", nil},
	{"/api/observe", nil},
	{"/api/billing/customers", nil},
	{"/api/billing/plans", nil},
	{"/api/billing/usage", []string{"GET"}},
	{"/api/studio", nil},
	{"/api/forge", nil},
	{"/api/exchange", nil},
	{"/api/recall", nil},
	{"/api/trust/ledger", []string{"GET"}},
	{"/api/fabric", nil},
	{"/api/apps", nil},
	{"/api/knowledge", nil},
	{"/api/mesh", nil},
	{"/api/memory", nil},
	{"/api/governance", nil},
	{"/api/reputation", nil},
}

var blockedSubstrings = []string{"key", "auth", "encrypt", "admin", "secret", "token"}

func isAllowedAction(method, path string) bool {
	lowerPath := strings.ToLower(path)
	for _, sub := range blockedSubstrings {
		if strings.Contains(lowerPath, sub) {
			return false
		}
	}

	if !strings.HasPrefix(path, "/api/") {
		return false
	}

	method = strings.ToUpper(method)
	switch method {
	case "GET", "POST", "PUT", "DELETE":
	default:
		return false
	}

	for _, ap := range allowedPrefixes {
		if strings.HasPrefix(path, ap.prefix) {
			if len(ap.methods) == 0 {
				return true
			}
			for _, m := range ap.methods {
				if m == method {
					return true
				}
			}
			return false
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// System prompt
// ---------------------------------------------------------------------------

const systemPrompt = `You are the Stockyard Copilot. You configure an LLM proxy platform by calling its REST API.
Respond ONLY with a JSON object: {"thinking": "...", "reply": "...", "actions": [{"method": "PUT", "path": "/api/proxy/modules/cost_cap", "body": {...}}]}

Available endpoints:
- PUT /api/proxy/modules/{name} — enable/disable modules: {"enabled": true/false, "config": {...}}
- GET /api/proxy/modules — list all middleware modules
- GET /api/proxy/providers — list configured providers
- POST /api/proxy/routing/rules — create routing rule
- GET /api/proxy/routing/rules — list routing rules
- GET /api/observe/traces — list recent traces
- GET /api/observe/costs — get cost overview
- GET /api/observe/costs/daily — daily cost breakdown
- POST /api/billing/customers — create billing customer
- GET /api/billing/customers — list customers
- GET /api/billing/usage — usage report
- POST /api/studio/templates — create prompt template
- GET /api/studio/templates — list templates
- POST /api/forge/workflows — create workflow
- GET /api/forge/workflows — list workflows
- POST /api/recall/incidents — create recall incident
- GET /api/recall/incidents — list incidents
- GET /api/trust/ledger — view audit ledger
- POST /api/fabric/deploy — deploy a Fabric manifest
- GET /api/fabric/deployments — list Fabric deployments
- POST /api/fabric/validate — validate a manifest
- POST /api/fabric/diff — preview changes
- GET /api/apps/store — browse app store
- POST /api/apps/builder/create — create new app
- GET /api/knowledge/bases — list knowledge bases
- GET /api/mesh/nodes — list mesh nodes
- GET /api/memory/{user_id}/relevant?query=X — search memory

Rules:
- Only use GET, POST, PUT, DELETE methods
- Paths must start with /api/
- Keep actions minimal — only what the user asked for
- If unsure, ask the user for clarification in your reply`

// ---------------------------------------------------------------------------
// LLM interaction
// ---------------------------------------------------------------------------

func callLLM(port int, messages []map[string]string) (string, error) {
	body := map[string]any{
		"model":    "gpt-4o-mini",
		"messages": messages,
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	url := fmt.Sprintf("http://127.0.0.1:%d/v1/chat/completions", port)
	req, err := http.NewRequest("POST", url, bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Stockyard-Source", "copilot")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("llm call: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("llm returned status %d: %s", resp.StatusCode, string(respBody))
	}

	// Parse OpenAI-style response to extract content.
	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("parse llm response: %w", err)
	}
	if len(result.Choices) == 0 {
		return "", fmt.Errorf("llm returned no choices")
	}
	return result.Choices[0].Message.Content, nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func genCopilotID(prefix string) string {
	b := make([]byte, 6)
	rand.Read(b)
	return prefix + hex.EncodeToString(b)
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// ---------------------------------------------------------------------------
// Rate limiting
// ---------------------------------------------------------------------------

func (a *App) checkRateLimit() bool {
	a.rateMu.Lock()
	defer a.rateMu.Unlock()

	now := time.Now()
	cutoff := now.Add(-1 * time.Minute)

	// Prune old entries.
	valid := a.rateRequests[:0]
	for _, t := range a.rateRequests {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}
	a.rateRequests = valid

	if len(a.rateRequests) >= 20 {
		return false
	}
	a.rateRequests = append(a.rateRequests, now)
	return true
}

// ---------------------------------------------------------------------------
// POST /api/copilot/chat
// ---------------------------------------------------------------------------

func (a *App) handleChat(w http.ResponseWriter, r *http.Request) {
	// Rate limit check.
	if !a.checkRateLimit() {
		writeJSON(w, http.StatusTooManyRequests, map[string]any{
			"reply":   "Rate limit exceeded. Please wait a moment before trying again.",
			"actions": []any{},
		})
		return
	}

	var req struct {
		Message   string `json:"message"`
		SessionID string `json:"session_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if req.Message == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "message is required"})
		return
	}

	// Load or create session.
	sessionID := req.SessionID
	var messages []map[string]string

	if sessionID != "" {
		var messagesJSON string
		err := a.conn.QueryRow("SELECT messages FROM copilot_sessions WHERE id = ?", sessionID).Scan(&messagesJSON)
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "session not found"})
			return
		}
		json.Unmarshal([]byte(messagesJSON), &messages)
	} else {
		sessionID = genCopilotID("cs_")
		now := time.Now().UTC().Format(time.RFC3339)
		_, err := a.conn.Exec(
			"INSERT INTO copilot_sessions (id, messages, created_at, updated_at) VALUES (?, '[]', ?, ?)",
			sessionID, now, now,
		)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to create session"})
			return
		}
	}

	// Build the full messages array for the LLM.
	llmMessages := []map[string]string{
		{"role": "system", "content": systemPrompt},
	}
	llmMessages = append(llmMessages, messages...)
	llmMessages = append(llmMessages, map[string]string{"role": "user", "content": req.Message})

	// Call LLM.
	if a.proxyPort <= 0 || a.proxyPort > 65535 {
		// Append user message to session and save.
		messages = append(messages, map[string]string{"role": "user", "content": req.Message})
		a.saveSession(sessionID, messages)
		writeJSON(w, http.StatusOK, map[string]any{
			"reply":      "Configure at least one LLM provider to use the Copilot.",
			"actions":    []any{},
			"session_id": sessionID,
		})
		return
	}

	llmContent, err := callLLM(a.proxyPort, llmMessages)
	if err != nil {
		// Append user message to session and save.
		messages = append(messages, map[string]string{"role": "user", "content": req.Message})
		a.saveSession(sessionID, messages)
		writeJSON(w, http.StatusOK, map[string]any{
			"reply":      "Configure at least one LLM provider to use the Copilot.",
			"actions":    []any{},
			"session_id": sessionID,
		})
		return
	}

	// Parse LLM response JSON.
	parsed, err := parseLLMResponse(llmContent)
	if err != nil {
		// Retry once with stricter prompt.
		retryMessages := append(llmMessages, map[string]string{
			"role":    "user",
			"content": "Your previous response was not valid JSON. Please respond ONLY with a JSON object: {\"thinking\": \"...\", \"reply\": \"...\", \"actions\": [...]}",
		})
		llmContent, err = callLLM(a.proxyPort, retryMessages)
		if err != nil {
			messages = append(messages, map[string]string{"role": "user", "content": req.Message})
			a.saveSession(sessionID, messages)
			writeJSON(w, http.StatusOK, map[string]any{
				"reply":      "I couldn't process that. Try rephrasing your request.",
				"actions":    []any{},
				"session_id": sessionID,
			})
			return
		}
		parsed, err = parseLLMResponse(llmContent)
		if err != nil {
			messages = append(messages, map[string]string{"role": "user", "content": req.Message})
			a.saveSession(sessionID, messages)
			writeJSON(w, http.StatusOK, map[string]any{
				"reply":      "I couldn't process that. Try rephrasing your request.",
				"actions":    []any{},
				"session_id": sessionID,
			})
			return
		}
	}

	// Enforce max 10 actions.
	if len(parsed.Actions) > 10 {
		parsed.Actions = parsed.Actions[:10]
	}

	// Execute valid actions.
	actionResults := make([]map[string]any, 0, len(parsed.Actions))
	for _, action := range parsed.Actions {
		result := map[string]any{
			"method": action.Method,
			"path":   action.Path,
		}
		if action.Body != nil {
			result["body"] = action.Body
		}

		if !isAllowedAction(action.Method, action.Path) {
			result["status"] = 403
			result["error"] = "action not allowed by security policy"
			actionResults = append(actionResults, result)
			continue
		}

		if a.mux == nil {
			result["status"] = 500
			result["error"] = "internal mux not configured"
			actionResults = append(actionResults, result)
			continue
		}

		// Execute action internally.
		statusCode, respBody := a.executeAction(action)
		result["status"] = statusCode
		if respBody != nil {
			result["response"] = respBody
		}

		// Audit.
		if a.audit != nil {
			a.audit("copilot_action", "copilot", action.Path, action.Method, map[string]any{
				"session_id": sessionID,
				"status":     statusCode,
			})
		}

		actionResults = append(actionResults, result)
	}

	// Save session with the new messages.
	messages = append(messages, map[string]string{"role": "user", "content": req.Message})
	messages = append(messages, map[string]string{"role": "assistant", "content": llmContent})
	a.saveSession(sessionID, messages)

	writeJSON(w, http.StatusOK, map[string]any{
		"reply":      parsed.Reply,
		"actions":    actionResults,
		"session_id": sessionID,
	})
}

// ---------------------------------------------------------------------------
// LLM response parsing
// ---------------------------------------------------------------------------

type llmAction struct {
	Method string `json:"method"`
	Path   string `json:"path"`
	Body   any    `json:"body,omitempty"`
}

type llmResponse struct {
	Thinking string      `json:"thinking"`
	Reply    string      `json:"reply"`
	Actions  []llmAction `json:"actions"`
}

func parseLLMResponse(content string) (*llmResponse, error) {
	// Try to extract JSON from the content — the LLM may wrap it in markdown.
	content = strings.TrimSpace(content)
	if idx := strings.Index(content, "{"); idx >= 0 {
		// Find the last closing brace.
		lastIdx := strings.LastIndex(content, "}")
		if lastIdx > idx {
			content = content[idx : lastIdx+1]
		}
	}

	var resp llmResponse
	if err := json.Unmarshal([]byte(content), &resp); err != nil {
		return nil, fmt.Errorf("invalid JSON from LLM: %w", err)
	}
	return &resp, nil
}

// ---------------------------------------------------------------------------
// Internal action execution
// ---------------------------------------------------------------------------

func (a *App) executeAction(action llmAction) (int, any) {
	var bodyReader io.Reader
	if action.Body != nil {
		bodyBytes, err := json.Marshal(action.Body)
		if err != nil {
			return 500, map[string]string{"error": "failed to marshal action body"}
		}
		bodyReader = bytes.NewReader(bodyBytes)
	}

	internalReq, err := http.NewRequest(action.Method, action.Path, bodyReader)
	if err != nil {
		return 500, map[string]string{"error": "failed to create internal request"}
	}
	internalReq.Header.Set("Content-Type", "application/json")
	internalReq.Header.Set("X-Stockyard-Source", "copilot")

	recorder := httptest.NewRecorder()
	a.mux.ServeHTTP(recorder, internalReq)

	var respBody any
	if recorder.Body.Len() > 0 {
		if err := json.Unmarshal(recorder.Body.Bytes(), &respBody); err != nil {
			respBody = recorder.Body.String()
		}
	}
	return recorder.Code, respBody
}

// ---------------------------------------------------------------------------
// Session persistence
// ---------------------------------------------------------------------------

func (a *App) saveSession(sessionID string, messages []map[string]string) {
	msgJSON, _ := json.Marshal(messages)
	now := time.Now().UTC().Format(time.RFC3339)
	a.conn.Exec("UPDATE copilot_sessions SET messages = ?, updated_at = ? WHERE id = ?",
		string(msgJSON), now, sessionID)
}

// ---------------------------------------------------------------------------
// GET /api/copilot/sessions
// ---------------------------------------------------------------------------

func (a *App) handleListSessions(w http.ResponseWriter, r *http.Request) {
	rows, err := a.conn.Query(
		"SELECT id, messages, created_at, updated_at FROM copilot_sessions ORDER BY updated_at DESC LIMIT 20",
	)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"sessions": []any{}})
		return
	}
	defer rows.Close()

	var sessions []map[string]any
	for rows.Next() {
		var id, messagesJSON, createdAt, updatedAt string
		if err := rows.Scan(&id, &messagesJSON, &createdAt, &updatedAt); err != nil {
			continue
		}
		var msgs []any
		json.Unmarshal([]byte(messagesJSON), &msgs)
		sessions = append(sessions, map[string]any{
			"id":            id,
			"message_count": len(msgs),
			"created_at":    createdAt,
			"updated_at":    updatedAt,
		})
	}
	if sessions == nil {
		sessions = []map[string]any{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"sessions": sessions})
}

// ---------------------------------------------------------------------------
// GET /api/copilot/sessions/{id}
// ---------------------------------------------------------------------------

func (a *App) handleGetSession(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var messagesJSON, createdAt, updatedAt string
	err := a.conn.QueryRow(
		"SELECT messages, created_at, updated_at FROM copilot_sessions WHERE id = ?", id,
	).Scan(&messagesJSON, &createdAt, &updatedAt)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "session not found"})
		return
	}

	var messages any
	json.Unmarshal([]byte(messagesJSON), &messages)

	writeJSON(w, http.StatusOK, map[string]any{
		"id":         id,
		"messages":   messages,
		"created_at": createdAt,
		"updated_at": updatedAt,
	})
}

// ---------------------------------------------------------------------------
// DELETE /api/copilot/sessions/{id}
// ---------------------------------------------------------------------------

func (a *App) handleDeleteSession(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	result, err := a.conn.Exec("DELETE FROM copilot_sessions WHERE id = ?", id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to delete session"})
		return
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "session not found"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
