// TraceLink — "Distributed tracing for multi-step LLM chains."
package main

import "github.com/stockyard-dev/stockyard/internal/engine"

var (
	version = "dev"
	commit  = ""
	date    = ""
)

func main() {
	engine.Boot(engine.ProductConfig{
		Name:    "TraceLink",
		Product: "tracelink",
		Version: version,
		Features: engine.Features{
			TraceLink:      true,
			RequestLogging: true,
		},
	})
}
