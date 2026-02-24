package main

import "github.com/stockyard-dev/stockyard/internal/engine"

var (version = "dev"; commit = ""; date = "")

func main() {
	engine.Boot(engine.ProductConfig{
		Name: "GeoPrice", Product: "geoprice", Version: version,
		Features: engine.Features{GeoPrice: true, RequestLogging: true, FullBodyLog: true},
	})
}
