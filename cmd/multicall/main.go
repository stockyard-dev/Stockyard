// MultiCall — "Get the best answer by asking multiple models."
package main

import "github.com/stockyard-dev/stockyard/internal/engine"

var (
	version = "dev"
	commit  = ""
	date    = ""
)

func main() {
	engine.Boot(engine.ProductConfig{
		Name:    "MultiCall",
		Product: "multicall",
		Version: version,
		Features: engine.Features{
			MultiCall:      true,
			SpendTracking:  true,
			RequestLogging: true,
			FullBodyLog:    true,
		},
	})
}
