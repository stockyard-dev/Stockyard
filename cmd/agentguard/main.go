// AgentGuard — Stockyard Phase 3 P2 product.
package main

import "github.com/stockyard-dev/stockyard/internal/engine"

var (
	version = "dev"
	commit  = ""
	date    = ""
)

func main() {
	engine.Boot(engine.ProductConfig{
		Name:    "AgentGuard",
		Product: "agentguard",
		Version: version,
		Features: engine.Features{
			AgentGuard:     true,
			RequestLogging: true,
			FullBodyLog:    true,
		},
	})
}
