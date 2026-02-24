// BatchQueue — "Fire-and-forget LLM requests with automatic retries."
package main

import "github.com/stockyard-dev/stockyard/internal/engine"

var (
	version = "dev"
	commit  = ""
	date    = ""
)

func main() {
	engine.Boot(engine.ProductConfig{
		Name:    "BatchQueue",
		Product: "batchqueue",
		Version: version,
		Features: engine.Features{
			BatchQueue:     true,
			SpendTracking:  true,
			RequestLogging: true,
			FullBodyLog:    true,
		},
	})
}
