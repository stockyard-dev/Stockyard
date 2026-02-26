package apiserver

// ─── Pricing Plans ─────────────────────────────────────────────────────
// Stockyard uses a 3-tier pricing model: Self-Hosted (free), Cloud, Enterprise.
// All tiers include the full platform (6 apps, 50+ modules, all providers).

// Plan represents a Stockyard pricing tier.
type Plan struct {
	Slug        string            `json:"slug"`
	Name        string            `json:"name"`
	Tagline     string            `json:"tagline"`
	PriceCents  int               `json:"price_cents"` // 0 = free, -1 = custom
	Features    []string          `json:"features"`
	Limits      map[string]string `json:"limits"`
	StripePriceID string          `json:"stripe_price_id,omitempty"`
}

// Plans returns the pricing tiers.
func Plans() []Plan {
	return []Plan{
		{
			Slug: "self-hosted", Name: "Self-Hosted", Tagline: "Full platform, your infrastructure.",
			PriceCents: 0,
			Features: []string{"All 6 apps", "50+ middleware modules", "All providers", "Community support", "Unlimited requests"},
			Limits: map[string]string{"requests": "unlimited", "retention": "unlimited", "support": "community"},
		},
		{
			Slug: "cloud", Name: "Cloud", Tagline: "Managed hosting. Zero ops.",
			PriceCents: 2900, // $29/mo
			Features: []string{"All 6 apps", "50+ middleware modules", "All providers", "Managed infrastructure", "Auto-scaling", "Email support"},
			Limits: map[string]string{"requests": "100,000/mo", "retention": "30 days", "support": "email"},
		},
		{
			Slug: "enterprise", Name: "Enterprise", Tagline: "Unlimited scale. Dedicated support.",
			PriceCents: -1, // Custom pricing
			Features: []string{"All 6 apps", "50+ middleware modules", "All providers", "Dedicated infrastructure", "SSO/SAML", "99.9% SLA", "Dedicated support engineer"},
			Limits: map[string]string{"requests": "unlimited", "retention": "1 year", "support": "dedicated"},
		},
	}
}

// PlanBySlug returns a plan by slug.
func PlanBySlug(slug string) *Plan {
	for _, p := range Plans() {
		if p.Slug == slug {
			return &p
		}
	}
	return nil
}

// ─── Legacy Product compat (keeps /api/products working) ───────────────

// Product represents a module in the catalog for backward compatibility.
type Product struct {
	Slug     string `json:"slug"`
	Name     string `json:"name"`
	Tagline  string `json:"tagline"`
	Category string `json:"category"`
}

// Catalog returns the module catalog (not individual products for sale).
// This replaces the old 125-product model. All modules are included in every tier.
func Catalog() []Product {
	return []Product{
		// The 6 apps
		{Slug: "proxy", Name: "Proxy", Tagline: "Core reverse-proxy, middleware chain, provider dispatch.", Category: "app"},
		{Slug: "observe", Name: "Observe", Tagline: "Tracing, cost attribution, alerts & anomaly detection.", Category: "app"},
		{Slug: "trust", Name: "Trust", Tagline: "Policy engine, audit ledger & compliance evidence.", Category: "app"},
		{Slug: "studio", Name: "Studio", Tagline: "Prompt templates, experiments & benchmarks.", Category: "app"},
		{Slug: "forge", Name: "Forge", Tagline: "DAG workflow engine, tools & sessions.", Category: "app"},
		{Slug: "exchange", Name: "Exchange", Tagline: "Config pack marketplace & environment sync.", Category: "app"},
	}
}

// CatalogCount returns the number of apps.
func CatalogCount() int {
	return len(Catalog())
}

// ProductBySlug returns an app/module by slug.
func ProductBySlug(slug string) *Product {
	for _, p := range Catalog() {
		if p.Slug == slug {
			return &p
		}
	}
	return nil
}
