package docs

import (
	"fmt"
	"strings"

	"github.com/stockyard-dev/stockyard/internal/apiserver"
)

// productDocData extends catalog data with documentation-specific content.
type productDocData struct {
	Product     apiserver.Product
	PainPoint   string
	Solution    string
	ConfigBlock string
	Features    []string
}

// generateProductPage creates a documentation page for a single product.
func generateProductPage(p apiserver.Product) Page {
	dd := enrichProduct(p)

	var sb strings.Builder

	// Title and lead
	sb.WriteString(fmt.Sprintf(`<h1>%s</h1>`, esc(p.Name)))
	sb.WriteString(fmt.Sprintf(`<p class="lead">%s</p>`, esc(p.Tagline)))

	// Tags
	sb.WriteString(`<p>`)
	sb.WriteString(fmt.Sprintf(`<span class="tag">%s</span>`, esc(categoryLabel(p.Category))))
	if p.IsSuite {
		sb.WriteString(`<span class="tag new">Suite</span>`)
	}
	sb.WriteString(`</p>`)

	// TOC
	sb.WriteString(`<div class="page-toc"><div class="page-toc-title">On this page</div><ul>`)
	sb.WriteString(`<li><a href="#overview">Overview</a></li>`)
	sb.WriteString(`<li><a href="#config">Configuration</a></li>`)
	sb.WriteString(`<li><a href="#features">Features</a></li>`)
	sb.WriteString(`<li><a href="#usage">Usage</a></li>`)
	sb.WriteString(`<li><a href="#api">API</a></li>`)
	sb.WriteString(`<li><a href="#pricing">Pricing</a></li>`)
	sb.WriteString(`</ul></div>`)

	// Overview
	sb.WriteString(`<h2 id="overview">Overview</h2>`)
	if dd.PainPoint != "" {
		sb.WriteString(fmt.Sprintf(`<h4>The Problem</h4><p>%s</p>`, dd.PainPoint))
	}
	if dd.Solution != "" {
		sb.WriteString(fmt.Sprintf(`<h4>The Solution</h4><p>%s</p>`, dd.Solution))
	}

	// Configuration
	sb.WriteString(`<h2 id="config">Configuration</h2>`)
	sb.WriteString(`<p>Add to your <code>config.yaml</code>:</p>`)
	sb.WriteString(fmt.Sprintf(`<pre><code>%s</code></pre>`, dd.ConfigBlock))

	if !p.IsSuite {
		sb.WriteString(fmt.Sprintf(`<p>Or run as a standalone binary:</p>`))
		sb.WriteString(fmt.Sprintf(`<div class="code-title">Terminal</div><pre><code><span class="comment"># Install</span>
<span class="cmd">brew install stockyard-dev/tap/%s</span>

<span class="comment"># Run</span>
<span class="cmd">%s --config config.yaml</span>

<span class="comment"># Or with npx</span>
<span class="cmd">npx @stockyard/%s</span></code></pre>`, p.Slug, p.Slug, p.Slug))
	}

	// Features
	sb.WriteString(`<h2 id="features">Features</h2>`)
	if len(dd.Features) > 0 {
		sb.WriteString(`<ul>`)
		for _, f := range dd.Features {
			sb.WriteString(fmt.Sprintf(`<li>%s</li>`, f))
		}
		sb.WriteString(`</ul>`)
	}

	// Usage
	sb.WriteString(`<h2 id="usage">Usage</h2>`)
	sb.WriteString(usageExample(p))

	// API
	sb.WriteString(`<h2 id="api">API</h2>`)
	sb.WriteString(apiSection(p))

	// Pricing
	sb.WriteString(`<h2 id="pricing">Pricing</h2>`)
	sb.WriteString(pricingSection(p))

	return Page{
		Title:       p.Name,
		Path:        "products/" + p.Slug + "/index.html",
		Section:     "Products",
		Description: fmt.Sprintf("%s: %s", p.Name, p.Tagline),
		Content:     sb.String(),
	}
}

