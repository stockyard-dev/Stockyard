package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
	"strings"

	"github.com/stockyard-dev/stockyard/internal/provider"
)

var (version = "dev"; commit = ""; date = "")

// Stockyard license tier pricing (monthly USD).
var tierPricing = map[string]float64{
	"community":  0,
	"individual": 9.99,
	"pro":        49.00,
	"team":       149.00,
	"enterprise": 499.00,
}

// Tier request limits (0 = unlimited).
var tierRequestLimits = map[string]int{
	"community":  1_000,
	"individual": 10_000,
	"pro":        0,
	"team":       0,
	"enterprise": 0,
}

// Scenario defines a usage scenario for profit calculation.
type Scenario struct {
	Name             string            `json:"name"`
	Tier             string            `json:"tier"`
	MonthlyRequests  int               `json:"monthly_requests"`
	AvgInputTokens   int               `json:"avg_input_tokens"`
	AvgOutputTokens  int               `json:"avg_output_tokens"`
	ModelMix         map[string]float64 `json:"model_mix"`          // model → percentage (0-1)
	MarkupPct        float64           `json:"markup_pct"`          // customer markup % (e.g. 20 = 20%)
	TeamSeats        int               `json:"team_seats"`          // extra seats beyond 5 included (Team tier)
	CacheHitRate     float64           `json:"cache_hit_rate"`      // 0-1, reduces provider costs
	InfraMonthlyUSD  float64           `json:"infra_monthly_usd"`   // server/hosting costs
}

// ProfitReport is the output of a profit calculation.
type ProfitReport struct {
	Scenario          string             `json:"scenario"`
	Tier              string             `json:"tier"`
	MonthlyRequests   int                `json:"monthly_requests"`

	// Revenue
	LicenseRevenue    float64            `json:"license_revenue"`
	SeatRevenue       float64            `json:"seat_revenue"`
	MarkupRevenue     float64            `json:"markup_revenue"`
	TotalRevenue      float64            `json:"total_revenue"`

	// Costs
	ProviderCost      float64            `json:"provider_cost"`
	InfraCost         float64            `json:"infra_cost"`
	StockyardLicense  float64            `json:"stockyard_license"`
	TotalCost         float64            `json:"total_cost"`

	// Profit
	GrossProfit       float64            `json:"gross_profit"`
	GrossMargin       float64            `json:"gross_margin_pct"`
	NetProfit         float64            `json:"net_profit"`
	NetMargin         float64            `json:"net_margin_pct"`

	// Per-model breakdown
	ModelBreakdown    []ModelCost        `json:"model_breakdown"`

	// Unit economics
	CostPerRequest    float64            `json:"cost_per_request"`
	RevenuePerRequest float64            `json:"revenue_per_request"`
	ProfitPerRequest  float64            `json:"profit_per_request"`
}

// ModelCost shows cost breakdown for a single model.
type ModelCost struct {
	Model      string  `json:"model"`
	Provider   string  `json:"provider"`
	Requests   int     `json:"requests"`
	InputCost  float64 `json:"input_cost"`
	OutputCost float64 `json:"output_cost"`
	TotalCost  float64 `json:"total_cost"`
	Share      float64 `json:"share_pct"`
}

