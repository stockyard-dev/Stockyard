package main

import "github.com/stockyard-dev/stockyard/internal/engine"

var (version = "dev"; commit = ""; date = "")

func main() {
	engine.Boot(engine.ProductConfig{
		Name: "PromptFuzz", Product: "promptfuzz", Version: version,
		Features: engine.Features{PromptFuzz: true, RequestLogging: true, FullBodyLog: true},
	})
}
