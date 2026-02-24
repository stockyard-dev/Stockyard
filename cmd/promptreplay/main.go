// PromptReplay — "Record and replay every LLM call."
package main

import "github.com/stockyard-dev/stockyard/internal/engine"

var (
	version = "dev"
	commit  = ""
	date    = ""
)

func main() {
	engine.Boot(engine.ProductConfig{
		Name:    "PromptReplay",
		Product: "promptreplay",
		Version: version,
		Features: engine.Features{
			RequestLogging: true,
			FullBodyLog:    true,
		},
	})
}
