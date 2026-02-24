package main

import "github.com/stockyard-dev/stockyard/internal/engine"

var (version = "dev"; commit = ""; date = "")

func main() {
	engine.Boot(engine.ProductConfig{
		Name: "PromptMarket", Product: "promptmarket", Version: version,
		Features: engine.Features{PromptMarket: true, RequestLogging: true, FullBodyLog: true},
	})
}
