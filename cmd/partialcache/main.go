package main

import "github.com/stockyard-dev/stockyard/internal/engine"

var (version = "dev"; commit = ""; date = "")

func main() {
	engine.Boot(engine.ProductConfig{
		Name: "PartialCache", Product: "partialcache", Version: version,
		Features: engine.Features{PartialCache: true, RequestLogging: true, FullBodyLog: true},
	})
}
