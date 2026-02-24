// CostCap — "Never get a surprise LLM bill again."
package main

import "github.com/stockyard-dev/stockyard/internal/engine"

var (
	version = "dev"
	commit  = ""
	date    = ""
)

func main() {
	engine.Boot(engine.ProductConfig{
		Name:    "CostCap",
		Product: "costcap",
		Version: version,
		Features: engine.Features{
			SpendTracking:  true,
			SpendCaps:      true,
			Alerts:         true,
			RequestLogging: true,
			FullBodyLog:    false,
		},
	})
}
