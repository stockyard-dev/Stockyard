package auth

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/stockyard-dev/stockyard/internal/provider"
)

// contextKey is unexported to prevent collisions.
type contextKey int

const (
	userKey contextKey = iota
	apiKeyKey
)

// WithUser adds a User to the context.
func WithUser(ctx context.Context, u *User) context.Context {
	return context.WithValue(ctx, userKey, u)
}

// UserFromContext extracts the authenticated User from context.
func UserFromContext(ctx context.Context) *User {
	u, _ := ctx.Value(userKey).(*User)
	return u
}

// WithAPIKey adds an APIKey to the context.
func WithAPIKey(ctx context.Context, k *APIKey) context.Context {
	return context.WithValue(ctx, apiKeyKey, k)
}

// APIKeyFromContext extracts the APIKey from context.
func APIKeyFromContext(ctx context.Context) *APIKey {
	k, _ := ctx.Value(apiKeyKey).(*APIKey)
	return k
}

// ─── Proxy Auth Middleware ──────────────────────────────────────────────────

// ProxyAuthMode controls how /v1/ endpoints are authenticated.
type ProxyAuthMode int

const (
	// ProxyAuthOpen allows all requests (dev mode, no STOCKYARD_REQUIRE_AUTH).
	ProxyAuthOpen ProxyAuthMode = iota
	// ProxyAuthRequired requires a valid sk-sy- key on all /v1/ requests.
	ProxyAuthRequired
)

// ProxyAuthMiddleware returns HTTP middleware that authenticates /v1/ proxy requests.
// It extracts the API key from Authorization header, validates it, and injects user context.
//
// In Open mode: unauthenticated requests pass through (backward compatible).
// In Required mode: unauthenticated requests get 401.
//
// Authenticated requests always get user context injected regardless of mode.
func ProxyAuthMiddleware(store *Store, mode ProxyAuthMode) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Only apply to proxy routes
			if !strings.HasPrefix(r.URL.Path, "/v1/") {
				next.ServeHTTP(w, r)
				return
			}

			// Extract bearer token
			key := extractBearerKey(r)

			// If it's a Stockyard user key (sk-sy-), validate it
			if strings.HasPrefix(key, "sk-sy-") {
				user, ak, err := store.ValidateKey(key)
				if err != nil {
					log.Printf("[auth] key validation error: %v", err)
					http.Error(w, `{"error":{"message":"authentication error","type":"auth_error"}}`, 500)
					return
				}
				if user == nil {
					http.Error(w, `{"error":{"message":"invalid or revoked API key","type":"auth_error"}}`, 401)
					return
				}

				// Inject user context
				ctx := WithUser(r.Context(), user)
				ctx = WithAPIKey(ctx, ak)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			// Not a Stockyard key — could be a direct provider key (pass-through)
			if mode == ProxyAuthRequired && key == "" {
				http.Error(w, `{"error":{"message":"API key required. Use Authorization: Bearer sk-sy-...","type":"auth_error"}}`, 401)
				return
			}

			// Open mode or has a non-Stockyard key — pass through
			next.ServeHTTP(w, r)
		})
	}
}

// SelfServiceAuthMiddleware authenticates /api/auth/me/* routes using API key.
func SelfServiceAuthMiddleware(store *Store) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !strings.HasPrefix(r.URL.Path, "/api/auth/me") {
				next.ServeHTTP(w, r)
				return
			}

			key := extractBearerKey(r)
			if !strings.HasPrefix(key, "sk-sy-") {
				http.Error(w, `{"error":"send Authorization: Bearer sk-sy-..."}`, 401)
				return
			}

			user, ak, err := store.ValidateKey(key)
			if err != nil || user == nil {
				http.Error(w, `{"error":"invalid or revoked API key"}`, 401)
				return
			}

			ctx := WithUser(r.Context(), user)
			ctx = WithAPIKey(ctx, ak)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func extractBearerKey(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}
	return ""
}

// ─── Provider Resolution with User Keys ────────────────────────────────────

// ProviderFactory creates provider instances from API keys.
// Caches per-user providers briefly to avoid re-creating on every request.
type ProviderFactory struct {
	store          *Store
	globalProviders map[string]provider.Provider
	cache          sync.Map // map[string]cachedProvider
}

