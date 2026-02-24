// KeyPool — "Stop hitting rate limits with a single API key."
package main

import "github.com/stockyard-dev/stockyard/internal/engine"

var (
	version = "dev"
	commit  = ""
	date    = ""
)

func main() {
	engine.Boot(engine.ProductConfig{
		Name:    "KeyPool",
		Product: "keypool",
		Version: version,
		Features: engine.Features{
			KeyPool:        true,
			SpendTracking:  true,
			RequestLogging: true,
		},
	})
}
