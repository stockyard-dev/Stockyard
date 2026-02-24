// IPFence — "IP allowlisting and geofencing for your LLM endpoints."
package main

import "github.com/stockyard-dev/stockyard/internal/engine"

var (
	version = "dev"
	commit  = ""
	date    = ""
)

func main() {
	engine.Boot(engine.ProductConfig{
		Name:    "IPFence",
		Product: "ipfence",
		Version: version,
		Features: engine.Features{
			IPFence:        true,
			RequestLogging: true,
			FullBodyLog:    true,
		},
	})
}
