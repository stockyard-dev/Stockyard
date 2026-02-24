// IdleKill — "Kill runaway LLM requests burning money doing nothing."
package main

import "github.com/stockyard-dev/stockyard/internal/engine"

var (
	version = "dev"
	commit  = ""
	date    = ""
)

func main() {
	engine.Boot(engine.ProductConfig{
		Name:    "IdleKill",
		Product: "idlekill",
		Version: version,
		Features: engine.Features{
			IdleKill:       true,
			RequestLogging: true,
			FullBodyLog:    true,
		},
	})
}
