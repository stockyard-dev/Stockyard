// PromptPad — "Version, A/B test, and deploy prompts without code changes."
package main

import "github.com/stockyard-dev/stockyard/internal/engine"

var (
	version = "dev"
	commit  = ""
	date    = ""
)

func main() {
	engine.Boot(engine.ProductConfig{
		Name:    "PromptPad",
		Product: "promptpad",
		Version: version,
		Features: engine.Features{
			PromptPad:      true,
			RequestLogging: true,
			FullBodyLog:    true,
		},
	})
}
