package main

import "github.com/stockyard-dev/stockyard/internal/engine"

var (version = "dev"; commit = ""; date = "")

func main() {
	engine.Boot(engine.ProductConfig{
		Name: "ToolMock", Product: "toolmock", Version: version,
		Features: engine.Features{ToolMock: true, RequestLogging: true, FullBodyLog: true},
	})
}
