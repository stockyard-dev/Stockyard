package main

import "github.com/stockyard-dev/stockyard/internal/engine"

var (version = "dev"; commit = ""; date = "")

func main() {
	engine.Boot(engine.ProductConfig{
		Name: "SlotFill", Product: "slotfill", Version: version,
		Features: engine.Features{SlotFill: true, RequestLogging: true, FullBodyLog: true},
	})
}
