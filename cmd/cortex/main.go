// Stockyard Cortex — A second brain for your codebase.
//
// Usage:
//
//	cortex index github <owner/repo>    Index a GitHub repository
//	cortex ask <question>               Ask a question about your codebase
//	cortex sources                      List indexed sources
//	cortex stats                        Show knowledge base statistics
//	cortex serve [flags]                Start HTTP API
//	cortex version                      Show version
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/stockyard-dev/stockyard/internal/cortex/cstore"
	"github.com/stockyard-dev/stockyard/internal/cortex/embed"
	"github.com/stockyard-dev/stockyard/internal/cortex/ingest"
	"github.com/stockyard-dev/stockyard/internal/cortex/query"
	"github.com/stockyard-dev/stockyard/internal/cortex/server"
	"github.com/stockyard-dev/stockyard/internal/cortex/source"
	"github.com/stockyard-dev/stockyard/internal/provider"
)

var version = "dev"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	dataDir := cortexDir()
	os.MkdirAll(dataDir, 0755)

	switch os.Args[1] {
	case "index":
		cmdIndex(os.Args[2:])
	case "ask":
		if len(os.Args) < 3 {
			fmt.Println("Usage: cortex ask <question>")
			os.Exit(1)
		}
		cmdAsk(strings.Join(os.Args[2:], " "))
	case "sources":
		cmdSources()
	case "stats":
		cmdStats()
	case "serve":
		cmdServe(os.Args[2:])
	case "version":
		fmt.Printf("cortex %s\n", version)
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Printf("Unknown command: %s\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func cmdIndex(args []string) {
	if len(args) < 2 {
		fmt.Println("Usage: cortex index github <owner/repo>")
		fmt.Println("       cortex index github stockyard-dev/Stockyard")
		os.Exit(1)
	}

	sourceType := args[0]
	if sourceType != "github" {
		fmt.Printf("Unknown source type: %s (supported: github)\n", sourceType)
		os.Exit(1)
	}

	repo := args[1]
	parts := strings.SplitN(repo, "/", 2)
	if len(parts) != 2 {
		fmt.Println("Error: repo must be in owner/repo format")
		os.Exit(1)
	}
	owner, repoName := parts[0], parts[1]

	// Parse optional flags
	fs := flag.NewFlagSet("index", flag.ExitOnError)
	ghToken := fs.String("token", "", "GitHub token (or set GITHUB_TOKEN)")
	noCode := fs.Bool("no-code", false, "Skip code file indexing")
	commitLimit := fs.Int("commits", 200, "Max commits to index")
	prLimit := fs.Int("prs", 100, "Max PRs to index")
	issueLimit := fs.Int("issues", 100, "Max issues to index")
	fs.Parse(args[2:])

	// Resolve GitHub token
	token := *ghToken
	if token == "" {
		token = os.Getenv("GITHUB_TOKEN")
	}

	// Open database
	db, err := cstore.Open(filepath.Join(cortexDir(), "cortex.db"))
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	gh := source.NewGitHub(token)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	// Handle SIGINT gracefully
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Println("\nInterrupted. Saving progress...")
		cancel()
	}()

	cfg := ingest.DefaultConfig()
	cfg.IndexCode = !*noCode
	cfg.CommitLimit = *commitLimit
	cfg.PRLimit = *prLimit
	cfg.IssueLimit = *issueLimit

	fmt.Printf("\n  🧠 Cortex: Indexing %s/%s\n\n", owner, repoName)

	err = ingest.IndexGitHubRepo(ctx, gh, owner, repoName, db, cfg, func(p ingest.Progress) {
		switch p.Phase {
		case "complete":
			fmt.Printf("\r  ✓ Indexing complete: %s\n", p.Detail)
		default:
			if p.Total > 0 {
				fmt.Printf("\r  [%s] %d/%d  %s", p.Phase, p.Current, p.Total, truncate(p.Detail, 50))
			} else {
				fmt.Printf("\r  [%s] %s", p.Phase, p.Detail)
			}
		}
	})
	if err != nil {
		fmt.Printf("\n  Error: %v\n", err)
		os.Exit(1)
	}

	// Generate embeddings
	apiKey := resolveAPIKey()
	if apiKey == "" {
		fmt.Println("\n  ⚠ No API key found. Skipping embedding generation.")
		fmt.Println("    Set OPENAI_API_KEY or CORTEX_API_KEY to enable semantic search.")
		fmt.Println("    Run 'cortex index github " + repo + "' again after setting the key.")
		return
	}

	fmt.Println("\n  Generating embeddings...")
	embedder := embed.New(embed.Config{APIKey: apiKey})
	err = embedder.EmbedAllDocuments(ctx, db, cfg.MaxChunkTokens, cfg.OverlapTokens, func(current, total int) {
		fmt.Printf("\r  Embedding: %d/%d chunks", current, total)
	})
	if err != nil {
		fmt.Printf("\n  Warning: embedding failed: %v\n", err)
		fmt.Println("  Documents are indexed but semantic search won't work until embeddings are generated.")
	} else {
		fmt.Println()
	}

	// Print summary
	stats := db.Stats()
	fmt.Printf("\n  Knowledge base:\n")
	fmt.Printf("    Documents:     %d\n", stats.Documents)
	fmt.Printf("    Chunks:        %d\n", stats.Chunks)
	fmt.Printf("    Entities:      %d\n", stats.Entities)
	fmt.Printf("    Relationships: %d\n", stats.Relationships)
	fmt.Printf("\n  Ready. Ask questions with: cortex ask \"why do we use SQLite?\"\n\n")
}

