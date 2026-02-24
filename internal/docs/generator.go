package docs

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/stockyard-dev/stockyard/internal/apiserver"
)

// GenerateConfig holds settings for site generation.
type GenerateConfig struct {
	OutputDir string // e.g. "./docs-site"
}

// Generate builds the complete documentation site as static HTML.
func Generate(cfg GenerateConfig) error {
	out := cfg.OutputDir
	if out == "" {
		out = "./docs-site"
	}

	// Create output directory structure
	dirs := []string{
		out,
		filepath.Join(out, "install"),
		filepath.Join(out, "quickstart"),
		filepath.Join(out, "config"),
		filepath.Join(out, "api"),
		filepath.Join(out, "license"),
		filepath.Join(out, "deploy"),
		filepath.Join(out, "products"),
	}
	for _, d := range dirs {
		os.MkdirAll(d, 0755)
	}

	// Get product catalog
	products := apiserver.Catalog()

	// Create product subdirectories
	for _, p := range products {
		os.MkdirAll(filepath.Join(out, "products", p.Slug), 0755)
	}

	// Build sidebar
	sidebar := buildSidebar(products)

	// Core pages
	corePages := []Page{
		pageDocsHome(products),
		pageInstall(),
		pageQuickstart(),
		pageConfig(),
		pageAPIRef(),
		pageLicense(),
		pageDeploy(),
	}

	for _, p := range corePages {
		html := Render(p, sidebar)
		path := filepath.Join(out, p.Path)
		if err := os.WriteFile(path, []byte(html), 0644); err != nil {
			return fmt.Errorf("write %s: %w", p.Path, err)
		}
	}

	// Product index page
	indexPage := pageProductIndex(products)
	html := Render(indexPage, sidebar)
	if err := os.WriteFile(filepath.Join(out, "products", "index.html"), []byte(html), 0644); err != nil {
		return fmt.Errorf("write product index: %w", err)
	}

	// Individual product pages
	for _, prod := range products {
		page := generateProductPage(prod)
		html := Render(page, sidebar)
		path := filepath.Join(out, "products", prod.Slug, "index.html")
		if err := os.WriteFile(path, []byte(html), 0644); err != nil {
			return fmt.Errorf("write product %s: %w", prod.Slug, err)
		}
	}

	return nil
}

