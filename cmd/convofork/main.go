package main

import "github.com/stockyard-dev/stockyard/internal/engine"

var (version = "dev"; commit = ""; date = "")

func main() {
	engine.Boot(engine.ProductConfig{
		Name: "ConvoFork", Product: "convofork", Version: version,
		Features: engine.Features{ConvoFork: true, RequestLogging: true, FullBodyLog: true},
	})
}
