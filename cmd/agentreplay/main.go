package main

import "github.com/stockyard-dev/stockyard/internal/engine"

var (version = "dev"; commit = ""; date = "")

func main() {
	engine.Boot(engine.ProductConfig{
		Name: "AgentReplay", Product: "agentreplay", Version: version,
		Features: engine.Features{AgentReplay: true, RequestLogging: true, FullBodyLog: true},
	})
}
