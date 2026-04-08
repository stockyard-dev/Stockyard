package apiserver

import "strings"

// ─── Pricing Plans ─────────────────────────────────────────────────────
// Stockyard's real pricing model (post-Stripe-cleanup, late March 2026):
//   Proxy            — free forever, Apache 2.0, separate GitHub release
//   Bundle           — $7.99/mo, one small-business bundle, 14-day trial
//   All Tools        — $29.99/mo, every Stockyard tool, 14-day trial
//   Veterans         — free for life, license grant, verification required
//
// The live checkout for paid plans goes through /api/billing/stripe/checkout
// (internal/apps/billing/subscriptions.go) using the tier names "bundle"
// and "all_tools". This file's Plans()/PlanBySlug() are consumed by the
// informational endpoints /api/plans and /api/products.
//
// FOLLOW-UP: the older "individual"/"pro"/"team"/"enterprise" tier
// vocabulary is still load-bearing in license generation, webhook
// handling, upgrade prompts, and the legacy /api/checkout fallback.
// That cleanup is intentionally out of scope here — see the priority
// list in the most recent HANDOFF.

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
			Slug: "proxy", Name: "Proxy", Tagline: "Free LLM proxy. Self-hosted. Apache 2.0.",
			PriceCents: 0,
			Features: []string{
				"Self-hosted single binary",
				"All major LLM providers",
				"Unlimited requests",
				"Your data never leaves your machine",
				"Apache 2.0 — fully open source",
				"Community support",
			},
			Limits: map[string]string{
				"requests":  "unlimited",
				"retention": "unlimited",
				"support":   "community",
				"users":     "unlimited",
			},
		},
		{
			Slug: "bundle", Name: "Bundle", Tagline: "One small-business toolkit. Your choice of bundle.",
			PriceCents:  799,  // $7.99/mo
			AnnualCents: 7990, // $79.90/yr (10x monthly per pricing convention)
			Features: []string{
				"Pick one bundle for your line of work",
				"Every tool in that bundle",
				"Self-hosted, runs on your computer",
				"14-day free trial, cancel anytime",
				"Email support",
			},
			Limits: map[string]string{
				"bundles":   "1",
				"retention": "unlimited",
				"support":   "email",
				"users":     "unlimited",
			},
			StripePriceID: "price_1TK19YRkoFWvoLHJzDqhOCzi",
		},
		{
			Slug: "all_tools", Name: "All Tools", Tagline: "Every Stockyard tool. Every bundle.",
			PriceCents:  2999,  // $29.99/mo
			AnnualCents: 29990, // $299.90/yr
			Features: []string{
				"Every tool in the catalog",
				"Every bundle",
				"Use as many as you want",
				"Self-hosted, runs on your computer",
				"14-day free trial, cancel anytime",
				"Priority email support",
			},
			Limits: map[string]string{
				"bundles":   "unlimited",
				"tools":     "unlimited",
				"retention": "unlimited",
				"support":   "priority",
				"users":     "unlimited",
			},
			StripePriceID: "price_1TK19YRkoFWvoLHJrcFsCi6V",
		},
		{
			Slug: "veterans", Name: "Veterans", Tagline: "Free for life. Thank you for your service.",
			PriceCents: 0,
			Features: []string{
				"Same as All Tools",
				"No cost, no expiration",
				"Verification required at signup",
			},
			Limits: map[string]string{
				"bundles":   "unlimited",
				"tools":     "unlimited",
				"retention": "unlimited",
				"support":   "email",
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
		{Slug: "proxy", Name: "Chute", Tagline: "The proxy. 76 middleware modules, 16 providers, 400ns overhead.", Category: "app"},
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
// Checks hardcoded plans first, then falls back to the tool price table
// for dynamically registered tools.
func ToolPlanBySlug(slug string) *ToolPlan {
	for _, p := range ToolPlans() {
		if p.Slug == slug {
			return &p
		}
	}
	// Dynamic: if slug matches "{tool}-pro" and tool has prices, create a plan
	if strings.HasSuffix(slug, "-pro") {
		tool := strings.TrimSuffix(slug, "-pro")
		if isKnownTool(tool) {
			return &ToolPlan{
				Slug:        slug,
				Name:        tool + " Pro",
				Tool:        tool,
				PriceCents:  99, // default; actual price comes from Stripe price object
				AnnualCents: 990,
				PageURL:     "https://stockyard.dev/" + tool + "/",
			}
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
