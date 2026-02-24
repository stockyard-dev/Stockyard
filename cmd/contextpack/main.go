// ContextPack — "RAG without the vector database."
package main

import "github.com/stockyard-dev/stockyard/internal/engine"

var (
	version = "dev"
	commit  = ""
	date    = ""
)

func main() {
	engine.Boot(engine.ProductConfig{
		Name:    "ContextPack",
		Product: "contextpack",
		Version: version,
		Features: engine.Features{
			ContextPack:    true,
			SpendTracking:  true,
			RequestLogging: true,
			FullBodyLog:    true,
		},
	})
}
