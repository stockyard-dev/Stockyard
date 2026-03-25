// Stockyard Verdikt — Your AI's AI.
// Independently evaluates every response your AI produces and learns
// what "good" means for your specific use case.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"time"

	"github.com/stockyard-dev/stockyard/internal/provider"
	"github.com/stockyard-dev/stockyard/internal/verdikt/judge"
	"github.com/stockyard-dev/stockyard/internal/verdikt/learn"
	"github.com/stockyard-dev/stockyard/internal/verdikt/rubric"
	"github.com/stockyard-dev/stockyard/internal/verdikt/server"
	"github.com/stockyard-dev/stockyard/internal/verdikt/store"
)

var version = "dev"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}
	switch os.Args[1] {
	case "eval":
		if len(os.Args) < 3 {
			fmt.Println("Usage: verdikt eval <interaction.json>")
			os.Exit(1)
		}
		cmdEval(os.Args[2])
	case "batch":
		if len(os.Args) < 3 {
			fmt.Println("Usage: verdikt batch <dir>")
			os.Exit(1)
		}
		cmdBatch(os.Args[2])
	case "serve":
		cmdServe(os.Args[2:])
	case "version":
		fmt.Printf("verdikt %s\n", version)
	default:
		printUsage()
		os.Exit(1)
	}
}

func cmdEval(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	var interaction judge.Interaction
	if err := json.Unmarshal(data, &interaction); err != nil {
		fmt.Printf("Error parsing JSON: %v\n", err)
		os.Exit(1)
	}

	llmKey := resolveLLMKey()
	if llmKey == "" {
		fmt.Println("Error: Set OPENAI_API_KEY or VERDIKT_API_KEY")
		os.Exit(1)
	}

	llm := provider.NewOpenAI(provider.ProviderConfig{APIKey: llmKey})
	engine := judge.New(llm, "gpt-4o-mini", "", nil)

	// Load rubrics if available
	loadRubricsInto(engine)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	fmt.Printf("\n  Evaluating...\n\n")
	eval, err := engine.Evaluate(ctx, interaction)
	if err != nil {
		fmt.Printf("  Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("  Score:       %.0f%% (%s)\n", eval.Score*100, eval.Verdict)
	fmt.Printf("  Relevance:   %.0f%%\n", eval.Dimensions.Relevance*100)
	fmt.Printf("  Accuracy:    %.0f%%\n", eval.Dimensions.Accuracy*100)
	fmt.Printf("  Helpfulness: %.0f%%\n", eval.Dimensions.Helpfulness*100)
	fmt.Printf("  Safety:      %.0f%%\n", eval.Dimensions.Safety*100)
	fmt.Printf("  Tone:        %.0f%%\n", eval.Dimensions.Tone*100)
	if len(eval.Issues) > 0 {
		fmt.Printf("  Issues:      %v\n", eval.Issues)
	}
	fmt.Printf("  Latency:     %dms\n\n", eval.LatencyMs)
}

func cmdBatch(dir string) {
	llmKey := resolveLLMKey()
	if llmKey == "" {
		fmt.Println("Error: Set OPENAI_API_KEY or VERDIKT_API_KEY")
		os.Exit(1)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		fmt.Printf("Error reading directory: %v\n", err)
		os.Exit(1)
	}

	var interactions []judge.Interaction
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			fmt.Printf("  Warning: skipping %s: %v\n", e.Name(), err)
			continue
		}
		var interaction judge.Interaction
		if err := json.Unmarshal(data, &interaction); err != nil {
			fmt.Printf("  Warning: skipping %s: %v\n", e.Name(), err)
			continue
		}
		interactions = append(interactions, interaction)
	}

	if len(interactions) == 0 {
		fmt.Println("No valid .json files found in directory")
		os.Exit(1)
	}

	llm := provider.NewOpenAI(provider.ProviderConfig{APIKey: llmKey})
	engine := judge.New(llm, "gpt-4o-mini", "", nil)
	loadRubricsInto(engine)

	fmt.Printf("\n  Batch evaluating %d interactions...\n\n", len(interactions))

	type dimScores struct {
		Relevance, Accuracy, Helpfulness, Safety, Tone []float64
	}
	scores := make([]float64, 0, len(interactions))
	ds := dimScores{
		Relevance:   make([]float64, 0, len(interactions)),
		Accuracy:    make([]float64, 0, len(interactions)),
		Helpfulness: make([]float64, 0, len(interactions)),
		Safety:      make([]float64, 0, len(interactions)),
		Tone:        make([]float64, 0, len(interactions)),
	}

	for i, interaction := range interactions {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		eval, err := engine.Evaluate(ctx, interaction)
		cancel()
		if err != nil {
			fmt.Printf("  [%d/%d] Error: %v\n", i+1, len(interactions), err)
			continue
		}
		scores = append(scores, eval.Score)
		ds.Relevance = append(ds.Relevance, eval.Dimensions.Relevance)
		ds.Accuracy = append(ds.Accuracy, eval.Dimensions.Accuracy)
		ds.Helpfulness = append(ds.Helpfulness, eval.Dimensions.Helpfulness)
		ds.Safety = append(ds.Safety, eval.Dimensions.Safety)
		ds.Tone = append(ds.Tone, eval.Dimensions.Tone)
		fmt.Printf("  [%d/%d] %.0f%% %s\n", i+1, len(interactions), eval.Score*100, eval.Verdict)
	}

	fmt.Printf("\n  ── Aggregate Statistics ──\n\n")
	printDimStats("Overall", scores)
	printDimStats("Relevance", ds.Relevance)
	printDimStats("Accuracy", ds.Accuracy)
	printDimStats("Helpfulness", ds.Helpfulness)
	printDimStats("Safety", ds.Safety)
	printDimStats("Tone", ds.Tone)
	fmt.Println()
}

