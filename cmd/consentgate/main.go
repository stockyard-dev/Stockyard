package main

import "github.com/stockyard-dev/stockyard/internal/engine"

var (version = "dev"; commit = ""; date = "")

func main() {
	engine.Boot(engine.ProductConfig{
		Name: "ConsentGate", Product: "consentgate", Version: version,
		Features: engine.Features{ConsentGate: true, RequestLogging: true, FullBodyLog: true},
	})
}
