package main

import "github.com/stockyard-dev/stockyard/internal/engine"

var (version = "dev"; commit = ""; date = "")

func main() {
	engine.Boot(engine.ProductConfig{
		Name: "RetentionWipe", Product: "retentionwipe", Version: version,
		Features: engine.Features{RetentionWipe: true, RequestLogging: true, FullBodyLog: true},
	})
}
