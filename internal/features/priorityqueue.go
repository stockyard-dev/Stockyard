package features

import (
	"context"
	"time"

	"github.com/stockyard-dev/stockyard/internal/provider"
	"github.com/stockyard-dev/stockyard/internal/proxy"
)

// PriorityQueueMiddleware reads the X-Stockyard-Priority value from
// req.Extra (set by the HTTP layer from request headers) and adjusts
// handling accordingly. "low" priority requests receive a small delay
// (100ms). The priority level is set in resp.Meta["X-Stockyard-Priority"].
func PriorityQueueMiddleware() proxy.Middleware {
	return func(next proxy.Handler) proxy.Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
			// Read priority from Extra (populated from headers during request parsing)
			priority := "normal"
			if req.Extra != nil {
				if p, ok := req.Extra["_priority"].(string); ok && p != "" {
					priority = p
				}
			}

			// Normalize priority value
			switch priority {
			case "high", "normal", "low":
				// valid
			default:
				priority = "normal"
			}

			// Apply delay for low priority requests
			if priority == "low" {
				select {
				case <-time.After(100 * time.Millisecond):
				case <-ctx.Done():
					return nil, ctx.Err()
				}
			}

			// Call next handler
			resp, err := next(ctx, req)
			if err != nil {
				return nil, err
			}

			// Set priority in response metadata
			if resp.Meta == nil {
				resp.Meta = make(map[string]string)
			}
			resp.Meta["X-Stockyard-Priority"] = priority

			return resp, nil
		}
	}
}