func cmdAsk(question string) {
	db, err := cstore.Open(filepath.Join(cortexDir(), "cortex.db"))
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	stats := db.Stats()
	if stats.Documents == 0 {
		fmt.Println("  No documents indexed. Run 'cortex index github <owner/repo>' first.")
		os.Exit(1)
	}
	if stats.Chunks == 0 {
		fmt.Println("  Documents indexed but no embeddings generated.")
		fmt.Println("  Set OPENAI_API_KEY and re-run 'cortex index' to generate embeddings.")
		os.Exit(1)
	}

	apiKey := resolveAPIKey()
	if apiKey == "" {
		fmt.Println("  Error: No API key. Set OPENAI_API_KEY or CORTEX_API_KEY.")
		os.Exit(1)
	}

	embedder := embed.New(embed.Config{APIKey: apiKey})
	llm := provider.NewOpenAI(provider.ProviderConfig{APIKey: apiKey})

	model := os.Getenv("CORTEX_MODEL")
	if model == "" {
		model = "gpt-4o-mini"
	}

	engine := query.New(db, embedder, llm, model)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	fmt.Printf("\n  🧠 Thinking...\n\n")

	answer, err := engine.Ask(ctx, question, 8)
	if err != nil {
		fmt.Printf("  Error: %v\n", err)
		os.Exit(1)
	}

	// Print answer
	fmt.Println("  " + strings.ReplaceAll(answer.Response, "\n", "\n  "))
	fmt.Println()

	// Print sources
	if len(answer.Sources) > 0 {
		fmt.Println("  Sources:")
		for _, s := range answer.Sources {
			score := fmt.Sprintf("%.0f%%", s.Score*100)
			fmt.Printf("    [%s] %s (%s)", score, s.Title, s.Type)
			if s.URL != "" {
				fmt.Printf("\n           %s", s.URL)
			}
			fmt.Println()
		}
	}

	fmt.Printf("\n  %d chunks searched · $%.4f · %dms\n\n", answer.ChunksUsed, answer.Cost, answer.Latency.Milliseconds())
}

func cmdSources() {
	db, err := cstore.Open(filepath.Join(cortexDir(), "cortex.db"))
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	sources, err := db.ListSources()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	if len(sources) == 0 {
		fmt.Println("\n  No sources indexed. Run 'cortex index github <owner/repo>' to start.\n")
		return
	}

	fmt.Println("\n  Indexed Sources:")
	for _, s := range sources {
		synced := "never"
		if s.LastSyncedAt != nil {
			synced = s.LastSyncedAt.Format("2006-01-02 15:04")
		}
		docs := db.CountDocuments(s.ID)
		fmt.Printf("    %-12s %-30s %d docs  (synced: %s)\n", s.Type, s.Name, docs, synced)
	}
	fmt.Println()
}

