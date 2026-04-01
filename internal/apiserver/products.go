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
		{Slug: "proxy", Name: "Chute", Tagline: "The proxy. 76 middleware modules, 40 providers, 400ns overhead.", Category: "app"},
		{Slug: "observe", Name: "Lookout", Tagline: "Request tracing, cost dashboards, anomaly detection.", Category: "app"},
		{Slug: "trust", Name: "Brand", Tagline: "Hash-chained audit ledger, policy engine, compliance evidence.", Category: "app"},
		{Slug: "studio", Name: "Tack Room", Tagline: "Prompt versioning, A/B testing, experiments, benchmarks.", Category: "app"},
		{Slug: "forge", Name: "Forge", Tagline: "DAG workflow engine, tools & sessions.", Category: "app"},
		{Slug: "exchange", Name: "Trading Post", Tagline: "Config pack marketplace & environment sync.", Category: "app"},
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

// ─── Focused Tool Plans ─────────────────────────────────────────────────────
// Standalone tools from the Stockyard family. Each has its own Free + Pro tier.
// License keys are Ed25519-signed and validated offline in each tool binary.

// ToolPlan represents a pricing plan for a standalone Stockyard tool.
type ToolPlan struct {
	Slug         string `json:"slug"`          // e.g. "corral"
	Name         string `json:"name"`          // e.g. "Corral Pro"
	Tool         string `json:"tool"`          // product slug for license issuance
	PriceCents   int    `json:"price_cents"`   // monthly
	AnnualCents  int    `json:"annual_cents"`  // annual
	FreeSummary  string `json:"free_summary"`  // what free includes
	ProSummary   string `json:"pro_summary"`   // what pro unlocks
	PageURL      string `json:"page_url"`
}

// ToolPlans returns all standalone tool pricing plans.
func ToolPlans() []ToolPlan {
	return []ToolPlan{
		{
			Slug:        "corral-pro",
			Name:        "Corral Pro",
			Tool:        "corral",
			PriceCents:  99,
			AnnualCents: 990,
			FreeSummary: "3 endpoints, 1,000 events/mo, 7-day retention",
			ProSummary:  "Unlimited endpoints, 90-day retention, retry, export, search",
			PageURL:     "https://stockyard.dev/corral/",
		},
		{
			Slug:        "gate-pro",
			Name:        "Gate Pro",
			Tool:        "gate",
			PriceCents:  299,
			AnnualCents: 2990,
			FreeSummary: "1 upstream, 5 users",
			ProSummary:  "Unlimited upstreams, users, per-route limits, IP lists, log export",
			PageURL:     "https://stockyard.dev/gate/",
		},
		{
			Slug:        "trough-pro",
			Name:        "Trough Pro",
			Tool:        "trough",
			PriceCents:  299,
			AnnualCents: 2990,
			FreeSummary: "1 service, 10,000 requests/mo, 7-day history",
			ProSummary:  "Unlimited services, anomaly detection, spend alerts, 90-day history",
			PageURL:     "https://stockyard.dev/trough/",
		},
		{
			Slug:        "fence-pro",
			Name:        "Fence Pro",
			Tool:        "fence",
			PriceCents:  499,
			AnnualCents: 4990,
			FreeSummary: "10 keys, 2 members, 2 vaults",
			ProSummary:  "Unlimited keys, members, RBAC, full audit trail, export",
			PageURL:     "https://stockyard.dev/fence/",
		},
		{
			Slug:        "brand-pro",
			Name:        "Brand Pro",
			Tool:        "brand",
			PriceCents:  499,
			AnnualCents: 4990,
			FreeSummary: "10,000 events/mo, 7-day retention",
			ProSummary:  "Unlimited events, 90-day retention, policy templates, signed bundles",
			PageURL:     "https://stockyard.dev/brand/",
		},
		{
			Slug:        "complete",
			Name:        "Stockyard Complete",
			Tool:        "complete",
			PriceCents:  2900,
			AnnualCents: 24900,
			FreeSummary: "Try any tool free",
			ProSummary:  "All 150 tools, unlimited Pro on everything",
			PageURL:     "https://stockyard.dev/complete/",
		},
	}
}

// ToolPlanBySlug returns a tool plan by slug (e.g. "corral-pro").
func ToolPlanBySlug(slug string) *ToolPlan {
	for _, p := range ToolPlans() {
		if p.Slug == slug {
			return &p
		}
	}
	return nil
}

// ToolPlanByTool returns the pro plan for a tool slug (e.g. "corral").
func ToolPlanByTool(tool string) *ToolPlan {
	for _, p := range ToolPlans() {
		if p.Tool == tool {
			return &p
		}
	}
	return nil
}