func usageExample(p apiserver.Product) string {
	port := "4000"
	portMap := map[string]string{
		"costcap": "4100", "llmcache": "4200", "jsonguard": "4300",
		"routefall": "4400", "rateshield": "4500", "promptreplay": "4600",
		"keypool": "4700", "promptguard": "4710", "modelswitch": "4720",
		"evalgate": "4730", "usagepulse": "4740", "promptpad": "4800",
		"tokentrim": "4900", "batchqueue": "5000", "multicall": "5100",
		"streamsnap": "5200", "llmtap": "5300", "contextpack": "5400",
		"retrypilot": "5500",
	}
	if pp, ok := portMap[p.Slug]; ok {
		port = pp
	}

	return fmt.Sprintf(`<p>Once running, point your application at the proxy:</p>

<div class="code-title">Python</div>
<pre><code><span class="keyword">from</span> openai <span class="keyword">import</span> OpenAI

client = OpenAI(
    base_url=<span class="string">"http://localhost:%s/v1"</span>,
    api_key=<span class="string">"sk-your-key"</span>,
)

response = client.chat.completions.create(
    model=<span class="string">"gpt-4o"</span>,
    messages=[{<span class="string">"role"</span>: <span class="string">"user"</span>, <span class="string">"content"</span>: <span class="string">"Hello"</span>}]
)</code></pre>

<div class="code-title">curl</div>
<pre><code><span class="cmd">curl http://localhost:%s/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer sk-your-key" \
  -d '{"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}'</span></code></pre>

<p>Dashboard: <a href="http://localhost:%s/ui">http://localhost:%s/ui</a></p>`, port, port, port, port)
}

func apiSection(p apiserver.Product) string {
	return fmt.Sprintf(`<p>%s exposes the standard Stockyard API surface:</p>

<table>
<tr><th>Endpoint</th><th>Description</th></tr>
<tr><td><span class="method post">POST</span> <code>/v1/chat/completions</code></td><td>Proxied chat completions (with %s middleware)</td></tr>
<tr><td><span class="method post">POST</span> <code>/v1/completions</code></td><td>Proxied legacy completions</td></tr>
<tr><td><span class="method post">POST</span> <code>/v1/embeddings</code></td><td>Proxied embedding requests</td></tr>
<tr><td><span class="method get">GET</span> <code>/health</code></td><td>Health check</td></tr>
<tr><td><span class="method get">GET</span> <code>/api/stats</code></td><td>%s statistics</td></tr>
<tr><td><span class="method get">GET</span> <code>/ui</code></td><td>Embedded dashboard</td></tr>
<tr><td><span class="method get">GET</span> <code>/ui/events</code></td><td>Real-time SSE stream</td></tr>
</table>

<p>See the full <a href="/docs/api/">API Reference</a> for details.</p>`, esc(p.Name), esc(p.Name), esc(p.Name))
}

func pricingSection(p apiserver.Product) string {
	if p.IsSuite {
		return `<table>
<tr><th>Tier</th><th>Price</th><th>Includes</th></tr>
<tr><td>Free</td><td>$0</td><td>All 125 products, 1,000 requests/day</td></tr>
<tr><td>Starter</td><td>$19/mo</td><td>10,000 requests/day</td></tr>
<tr><td>Pro</td><td>$59/mo</td><td>100,000 requests/day</td></tr>
<tr><td>Team</td><td>$149/mo</td><td>Unlimited</td></tr>
</table>

<p><a href="/pricing/">View full pricing</a></p>`
	}

	return fmt.Sprintf(`<table>
<tr><th>Tier</th><th>Individual</th><th>Via Suite</th></tr>
<tr><td>Free</td><td>$0</td><td>$0</td></tr>
<tr><td>Starter</td><td>$9/mo</td><td>$19/mo (125 products)</td></tr>
<tr><td>Pro</td><td>$29/mo</td><td>$59/mo (125 products)</td></tr>
<tr><td>Team</td><td>$79/mo</td><td>$149/mo (125 products)</td></tr>
</table>

<div class="callout tip">
<span class="callout-label">Save with the suite</span>
Get %s and 124 other products for $59/mo with the <a href="/products/stockyard/">Stockyard Suite</a>.
</div>`, esc(p.Name))
}

