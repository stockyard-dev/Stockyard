// SecretScan — "Catch API keys and secrets leaking in requests and responses."
package main

import "github.com/stockyard-dev/stockyard/internal/engine"

var (
	version = "dev"
	commit  = ""
	date    = ""
)

func main() {
	engine.Boot(engine.ProductConfig{
		Name:    "SecretScan",
		Product: "secretscan",
		Version: version,
		Features: engine.Features{
			SecretScan:     true,
			RequestLogging: true,
			FullBodyLog:    true,
		},
	})
}
