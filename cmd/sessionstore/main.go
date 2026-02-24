package main

import "github.com/stockyard-dev/stockyard/internal/engine"

var (version = "dev"; commit = ""; date = "")

func main() {
	engine.Boot(engine.ProductConfig{
		Name: "SessionStore", Product: "sessionstore", Version: version,
		Features: engine.Features{SessionStore: true, RequestLogging: true, FullBodyLog: true},
	})
}
