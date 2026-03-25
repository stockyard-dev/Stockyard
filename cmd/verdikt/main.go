// Stockyard Verdikt — Your AI's AI.
//
// A meta-layer that independently evaluates every AI response your
// application produces. Learns what "good" looks like from user reactions.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/stockyard-dev/stockyard/internal/provider"
	"github.com/stockyard-dev/stockyard/internal/verdikt/judge"
	"github.com/stockyard-dev/stockyard/internal/verdikt/server"
)

var version = "dev"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "eval":
		cmdEval(os.Args[2:])
	case "serve":
		cmdServe(os.Args[2:])
	case "version":
		fmt.Printf("verdikt %s\n", version)
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Printf("Unknown command: %s\n", os.Args[1])
		os.Exit(1)
	}
}

func cmdEval(args []string) {
	fs := flag.NewFlagSet("eval", flag.ExitOnError)
	prompt := fs.String("prompt", "", "The prompt that was sent")
	response := fs.String("response", "", "The AI response to evaluate")
	model := fs.String("model", "", "Model that generated the response")
	criteria := fs.String("criteria", "", "Domain-specific criteria")
	fs.Parse(args)

	if *prompt == "" || *response == "" {
		fmt.Println("Usage: verdikt eval --prompt '...' --response '...'")
		os.Exit(1)
	}

	j := setupJudge()
	if *criteria != "" {
		j.SetCriteria(*criteria)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	eval, err := j.Evaluate(ctx, "cli", *prompt, *response, *model)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	data, _ := json.MarshalIndent(eval, "", "  ")
	fmt.Println(string(data))
}

func cmdServe(args []string) {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	port := fs.Int("port", 9300, "HTTP server port")
	criteria := fs.String("criteria", "", "Domain-specific evaluation criteria")
	fs.Parse(args)

	if envPort := os.Getenv("PORT"); envPort != "" {
		if p, err := strconv.Atoi(envPort); err == nil {
			*port = p
		}
	}

	j := setupJudge()
	if *criteria != "" {
		j.SetCriteria(*criteria)
	}

	fmt.Printf("\n  ⚖️ Stockyard Verdikt\n\n")
	fmt.Printf("    Dashboard: http://localhost:%d/ui\n", *port)
	fmt.Printf("    Evaluate:  POST http://localhost:%d/api/evaluate\n", *port)
	fmt.Printf("    React:     POST http://localhost:%d/api/react\n\n", *port)

	srv := server.New(server.Config{Port: *port, Judge: j})
	if err := srv.ListenAndServe(); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}

func setupJudge() *judge.Judge {
	key := ""
	for _, env := range []string{"VERDIKT_API_KEY", "OPENAI_API_KEY", "ANTHROPIC_API_KEY"} {
		if k := os.Getenv(env); k != "" {
			key = k
			break
		}
	}
	if key == "" {
		fmt.Println("Error: Need LLM key. Set OPENAI_API_KEY or VERDIKT_API_KEY.")
		os.Exit(1)
	}
	llm := provider.NewOpenAI(provider.ProviderConfig{APIKey: key})
	return judge.New(llm, "gpt-4o-mini")
}

func printUsage() {
	fmt.Println(`
  Stockyard Verdikt — Your AI's AI.

  Independently evaluates every AI response. Learns what "good" looks
  like from user reactions. A quality function that improves itself.

  Usage:
    verdikt eval [flags]        Evaluate a single AI response
    verdikt serve [flags]       Start evaluation server
    verdikt version             Show version

  Eval flags:
    --prompt <text>     The input prompt
    --response <text>   The AI response to judge
    --model <name>      Model that generated it
    --criteria <text>   Domain-specific criteria

  Dimensions scored: relevance, accuracy, coherence, safety, helpfulness

  Examples:
    verdikt eval --prompt "What is 2+2?" --response "The answer is 4."
    verdikt serve --port 9300
    curl -X POST localhost:9300/api/evaluate \
      -d '{"prompt":"...","response":"...","model":"gpt-4o"}'
    curl -X POST localhost:9300/api/react \
      -d '{"request_id":"...","type":"thumbs_up"}'`)
}
