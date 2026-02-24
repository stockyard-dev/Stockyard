package main

import "github.com/stockyard-dev/stockyard/internal/engine"

var (version = "dev"; commit = ""; date = "")

func main() {
	engine.Boot(engine.ProductConfig{
		Name: "VisionProxy", Product: "visionproxy", Version: version,
		Features: engine.Features{VisionProxy: true, RequestLogging: true, FullBodyLog: true},
	})
}