func printDimStats(name string, vals []float64) {
	if len(vals) == 0 {
		fmt.Printf("  %-14s no data\n", name+":")
		return
	}
	sorted := make([]float64, len(vals))
	copy(sorted, vals)
	sort.Float64s(sorted)
	var sum float64
	mn := math.MaxFloat64
	mx := -math.MaxFloat64
	for _, v := range sorted {
		sum += v
		if v < mn { mn = v }
		if v > mx { mx = v }
	}
	median := sorted[len(sorted)/2]
	if len(sorted)%2 == 0 && len(sorted) > 1 {
		median = (sorted[len(sorted)/2-1] + sorted[len(sorted)/2]) / 2
	}
	mean := sum / float64(len(sorted))
	fmt.Printf("  %-14s mean=%.0f%%  median=%.0f%%  min=%.0f%%  max=%.0f%%\n",
		name+":", mean*100, median*100, mn*100, mx*100)
}

func cmdServe(args []string) {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	port := fs.Int("port", 9300, "HTTP port")
	domain := fs.String("domain", "", "Domain context (customer-support, coding, etc.)")
	rubricDir := fs.String("rubrics", "", "Directory containing YAML rubric files")
	fs.Parse(args)
	if p := os.Getenv("PORT"); p != "" {
		if n, err := strconv.Atoi(p); err == nil {
			*port = n
		}
	}

	dataDir := os.Getenv("VERDIKT_DATA_DIR")
	if dataDir == "" {
		dataDir = "/tmp/verdikt"
	}
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		fmt.Printf("  Error creating data dir: %v\n", err)
		os.Exit(1)
	}

	db, err := store.Open(dataDir + "/verdikt.db")
	if err != nil {
		fmt.Printf("  Error opening database: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	llmKey := resolveLLMKey()
	if llmKey == "" {
		fmt.Println("  Warning: No LLM key — evaluations will fail.")
	}

	var engine *judge.Engine
	if llmKey != "" {
		llm := provider.NewOpenAI(provider.ProviderConfig{APIKey: llmKey})
		engine = judge.New(llm, "gpt-4o-mini", *domain, db)
	}

	// Load rubrics
	if engine != nil {
		if *rubricDir != "" {
			if rubrics, err := rubric.LoadRubrics(*rubricDir); err == nil && len(rubrics) > 0 {
				engine.SetRubrics(rubrics)
				fmt.Printf("    Rubrics:   %d loaded from %s\n", len(rubrics), *rubricDir)
			}
		} else {
			loadRubricsInto(engine)
		}
	}

	cal := learn.New(db)

	fmt.Printf("\n  Stockyard Verdikt\n\n")
	fmt.Printf("    Port:      :%d\n", *port)
	fmt.Printf("    Domain:    %s\n", *domain)
	fmt.Printf("    Data:      %s\n", dataDir)
	fmt.Printf("    Dashboard: http://localhost:%d/ui\n", *port)
	fmt.Printf("    Evaluate:  POST http://localhost:%d/api/evaluate\n", *port)
	fmt.Printf("    Batch:     POST http://localhost:%d/api/evaluate/batch\n\n", *port)

	srv := server.New(server.Config{Port: *port, Judge: engine, Calibrator: cal, Store: db})
	srv.ListenAndServe()
}

// loadRubricsInto loads rubrics from VERDIKT_RUBRICS_DIR env var if set.
func loadRubricsInto(engine *judge.Engine) {
	dir := os.Getenv("VERDIKT_RUBRICS_DIR")
	if dir == "" {
		return
	}
	rubrics, err := rubric.LoadRubrics(dir)
	if err != nil || len(rubrics) == 0 {
		return
	}
	engine.SetRubrics(rubrics)
}

func resolveLLMKey() string {
	for _, env := range []string{"VERDIKT_API_KEY", "OPENAI_API_KEY", "ANTHROPIC_API_KEY"} {
		if k := os.Getenv(env); k != "" {
			return k
		}
	}
	return ""
}

func printUsage() {
	fmt.Println(`
  Stockyard Verdikt — Your AI's AI.

  Independently evaluates every AI response. Learns what "good" means
  for your specific use case from user feedback signals.

  Usage:
    verdikt eval <interaction.json>   Evaluate a single interaction
    verdikt batch <dir>               Batch evaluate all .json files in a directory
    verdikt serve [--port N]          Start evaluation API + dashboard
    verdikt version                   Show version

  Serve flags:
    --port N        HTTP port (default 9300, or PORT env)
    --domain NAME   Domain context (customer-support, coding, medical, etc.)
    --rubrics DIR   Directory containing YAML rubric files

  Environment:
    VERDIKT_API_KEY       LLM API key (or OPENAI_API_KEY / ANTHROPIC_API_KEY)
    VERDIKT_DATA_DIR      Data directory (default /tmp/verdikt)
    VERDIKT_RUBRICS_DIR   Rubric YAML directory (used by eval/batch commands)

  Evaluation dimensions: relevance, accuracy, helpfulness, safety, tone
  Feedback loop: accepted/rephrased/escalated/abandoned -> calibration`)
}
