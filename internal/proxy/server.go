// Package proxy implements the core HTTP reverse proxy server.
package proxy

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/stockyard-dev/stockyard/internal/provider"
)

// Handler is the core request processing function signature.
type Handler func(ctx context.Context, req *provider.Request) (*provider.Response, error)

// Middleware wraps a Handler, adding functionality before/after the inner handler.
type Middleware func(next Handler) Handler

// Chain applies middleware in order. The first middleware is outermost (runs first).
func Chain(h Handler, mw ...Middleware) Handler {
	for i := len(mw) - 1; i >= 0; i-- {
		h = mw[i](h)
	}
	return h
}

// EmbeddingCacheProcessor handles cached embedding processing.
// This interface breaks the circular dependency between proxy and features.
type EmbeddingCacheProcessor interface {
	ProcessEmbeddingRequestRaw(body []byte, forward func(body []byte) ([]byte, error)) ([]byte, error)
}

// ServerConfig holds configuration for the proxy server.
type ServerConfig struct {
	Port        int
	ProductName string
	Handler     Handler
	Providers   map[string]provider.Provider
	PreFlight   StreamPreFlight
	EmbedCache  EmbeddingCacheProcessor // nil if embedding caching disabled
}

// Server is the main HTTP server that proxies LLM requests.
type Server struct {
	config     ServerConfig
	httpServer *http.Server
	mux        *http.ServeMux
}

// NewServer creates a new proxy server.
func NewServer(cfg ServerConfig) *Server {
	mux := http.NewServeMux()
	s := &Server{
		config: cfg,
		mux:    mux,
		httpServer: &http.Server{
			Addr:         fmt.Sprintf(":%d", cfg.Port),
			Handler:      mux,
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 120 * time.Second, // Long for streaming
			IdleTimeout:  60 * time.Second,
		},
	}
	s.registerRoutes()
	return s
}

// registerRoutes sets up the HTTP routes.
func (s *Server) registerRoutes() {
	// Proxied LLM endpoints (OpenAI-compatible)
	s.mux.HandleFunc("POST /v1/chat/completions", s.handleChatCompletions)
	s.mux.HandleFunc("POST /v1/completions", s.handleCompletions)
	s.mux.HandleFunc("POST /v1/embeddings", s.handleEmbeddings)

	// Health check
	s.mux.HandleFunc("GET /health", s.handleHealth)
}

// Start begins listening for requests.
func (s *Server) Start() error {
	log.Printf("%s proxy listening on :%d", s.config.ProductName, s.config.Port)
	return s.httpServer.ListenAndServe()
}

// Shutdown gracefully stops the server.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}

// Mux returns the underlying ServeMux for registering additional routes
// (dashboard, management API, etc.).
func (s *Server) Mux() *http.ServeMux {
	return s.mux
}

// WrapHandler wraps the server's HTTP handler with middleware.
// Call after all routes are registered but before Start().
func (s *Server) WrapHandler(wrapper func(http.Handler) http.Handler) {
	s.httpServer.Handler = wrapper(s.httpServer.Handler)
}
