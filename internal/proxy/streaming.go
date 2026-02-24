package proxy

import (
	"fmt"
	"log"
	"net/http"

	"github.com/stockyard-dev/stockyard/internal/provider"
)

// StreamHandler is like Handler but returns a stream channel.
type StreamHandler func(req *provider.Request) (<-chan provider.StreamChunk, error)

// StreamPreFlight defines pre-flight checks that streaming requests must pass.
// These are set during server configuration to bridge middleware logic.
type StreamPreFlight struct {
	// CheckRateLimit returns an error if the request should be rate-limited.
	CheckRateLimit func(req *provider.Request) error

	// CheckCaps returns an error if the request would exceed spending caps.
	CheckCaps func(req *provider.Request) error

	// ResolveProvider returns the ordered list of providers to try for streaming.
	ResolveProvider func(req *provider.Request) []string

	// OnStreamComplete is called after a stream finishes with token/provider info.
	// This enables spend tracking and logging for streaming requests.
	OnStreamComplete func(req *provider.Request, providerName string, tokensSoFar int)
}

// handleStream handles streaming SSE responses from providers.
// It runs pre-flight checks (rate limit, caps) then streams from the
// resolved provider with failover support.
func (s *Server) handleStream(w http.ResponseWriter, r *http.Request, req *provider.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	// Pre-flight: rate limit check
	if s.config.PreFlight.CheckRateLimit != nil {
		if err := s.config.PreFlight.CheckRateLimit(req); err != nil {
			writeError(w, http.StatusTooManyRequests, err.Error())
			return
		}
	}

	// Pre-flight: spending cap check
	if s.config.PreFlight.CheckCaps != nil {
		if err := s.config.PreFlight.CheckCaps(req); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			fmt.Fprintf(w, `{"error":{"message":%q,"type":"cap_exceeded"}}`, err.Error())
			return
		}
	}

	// Build provider chain (respects failover order)
	providerChain := s.resolveStreamProviders(req)
	if len(providerChain) == 0 {
		writeSSEError(w, flusher, "no providers configured for model: "+req.Model)
		return
	}

	// Try providers in order (failover)
	var stream <-chan provider.StreamChunk
	var streamErr error
	var usedProvider string

	for _, name := range providerChain {
		p, ok := s.config.Providers[name]
		if !ok {
			continue
		}

		stream, streamErr = p.SendStream(r.Context(), req)
		if streamErr == nil {
			usedProvider = name
			break
		}

		// Check if error is retryable
		if apiErr, ok := streamErr.(*provider.ProviderAPIError); ok && !apiErr.IsRetryable() {
			// Non-retryable error (4xx) — don't failover
			writeSSEError(w, flusher, streamErr.Error())
			return
		}

		log.Printf("stream failover: %s failed (%v), trying next", name, streamErr)
	}

	if streamErr != nil {
		writeSSEError(w, flusher, fmt.Sprintf("all providers failed: %v", streamErr))
		return
	}

	// Set SSE headers before any data
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	_ = usedProvider // tracked via OnStreamComplete below

	// Pipe chunks directly to client, flushing each one
	var lastTokens int
	for chunk := range stream {
		if chunk.Error != nil {
			writeSSEError(w, flusher, chunk.Error.Error())
			return
		}

		lastTokens = chunk.TokensSoFar

		_, writeErr := w.Write(chunk.Data)
		if writeErr != nil {
			log.Printf("stream write error: %v", writeErr)
			return
		}
		flusher.Flush()

		if chunk.Done {
			break
		}
	}

	// Post-stream: track spend and log
	if s.config.PreFlight.OnStreamComplete != nil {
		s.config.PreFlight.OnStreamComplete(req, usedProvider, lastTokens)
	}
}

// resolveStreamProviders returns the ordered list of provider names to try.
func (s *Server) resolveStreamProviders(req *provider.Request) []string {
	// If pre-flight provides a custom resolver (failover-aware), use it
	if s.config.PreFlight.ResolveProvider != nil {
		chain := s.config.PreFlight.ResolveProvider(req)
		if len(chain) > 0 {
			return chain
		}
	}

	// Default: use X-Provider header or auto-detect from model
	name := req.Provider
	if name == "" {
		name = provider.ProviderForModel(req.Model)
	}

	// If failover providers are configured, build the full chain
	// starting with the resolved provider, then adding others
	chain := []string{name}
	for provName := range s.config.Providers {
		if provName != name {
			chain = append(chain, provName)
		}
	}
	return chain
}

// writeSSEError sends an error as an SSE event and closes the stream.
func writeSSEError(w http.ResponseWriter, flusher http.Flusher, msg string) {
	errJSON := fmt.Sprintf(`{"error":{"message":%q,"type":"proxy_error"}}`, msg)
	fmt.Fprintf(w, "data: %s\n\n", errJSON)
	flusher.Flush()
	fmt.Fprint(w, "data: [DONE]\n\n")
	flusher.Flush()
}