func main() {
	if len(os.Args) > 1 && (os.Args[1] == "--version" || os.Args[1] == "-v") {
		fmt.Printf("profitcalc %s\n", version)
		os.Exit(0)
	}

	if len(os.Args) > 1 && (os.Args[1] == "--help" || os.Args[1] == "-h") {
		printUsage()
		os.Exit(0)
	}

	if len(os.Args) > 1 && os.Args[1] == "--models" {
		printModels()
		os.Exit(0)
	}

	// If a JSON file is provided, load scenarios from it
	if len(os.Args) > 1 {
		data, err := os.ReadFile(os.Args[1])
		if err != nil {
			fmt.Fprintf(os.Stderr, "error reading %s: %v\n", os.Args[1], err)
			os.Exit(1)
		}
		var scenarios []Scenario
		if err := json.Unmarshal(data, &scenarios); err != nil {
			// Try single scenario
			var single Scenario
			if err2 := json.Unmarshal(data, &single); err2 != nil {
				fmt.Fprintf(os.Stderr, "error parsing JSON: %v\n", err)
				os.Exit(1)
			}
			scenarios = []Scenario{single}
		}
		for _, s := range scenarios {
			report := calculate(s)
			printReport(report)
		}
		os.Exit(0)
	}

	// Default: run built-in example scenarios
	scenarios := exampleScenarios()
	fmt.Println("═══════════════════════════════════════════════════════════════════")
	fmt.Println("  Stockyard Expected Profit Calculator")
	fmt.Println("  Run with --help for usage, or pass a JSON scenario file")
	fmt.Println("═══════════════════════════════════════════════════════════════════")

	for _, s := range scenarios {
		report := calculate(s)
		printReport(report)
	}

	printComparison(scenarios)
}

func calculate(s Scenario) ProfitReport {
	r := ProfitReport{
		Scenario:        s.Name,
		Tier:            s.Tier,
		MonthlyRequests: s.MonthlyRequests,
	}

	// Enforce tier request limits
	limit := tierRequestLimits[s.Tier]
	effectiveRequests := s.MonthlyRequests
	if limit > 0 && effectiveRequests > limit {
		effectiveRequests = limit
	}

	// Revenue: what the operator charges their customers
	// The Stockyard license is a cost to the operator, not revenue from customers.
	// Revenue comes from marking up LLM API access to end customers.

	// Provider costs (what the operator pays LLM providers)
	cacheMultiplier := 1.0 - s.CacheHitRate
	for model, share := range s.ModelMix {
		requests := int(math.Round(float64(effectiveRequests) * share))
		if requests == 0 {
			continue
		}

		pricing, ok := provider.GetPricing(model)
		if !ok {
			pricing = provider.ModelPricing{Provider: "unknown", Input: 0, Output: 0}
		}

		inputCost := float64(requests) * float64(s.AvgInputTokens) * pricing.Input / 1_000_000 * cacheMultiplier
		outputCost := float64(requests) * float64(s.AvgOutputTokens) * pricing.Output / 1_000_000 * cacheMultiplier
		totalCost := inputCost + outputCost

		r.ModelBreakdown = append(r.ModelBreakdown, ModelCost{
			Model:      model,
			Provider:   pricing.Provider,
			Requests:   requests,
			InputCost:  inputCost,
			OutputCost: outputCost,
			TotalCost:  totalCost,
			Share:      share * 100,
		})
		r.ProviderCost += totalCost
	}

	// Sort breakdown by cost descending
	sort.Slice(r.ModelBreakdown, func(i, j int) bool {
		return r.ModelBreakdown[i].TotalCost > r.ModelBreakdown[j].TotalCost
	})

	// Revenue from markup on provider costs
	if s.MarkupPct > 0 {
		r.MarkupRevenue = r.ProviderCost * (s.MarkupPct / 100)
	}

	// Seat revenue (Team tier: extra seats beyond 5 at $25/seat/month)
	if s.Tier == "team" && s.TeamSeats > 0 {
		r.SeatRevenue = float64(s.TeamSeats) * 25.0
	}

	// License revenue is what the operator charges their customers, modeled as
	// a flat monthly fee the operator passes through or marks up
	r.LicenseRevenue = 0 // operator sets this; we model markup revenue instead

	r.TotalRevenue = r.MarkupRevenue + r.SeatRevenue + r.ProviderCost // passthrough + markup

	// Costs
	r.StockyardLicense = tierPricing[s.Tier]
	if s.Tier == "team" && s.TeamSeats > 0 {
		r.StockyardLicense += float64(s.TeamSeats) * 25.0
	}
	r.InfraCost = s.InfraMonthlyUSD
	r.TotalCost = r.ProviderCost + r.InfraCost + r.StockyardLicense

	// Profit
	r.GrossProfit = r.TotalRevenue - r.ProviderCost
	if r.TotalRevenue > 0 {
		r.GrossMargin = (r.GrossProfit / r.TotalRevenue) * 100
	}
	r.NetProfit = r.TotalRevenue - r.TotalCost
	if r.TotalRevenue > 0 {
		r.NetMargin = (r.NetProfit / r.TotalRevenue) * 100
	}

	// Unit economics
	if effectiveRequests > 0 {
		r.CostPerRequest = r.TotalCost / float64(effectiveRequests)
		r.RevenuePerRequest = r.TotalRevenue / float64(effectiveRequests)
		r.ProfitPerRequest = r.NetProfit / float64(effectiveRequests)
	}

	return r
}

