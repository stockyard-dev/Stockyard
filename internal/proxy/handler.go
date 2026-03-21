package proxy

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strings"

	"github.com/stockyard-dev/stockyard/internal/billingerr"
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
		// Check for billing limit errors (429 for limit, 403 for model)
		if isBillingError(err) {
			status := http.StatusTooManyRequests
			errType := "billing_limit_exceeded"
			if _, ok := billingerr.IsBillingModelError(err); ok {
				status = http.StatusForbidden
				errType = "billing_model_denied"
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(status)
			json.NewEncoder(w).Encode(map[string]any{
				"error": map[string]any{
					"message": err.Error(),
					"type":    errType,
				},
			})
			return
		}
		writeError(w, classifyError(err), sanitizeError(err))
		return
	}

	// Set custom response headers from middleware metadata
	if resp.Meta != nil {
		for k, v := range resp.Meta {
			w.Header().Set(k, v)
		}
		// Expose custom headers to browser JS via CORS
		w.Header().Set("Access-Control-Expose-Headers", "X-Stockyard-Trace-Id, X-Stockyard-Cost, X-Stockyard-Model, X-Stockyard-Provider, X-Stockyard-Cached, X-Stockyard-Latency-Ms, X-Stockyard-Tokens-In, X-Stockyard-Tokens-Out, X-Stockyard-Cost-Cents")
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
	r.Body = http.MaxBytesReader(nil, r.Body, maxRequestBodySize)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusRequestEntityTooLarge, "request body too large or unreadable")
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
			log.Printf("health check: %s: %v", name, err)
			providerStatus[name] = "unhealthy"
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

// handleProviderTest bypasses all middleware and sends a minimal request
// directly to the first available provider. For debugging only.
func (s *Server) handleProviderTest(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	names := make([]string, 0)
	for name := range s.config.Providers {
		names = append(names, name)
	}

	for name, p := range s.config.Providers {
		temp := float64(0)
		maxTok := 5
		req := &provider.Request{
			Model:       "gpt-4o-mini",
			Messages:    []provider.Message{{Role: "user", Content: "Say ok"}},
			Temperature: &temp,
			MaxTokens:   &maxTok,
			Extra:       map[string]any{},
		}
		resp, err := p.Send(r.Context(), req)
		if err != nil {
			json.NewEncoder(w).Encode(map[string]any{
				"provider":          name,
				"provider_map_keys": names,
				"error":             err.Error(),
			})
			return
		}
		json.NewEncoder(w).Encode(map[string]any{
			"provider":          name,
			"provider_map_keys": names,
			"model":             resp.Model,
			"content":           resp.Choices[0].Message.Content,
			"ok":                true,
		})
		return
	}

	// Fallback: try via the full handler chain
	if s.config.Handler != nil {
		temp := float64(0)
		maxTok := 5
		req := &provider.Request{
			Model:       "gpt-4o-mini",
			Messages:    []provider.Message{{Role: "user", Content: "Say ok"}},
			Temperature: &temp,
			MaxTokens:   &maxTok,
			Extra:       map[string]any{},
		}
		resp, err := s.config.Handler(r.Context(), req)
		if err != nil {
			json.NewEncoder(w).Encode(map[string]any{
				"provider_map_keys": names,
				"handler_error":     err.Error(),
			})
			return
		}
		content := ""
		if len(resp.Choices) > 0 {
			content = resp.Choices[0].Message.Content
		}
		json.NewEncoder(w).Encode(map[string]any{
			"provider_map_keys": names,
			"model":             resp.Model,
			"content":           content,
			"ok":                true,
			"via":               "handler",
		})
		return
	}

	json.NewEncoder(w).Encode(map[string]any{
		"error":             "no providers",
		"provider_map_keys": names,
	})
}

// maxRequestBodySize is the maximum allowed request body size (10 MB).
const maxRequestBodySize = 10 * 1024 * 1024

// parseRequest extracts a canonical Request from an HTTP request.
// Returns the request and the raw body bytes.
func (s *Server) parseRequest(r *http.Request) (*provider.Request, []byte, error) {
	r.Body = http.MaxBytesReader(nil, r.Body, maxRequestBodySize)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("request body too large or unreadable")
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
	req.CustomerID = r.Header.Get("X-Customer-ID")
	req.Schema = r.Header.Get("X-Schema")
	req.Provider = r.Header.Get("X-Provider")

	// Store auth header for JWT claim extraction by billing meter
	if authHeader := r.Header.Get("Authorization"); authHeader != "" {
		req.Extra["_auth_header"] = authHeader
	}

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
// classifyError returns an appropriate HTTP status code based on the error message.
func classifyError(err error) int {
	msg := err.Error()
	switch {
	case strings.Contains(msg, "license"):
		return http.StatusPaymentRequired // 402
	case strings.Contains(msg, "no providers configured"):
		return http.StatusServiceUnavailable // 503
	case strings.Contains(msg, "circuit open"):
		return http.StatusServiceUnavailable // 503
	case strings.Contains(msg, "rate limit") || strings.Contains(msg, "status 429"):
		return http.StatusTooManyRequests // 429
	case strings.Contains(msg, "status 401") || strings.Contains(msg, "invalid API key"):
		return http.StatusUnauthorized // 401
	case strings.Contains(msg, "status 403"):
		return http.StatusForbidden // 403
	default:
		return http.StatusBadGateway // 502
	}
}

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

// isBillingError checks if an error originates from the billing meter middleware.
func isBillingError(err error) bool {
	if err == nil {
		return false
	}
	if _, ok := billingerr.IsBillingLimitError(err); ok {
		return true
	}
	if _, ok := billingerr.IsBillingModelError(err); ok {
		return true
	}
	return false
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// sanitizeError returns a client-safe error message, stripping internal details
// like file paths, SQL errors, and provider implementation details.
func sanitizeError(err error) string {
	if err == nil {
		return "internal error"
	}
	msg := err.Error()
	// Allow through well-known client-facing error messages
	switch {
	case strings.Contains(msg, "cap exceeded") || strings.Contains(msg, "cap_exceeded"):
		return msg
	case strings.Contains(msg, "rate limit") || strings.Contains(msg, "status 429"):
		return "rate limit exceeded"
	case strings.Contains(msg, "no providers configured"):
		return "no providers configured for the requested model"
	case strings.Contains(msg, "circuit open"):
		return "service temporarily unavailable"
	case strings.Contains(msg, "status 401") || strings.Contains(msg, "invalid API key"):
		return "authentication error with upstream provider"
	case strings.Contains(msg, "status 403"):
		return "forbidden by upstream provider"
	case strings.Contains(msg, "all providers failed"):
		return "all providers failed to process the request"
	default:
		log.Printf("proxy error (sanitized): %v", err)
		return "internal proxy error"
	}
}

// extractClientIP extracts the real client IP from the HTTP request.
// Only trusts X-Forwarded-For and X-Real-IP if STOCKYARD_TRUST_PROXY is set,
// since these headers can be spoofed by direct clients.
// Falls back to RemoteAddr which is always the actual TCP peer.
func extractClientIP(r *http.Request) string {
	// Only trust proxy headers if explicitly configured
	if os.Getenv("STOCKYARD_TRUST_PROXY") != "" {
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
	}
	// Fall back to RemoteAddr
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
