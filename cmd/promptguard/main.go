// PromptGuard — "Stop PII leaks and prompt injection before they happen."
package main

import "github.com/stockyard-dev/stockyard/internal/engine"

var (
	version = "dev"
	commit  = ""
	date    = ""
)

func main() {
	engine.Boot(engine.ProductConfig{
		Name:    "PromptGuard",
		Product: "promptguard",
		Version: version,
		Features: engine.Features{
			PromptGuard:    true,
			RequestLogging: true,
			FullBodyLog:    true,
		},
	})
}
