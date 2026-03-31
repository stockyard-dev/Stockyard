// Stockyard Proxy — Open Source LLM Proxy
//
// The open-source core of Stockyard: OpenAI-compatible reverse proxy
// with provider routing, model aliasing, caching, failover, rate limiting,
// spend tracking, and request logging.
//
// For the full platform (dashboard, Observe, Trust, Studio, Forge, Exchange,
// and 29 additional products), see: https://stockyard.dev
//
// License: Apache 2.0
package main

import "github.com/stockyard-dev/stockyard/internal/engine"

var (
	version = "dev"
	commit  = ""
	date    = ""
)

func main() {
	engine.BootProxy(engine.ProxyConfig{
		Version: version,
	})
}
