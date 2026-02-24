package main

import "github.com/stockyard-dev/stockyard/internal/engine"

var (version = "dev"; commit = ""; date = "")

func main() {
	engine.Boot(engine.ProductConfig{
		Name: "CanaryDeploy", Product: "canarydeploy", Version: version,
		Features: engine.Features{CanaryDeploy: true, RequestLogging: true, FullBodyLog: true},
	})
}
