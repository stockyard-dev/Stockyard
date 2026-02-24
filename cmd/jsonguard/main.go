// JSONGuard — "Guarantee valid JSON from any LLM."
package main

import "github.com/stockyard-dev/stockyard/internal/engine"

var (
	version = "dev"
	commit  = ""
	date    = ""
)

func main() {
	engine.Boot(engine.ProductConfig{
		Name:    "JSONGuard",
		Product: "jsonguard",
		Version: version,
		Features: engine.Features{
			Validation:     true,
			RequestLogging: true,
			FullBodyLog:    true,
		},
	})
}
