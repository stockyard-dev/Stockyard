// StreamSnap — "Capture, replay, and analyze every SSE stream."
package main

import "github.com/stockyard-dev/stockyard/internal/engine"

var (
	version = "dev"
	commit  = ""
	date    = ""
)

func main() {
	engine.Boot(engine.ProductConfig{
		Name:    "StreamSnap",
		Product: "streamsnap",
		Version: version,
		Features: engine.Features{
			StreamSnap:     true,
			SpendTracking:  true,
			RequestLogging: true,
		},
	})
}
