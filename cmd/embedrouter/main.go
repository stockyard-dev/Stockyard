package main

import "github.com/stockyard-dev/stockyard/internal/engine"

var (version = "dev"; commit = ""; date = "")

func main() {
	engine.Boot(engine.ProductConfig{
		Name: "EmbedRouter", Product: "embedrouter", Version: version,
		Features: engine.Features{EmbedRouter: true, RequestLogging: true, FullBodyLog: true},
	})
}
