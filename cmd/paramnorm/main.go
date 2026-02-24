package main

import "github.com/stockyard-dev/stockyard/internal/engine"

var (version = "dev"; commit = ""; date = "")

func main() {
	engine.Boot(engine.ProductConfig{
		Name: "ParamNorm", Product: "paramnorm", Version: version,
		Features: engine.Features{ParamNorm: true, RequestLogging: true, FullBodyLog: true},
	})
}
