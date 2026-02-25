package apiserver

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)

// CloudTenant represents a Stockyard Cloud tenant account.
type CloudTenant struct {
	ID                   string            `json:"id"`
	Email                string            `json:"email"`
	Name                 string            `json:"name,omitempty"`
	APIKey               string            `json:"api_key"`
	Plan                 string            `json:"plan"`
	CreatedAt            time.Time         `json:"created_at"`
	DailyRequestLimit    int               `json:"daily_request_limit"`
	StripeCustomerID     string            `json:"stripe_customer_id,omitempty"`
	StripeSubscriptionID string            `json:"stripe_subscription_id,omitempty"`
	ProviderKeys         map[string]string `json:"provider_keys"`
	ProxyConfig          map[string]any    `json:"proxy_config"`
	EnabledProducts      []string          `json:"enabled_products"`
}

// CloudUsage holds daily usage counters for a tenant.
type CloudUsage struct {
	TenantID string  `json:"tenant_id"`
	Date     string  `json:"date"`
	Requests int64   `json:"requests"`
	TokensIn int64   `json:"tokens_in"`
	TokensOut int64  `json:"tokens_out"`
	CostUSD  float64 `json:"cost_usd"`
	CacheHits int64  `json:"cache_hits"`
	Errors   int64   `json:"errors"`
}

// ExchangeItem represents a shared config, chain, or pack in the Exchange marketplace.
type ExchangeItem struct {
	ID          string    `json:"id"`
	Slug        string    `json:"slug"`
	Type        string    `json:"type"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	AuthorEmail string    `json:"author_email"`
	AuthorName  string    `json:"author_name"`
	Content     string    `json:"content"`
	Tags        []string  `json:"tags"`
	Products    []string  `json:"products"`
	Providers   []string  `json:"providers"`
	Downloads   int64     `json:"downloads"`
	Stars       int64     `json:"stars"`
	Forks       int64     `json:"forks"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// --- ID generation helpers ---

func generateID(prefix string, length int) string {
	b := make([]byte, length)
	rand.Read(b)
	return prefix + hex.EncodeToString(b)[:length]
}

func generateTenantID() string {
	return generateID("ct_", 12)
}

func generateAPIKey() string {
	b := make([]byte, 24)
	rand.Read(b)
	return fmt.Sprintf("sk_sy_%s", hex.EncodeToString(b))
}
