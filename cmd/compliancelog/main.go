// ComplianceLog — "Immutable audit trail for every LLM interaction."
package main

import "github.com/stockyard-dev/stockyard/internal/engine"

var (
	version = "dev"
	commit  = ""
	date    = ""
)

func main() {
	engine.Boot(engine.ProductConfig{
		Name:    "ComplianceLog",
		Product: "compliancelog",
		Version: version,
		Features: engine.Features{
			ComplianceLog:  true,
			RequestLogging: true,
			FullBodyLog:    true,
		},
	})
}
