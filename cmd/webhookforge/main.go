package main

import "github.com/stockyard-dev/stockyard/internal/engine"

var (version = "dev"; commit = ""; date = "")

func main() {
	engine.Boot(engine.ProductConfig{
		Name: "WebhookForge", Product: "webhookforge", Version: version,
		Features: engine.Features{WebhookForge: true, RequestLogging: true, FullBodyLog: true},
	})
}
