package main

import "github.com/stockyard-dev/stockyard/internal/engine"

var (version = "dev"; commit = ""; date = "")

func main() {
	engine.Boot(engine.ProductConfig{
		Name: "ToolShield", Product: "toolshield", Version: version,
		Features: engine.Features{ToolShield: true, RequestLogging: true, FullBodyLog: true},
	})
}