// enrichProduct adds documentation-specific content based on product metadata.
func enrichProduct(p apiserver.Product) productDocData {
	dd := productDocData{Product: p}

	// Category-specific content
	switch p.Category {
	case "cost":
		dd.PainPoint = "LLM costs are unpredictable. A single misconfigured agent can burn through hundreds of dollars in minutes. You need visibility and control over every dollar spent."
	case "performance":
		dd.PainPoint = "LLM responses are slow and expensive. Identical requests hit the API every time, wasting tokens and adding latency. Your users are waiting."
	case "reliability":
		dd.PainPoint = "LLM providers go down. Rate limits hit at the worst moments. A single provider failure takes your entire application offline."
	case "security":
		dd.PainPoint = "Sensitive data leaks into LLM prompts and responses. API keys get pasted into chat messages. Secrets from training data appear in outputs."
	case "safety":
		dd.PainPoint = "LLM outputs are unpredictable. Harmful content, hallucinated facts, and off-topic responses erode user trust and create liability."
	case "devex":
		dd.PainPoint = "Developing against live LLM APIs is slow, expensive, and non-deterministic. Tests hit real APIs. CI breaks randomly. Debugging is guesswork."
	case "observability":
		dd.PainPoint = "You cannot improve what you cannot measure. LLM costs, latency, error rates, and quality metrics are invisible without proper instrumentation."
	case "compliance":
		dd.PainPoint = "Regulators want audit trails. GDPR requires data flow documentation. SOC2 auditors ask for proof of what your AI said six months ago."
	case "saas":
		dd.PainPoint = "Multi-tenant LLM applications need per-customer isolation, rate limits, spend caps, and usage-based billing. Building this from scratch takes months."
	case "prompt":
		dd.PainPoint = "Prompt engineering is trial and error. No version control, no quality checks, no approval workflow. One bad change breaks everything."
	case "usecase":
		dd.PainPoint = "General-purpose LLM infrastructure does not account for the specific requirements of your use case. Voice apps, chatbots, RAG pipelines, and code generation each have unique needs."
	case "data":
		dd.PainPoint = "Thousands of LLM interactions happen daily, but the data goes nowhere. No feedback capture. No export for fine-tuning. No way to close the improvement loop."
	case "provider":
		dd.PainPoint = "Different LLM providers have different API formats, quirks, and failure modes. Switching between them requires rewriting application code."
	case "workflow":
		dd.PainPoint = "Complex tasks require multi-step LLM workflows. Today this means custom Python scripts, cron jobs, and glue code for every pipeline."
	default:
		dd.PainPoint = "LLM infrastructure is complex and fragmented. Every team rebuilds the same patterns from scratch."
	}

	dd.Solution = fmt.Sprintf("%s solves this as a transparent proxy middleware. Drop it between your application and your LLM provider. No code changes required beyond updating the base URL.", p.Name)

	// Build config block
	dd.ConfigBlock = buildConfigBlock(p)

	// Build feature list
	dd.Features = buildFeatureList(p)

	return dd
}

