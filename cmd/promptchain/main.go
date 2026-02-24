package main

import "github.com/stockyard-dev/stockyard/internal/engine"

var (version = "dev"; commit = ""; date = "")

func main() {
	engine.Boot(engine.ProductConfig{
		Name: "PromptChain", Product: "promptchain", Version: version,
		Features: engine.Features{PromptChain: true, RequestLogging: true, FullBodyLog: true},
	})
}