func cmdStats() {
	db, err := cstore.Open(filepath.Join(cortexDir(), "cortex.db"))
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	stats := db.Stats()
	fmt.Printf("\n  🧠 Cortex Knowledge Base\n\n")
	fmt.Printf("    Sources:       %d\n", stats.Sources)
	fmt.Printf("    Documents:     %d\n", stats.Documents)
	fmt.Printf("    Chunks:        %d\n", stats.Chunks)
	fmt.Printf("    Entities:      %d\n", stats.Entities)
	fmt.Printf("    Relationships: %d\n", stats.Relationships)
	fmt.Printf("    Queries:       %d\n", stats.Queries)
	fmt.Println()
}

func cmdServe(args []string) {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	port := fs.Int("port", 9950, "HTTP server port")
	model := fs.String("model", "", "LLM model for answers (default: gpt-4o-mini)")
	fs.Parse(args)

	// Check for PORT env var (Railway)
	if envPort := os.Getenv("PORT"); envPort != "" {
		if p, err := strconv.Atoi(envPort); err == nil {
			*port = p
		}
	}

	apiKey := resolveAPIKey()
	ghToken := os.Getenv("GITHUB_TOKEN")
	mdl := *model
	if mdl == "" {
		mdl = os.Getenv("CORTEX_MODEL")
	}
	if mdl == "" {
		mdl = "gpt-4o-mini"
	}

	db, err := cstore.Open(filepath.Join(cortexDir(), "cortex.db"))
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	srv := server.New(server.Config{
		Port:        *port,
		DB:          db,
		APIKey:      apiKey,
		GitHubToken: ghToken,
		Model:       mdl,
	})

	fmt.Printf("🧠 Cortex server starting on :%d\n", *port)
	fmt.Printf("   Dashboard: http://localhost:%d/ui\n", *port)
	fmt.Printf("   API:       http://localhost:%d/api/health\n", *port)
	stats := db.Stats()
	fmt.Printf("   Knowledge: %d docs, %d chunks, %d entities\n", stats.Documents, stats.Chunks, stats.Entities)
	if apiKey == "" {
		fmt.Println("   ⚠ No API key — queries will fail. Set OPENAI_API_KEY or CORTEX_API_KEY.")
	}
	fmt.Println()

	if err := srv.ListenAndServe(); err != nil {
		fmt.Printf("Server error: %v\n", err)
		os.Exit(1)
	}
}

func cortexDir() string {
	if d := os.Getenv("CORTEX_DATA_DIR"); d != "" {
		return d
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".cortex")
}

func resolveAPIKey() string {
	if k := os.Getenv("CORTEX_API_KEY"); k != "" {
		return k
	}
	if k := os.Getenv("OPENAI_API_KEY"); k != "" {
		return k
	}
	return ""
}

func truncate(s string, n int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) > n {
		return s[:n] + "..."
	}
	return s
}

func printUsage() {
	fmt.Println(`
  Stockyard Cortex — A second brain for your codebase.

  Usage:
    cortex index github <owner/repo>   Index a GitHub repository
    cortex ask <question>              Ask a question about your codebase
    cortex sources                     List indexed sources
    cortex stats                       Show knowledge base statistics
    cortex serve [flags]               Start HTTP API
    cortex version                     Show version

  Index flags:
    --token <token>   GitHub token (or set GITHUB_TOKEN)
    --no-code         Skip code file indexing
    --commits <n>     Max commits to index (default: 200)
    --prs <n>         Max PRs to index (default: 100)
    --issues <n>      Max issues to index (default: 100)

  Environment:
    GITHUB_TOKEN      GitHub personal access token
    OPENAI_API_KEY    OpenAI API key (for embeddings + synthesis)
    CORTEX_API_KEY    Alternative API key env var
    CORTEX_MODEL      LLM model for answers (default: gpt-4o-mini)
    CORTEX_DATA_DIR   Data directory (default: ~/.cortex)

  Examples:
    cortex index github stockyard-dev/Stockyard
    cortex ask "why do we use SQLite instead of PostgreSQL?"
    cortex ask "who has worked on the middleware chain?"
    cortex ask "what did we try for the dashboard screenshot problem?"`)
}
