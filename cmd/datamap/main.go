package main

import "github.com/stockyard-dev/stockyard/internal/engine"

var (version = "dev"; commit = ""; date = "")

func main() {
	engine.Boot(engine.ProductConfig{
		Name: "DataMap", Product: "datamap", Version: version,
		Features: engine.Features{DataMap: true, RequestLogging: true, FullBodyLog: true},
	})
}
