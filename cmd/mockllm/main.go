// MockLLM — "Deterministic LLM responses for testing and CI/CD."
package main

import "github.com/stockyard-dev/stockyard/internal/engine"

var (
	version = "dev"
	commit  = ""
	date    = ""
)

func main() {
	engine.Boot(engine.ProductConfig{
		Name:    "MockLLM",
		Product: "mockllm",
		Version: version,
		Features: engine.Features{
			MockLLM:        true,
			RequestLogging: true,
			FullBodyLog:    true,
		},
	})
}
