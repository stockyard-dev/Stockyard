package main

import "github.com/stockyard-dev/stockyard/internal/engine"

var (version = "dev"; commit = ""; date = "")

func main() {
	engine.Boot(engine.ProductConfig{
		Name: "ToolRouter", Product: "toolrouter", Version: version,
		Features: engine.Features{ToolRouter: true, RequestLogging: true, FullBodyLog: true},
	})
}
