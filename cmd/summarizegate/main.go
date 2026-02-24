package main

import "github.com/stockyard-dev/stockyard/internal/engine"

var (version = "dev"; commit = ""; date = "")

func main() {
	engine.Boot(engine.ProductConfig{
		Name: "SummarizeGate", Product: "summarizegate", Version: version,
		Features: engine.Features{SummarizeGate: true, RequestLogging: true, FullBodyLog: true},
	})
}
