package main

import "github.com/stockyard-dev/stockyard/internal/engine"

var (version = "dev"; commit = ""; date = "")

func main() {
	engine.Boot(engine.ProductConfig{
		Name: "PromptRank", Product: "promptrank", Version: version,
		Features: engine.Features{PromptRank: true, RequestLogging: true, FullBodyLog: true},
	})
}
