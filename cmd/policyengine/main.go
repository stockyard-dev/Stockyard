package main

import "github.com/stockyard-dev/stockyard/internal/engine"

var (version = "dev"; commit = ""; date = "")

func main() {
	engine.Boot(engine.ProductConfig{
		Name: "PolicyEngine", Product: "policyengine", Version: version,
		Features: engine.Features{PolicyEngine: true, RequestLogging: true, FullBodyLog: true},
	})
}
