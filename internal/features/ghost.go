package features

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/stockyard-dev/stockyard/internal/provider"
	"github.com/stockyard-dev/stockyard/internal/proxy"
)

// GhostMiddleware returns synthetic responses without calling any provider.
// Useful for testing, load simulation, and cost estimation.
func GhostMiddleware(conn *sql.DB) proxy.Middleware {
	return func(next proxy.Handler) proxy.Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
			// Estimate tokens
			var totalChars int
			for _, m := range req.Messages {
				totalChars += len(m.Content)
			}
			estInput := totalChars / 4
			if estInput < 1 {
				estInput = 1
			}
			estOutput := 128 + rand.Intn(256)

			// Simulate latency
			latency := time.Duration(50+rand.Intn(200)) * time.Millisecond
			time.Sleep(latency)

			costUSD := provider.CalculateCost(req.Model, estInput, estOutput)

			resp := &provider.Response{
				ID:       fmt.Sprintf("ghost-%d", time.Now().UnixNano()),
				Object:   "chat.completion",
				Model:    req.Model,
				Provider: "ghost",
				Latency:  latency,
				Choices: []provider.Choice{
					{
						Index:        0,
						Message:      provider.Message{Role: "assistant", Content: "[Ghost Mode] Synthetic response — no provider was called."},
						FinishReason: "stop",
					},
				},
				Usage: provider.Usage{
					PromptTokens:     estInput,
					CompletionTokens: estOutput,
					TotalTokens:      estInput + estOutput,
				},
				Meta: map[string]string{
					"X-Stockyard-Ghost":    "true",
					"X-Stockyard-Provider": "ghost",
					"X-Stockyard-Cost":     fmt.Sprintf("%.6f", costUSD),
				},
			}

			log.Printf("[ghost] synthetic response: model=%s tokens=%d cost=$%.4f",
				req.Model, estInput+estOutput, costUSD)

			return resp, nil
		}
	}
}

// GhostConfig holds configuration for ghost mode.
type GhostConfig struct {
	Enabled bool `json:"enabled"`
}

// GhostFromDB loads ghost config.
func GhostFromDB(conn *sql.DB) GhostConfig {
	cfg := GhostConfig{}
	var raw string
	err := conn.QueryRow("SELECT config_json FROM proxy_modules WHERE name = 'ghost'").Scan(&raw)
	if err != nil || raw == "" || raw == "{}" {
		return cfg
	}
	json.Unmarshal([]byte(raw), &cfg)
	return cfg
}
