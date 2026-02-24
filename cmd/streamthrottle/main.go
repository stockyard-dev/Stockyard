package main

import "github.com/stockyard-dev/stockyard/internal/engine"

var (version = "dev"; commit = ""; date = "")

func main() {
	engine.Boot(engine.ProductConfig{
		Name: "StreamThrottle", Product: "streamthrottle", Version: version,
		Features: engine.Features{StreamThrottle: true, RequestLogging: true, FullBodyLog: true},
	})
}
