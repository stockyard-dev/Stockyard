package main

import "github.com/stockyard-dev/stockyard/internal/engine"

var (version = "dev"; commit = ""; date = "")

func main() {
	engine.Boot(engine.ProductConfig{
		Name: "ErrorNorm", Product: "errornorm", Version: version,
		Features: engine.Features{ErrorNorm: true, RequestLogging: true, FullBodyLog: true},
	})
}
