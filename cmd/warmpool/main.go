package main

import "github.com/stockyard-dev/stockyard/internal/engine"

var (version = "dev"; commit = ""; date = "")

func main() {
	engine.Boot(engine.ProductConfig{
		Name: "WarmPool", Product: "warmpool", Version: version,
		Features: engine.Features{WarmPool: true, RequestLogging: true, FullBodyLog: true},
	})
}
