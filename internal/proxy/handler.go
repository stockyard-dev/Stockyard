package proxy

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"

	"github.com/stockyard-dev/stockyard/internal/provider"
)

// handleChatCompletions handles POST /v1/chat/completions
func (s *Server) handleChatCompletions(w http.ResponseWriter, r *http.Request) {
	req, rawBody, err := s.parseRequest(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Store raw body for potential logging/replay
	req.Extra["_raw_body"] = string(rawBody)

	if req.Stream {
		s.handleStream(w, r, req)
		return
	}

	resp, err := s.config.Handler(r.Context(), req)
	if err != nil {
		// Check for cap exceeded error
		if capErr, ok := isCapError(err); ok {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			json.NewEncoder(w).Encode(map[string]any{
				"error": map[string]any{
					"message": capErr.Error(),
					"type":    "cap_exceeded",
				},
			})
			return
		}
		writeError(w, http.StatusBadGateway, fmt.Sprintf("provider error: %v", err))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// handleCompletions handles POST /v1/completions (legacy)
func (s *Server) handleCompletions(w http.ResponseWriter, r *http.Request) {
	s.handleChatCompletions(w, r)
}

// handleEmbeddings handles POST /v1/embeddings
func (s *Server) handleEmbeddings(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "read body: "+err.Error())
		return
	}
	defer r.Body.Close()

	// Find an embedding-capable provider
	var embProvider provider.EmbeddingProvider
	// Prefer provider specified in X-Provider header
	if pname := r.Header.Get("X-Provider"); pname != "" {
		// Try user-specific provider first
		if s.config.ProviderResolver != nil {
			if resolved, err := s.config.ProviderResolver(r.Context(), pname); err == nil && resolved != nil {
				if ep, ok := resolved.(provider.EmbeddingProvider); ok {
					embProvider = ep
				}
			}
		}
		if embProvider == nil {
			if p, ok := s.config.Providers[pname]; ok {
				if ep, ok := p.(provider.EmbeddingProvider); ok {
					embProvider = ep
				}
			}
		}
	}
	// Fall back to any provider that supports embeddings
	if embProvider == nil {
		for _, p := range s.config.Providers {
			if ep, ok := p.(provider.EmbeddingProvider); ok {
				embProvider = ep
				break
			}
		}
	}
	if embProvider == nil {
		writeError(w, http.StatusBadGateway, "no embedding-capable provider configured")
		return
	}

	// Create the forward function
	forward := func(fwdBody []byte) ([]byte, error) {
		return embProvider.SendEmbedding(r.Context(), fwdBody)
	}

	// If EmbedCache is enabled, use it
	if s.config.EmbedCache != nil {
		respBody, err := s.config.EmbedCache.ProcessEmbeddingRequestRaw(body, forward)
		if err != nil {
			writeError(w, http.StatusBadGateway, fmt.Sprintf("embedding error: %v", err))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(respBody)
		return
	}

	// No cache — direct forward
	respBody, err := forward(body)
	if err != nil {
		writeError(w, http.StatusBadGateway, fmt.Sprintf("embedding error: %v", err))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(respBody)
}

// handleHealth handles GET /health
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	providerStatus := make(map[string]string)
	for name, p := range s.config.Providers {
		if err := p.HealthCheck(r.Context()); err != nil {
			providerStatus[name] = "unhealthy: " + err.Error()
		} else {
			providerStatus[name] = "healthy"
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"status":    "ok",
		"product":   s.config.ProductName,
		"providers": providerStatus,
	})
}

// parseRequest extracts a canonical Request from an HTTP request.
// Returns the request and the raw body bytes.
func (s *Server) parseRequest(r *http.Request) (*provider.Request, []byte, error) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("read body: %w", err)
	}
	defer r.Body.Close()

	var req provider.Request
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, nil, fmt.Errorf("parse request: %w", err)
	}

	// Parse extra fields not in the struct
	var extra map[string]any
	if err := json.Unmarshal(body, &extra); err == nil {
		delete(extra, "model")
		delete(extra, "messages")
		delete(extra, "stream")
		delete(extra, "temperature")
		delete(extra, "max_tokens")
	}
	if extra == nil {
		extra = make(map[string]any)
	}
	req.Extra = extra

	// Extract routing headers
	req.Project = r.Header.Get("X-Project")
	if req.Project == "" {
		req.Project = "default"
	}
	req.UserID = r.Header.Get("X-User-Id")
	req.Schema = r.Header.Get("X-Schema")
	req.Provider = r.Header.Get("X-Provider")

	// Extract client IP for IP-based access control
	req.ClientIP = extractClientIP(r)

	if req.Model == "" {
		return nil, nil, fmt.Errorf("model is required")
	}
	if len(req.Messages) == 0 {
		return nil, nil, fmt.Errorf("messages is required")
	}

	return &req, body, nil
}

// writeError writes a JSON error response.
func writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]any{
			"message": message,
			"type":    "proxy_error",
		},
	})
}

// isCapError checks if an error is a cap exceeded error.
func isCapError(err error) (interface{ Error() string }, bool) {
	type capErr interface {
		Error() string
	}
	// Check if the error message contains "cap exceeded"
	if err != nil && (contains(err.Error(), "cap exceeded") || contains(err.Error(), "cap_exceeded")) {
		return err, true
	}
	return nil, false
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// extractClientIP extracts the real client IP from the HTTP request.
// Checks X-Forwarded-For, X-Real-IP, then falls back to RemoteAddr.
func extractClientIP(r *http.Request) string {
	// Try X-Forwarded-For first (leftmost IP is the client)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.SplitN(xff, ",", 2)
		ip := strings.TrimSpace(parts[0])
		if net.ParseIP(ip) != nil {
			return ip
		}
	}
	// Try X-Real-IP
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		ip := strings.TrimSpace(xri)
		if net.ParseIP(ip) != nil {
			return ip
		}
	}
	// Fall back to RemoteAddr
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