func buildSidebar(products []apiserver.Product) []SidebarSection {
	sections := []SidebarSection{
		{
			Title: "Getting Started",
			Items: []SidebarItem{
				{Title: "Introduction", Path: "index.html"},
				{Title: "Installation", Path: "install/index.html"},
				{Title: "Quickstart", Path: "quickstart/index.html"},
			},
		},
		{
			Title: "Guides",
			Items: []SidebarItem{
				{Title: "Deployment", Path: "deploy/index.html"},
			},
		},
		{
			Title: "Reference",
			Items: []SidebarItem{
				{Title: "API Reference", Path: "api/index.html"},
				{Title: "Configuration", Path: "config/index.html"},
				{Title: "Licensing", Path: "license/index.html"},
			},
		},
	}

	// Group products by category
	catMap := map[string][]apiserver.Product{}
	var catOrder []string
	for _, p := range products {
		if _, seen := catMap[p.Category]; !seen {
			catOrder = append(catOrder, p.Category)
		}
		catMap[p.Category] = append(catMap[p.Category], p)
	}

	// Sort categories by a fixed priority
	catPriority := map[string]int{
		"suite": 0, "cost": 1, "performance": 2, "reliability": 3,
		"security": 4, "safety": 5, "routing": 6, "quality": 7,
		"prompt": 8, "devex": 9, "observability": 10, "compliance": 11,
		"saas": 12, "usecase": 13, "rag": 14, "data": 15,
		"provider": 16, "workflow": 17, "niche": 18,
	}
	sort.Slice(catOrder, func(i, j int) bool {
		pi, oki := catPriority[catOrder[i]]
		pj, okj := catPriority[catOrder[j]]
		if !oki {
			pi = 99
		}
		if !okj {
			pj = 99
		}
		return pi < pj
	})

	// Add product sections to sidebar (only show built categories in sidebar to keep it manageable)
	// Show "Products" heading with a link to the index, then the first ~30 products grouped
	productItems := []SidebarItem{
		{Title: "All Products", Path: "products/index.html"},
	}

	// Add the built/original products individually (Phase 1+2), group the rest by category
	builtSlugs := map[string]bool{
		"stockyard": true, "costcap": true, "llmcache": true, "jsonguard": true,
		"routefall": true, "rateshield": true, "promptreplay": true,
		"keypool": true, "promptguard": true, "modelswitch": true,
		"evalgate": true, "usagepulse": true, "promptpad": true,
		"tokentrim": true, "batchqueue": true, "multicall": true,
		"streamsnap": true, "llmtap": true, "contextpack": true,
		"retrypilot": true,
	}

	for _, p := range products {
		if builtSlugs[p.Slug] {
			productItems = append(productItems, SidebarItem{
				Title: p.Name,
				Path:  "products/" + p.Slug + "/index.html",
			})
		}
	}

	sections = append(sections, SidebarSection{
		Title: "Products",
		Items: productItems,
	})

	// Add category group links for the remaining
	catLinks := []SidebarItem{}
	for _, cat := range catOrder {
		if cat == "suite" || cat == "cost" || cat == "performance" || cat == "reliability" ||
			cat == "security" || cat == "devex" || cat == "routing" || cat == "quality" ||
			cat == "prompt" || cat == "rag" {
			continue // Already covered by individual product links above
		}
		label := categoryLabel(cat)
		count := len(catMap[cat])
		catLinks = append(catLinks, SidebarItem{
			Title: fmt.Sprintf("%s (%d)", label, count),
			Path:  fmt.Sprintf("products/index.html#%s", cat),
		})
	}

	if len(catLinks) > 0 {
		sections = append(sections, SidebarSection{
			Title: "By Category",
			Items: catLinks,
		})
	}

	return sections
}

// pageDocsHome returns the documentation home page.
func pageDocsHome(products []apiserver.Product) Page {
	// Count categories
	cats := map[string]int{}
	for _, p := range products {
		cats[p.Category]++
	}

	return Page{
		Title:       "Documentation",
		Path:        "index.html",
		Section:     "Getting Started",
		Description: "Stockyard documentation. Install, configure, and deploy your LLM proxy infrastructure.",
		Content: fmt.Sprintf(`<h1>Stockyard Documentation</h1>
<p class="lead">Where LLM traffic gets sorted. 125 products. Single binary. No dependencies.</p>

<p>Stockyard is a shared Go proxy engine that sits between your application and LLM providers. It ships as 125 standalone products plus a unified suite, each a different middleware configuration of the same core binary.</p>

<h2>Get started</h2>

<table>
<tr><td><strong><a href="/docs/install/">Installation</a></strong></td><td>Download and install Stockyard via Homebrew, npm, curl, Docker, or Go.</td></tr>
<tr><td><strong><a href="/docs/quickstart/">Quickstart</a></strong></td><td>Proxy your first LLM request in 30 seconds.</td></tr>
<tr><td><strong><a href="/docs/config/">Configuration</a></strong></td><td>Complete YAML config reference for all products.</td></tr>
<tr><td><strong><a href="/docs/deploy/">Deployment</a></strong></td><td>Railway, Fly.io, Docker, Kubernetes, bare metal.</td></tr>
</table>

<h2>Reference</h2>

<table>
<tr><td><strong><a href="/docs/api/">API Reference</a></strong></td><td>OpenAI-compatible proxy endpoints, management API, SSE.</td></tr>
<tr><td><strong><a href="/docs/license/">Licensing</a></strong></td><td>Free tier, paid tiers, offline key validation.</td></tr>
<tr><td><strong><a href="/docs/products/">All Products</a></strong></td><td>%d products across %d categories.</td></tr>
</table>

<h2>Architecture</h2>

<p>Every Stockyard product is a Go middleware in the proxy chain. The full suite runs a 63-step middleware chain covering security, transformation, routing, quality validation, and logging. Each product can also run standalone as its own binary.</p>

<pre><code>Your App  --->  Stockyard Proxy  --->  LLM Provider
                    |                      (OpenAI, Anthropic,
                    |                       Gemini, Groq, Ollama,
                    v                       and 13+ more)
              [Middleware Chain]
              IPFence -> RateShield -> KeyPool -> CostCap
              -> Cache -> PromptGuard -> ... -> FallbackRouter
              -> RetryPilot -> EvalGate -> ComplianceLog</code></pre>

<p>The proxy is a single static Go binary with no CGO dependencies. All storage uses embedded SQLite. Dashboards are embedded Preact SPAs served from the binary itself. No external services required.</p>

<h2>Supported providers</h2>

<table>
<tr><td>OpenAI</td><td>Anthropic</td><td>Google Gemini</td><td>Groq</td></tr>
<tr><td>Ollama</td><td>Azure OpenAI</td><td>Amazon Bedrock</td><td>Google Vertex AI</td></tr>
<tr><td>Mistral</td><td>Cohere</td><td>DeepSeek</td><td>xAI / Grok</td></tr>
<tr><td>Together AI</td><td>Fireworks AI</td><td>Replicate</td><td>Perplexity</td></tr>
<tr><td>OpenRouter</td><td colspan="3">Any OpenAI-compatible endpoint</td></tr>
</table>`, len(products), len(cats)),
	}
}

