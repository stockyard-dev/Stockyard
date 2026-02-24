package features

import (
	"context"
	"log"
	"time"

	"github.com/stockyard-dev/stockyard/internal/provider"
	"github.com/stockyard-dev/stockyard/internal/proxy"
)

// RetryMiddleware returns middleware that retries failed requests.
func RetryMiddleware(maxRetries int) proxy.Middleware {
	return func(next proxy.Handler) proxy.Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
			var lastErr error
			for attempt := 0; attempt <= maxRetries; attempt++ {
				resp, err := next(ctx, req)
				if err == nil {
					return resp, nil
				}
				lastErr = err
				if attempt < maxRetries {
					backoff := time.Duration(attempt+1) * 500 * time.Millisecond
					log.Printf("retry: attempt %d/%d failed: %v, retrying in %v",
						attempt+1, maxRetries, err, backoff)

					select {
					case <-time.After(backoff):
					case <-ctx.Done():
						return nil, ctx.Err()
					}
				}
			}
			return nil, lastErr
		}
	}
}
