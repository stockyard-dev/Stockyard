// ImageProxy — Stockyard Phase 3 P2 product.
package main

import "github.com/stockyard-dev/stockyard/internal/engine"

var (
	version = "dev"
	commit  = ""
	date    = ""
)

func main() {
	engine.Boot(engine.ProductConfig{
		Name:    "ImageProxy",
		Product: "imageproxy",
		Version: version,
		Features: engine.Features{
			ImageProxy:     true,
			RequestLogging: true,
			FullBodyLog:    true,
		},
	})
}
