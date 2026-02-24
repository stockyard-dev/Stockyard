package main

import "github.com/stockyard-dev/stockyard/internal/engine"

var (version = "dev"; commit = ""; date = "")

func main() {
	engine.Boot(engine.ProductConfig{
		Name: "PlaybackStudio", Product: "playbackstudio", Version: version,
		Features: engine.Features{PlaybackStudio: true, RequestLogging: true, FullBodyLog: true},
	})
}
