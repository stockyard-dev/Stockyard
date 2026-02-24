package main

import "github.com/stockyard-dev/stockyard/internal/engine"

var (version = "dev"; commit = ""; date = "")

func main() {
	engine.Boot(engine.ProductConfig{
		Name: "CliDash", Product: "clidash", Version: version,
		Features: engine.Features{CliDash: true, RequestLogging: true, FullBodyLog: true},
	})
}
