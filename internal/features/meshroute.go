package features

import (
	"bytes"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/stockyard-dev/stockyard/internal/provider"
	"github.com/stockyard-dev/stockyard/internal/proxy"
)

// MeshRouteConfig controls mesh routing behavior.
type MeshRouteConfig struct {
	Mode           string   `json:"mode"`            // mesh-first, cloud-first, cheapest, fastest
	MaxLatencyMs   int      `json:"max_latency_ms"`
	MinReputation  float64  `json:"min_reputation"`
	AllowedRegions []string `json:"allowed_regions"`
}

// meshNode represents a registered mesh node for routing decisions.
type meshNode struct {
	ID              string
	Name            string
	URL             string
	Region          string
	GPUModel        string
	VramGB          int
	CurrentLoad     float64
	MaxConcurrent   int
	PricePer1kCents int
	Reputation      float64
}

// MeshRouteMiddleware routes requests to mesh nodes when available.
// Modes:
//   - cloud-first (default): always use cloud providers, mesh is unused
//   - mesh-first: prefer mesh nodes, fall back to cloud on failure
//   - cheapest: pick the node with lowest price_per_1k_cents
//   - fastest: pick the node with lowest current_load
func MeshRouteMiddleware(conn *sql.DB) proxy.Middleware {
	var (
		mu       sync.RWMutex
		cfg      MeshRouteConfig
		lastLoad time.Time
	)

	client := &http.Client{Timeout: 30 * time.Second}

	loadConfig := func() MeshRouteConfig {
		mu.RLock()
		if time.Since(lastLoad) < 60*time.Second {
			c := cfg
			mu.RUnlock()
			return c
		}
		mu.RUnlock()

		mu.Lock()
		defer mu.Unlock()
		if time.Since(lastLoad) < 60*time.Second {
			return cfg
		}

		cfg = MeshRouteConfig{Mode: "cloud-first", MaxLatencyMs: 5000, MinReputation: 30}
		var raw string
		err := conn.QueryRow("SELECT config_json FROM proxy_modules WHERE name = 'meshroute'").Scan(&raw)
		if err == nil && raw != "" && raw != "{}" {
			json.Unmarshal([]byte(raw), &cfg)
		}
		lastLoad = time.Now()
		return cfg
	}

	// pickNode selects the best available mesh node.
	pickNode := func(c MeshRouteConfig) *meshNode {
		query := `SELECT id, name, url, region, gpu_model, vram_gb,
			current_load, max_concurrent, price_per_1k_cents, reputation
			FROM mesh_nodes
			WHERE status = 'healthy'
			AND last_heartbeat > datetime('now', '-120 seconds')
			AND current_load < max_concurrent
			AND reputation >= ?`
		args := []any{c.MinReputation}

		switch c.Mode {
		case "cheapest":
			query += " ORDER BY price_per_1k_cents ASC, reputation DESC"
		case "fastest":
			query += " ORDER BY current_load ASC, reputation DESC"
		default: // mesh-first
			query += " ORDER BY reputation DESC, current_load ASC"
		}
		query += " LIMIT 1"

		var n meshNode
		err := conn.QueryRow(query, args...).Scan(
			&n.ID, &n.Name, &n.URL, &n.Region, &n.GPUModel, &n.VramGB,
			&n.CurrentLoad, &n.MaxConcurrent, &n.PricePer1kCents, &n.Reputation,
		)
		if err != nil {
			return nil
		}
		return &n
	}

	// forwardToNode sends the request to a mesh node's OpenAI-compatible endpoint.
	forwardToNode := func(ctx context.Context, node *meshNode, req *provider.Request) (*provider.Response, error) {
		start := time.Now()

		body := map[string]any{
			"model":    req.Model,
			"messages": req.Messages,
		}
		if req.Temperature != nil {
			body["temperature"] = *req.Temperature
		}
		if req.MaxTokens != nil {
			body["max_tokens"] = *req.MaxTokens
		}

		bodyBytes, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("meshroute: marshal: %w", err)
		}

		url := node.URL + "/v1/chat/completions"
		httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(bodyBytes))
		if err != nil {
			return nil, fmt.Errorf("meshroute: create request: %w", err)
		}
		httpReq.Header.Set("Content-Type", "application/json")

		httpResp, err := client.Do(httpReq)
		if err != nil {
			return nil, fmt.Errorf("meshroute: node %s: %w", node.Name, err)
		}
		defer httpResp.Body.Close()

		if httpResp.StatusCode != http.StatusOK {
			respBody, _ := io.ReadAll(httpResp.Body)
			limit := len(respBody)
			if limit > 200 {
				limit = 200
			}
			return nil, fmt.Errorf("meshroute: node %s returned %d: %s", node.Name, httpResp.StatusCode, string(respBody[:limit]))
		}

		var oaiResp struct {
			ID      string `json:"id"`
			Object  string `json:"object"`
			Model   string `json:"model"`
			Choices []struct {
				Index   int              `json:"index"`
				Message provider.Message `json:"message"`
				Finish  string           `json:"finish_reason"`
			} `json:"choices"`
			Usage struct {
				Prompt     int `json:"prompt_tokens"`
				Completion int `json:"completion_tokens"`
				Total      int `json:"total_tokens"`
			} `json:"usage"`
		}
		if err := json.NewDecoder(httpResp.Body).Decode(&oaiResp); err != nil {
			return nil, fmt.Errorf("meshroute: decode from %s: %w", node.Name, err)
		}

		latency := time.Since(start)
		choices := make([]provider.Choice, len(oaiResp.Choices))
		for i, c := range oaiResp.Choices {
			choices[i] = provider.Choice{Index: c.Index, Message: c.Message, FinishReason: c.Finish}
		}

		return &provider.Response{
			ID:       oaiResp.ID,
			Object:   oaiResp.Object,
			Model:    oaiResp.Model,
			Choices:  choices,
			Provider: "mesh:" + node.Name,
			Latency:  latency,
			Usage: provider.Usage{
				PromptTokens:     oaiResp.Usage.Prompt,
				CompletionTokens: oaiResp.Usage.Completion,
				TotalTokens:      oaiResp.Usage.Total,
			},
		}, nil
	}

	// recordTransaction logs the mesh transaction and credits the operator.
	recordTransaction := func(node *meshNode, model string, usage provider.Usage, latencyMs int) {
		b := make([]byte, 8)
		rand.Read(b)
		txID := "mtx_" + hex.EncodeToString(b)

		costCents := (usage.TotalTokens * node.PricePer1kCents) / 1000
		if costCents < 1 && usage.TotalTokens > 0 {
			costCents = 1
		}

		conn.Exec(`INSERT INTO mesh_transactions (id, demand_node, supply_node, model,
			input_tokens, output_tokens, cost_cents, latency_ms, verified, created_at)
			VALUES (?, 'self', ?, ?, ?, ?, ?, ?, 0, datetime('now'))`,
			txID, node.ID, model, usage.PromptTokens, usage.CompletionTokens,
			costCents, latencyMs)

		// Credit operator earnings (12% platform fee)
		feeCents := costCents * 12 / 100
		if feeCents < 1 && costCents > 0 {
			feeCents = 1
		}
		operatorEarnings := costCents - feeCents
		period := time.Now().UTC().Format("2006-01")

		conn.Exec(`INSERT INTO mesh_earnings (operator_id, period, tokens_served, earnings_cents, fee_cents)
			VALUES (?, ?, ?, ?, ?)
			ON CONFLICT(operator_id, period) DO UPDATE SET
				tokens_served = tokens_served + excluded.tokens_served,
				earnings_cents = earnings_cents + excluded.earnings_cents,
				fee_cents = fee_cents + excluded.fee_cents`,
			node.ID, period, usage.TotalTokens, operatorEarnings, feeCents)

		// Bump node load, decrement after a delay
		conn.Exec("UPDATE mesh_nodes SET current_load = current_load + 1 WHERE id = ?", node.ID)
		go func(id string) {
			time.Sleep(2 * time.Second)
			conn.Exec("UPDATE mesh_nodes SET current_load = MAX(current_load - 1, 0) WHERE id = ?", id)
		}(node.ID)

		log.Printf("[meshroute] tx %s: node=%s model=%s tokens=%d cost=%d¢ (op=%d¢ fee=%d¢)",
			txID, node.Name, model, usage.TotalTokens, costCents, operatorEarnings, feeCents)
	}

	return func(next proxy.Handler) proxy.Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
			c := loadConfig()

			// Cloud-first: never use mesh
			if c.Mode == "cloud-first" || c.Mode == "" {
				return next(ctx, req)
			}

			// Streaming not yet supported via mesh (requires SSE relay)
			if req.Stream {
				return next(ctx, req)
			}

			// Find best available mesh node
			node := pickNode(c)
			if node == nil {
				if c.Mode == "mesh-first" {
					log.Printf("[meshroute] no healthy mesh nodes, falling back to cloud")
				}
				return next(ctx, req)
			}

			log.Printf("[meshroute] routing to %s (%s, %s, gpu=%s, load=%.0f/%d, rep=%.0f, price=%d¢/1k)",
				node.Name, node.URL, node.Region, node.GPUModel,
				node.CurrentLoad, node.MaxConcurrent, node.Reputation, node.PricePer1kCents)

			resp, err := forwardToNode(ctx, node, req)
			if err != nil {
				log.Printf("[meshroute] node %s failed: %v — falling back to cloud", node.Name, err)
				// Penalize reputation on failure
				conn.Exec("UPDATE mesh_nodes SET reputation = MAX(reputation - 5, 0) WHERE id = ?", node.ID)
				return next(ctx, req)
			}

			// Record transaction and credit operator
			recordTransaction(node, req.Model, resp.Usage, int(resp.Latency.Milliseconds()))

			// Reward reputation on success
			conn.Exec("UPDATE mesh_nodes SET reputation = MIN(reputation + 0.5, 100) WHERE id = ?", node.ID)

			return resp, nil
		}
	}
}
