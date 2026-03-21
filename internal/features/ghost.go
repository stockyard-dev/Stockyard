package features

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/stockyard-dev/stockyard/internal/provider"
	"github.com/stockyard-dev/stockyard/internal/proxy"
)

// ghostConfig holds the ghost module configuration.
type ghostConfig struct {
	Enabled bool `json:"enabled"`
}

// ghostEnabled checks whether the ghost module is enabled in proxy_modules.
func ghostEnabled(conn *sql.DB) bool {
	var configJSON sql.NullString
	err := conn.QueryRow("SELECT config_json FROM proxy_modules WHERE name = ?", "ghost").Scan(&configJSON)
	if err != nil || !configJSON.Valid || configJSON.String == "" {
		return false
	}
	var cfg ghostConfig
	if json.Unmarshal([]byte(configJSON.String), &cfg) != nil {
		return false
	}
	return cfg.Enabled
}

// GhostMiddleware returns synthetic responses without calling the upstream provider.
// When enabled via proxy_modules config_json (ghost.enabled), it estimates token
// usage from message length and returns immediately with provider="ghost".
func GhostMiddleware(conn *sql.DB) proxy.Middleware {
	return func(next proxy.Handler) proxy.Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
			if !ghostEnabled(conn) {
				return next(ctx, req)
			}

			// Estimate prompt tokens from message content (~1 token per 4 chars)
			promptTokens := 0
			for _, msg := range req.Messages {
				promptTokens += len(msg.Content) / 4
			}
			if promptTokens < 1 {
				promptTokens = 1
			}

			// Build a synthetic response content from the last user message
			lastUserMsg := ""
			for i := len(req.Messages) - 1; i >= 0; i-- {
				if req.Messages[i].Role == "user" {
					lastUserMsg = req.Messages[i].Content
					break
				}
			}
			content := fmt.Sprintf("[ghost] synthetic response for: %s", ghostTruncate(lastUserMsg, 100))
			completionTokens := len(strings.Fields(content))

			resp := &provider.Response{
				ID:     fmt.Sprintf("ghost-%d", time.Now().UnixNano()),
				Object: "chat.completion",
				Model:  req.Model,
				Choices: []provider.Choice{
					{
						Index:        0,
						Message:      provider.Message{Role: "assistant", Content: content},
						FinishReason: "stop",
					},
				},
				Usage: provider.Usage{
					PromptTokens:     promptTokens,
					CompletionTokens: completionTokens,
					TotalTokens:      promptTokens + completionTokens,
				},
				Provider: "ghost",
				CacheHit: false,
				Meta:     map[string]string{"X-Stockyard-Provider": "ghost"},
			}

			return resp, nil
		}
	}
}

// ghostTruncate shortens a string to maxLen, appending "..." if truncated.
func ghostTruncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
