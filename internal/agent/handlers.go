package agent

import (
	"encoding/json"
	"net/http"
	"strconv"
)

// RegisterRoutes adds agent API endpoints to the mux.
func RegisterRoutes(mux *http.ServeMux, a *Agent) {
	mux.HandleFunc("POST /api/agent/run", a.handleRun)
	mux.HandleFunc("GET /api/agent/runs", a.handleListRuns)
	mux.HandleFunc("GET /api/agent/runs/{id}", a.handleGetRun)
	mux.HandleFunc("GET /api/agent/tools", a.handleListTools)
	mux.HandleFunc("POST /api/agent/tools", a.handleAddTool)
	mux.HandleFunc("GET /api/agent/status", a.handleStatus)
}

func (a *Agent) handleRun(w http.ResponseWriter, r *http.Request) {
	var req RunRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid request: " + err.Error()})
		return
	}
	if req.Prompt == "" {
		writeJSON(w, 400, map[string]string{"error": "prompt is required"})
		return
	}

	run, err := a.Execute(r.Context(), req)
	if err != nil && run == nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, 200, run)
}

func (a *Agent) handleListRuns(w http.ResponseWriter, r *http.Request) {
	limit := 20
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}

	runs, err := a.ListRuns(limit)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	if runs == nil {
		runs = []Run{}
	}
	writeJSON(w, 200, map[string]any{"runs": runs, "count": len(runs)})
}

func (a *Agent) handleGetRun(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	run, err := a.GetRun(id)
	if err != nil {
		writeJSON(w, 404, map[string]string{"error": "run not found"})
		return
	}
	writeJSON(w, 200, run)
}

func (a *Agent) handleListTools(w http.ResponseWriter, r *http.Request) {
	tools := a.ConnectedTools()
	var out []map[string]any
	for _, ct := range tools {
		toolNames := make([]string, len(ct.Def.Tools))
		for i, t := range ct.Def.Tools {
			toolNames[i] = t.Name
		}
		out = append(out, map[string]any{
			"product":     ct.Product,
			"displayName": ct.Def.DisplayName,
			"baseURL":     ct.BaseURL,
			"tools":       toolNames,
			"toolCount":   len(ct.Def.Tools),
		})
	}
	if out == nil {
		out = []map[string]any{}
	}
	writeJSON(w, 200, map[string]any{"connected": out, "count": len(out)})
}

func (a *Agent) handleAddTool(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Product string `json:"product"`
		Host    string `json:"host"`
		Port    int    `json:"port"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid request"})
		return
	}
	if req.Host == "" {
		req.Host = "127.0.0.1"
	}
	if req.Product == "" || req.Port == 0 {
		writeJSON(w, 400, map[string]string{"error": "product and port are required"})
		return
	}
	if err := a.AddTool(req.Product, req.Host, req.Port); err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]string{"status": "connected", "product": req.Product})
}

func (a *Agent) handleStatus(w http.ResponseWriter, r *http.Request) {
	tools := a.ConnectedTools()
	totalTools := 0
	for _, ct := range tools {
		totalTools += len(ct.Def.Tools)
	}

	runs, _ := a.ListRuns(1)
	var lastRun *Run
	if len(runs) > 0 {
		lastRun = &runs[0]
	}

	writeJSON(w, 200, map[string]any{
		"status":            "ready",
		"connected_products": len(tools),
		"available_tools":   totalTools,
		"last_run":          lastRun,
		"proxy_url":         a.proxyURL,
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
