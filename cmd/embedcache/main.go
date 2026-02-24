// EmbedCache — Never compute the same embedding twice.
// Part of the Stockyard LLM infrastructure suite.
package main

import "github.com/stockyard-dev/stockyard/internal/engine"

var version = "dev"

func main() {
	engine.Boot(engine.ProductConfig{
		Name:    "EmbedCache",
		Product: "embedcache",
		Version: version,
		Features: engine.Features{
			EmbedCache:     true,
			RequestLogging: true,
		},
	})
}