func buildConfigBlock(p apiserver.Product) string {
	switch p.Slug {
	case "costcap":
		return `<span class="keyword">costcap</span>:
  enabled: true
  daily_limit_usd: 25.00
  monthly_limit_usd: 500.00
  alert_threshold: 0.80
  hard_cap: true
  per_model: true
  per_key: false`
	case "llmcache":
		return `<span class="keyword">cache</span>:
  enabled: true
  ttl: 3600
  max_size_mb: 500
  semantic: true
  semantic_threshold: 0.92`
	case "jsonguard":
		return `<span class="keyword">structured</span>:
  enabled: true
  max_retries: 3
  strict: true`
	case "routefall":
		return `<span class="keyword">fallback</span>:
  enabled: true
  strategy: priority
  health_check_interval: 30
  circuit_breaker:
    threshold: 5
    timeout: 60`
	case "rateshield":
		return `<span class="keyword">rate_limit</span>:
  enabled: true
  requests_per_minute: 60
  tokens_per_minute: 100000
  per_key: true
  burst: 10`
	case "promptreplay":
		return `<span class="keyword">replay</span>:
  enabled: true
  max_entries: 10000
  retention_days: 30`
	case "keypool":
		return `<span class="keyword">key_pool</span>:
  enabled: true
  strategy: round-robin  <span class="comment"># round-robin, least-used, random</span>
  keys:
    - ${OPENAI_KEY_1}
    - ${OPENAI_KEY_2}
    - ${OPENAI_KEY_3}
  auto_rotate_on_429: true`
	case "promptguard":
		return `<span class="keyword">prompt_guard</span>:
  enabled: true
  mode: redact  <span class="comment"># redact or block</span>
  patterns:
    - email
    - phone
    - ssn
    - credit_card
  injection_detection: true`
	case "stockyard":
		return `<span class="comment"># The suite enables all products in one binary</span>
<span class="keyword">port</span>: 4000

<span class="keyword">providers</span>:
  - name: openai
    base_url: https://api.openai.com/v1
    api_key: ${OPENAI_API_KEY}

<span class="comment"># Enable individual features as needed</span>
<span class="keyword">costcap</span>:
  enabled: true
  daily_limit_usd: 50.00

<span class="keyword">cache</span>:
  enabled: true
  ttl: 1800

<span class="keyword">rate_limit</span>:
  enabled: true
  requests_per_minute: 120`
	default:
		slug := p.Slug
		return fmt.Sprintf(`<span class="keyword">%s</span>:
  enabled: true`, slug)
	}
}

func buildFeatureList(p apiserver.Product) []string {
	base := []string{
		"Single static binary, no runtime dependencies",
		"Embedded web dashboard with real-time updates",
		"OpenAI-compatible API surface (works with any SDK)",
		"SSE streaming pass-through",
		"SQLite storage (no external database required)",
		"YAML configuration with environment variable interpolation",
	}

	switch p.Category {
	case "cost":
		return append([]string{
			"Real-time spend tracking per model, key, and time period",
			"Configurable soft and hard limits",
			"Dashboard with cost visualizations",
		}, base...)
	case "performance":
		return append([]string{
			"Sub-millisecond cache lookups",
			"Content-hash and semantic matching",
			"Cache hit rate metrics on dashboard",
		}, base...)
	case "reliability":
		return append([]string{
			"Automatic provider failover",
			"Circuit breaker with configurable thresholds",
			"Health check monitoring",
		}, base...)
	case "security":
		return append([]string{
			"Bidirectional scanning (requests and responses)",
			"Configurable pattern libraries",
			"Block or redact modes",
		}, base...)
	case "safety":
		return append([]string{
			"Output content filtering",
			"Configurable rule engines",
			"Moderation statistics on dashboard",
		}, base...)
	default:
		return base
	}
}

// categoryLabel returns a human-readable label for a product category.
func categoryLabel(cat string) string {
	labels := map[string]string{
		"suite": "Suite", "cost": "Cost Control", "performance": "Performance",
		"reliability": "Reliability", "security": "Security", "devex": "Developer Experience",
		"routing": "Routing", "quality": "Quality", "prompt": "Prompt Engineering",
		"observability": "Observability", "safety": "Safety", "compliance": "Compliance",
		"saas": "Multi-Tenant SaaS", "usecase": "Use Case", "data": "Data & Feedback",
		"provider": "Provider", "workflow": "Workflow", "niche": "Niche & Emerging",
		"structured": "Structured Data", "tools": "Function Calling", "auth": "Auth & API",
		"multimodal": "Multimodal", "sessions": "Sessions", "caching": "Caching",
		"testing": "Testing & QA", "streaming": "Streaming", "analytics": "Analytics",
		"devworkflow": "Developer Workflow", "specialized": "Specialized",
		"infrastructure": "Infrastructure", "rag": "RAG",
	}
	if l, ok := labels[cat]; ok {
		return l
	}
	return cat
}