func printReport(r ProfitReport) {
	fmt.Println()
	fmt.Printf("┌─── %s (%s tier) ───\n", r.Scenario, r.Tier)
	fmt.Printf("│  %d requests/month\n", r.MonthlyRequests)
	fmt.Println("│")

	// Model breakdown
	if len(r.ModelBreakdown) > 0 {
		fmt.Println("│  Model Breakdown:")
		for _, m := range r.ModelBreakdown {
			fmt.Printf("│    %-45s %5.0f%% │ %6d reqs │ $%10.2f\n",
				m.Model+" ("+m.Provider+")", m.Share, m.Requests, m.TotalCost)
		}
		fmt.Println("│")
	}

	// Revenue
	fmt.Println("│  Revenue:")
	fmt.Printf("│    Provider passthrough:    $%10.2f\n", r.ProviderCost)
	fmt.Printf("│    Markup revenue:          $%10.2f\n", r.MarkupRevenue)
	if r.SeatRevenue > 0 {
		fmt.Printf("│    Seat revenue:            $%10.2f\n", r.SeatRevenue)
	}
	fmt.Printf("│    ─────────────────────────────────────\n")
	fmt.Printf("│    Total revenue:           $%10.2f\n", r.TotalRevenue)
	fmt.Println("│")

	// Costs
	fmt.Println("│  Costs:")
	fmt.Printf("│    LLM provider costs:      $%10.2f\n", r.ProviderCost)
	fmt.Printf("│    Stockyard license:       $%10.2f\n", r.StockyardLicense)
	if r.InfraCost > 0 {
		fmt.Printf("│    Infrastructure:          $%10.2f\n", r.InfraCost)
	}
	fmt.Printf("│    ─────────────────────────────────────\n")
	fmt.Printf("│    Total costs:             $%10.2f\n", r.TotalCost)
	fmt.Println("│")

	// Profit
	fmt.Println("│  Profit:")
	fmt.Printf("│    Gross profit:            $%10.2f  (%.1f%% margin)\n", r.GrossProfit, r.GrossMargin)
	fmt.Printf("│    Net profit:              $%10.2f  (%.1f%% margin)\n", r.NetProfit, r.NetMargin)
	fmt.Println("│")

	// Unit economics
	fmt.Println("│  Unit Economics (per request):")
	fmt.Printf("│    Cost:     $%.6f\n", r.CostPerRequest)
	fmt.Printf("│    Revenue:  $%.6f\n", r.RevenuePerRequest)
	fmt.Printf("│    Profit:   $%.6f\n", r.ProfitPerRequest)

	fmt.Println("└───────────────────────────────────────────────────────")
}

