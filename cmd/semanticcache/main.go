package main

import "github.com/stockyard-dev/stockyard/internal/engine"

var (version = "dev"; commit = ""; date = "")

func main() {
	engine.Boot(engine.ProductConfig{
		Name: "SemanticCache", Product: "semanticcache", Version: version,
		Features: engine.Features{SemanticCache: true, RequestLogging: true, FullBodyLog: true},
	})
}
