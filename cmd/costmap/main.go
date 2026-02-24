package main

import "github.com/stockyard-dev/stockyard/internal/engine"

var (version = "dev"; commit = ""; date = "")

func main() {
	engine.Boot(engine.ProductConfig{
		Name: "CostMap", Product: "costmap", Version: version,
		Features: engine.Features{CostMap: true, RequestLogging: true, FullBodyLog: true},
	})
}
