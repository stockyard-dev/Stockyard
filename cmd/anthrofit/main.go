// AnthroFit — Use Claude with OpenAI SDKs.
// Deep Anthropic compatibility — system messages, tool schemas, streaming format.
// Part of the Stockyard LLM infrastructure suite.
package main

import "github.com/stockyard-dev/stockyard/internal/engine"

var version = "dev"

func main() {
	engine.Boot(engine.ProductConfig{
		Name:    "AnthroFit",
		Product: "anthrofit",
		Version: version,
		Features: engine.Features{
			AnthroFit:      true,
			Failover:       true,  // Need failover for routing to Anthropic
			RequestLogging: true,
		},
	})
}
