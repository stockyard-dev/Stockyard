// ToxicFilter — "Content moderation middleware for LLM outputs."
package main

import "github.com/stockyard-dev/stockyard/internal/engine"

var (
	version = "dev"
	commit  = ""
	date    = ""
)

func main() {
	engine.Boot(engine.ProductConfig{
		Name:    "ToxicFilter",
		Product: "toxicfilter",
		Version: version,
		Features: engine.Features{
			ToxicFilter:    true,
			RequestLogging: true,
			FullBodyLog:    true,
		},
	})
}
