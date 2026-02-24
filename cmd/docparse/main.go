package main

import "github.com/stockyard-dev/stockyard/internal/engine"

var (version = "dev"; commit = ""; date = "")

func main() {
	engine.Boot(engine.ProductConfig{
		Name: "DocParse", Product: "docparse", Version: version,
		Features: engine.Features{DocParse: true, RequestLogging: true, FullBodyLog: true},
	})
}
