// UsagePulse — "Know exactly who's spending what on your LLM calls."
package main

import "github.com/stockyard-dev/stockyard/internal/engine"

var (
	version = "dev"
	commit  = ""
	date    = ""
)

func main() {
	engine.Boot(engine.ProductConfig{
		Name:    "UsagePulse",
		Product: "usagepulse",
		Version: version,
		Features: engine.Features{
			UsagePulse:     true,
			SpendTracking:  true,
			SpendCaps:      true,
			RequestLogging: true,
		},
	})
}
