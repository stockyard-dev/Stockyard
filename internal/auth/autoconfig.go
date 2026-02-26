package auth

import (
	"context"
	"log"
	"net/http"
	"strings"

	"github.com/stockyard-dev/stockyard/internal/provider"
)

// AutoConfigMiddleware detects when a user sends a raw provider API key
// (not a sk-sy- Stockyard key) and auto-configures the provider for them.
//
// Flow:
//  1. User sends: Authorization: Bearer sk-abc123... with model: gpt-4o
//  2. Middleware detects this is an OpenAI key (prefix sk-)
//  3. Creates an ephemeral provider and injects it into context
//  4. If user is authenticated (has sk-sy- key in X-Stockyard-Key),
//     also saves the provider key to their account for next time.
//
// This enables zero-config: first request "just works" without any setup.
func AutoConfigMiddleware(store *Store, factory *ProviderFactory) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !strings.HasPrefix(r.URL.Path, "/v1/") {
				next.ServeHTTP(w, r)
				return
			}

			// If there's already a user in context, the key was a sk-sy- key
			// and the provider factory will handle resolution
			if UserFromContext(r.Context()) != nil {
				next.ServeHTTP(w, r)
				return
			}

			// Extract the bearer token
			key := extractBearerKey(r)
			if key == "" {
				next.ServeHTTP(w, r)
				return
			}

			// Skip if it's a Stockyard key (handled by ProxyAuthMiddleware)
			if strings.HasPrefix(key, "sk-sy-") {
				next.ServeHTTP(w, r)
				return
			}

			// Detect provider from key prefix
			providerName := detectProviderFromKey(key)
			if providerName == "" {
				next.ServeHTTP(w, r)
				return
			}

			// Create ephemeral provider and inject via ProviderResolver context
			p := createProvider(providerName, key, "")
			if p == nil {
				next.ServeHTTP(w, r)
				return
			}

			log.Printf("[autoconfig] detected %s key, creating ephemeral provider", providerName)

			// Inject the provider into context for the send handler
			ctx := context.WithValue(r.Context(), autoProviderKey, p)
			ctx = context.WithValue(ctx, autoProviderNameKey, providerName)

			// If user also sent X-Stockyard-Key, save the provider key for future use
			syKey := r.Header.Get("X-Stockyard-Key")
			if strings.HasPrefix(syKey, "sk-sy-") {
				go func() {
					user, _, err := store.ValidateKey(syKey)
					if err == nil && user != nil {
						if err := store.SetProviderKey(user.ID, providerName, key, ""); err == nil {
							log.Printf("[autoconfig] saved %s key for user %s", providerName, user.Email)
							factory.InvalidateCache(user.ID)
						}
					}
				}()
			}

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

type autoConfigContextKey int

const (
	autoProviderKey     autoConfigContextKey = iota
	autoProviderNameKey
)

// AutoProviderFromContext returns an ephemeral provider set by AutoConfigMiddleware.
func AutoProviderFromContext(ctx context.Context) (provider.Provider, string) {
	p, _ := ctx.Value(autoProviderKey).(provider.Provider)
	name, _ := ctx.Value(autoProviderNameKey).(string)
	return p, name
}

// detectProviderFromKey guesses the provider from an API key's prefix/format.
func detectProviderFromKey(key string) string {
	switch {
	// OpenAI: sk-proj-..., sk-... (but not sk-sy- which is ours)
	case strings.HasPrefix(key, "sk-proj-") || strings.HasPrefix(key, "sk-svcacct-"):
		return "openai"
	case strings.HasPrefix(key, "sk-") && !strings.HasPrefix(key, "sk-sy-") && !strings.HasPrefix(key, "sk-ant-"):
		return "openai"

	// Anthropic: sk-ant-...
	case strings.HasPrefix(key, "sk-ant-"):
		return "anthropic"

	// Groq: gsk_...
	case strings.HasPrefix(key, "gsk_"):
		return "groq"

	// Together: various formats, often starts with a hex string
	// (not reliably detectable by prefix alone)

	// Mistral: typically a UUID-like string, not easily detectable

	// DeepSeek: sk-... (same as OpenAI prefix, would be caught above)

	// Cohere: various formats

	// Fireworks: fw_...
	case strings.HasPrefix(key, "fw_"):
		return "fireworks"

	// Perplexity: pplx-...
	case strings.HasPrefix(key, "pplx-"):
		return "perplexity"

	// xAI: xai-...
	case strings.HasPrefix(key, "xai-"):
		return "xai"

	default:
		return ""
	}
}
