package apiserver

// ─── Pricing Plans ─────────────────────────────────────────────────────
// Stockyard uses a 5-tier pricing model:
//   Community (free) → Individual ($9.99) → Pro ($49) → Team ($149) → Enterprise ($499)
// All tiers include the full platform (6 apps, 58 modules, all providers).

// Plan represents a Stockyard pricing tier.
type Plan struct {
	Slug        string            `json:"slug"`
	Name        string            `json:"name"`
	Tagline     string            `json:"tagline"`
	PriceCents  int               `json:"price_cents"` // 0 = free or custom
	Custom      bool              `json:"custom"`      // true = contact sales
	Features    []string          `json:"features"`
	Limits      map[string]string `json:"limits"`
	StripePriceID string          `json:"stripe_price_id,omitempty"`
}

// Plans returns the pricing tiers.
func Plans() []Plan {
	return []Plan{
		{
			Slug: "community", Name: "Community", Tagline: "Full platform. Self-hosted. Free forever.",
			PriceCents: 0,
			Features: []string{
				"All 6 apps",
				"58 middleware modules",
				"All 16 providers",
				"10,000 requests/mo",
				"SQLite storage",
				"Community support",
			},
			Limits: map[string]string{
				"requests":  "10,000/mo",
				"retention": "7 days",
				"support":   "community",
				"users":     "3",
			},
		},
		{
			Slug: "individual", Name: "Individual", Tagline: "Unlimited self-hosted. Priority support.",
			PriceCents: 999, // $9.99/mo
			Features: []string{
				"Everything in Community",
				"Unlimited requests",
				"Unlimited retention",
				"Auto-backups",
				"Email alerts",
				"Priority support",
				"Unlimited users",
			},
			Limits: map[string]string{
				"requests":  "unlimited",
				"retention": "unlimited",
				"support":   "priority",
				"users":     "unlimited",
			},
		},
		{
			Slug: "pro", Name: "Pro", Tagline: "Cloud-managed. Zero ops.",
			PriceCents: 4900, // $49/mo
			Features: []string{
				"Everything in Individual",
				"Managed infrastructure",
				"Auto-scaling",
				"90-day trace retention",
				"Daily backups",
				"Email support",
				"Custom domain",
			},
			Limits: map[string]string{
				"requests":  "500,000/mo",
				"retention": "90 days",
				"support":   "email",
				"users":     "unlimited",
			},
		},
		{
			Slug: "team", Name: "Team", Tagline: "Multi-seat. Shared configs.",
			PriceCents: 14900, // $149/mo
			Features: []string{
				"Everything in Pro",
				"SSO / SAML",
				"Team dashboards",
				"Shared configs",
				"Role-based access",
				"Priority support",
			},
			Limits: map[string]string{
				"requests":  "unlimited",
				"retention": "180 days",
				"support":   "priority",
				"users":     "unlimited",
			},
		},
		{
			Slug: "enterprise", Name: "Enterprise", Tagline: "Dedicated infrastructure. Custom SLAs.",
			PriceCents: 49900, // $499/mo
			Features: []string{
				"Everything in Team",
				"Dedicated infrastructure",
				"99.9% SLA",
				"1-year retention",
				"Dedicated support engineer",
				"Custom integrations",
				"On-prem option",
			},
			Limits: map[string]string{
				"requests":  "unlimited",
				"retention": "1 year",
				"support":   "dedicated",
				"users":     "unlimited",
			},
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