type cachedProvider struct {
	provider provider.Provider
	expires  time.Time
}

// NewProviderFactory creates a factory that resolves providers for users.
func NewProviderFactory(store *Store, globalProviders map[string]provider.Provider) *ProviderFactory {
	return &ProviderFactory{
		store:           store,
		globalProviders: globalProviders,
	}
}

// ResolveProvider returns the best provider for a request.
// Priority: user's own key > global provider.
func (f *ProviderFactory) ResolveProvider(ctx context.Context, providerName string) (provider.Provider, error) {
	user := UserFromContext(ctx)
	if user == nil {
		// No authenticated user — use global
		if p, ok := f.globalProviders[providerName]; ok {
			return p, nil
		}
		return nil, nil
	}

	// Check cache first
	cacheKey := cacheKeyFor(user.ID, providerName)
	if cached, ok := f.cache.Load(cacheKey); ok {
		cp := cached.(cachedProvider)
		if time.Now().Before(cp.expires) {
			return cp.provider, nil
		}
		f.cache.Delete(cacheKey)
	}

	// Look up user's provider key
	apiKey, baseURL, err := f.store.GetProviderKey(user.ID, providerName)
	if err != nil {
		return nil, err
	}

	if apiKey == "" {
		// User has no key for this provider — fall back to global
		if p, ok := f.globalProviders[providerName]; ok {
			return p, nil
		}
		return nil, nil
	}

	// Create provider with user's key
	p := createProvider(providerName, apiKey, baseURL)
	if p == nil {
		return nil, nil
	}

	// Cache for 5 minutes
	f.cache.Store(cacheKey, cachedProvider{
		provider: p,
		expires:  time.Now().Add(5 * time.Minute),
	})

	return p, nil
}

// ResolveAllProviders returns a provider map for the user (user keys + global fallbacks).
func (f *ProviderFactory) ResolveAllProviders(ctx context.Context) map[string]provider.Provider {
	result := make(map[string]provider.Provider)

	// Start with global providers
	for name, p := range f.globalProviders {
		result[name] = p
	}

	// Override with user providers
	user := UserFromContext(ctx)
	if user == nil {
		return result
	}

	userKeys, err := f.store.GetAllProviderKeys(user.ID)
	if err != nil {
		log.Printf("[auth] error loading user provider keys: %v", err)
		return result
	}

	for name, pk := range userKeys {
		p := createProvider(name, pk.APIKey, pk.BaseURL)
		if p != nil {
			result[name] = p
		}
	}

	return result
}

// InvalidateCache removes cached providers for a user.
func (f *ProviderFactory) InvalidateCache(userID int64) {
	f.cache.Range(func(key, value any) bool {
		if strings.HasPrefix(key.(string), fmt.Sprintf("%d:", userID)) {
			f.cache.Delete(key)
		}
		return true
	})
}

func cacheKeyFor(userID int64, providerName string) string {
	return fmt.Sprintf("%d:%s", userID, providerName)
}

// createProvider creates a provider instance for the given name and API key.
// This uses the provider package constructors.
func createProvider(name, apiKey, baseURL string) provider.Provider {
	cfg := provider.ProviderConfig{
		APIKey:  apiKey,
		Timeout: 60 * time.Second,
	}
	if baseURL != "" {
		cfg.BaseURL = baseURL
	}

	switch name {
	case "openai":
		return provider.NewOpenAI(cfg)
	case "anthropic":
		return provider.NewAnthropic(cfg)
	case "gemini":
		return provider.NewGemini(cfg)
	case "groq":
		return provider.NewGroq(cfg)
	default:
		// Unknown provider — try as OpenAI-compatible
		if cfg.BaseURL != "" {
			return provider.NewOpenAI(cfg)
		}
		return nil
	}
}

// GetProxyAuthMode reads the auth mode from environment.
func GetProxyAuthMode() ProxyAuthMode {
	if os.Getenv("STOCKYARD_REQUIRE_AUTH") == "true" || os.Getenv("STOCKYARD_REQUIRE_AUTH") == "1" {
		return ProxyAuthRequired
	}
	return ProxyAuthOpen
}
