package main

import "github.com/stockyard-dev/stockyard/internal/engine"

var (version = "dev"; commit = ""; date = "")

func main() {
	engine.Boot(engine.ProductConfig{
		Name: "CohortTrack", Product: "cohorttrack", Version: version,
		Features: engine.Features{CohortTrack: true, RequestLogging: true, FullBodyLog: true},
	})
}
