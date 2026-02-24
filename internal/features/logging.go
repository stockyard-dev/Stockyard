package features

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log"
	"time"

	"github.com/stockyard-dev/stockyard/internal/provider"
	"github.com/stockyard-dev/stockyard/internal/proxy"
	"github.com/stockyard-dev/stockyard/internal/storage"
	"github.com/stockyard-dev/stockyard/internal/tracker"
)

// LoggingConfig controls what gets logged.
type LoggingConfig struct {
	StoreBodies  bool
	MaxBodySize  int
	DB           *storage.DB
	Broadcaster  EventBroadcaster
}

// EventBroadcaster is an interface for sending real-time events to the dashboard.
type EventBroadcaster interface {
	Send(event interface{})
}

// LoggingMiddleware returns middleware that logs every proxied request to SQLite.
func LoggingMiddleware(cfg LoggingConfig) proxy.Middleware {
	return func(next proxy.Handler) proxy.Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
			start := time.Now()
			reqID := generateID()

			// Capture request body if configured
			var reqBody string
			if cfg.StoreBodies {
				if raw, ok := req.Extra["_raw_body"].(string); ok {
					reqBody = truncate(raw, cfg.MaxBodySize)
				}
			}

			// Call the next handler
			resp, err := next(ctx, req)

			latency := time.Since(start)
			status := 200
			var respBody string
			var errMsg string
			var tokensIn, tokensOut int
			var costUSD float64
			var cacheHit bool

			if err != nil {
				status = 502
				errMsg = err.Error()
				// Still estimate input tokens for cost tracking
				tokensIn = tracker.CountInputTokens(req.Model, req.Messages)
			} else {
				// Use provider-reported usage if available, else estimate
				if resp.Usage.PromptTokens > 0 {
					tokensIn = resp.Usage.PromptTokens
					tokensOut = resp.Usage.CompletionTokens
				} else {
					tokensIn = tracker.CountInputTokens(req.Model, req.Messages)
					if len(resp.Choices) > 0 {
						tokensOut = tracker.CountOutputTokens(resp.Choices[0].Message.Content)
					}
				}
				costUSD = provider.CalculateCost(req.Model, tokensIn, tokensOut)
				cacheHit = resp.CacheHit

				if cfg.StoreBodies && len(resp.Choices) > 0 {
					b, _ := json.Marshal(resp)
					respBody = truncate(string(b), cfg.MaxBodySize)
				}
			}

			// Write to SQLite
			logEntry := &storage.RequestLog{
				ID:           reqID,
				Timestamp:    start,
				Project:      req.Project,
				UserID:       req.UserID,
				Provider:     providerName(req, resp),
				Model:        req.Model,
				TokensIn:     tokensIn,
				TokensOut:    tokensOut,
				CostUSD:      costUSD,
				LatencyMs:    latency.Milliseconds(),
				Status:       status,
				CacheHit:     cacheHit,
				RequestBody:  reqBody,
				ResponseBody: respBody,
				Error:        errMsg,
			}

			if cfg.DB != nil {
				if insertErr := cfg.DB.InsertRequest(logEntry); insertErr != nil {
					log.Printf("logging: insert failed: %v", insertErr)
				}
			}

			// Broadcast event for live dashboard
			if cfg.Broadcaster != nil {
				cfg.Broadcaster.Send(map[string]any{
					"type":      "request_logged",
					"id":        reqID,
					"model":     req.Model,
					"tokens":    tokensIn + tokensOut,
					"cost":      costUSD,
					"latency":   latency.Milliseconds(),
					"cache_hit": cacheHit,
					"status":    status,
				})
			}

			return resp, err
		}
	}
}

func providerName(req *provider.Request, resp *provider.Response) string {
	if resp != nil && resp.Provider != "" {
		return resp.Provider
	}
	if req.Provider != "" {
		return req.Provider
	}
	return provider.ProviderForModel(req.Model)
}

func generateID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func truncate(s string, max int) string {
	if max <= 0 || len(s) <= max {
		return s
	}
	return s[:max]
}
