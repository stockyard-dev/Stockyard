package apiserver

// ─── Pricing Plans ─────────────────────────────────────────────────────
// Stockyard uses a 4-tier pricing model:
//   Free (self-hosted) → Pro ($29/mo cloud) → Team ($99/mo) → Enterprise ($299/mo)
// All tiers include the full platform (16 apps, 66 modules, all providers).
// Platform fees: 20% app store, 12% mesh, 20% knowledge, 2.5% billing.

// Plan represents a Stockyard pricing tier.
type Plan struct {
	Slug          string            `json:"slug"`
	Name          string            `json:"name"`
	Tagline       string            `json:"tagline"`
	PriceCents    int               `json:"price_cents"`            // 0 = free or custom
	AnnualCents   int               `json:"annual_cents,omitempty"` // annual price (2 months free)
	Custom        bool              `json:"custom"`                 // true = contact sales
	Features      []string          `json:"features"`
	Limits        map[string]string `json:"limits"`
	StripePriceID string            `json:"stripe_price_id,omitempty"`
}

// Plans returns the pricing tiers.
func Plans() []Plan {
	return []Plan{
		{
			Slug: "free", Name: "Free", Tagline: "Full platform. Self-hosted. Free forever.",
			PriceCents: 0,
			Features: []string{
				"All 16 apps",
				"66 middleware modules",
				"All 16 providers",
				"Unlimited requests",
				"SQLite storage",
				"Community support",
				"Publish apps, contribute to mesh, sell knowledge",
			},
			Limits: map[string]string{
				"requests":  "unlimited",
				"retention": "unlimited",
				"support":   "community",
				"users":     "unlimited",
			},
		},
		{
			Slug: "pro", Name: "Pro", Tagline: "Cloud-managed. Zero ops.",
			PriceCents:  2900,  // $29/mo
			AnnualCents: 29000, // $290/yr (save $58)
			Features: []string{
				"Everything in Free",
				"Managed cloud infrastructure",
				"Auto-scaling",
				"90-day audit retention",
				"Daily backups",
				"Email support",
				"Custom domain",
			},
			Limits: map[string]string{
				"requests":  "unlimited",
				"retention": "90 days",
				"support":   "email",
				"users":     "unlimited",
			},
		},
		{
			Slug: "team", Name: "Team", Tagline: "5 seats. RBAC. Compliance.",
			PriceCents:  9900,  // $99/mo
			AnnualCents: 99000, // $990/yr (save $198)
			Features: []string{
				"Everything in Pro",
				"5 seats included ($20/seat/mo additional)",
				"Role-based access control",
				"Team dashboards",
				"Shared configs",
				"Compliance export",
				"Priority support",
			},
			Limits: map[string]string{
				"requests":  "unlimited",
				"retention": "1 year",
				"support":   "priority",
				"users":     "5 included",
			},
		},
		{
			Slug: "enterprise", Name: "Enterprise", Tagline: "SSO. SLA. Federation.",
			PriceCents:  29900,  // $299/mo
			AnnualCents: 299000, // $2,990/yr (save $598)
			Features: []string{
				"Everything in Team",
				"Unlimited seats",
				"SSO / SAML",
				"Trust federation",
				"99.9% SLA with uptime guarantees",
				"Safety certification",
				"Unlimited audit retention",
				"Dedicated support channel",
			},
			Limits: map[string]string{
				"requests":  "unlimited",
				"retention": "unlimited",
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
		// Core apps
		{Slug: "proxy", Name: "Proxy", Tagline: "Core reverse-proxy, middleware chain, provider dispatch.", Category: "app"},
		{Slug: "observe", Name: "Observe", Tagline: "Tracing, cost attribution, alerts & anomaly detection.", Category: "app"},
		{Slug: "trust", Name: "Trust", Tagline: "Policy engine, audit ledger & compliance evidence.", Category: "app"},
		{Slug: "studio", Name: "Studio", Tagline: "Prompt templates, experiments & benchmarks.", Category: "app"},
		{Slug: "forge", Name: "Forge", Tagline: "DAG workflow engine, tools & sessions.", Category: "app"},
		{Slug: "exchange", Name: "Exchange", Tagline: "Config pack marketplace & environment sync.", Category: "app"},
		// Platform apps
		{Slug: "billing", Name: "Billing", Tagline: "Usage metering, per-customer billing, Stripe integration.", Category: "app"},
		{Slug: "team", Name: "Team", Tagline: "Multi-seat workspaces, RBAC, shared configs.", Category: "app"},
		{Slug: "memory", Name: "Memory", Tagline: "Conversation history, context windows, semantic recall.", Category: "app"},
		{Slug: "recall", Name: "Recall", Tagline: "Incident response, automated diagnostics, remediation.", Category: "app"},
		{Slug: "copilot", Name: "Copilot", Tagline: "Natural-language platform control and automation.", Category: "app"},
		{Slug: "appbuilder", Name: "App Builder", Tagline: "No-code AI app creation and marketplace.", Category: "app"},
		// Network apps
		{Slug: "knowledge", Name: "Knowledge", Tagline: "Expertise marketplace, domain knowledge bases.", Category: "app"},
		{Slug: "reputation", Name: "Reputation", Tagline: "Scoring, gamification, karma & community recognition.", Category: "app"},
		{Slug: "governance", Name: "Governance", Tagline: "Democratic framework, proposals, voting, compliance.", Category: "app"},
		{Slug: "marketing", Name: "Marketing", Tagline: "Campaign management, audience targeting, analytics.", Category: "app"},
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
