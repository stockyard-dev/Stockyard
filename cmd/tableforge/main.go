package main

import "github.com/stockyard-dev/stockyard/internal/engine"

var (version = "dev"; commit = ""; date = "")

func main() {
	engine.Boot(engine.ProductConfig{
		Name: "TableForge", Product: "tableforge", Version: version,
		Features: engine.Features{TableForge: true, RequestLogging: true, FullBodyLog: true},
	})
}
