package main

import "github.com/stockyard-dev/stockyard/internal/engine"

var (version = "dev"; commit = ""; date = "")

func main() {
	engine.Boot(engine.ProductConfig{
		Name: "AnomalyRadar", Product: "anomalyradar", Version: version,
		Features: engine.Features{AnomalyRadar: true, RequestLogging: true, FullBodyLog: true},
	})
}
