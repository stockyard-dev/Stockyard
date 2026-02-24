package main

import "github.com/stockyard-dev/stockyard/internal/engine"

var (version = "dev"; commit = ""; date = "")

func main() {
	engine.Boot(engine.ProductConfig{
		Name: "ChaosLLM", Product: "chaosllm", Version: version,
		Features: engine.Features{ChaosLLM: true, RequestLogging: true, FullBodyLog: true},
	})
}
