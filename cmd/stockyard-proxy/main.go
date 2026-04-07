// Stockyard Proxy — Open Source LLM Proxy
//
// The open-source core of Stockyard: OpenAI-compatible reverse proxy
// with provider routing, model aliasing, caching, failover, rate limiting,
// spend tracking, and request logging.
//
// For the full platform (dashboard, Observe, Trust, Studio, Forge, Exchange,
// and 29 additional products), see: https://stockyard.dev
//
// License: Apache 2.0 — see LICENSE-APACHE at the repository root.
//
// Open-core boundary status: this binary is built from a source tree currently
// shared with the BSL platform. The compiled binary contains only the proxy
// stack (verified ~16MB vs 63MB for the full binary, with Go linker dead-code
// elimination). See NOTICE.md in this directory for the full audit and the
// roadmap to clean source-level separation.
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
