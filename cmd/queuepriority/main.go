package main

import "github.com/stockyard-dev/stockyard/internal/engine"

var (version = "dev"; commit = ""; date = "")

func main() {
	engine.Boot(engine.ProductConfig{
		Name: "QueuePriority", Product: "queuepriority", Version: version,
		Features: engine.Features{QueuePriority: true, RequestLogging: true, FullBodyLog: true},
	})
}