func printComparison(scenarios []Scenario) {
	fmt.Println()
	fmt.Println("═══════════════════════════════════════════════════════════════════")
	fmt.Println("  Side-by-Side Comparison")
	fmt.Println("═══════════════════════════════════════════════════════════════════")
	fmt.Println()

	// Header
	fmt.Printf("  %-25s", "")
	for _, s := range scenarios {
		name := s.Name
		if len(name) > 18 {
			name = name[:18]
		}
		fmt.Printf("  %18s", name)
	}
	fmt.Println()
	fmt.Printf("  %-25s", strings.Repeat("─", 25))
	for range scenarios {
		fmt.Printf("  %s", strings.Repeat("─", 18))
	}
	fmt.Println()

	reports := make([]ProfitReport, len(scenarios))
	for i, s := range scenarios {
		reports[i] = calculate(s)
	}

	printRow := func(label string, vals []float64, format string) {
		fmt.Printf("  %-25s", label)
		for _, v := range vals {
			fmt.Printf(fmt.Sprintf("  %s", format), v)
		}
		fmt.Println()
	}

	providerCosts := make([]float64, len(reports))
	stockyardCosts := make([]float64, len(reports))
	totalRevenue := make([]float64, len(reports))
	totalCosts := make([]float64, len(reports))
	netProfit := make([]float64, len(reports))
	netMargin := make([]float64, len(reports))
	costPerReq := make([]float64, len(reports))
	profitPerReq := make([]float64, len(reports))

	for i, r := range reports {
		providerCosts[i] = r.ProviderCost
		stockyardCosts[i] = r.StockyardLicense
		totalRevenue[i] = r.TotalRevenue
		totalCosts[i] = r.TotalCost
		netProfit[i] = r.NetProfit
		netMargin[i] = r.NetMargin
		costPerReq[i] = r.CostPerRequest
		profitPerReq[i] = r.ProfitPerRequest
	}

	printRow("Provider costs", providerCosts, "%18.2f")
	printRow("Stockyard license", stockyardCosts, "%18.2f")
	printRow("Total revenue", totalRevenue, "%18.2f")
	printRow("Total costs", totalCosts, "%18.2f")
	fmt.Printf("  %-25s", strings.Repeat("─", 25))
	for range scenarios {
		fmt.Printf("  %s", strings.Repeat("─", 18))
	}
	fmt.Println()
	printRow("Net profit", netProfit, "%18.2f")
	printRow("Net margin %", netMargin, "%17.1f%%")
	printRow("Cost/request", costPerReq, "%18.6f")
	printRow("Profit/request", profitPerReq, "%18.6f")

	fmt.Println()
}

func exampleScenarios() []Scenario {
	return []Scenario{
		{
			Name:            "Solo Dev (Free)",
			Tier:            "community",
			MonthlyRequests: 800,
			AvgInputTokens:  500,
			AvgOutputTokens: 300,
			ModelMix: map[string]float64{
				"gpt-4o-mini": 0.7,
				"gpt-4o":      0.3,
			},
			MarkupPct: 0,
		},
		{
			Name:            "Startup (Pro)",
			Tier:            "pro",
			MonthlyRequests: 50_000,
			AvgInputTokens:  800,
			AvgOutputTokens: 500,
			ModelMix: map[string]float64{
				"gpt-4o":                         0.40,
				"claude-sonnet-4-5-20250929":      0.30,
				"gemini-2.0-flash":               0.20,
				"claude-haiku-4-5-20251001":       0.10,
			},
			MarkupPct:       25,
			CacheHitRate:    0.15,
			InfraMonthlyUSD: 20,
		},
		{
			Name:            "SaaS Platform (Team)",
			Tier:            "team",
			MonthlyRequests: 500_000,
			AvgInputTokens:  1200,
			AvgOutputTokens: 800,
			ModelMix: map[string]float64{
				"gpt-4o":                         0.25,
				"claude-sonnet-4-5-20250929":      0.25,
				"gpt-4o-mini":                    0.20,
				"gemini-2.5-pro":                 0.15,
				"deepseek-chat":                  0.15,
			},
			MarkupPct:       30,
			TeamSeats:       5,
			CacheHitRate:    0.20,
			InfraMonthlyUSD: 50,
		},
		{
			Name:            "Enterprise (High Vol)",
			Tier:            "enterprise",
			MonthlyRequests: 2_000_000,
			AvgInputTokens:  1500,
			AvgOutputTokens: 1000,
			ModelMix: map[string]float64{
				"gpt-4o":                         0.20,
				"claude-sonnet-4-5-20250929":      0.20,
				"gpt-4o-mini":                    0.25,
				"gemini-2.0-flash":               0.20,
				"deepseek-chat":                  0.15,
			},
			MarkupPct:       20,
			CacheHitRate:    0.25,
			InfraMonthlyUSD: 200,
		},
	}
}

