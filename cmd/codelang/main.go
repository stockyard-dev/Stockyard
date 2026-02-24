package main

import "github.com/stockyard-dev/stockyard/internal/engine"

var (version = "dev"; commit = ""; date = "")

func main() {
	engine.Boot(engine.ProductConfig{
		Name: "CodeLang", Product: "codelang", Version: version,
		Features: engine.Features{CodeLang: true, RequestLogging: true, FullBodyLog: true},
	})
}
