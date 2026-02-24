// EvalGate — "Never ship a broken LLM response to your users."
package main

import "github.com/stockyard-dev/stockyard/internal/engine"

var (
	version = "dev"
	commit  = ""
	date    = ""
)

func main() {
	engine.Boot(engine.ProductConfig{
		Name:    "EvalGate",
		Product: "evalgate",
		Version: version,
		Features: engine.Features{
			EvalGate:       true,
			SpendTracking:  true,
			RequestLogging: true,
			FullBodyLog:    true,
		},
	})
}
