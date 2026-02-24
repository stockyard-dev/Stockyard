// TenantWall — "Per-tenant isolation for multi-tenant LLM apps."
package main

import "github.com/stockyard-dev/stockyard/internal/engine"

var (
	version = "dev"
	commit  = ""
	date    = ""
)

func main() {
	engine.Boot(engine.ProductConfig{
		Name:    "TenantWall",
		Product: "tenantwall",
		Version: version,
		Features: engine.Features{
			TenantWall:     true,
			RequestLogging: true,
			FullBodyLog:    true,
		},
	})
}
