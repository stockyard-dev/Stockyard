package features

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/stockyard-dev/stockyard/internal/provider"
	"github.com/stockyard-dev/stockyard/internal/proxy"
)

// ResponseHeadersMiddleware adds X-Stockyard-* headers to every proxied response.
// Headers include trace ID, model, provider, cost, token counts, cache status, and latency.
// For streaming responses, only pre-request data (model, provider) is available in headers;
// final token/cost data arrives as trailing SSE metadata.
func ResponseHeadersMiddleware() proxy.Middleware {
	return func(next proxy.Handler) proxy.Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
			start := time.Now()

			resp, err := next(ctx, req)
			if err != nil {
				return nil, err
			}

			latency := time.Since(start)

			if resp.Meta == nil {
				resp.Meta = make(map[string]string)
			}

			// Core metadata
			resp.Meta["X-Stockyard-Model"] = resp.Model
			if resp.Provider != "" {
				resp.Meta["X-Stockyard-Provider"] = resp.Provider
			} else if req.Provider != "" {
				resp.Meta["X-Stockyard-Provider"] = req.Provider
			}

			// Token counts
			resp.Meta["X-Stockyard-Tokens-In"] = fmt.Sprintf("%d", resp.Usage.PromptTokens)
			resp.Meta["X-Stockyard-Tokens-Out"] = fmt.Sprintf("%d", resp.Usage.CompletionTokens)

			// Cost estimate
			costUSD := provider.CalculateCost(resp.Model, resp.Usage.PromptTokens, resp.Usage.CompletionTokens)
			resp.Meta["X-Stockyard-Cost"] = fmt.Sprintf("%.6f", costUSD)
			resp.Meta["X-Stockyard-Cost-Cents"] = fmt.Sprintf("%d", int64(math.Round(costUSD*100)))

			// Cache status
			if resp.CacheHit {
				resp.Meta["X-Stockyard-Cached"] = "true"
			} else {
				resp.Meta["X-Stockyard-Cached"] = "false"
			}

			// Latency
			resp.Meta["X-Stockyard-Latency-Ms"] = fmt.Sprintf("%d", latency.Milliseconds())

			// Trace ID from context (if available)
			if tid, ok := ctx.Value(billingTraceKey{}).(string); ok && tid != "" {
				resp.Meta["X-Stockyard-Trace-Id"] = tid
			}

			return resp, nil
		}
	}
}
