// TokenTrim — "Never blow a context window again."
package main

import "github.com/stockyard-dev/stockyard/internal/engine"

var (
	version = "dev"
	commit  = ""
	date    = ""
)

func main() {
	engine.Boot(engine.ProductConfig{
		Name:    "TokenTrim",
		Product: "tokentrim",
		Version: version,
		Features: engine.Features{
			TokenTrim:      true,
			SpendTracking:  true,
			RequestLogging: true,
		},
	})
}
