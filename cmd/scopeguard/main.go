package main

import "github.com/stockyard-dev/stockyard/internal/engine"

var (version = "dev"; commit = ""; date = "")

func main() {
	engine.Boot(engine.ProductConfig{
		Name: "ScopeGuard", Product: "scopeguard", Version: version,
		Features: engine.Features{ScopeGuard: true, RequestLogging: true, FullBodyLog: true},
	})
}
