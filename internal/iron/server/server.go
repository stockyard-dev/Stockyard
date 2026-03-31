// Package server provides the Iron HTTP server and admin UI.
//
// The server exposes a REST API for managing specs and triggering builds,
// plus an embedded single-page admin UI. It uses Go 1.22 ServeMux with
// method-based routing.
package server

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/stockyard-dev/stockyard/internal/iron/store"
)

// Server is the Iron HTTP server.
type Server struct {
	store      *store.Store
	mux        *http.ServeMux
	httpServer *http.Server
	port       int
}

// Config holds server configuration.
type Config struct {
	Port    int
	DataDir string
}

// New creates and configures an Iron server.
func New(st *store.Store, cfg Config) *Server {
	if cfg.Port == 0 {
		cfg.Port = 9650
	}

	mux := http.NewServeMux()
	s := &Server{
		store: st,
		mux:   mux,
		port:  cfg.Port,
		httpServer: &http.Server{
			Addr:              fmt.Sprintf(":%d", cfg.Port),
			Handler:           mux,
			ReadTimeout:       30 * time.Second,
			ReadHeaderTimeout: 10 * time.Second,
			WriteTimeout:      300 * time.Second, // Long for builds
			IdleTimeout:       60 * time.Second,
		},
	}
	s.registerRoutes()
	return s
}

// ListenAndServe starts the HTTP server.
func (s *Server) ListenAndServe() error {
	log.Printf("Iron server listening on :%d", s.port)
	return s.httpServer.ListenAndServe()
}

// ServeHTTP implements http.Handler, allowing this server to be
// mounted as a sub-handler in the Stockyard platform.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

// registerRoutes sets up all HTTP routes using Go 1.22 method routing.
func (s *Server) registerRoutes() {
	// API endpoints
	s.mux.HandleFunc("GET /api/health", s.handleHealth)
	s.mux.HandleFunc("GET /api/specs", s.handleListSpecs)
	s.mux.HandleFunc("POST /api/specs", s.handleCreateSpec)
	s.mux.HandleFunc("GET /api/specs/{id}", s.handleGetSpec)
	s.mux.HandleFunc("DELETE /api/specs/{id}", s.handleDeleteSpec)
	s.mux.HandleFunc("POST /api/specs/{id}/build", s.handleBuildSpec)
	s.mux.HandleFunc("GET /api/builds", s.handleListBuilds)
	s.mux.HandleFunc("GET /api/builds/{id}", s.handleGetBuild)

	// Admin UI
	s.mux.HandleFunc("GET /", s.handleUI)
}

// =========================================================================
// Health
// =========================================================================

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// =========================================================================
// Specs
// =========================================================================

func (s *Server) handleListSpecs(w http.ResponseWriter, r *http.Request) {
	specs, err := s.store.ListSpecs()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if specs == nil {
		specs = []store.Spec{}
	}
	writeJSON(w, http.StatusOK, specs)
}

func (s *Server) handleCreateSpec(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 10<<20)) // 10MB limit
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "failed to read body"})
		return
	}
	defer r.Body.Close()

	var req struct {
		Name    string `json:"name"`
		Content string `json:"content"`
		Format  string `json:"format"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON: " + err.Error()})
		return
	}

	if req.Name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name is required"})
		return
	}
	if req.Content == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "content is required"})
		return
	}
	if req.Format == "" {
		req.Format = detectFormat(req.Content)
	}

	now := time.Now().UTC()
	spec := &store.Spec{
		ID:        newID(),
		Name:      req.Name,
		Content:   req.Content,
		Format:    req.Format,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := s.store.CreateSpec(spec); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, spec)
}

func (s *Server) handleGetSpec(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	spec, err := s.store.GetSpec(id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if spec == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "spec not found"})
		return
	}
	writeJSON(w, http.StatusOK, spec)
}

func (s *Server) handleDeleteSpec(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.store.DeleteSpec(id); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "spec not found"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// =========================================================================
// Builds
// =========================================================================

func (s *Server) handleBuildSpec(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	spec, err := s.store.GetSpec(id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if spec == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "spec not found"})
		return
	}

	now := time.Now().UTC()
	buildID := newID()
	outputDir := filepath.Join(s.store.DataDir(), "builds", buildID)
	os.MkdirAll(outputDir, 0755)

	build := &store.Build{
		ID:        buildID,
		SpecID:    spec.ID,
		Status:    "pending",
		OutputDir: outputDir,
		FilesJSON: "[]",
		StartedAt: now,
	}

	if err := s.store.CreateBuild(build); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	// Capture response before goroutine mutates the build
	resp := map[string]any{
		"id":         build.ID,
		"spec_id":    build.SpecID,
		"status":     build.Status,
		"output_dir": build.OutputDir,
		"started_at": build.StartedAt,
	}

	// Run build asynchronously
	go s.runBuild(build, spec)

	writeJSON(w, http.StatusAccepted, resp)
}

func (s *Server) handleListBuilds(w http.ResponseWriter, r *http.Request) {
	builds, err := s.store.ListBuilds()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if builds == nil {
		builds = []store.Build{}
	}
	writeJSON(w, http.StatusOK, builds)
}

func (s *Server) handleGetBuild(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	build, err := s.store.GetBuild(id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if build == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "build not found"})
		return
	}

	// Parse files for the response
	files, _ := store.ParseBuildFiles(build.FilesJSON)
	resp := map[string]any{
		"id":           build.ID,
		"spec_id":      build.SpecID,
		"status":       build.Status,
		"output_dir":   build.OutputDir,
		"files":        files,
		"error":        build.Error,
		"started_at":   build.StartedAt,
		"completed_at": build.CompletedAt,
	}
	writeJSON(w, http.StatusOK, resp)
}

// =========================================================================
// Admin UI
// =========================================================================

func (s *Server) handleUI(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(adminHTML))
}

// =========================================================================
// Build execution
// =========================================================================

// runBuild writes the spec content to disk and records the result.
// Full LLM-powered code generation requires an API key and provider setup.
// When run from the server, we write the spec file so the CLI can be used
// to perform the actual build: iron build <output_dir>
func (s *Server) runBuild(build *store.Build, spec *store.Spec) {
	build.Status = "running"
	s.store.UpdateBuild(build)

	// Write spec file to the build output directory
	ext := ".spec.yaml"
	if spec.Format == "json" {
		ext = ".spec.json"
	}
	specPath := filepath.Join(build.OutputDir, spec.Name+ext)
	if err := os.WriteFile(specPath, []byte(spec.Content), 0644); err != nil {
		build.Status = "failed"
		build.Error = fmt.Sprintf("write spec file: %v", err)
		build.CompletedAt = time.Now().UTC()
		s.store.UpdateBuild(build)
		return
	}

	// Record the spec file as a build output
	files := []store.BuildFile{{Path: spec.Name + ext, Content: spec.Content}}
	filesJSON, marshalErr := json.Marshal(files)
	if marshalErr != nil {
		filesJSON = []byte("{}")
	}
	build.FilesJSON = string(filesJSON)
	build.Status = "success"
	build.CompletedAt = time.Now().UTC()
	s.store.UpdateBuild(build)

	log.Printf("Build %s completed for spec %s (%s)", build.ID, spec.Name, spec.ID)
}

// =========================================================================
// Helpers
// =========================================================================

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func newID() string {
	b := make([]byte, 12)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func detectFormat(content string) string {
	trimmed := strings.TrimSpace(content)
	if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
		return "json"
	}
	return "yaml"
}