// pageProductIndex returns the product catalog page for the docs site.
func pageProductIndex(products []apiserver.Product) Page {
	// Group by category
	catMap := map[string][]apiserver.Product{}
	var catOrder []string
	for _, p := range products {
		if _, seen := catMap[p.Category]; !seen {
			catOrder = append(catOrder, p.Category)
		}
		catMap[p.Category] = append(catMap[p.Category], p)
	}

	// Sort categories
	catPriority := map[string]int{
		"suite": 0, "cost": 1, "performance": 2, "reliability": 3,
		"security": 4, "safety": 5, "routing": 6, "quality": 7,
		"prompt": 8, "devex": 9, "observability": 10, "compliance": 11,
		"saas": 12, "usecase": 13, "rag": 14, "data": 15,
		"provider": 16, "workflow": 17, "niche": 18,
	}
	sort.Slice(catOrder, func(i, j int) bool {
		pi, oki := catPriority[catOrder[i]]
		pj, okj := catPriority[catOrder[j]]
		if !oki {
			pi = 99
		}
		if !okj {
			pj = 99
		}
		return pi < pj
	})

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf(`<h1>All Products</h1>
<p class="lead">%d products across %d categories. Each runs as a standalone binary or as part of the unified suite.</p>
`, len(products), len(catOrder)))

	// Category jump links
	sb.WriteString(`<div class="page-toc"><div class="page-toc-title">Categories</div><ul>`)
	for _, cat := range catOrder {
		count := len(catMap[cat])
		sb.WriteString(fmt.Sprintf(`<li><a href="#%s">%s (%d)</a></li>`, cat, esc(categoryLabel(cat)), count))
	}
	sb.WriteString(`</ul></div>`)

	// Each category
	for _, cat := range catOrder {
		prods := catMap[cat]
		sb.WriteString(fmt.Sprintf(`<h2 id="%s">%s</h2>`, cat, esc(categoryLabel(cat))))
		sb.WriteString(`<table><tr><th>Product</th><th>Description</th></tr>`)
		for _, p := range prods {
			sb.WriteString(fmt.Sprintf(`<tr><td><strong><a href="/docs/products/%s/">%s</a></strong></td><td>%s</td></tr>`,
				p.Slug, esc(p.Name), esc(p.Tagline)))
		}
		sb.WriteString(`</table>`)
	}

	return Page{
		Title:       "All Products",
		Path:        "products/index.html",
		Section:     "Products",
		Description: fmt.Sprintf("All %d Stockyard products. LLM infrastructure tools for cost, caching, routing, safety, compliance, and more.", len(products)),
		Content:     sb.String(),
	}
}
