// LLMTap — "Stripe Dashboard for your LLM spend."
package main

import "github.com/stockyard-dev/stockyard/internal/engine"

var (
	version = "dev"
	commit  = ""
	date    = ""
)

func main() {
	engine.Boot(engine.ProductConfig{
		Name:    "LLMTap",
		Product: "llmtap",
		Version: version,
		Features: engine.Features{
			LLMTap:         true,
			SpendTracking:  true,
			RequestLogging: true,
		},
	})
}