func printModels() {
	fmt.Println("Available models and pricing (USD per 1M tokens):")
	fmt.Println()
	fmt.Printf("  %-50s  %10s  %10s  %s\n", "Model", "Input", "Output", "Provider")
	fmt.Printf("  %-50s  %10s  %10s  %s\n", strings.Repeat("─", 50), strings.Repeat("─", 10), strings.Repeat("─", 10), strings.Repeat("─", 12))

	// Use GetPricing with known model names to list all
	models := []string{
		"gpt-4o", "gpt-4o-mini", "gpt-4-turbo", "gpt-4.1", "gpt-4.1-mini", "gpt-4.1-nano",
		"gpt-3.5-turbo", "o1", "o1-mini", "o3", "o3-mini", "o4-mini",
		"text-embedding-3-small", "text-embedding-3-large",
		"claude-opus-4-6", "claude-sonnet-4-5-20250929", "claude-haiku-4-5-20251001",
		"gemini-2.0-flash", "gemini-2.0-pro", "gemini-2.5-pro", "gemini-2.5-flash",
		"gemini-1.5-flash", "gemini-1.5-pro",
		"llama-3.3-70b-versatile", "llama-3.1-8b-instant", "mixtral-8x7b-32768", "gemma2-9b-it",
		"mistral-large-latest", "mistral-medium-latest", "mistral-small-latest",
		"codestral-latest", "open-mistral-nemo", "pixtral-large-latest",
		"deepseek-chat", "deepseek-reasoner",
		"command-r-plus", "command-r", "command-a",
		"sonar-pro", "sonar",
		"grok-2", "grok-3", "grok-3-mini",
	}

	for _, m := range models {
		if p, ok := provider.GetPricing(m); ok {
			fmt.Printf("  %-50s  $%9.3f  $%9.3f  %s\n", m, p.Input, p.Output, p.Provider)
		}
	}
}

func printUsage() {
	fmt.Println(`Stockyard Expected Profit Calculator

Usage:
  profitcalc                    Run with built-in example scenarios
  profitcalc <scenario.json>    Calculate from a JSON scenario file
  profitcalc --models           List all models and their pricing
  profitcalc --help             Show this help

Scenario JSON format (single or array):

  {
    "name": "My App",
    "tier": "pro",
    "monthly_requests": 100000,
    "avg_input_tokens": 800,
    "avg_output_tokens": 500,
    "model_mix": {
      "gpt-4o": 0.5,
      "claude-sonnet-4-5-20250929": 0.3,
      "gemini-2.0-flash": 0.2
    },
    "markup_pct": 25,
    "cache_hit_rate": 0.15,
    "infra_monthly_usd": 20,
    "team_seats": 0
  }

Fields:
  name               Scenario label
  tier               Stockyard tier: community, individual, pro, team, enterprise
  monthly_requests   Expected requests per month
  avg_input_tokens   Average input tokens per request
  avg_output_tokens  Average output tokens per request
  model_mix          Map of model name → traffic share (0.0-1.0, must sum to 1.0)
  markup_pct         Percentage markup on provider costs charged to your customers
  cache_hit_rate     Fraction of requests served from cache (0.0-1.0)
  infra_monthly_usd  Monthly infrastructure/hosting costs
  team_seats         Extra team seats beyond 5 included (Team tier only, $25/seat)

Tier pricing:
  community     $0/mo     (1,000 req limit, 3 providers, 20 modules)
  individual    $9.99/mo  (10,000 req limit, all providers, all modules)
  pro           $49/mo    (unlimited requests)
  team          $149/mo   (unlimited, 5 seats included + $25/extra seat)
  enterprise    $499/mo   (unlimited, unlimited seats)`)
}
