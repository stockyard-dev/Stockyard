// Package stockyard provides helpers for routing OpenAI Go SDK traffic through Stockyard.
//
// Usage with github.com/sashabaranov/go-openai:
//
//	import stockyard "github.com/stockyard-dev/stockyard-openai-go"
//
//	config := openai.DefaultConfig(os.Getenv("OPENAI_API_KEY"))
//	config.BaseURL = stockyard.BaseURL()
//	config.HTTPClient = &http.Client{Transport: stockyard.Transport()}
//	client := openai.NewClientWithConfig(config)
package stockyard

import (
	"net/http"
	"os"
	"strings"
)

// BaseURL returns the Stockyard proxy URL for use as the OpenAI base URL.
// Reads STOCKYARD_URL env var, defaults to http://localhost:4200/v1.
// If STOCKYARD_ENABLED=false, returns the default OpenAI API URL.
func BaseURL() string {
	if strings.EqualFold(os.Getenv("STOCKYARD_ENABLED"), "false") {
		return "https://api.openai.com/v1"
	}
	if url := os.Getenv("STOCKYARD_URL"); url != "" {
		return strings.TrimRight(url, "/") + "/v1"
	}
	return "http://localhost:4200/v1"
}

// Transport returns an http.RoundTripper that adds Stockyard tracking headers.
type transport struct {
	base http.RoundTripper
}

// Transport creates an http.RoundTripper that adds X-Stockyard-Source header.
func Transport() http.RoundTripper {
	return &transport{base: http.DefaultTransport}
}

func (t *transport) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	req.Header.Set("X-Stockyard-Source", "sdk-go")
	base := t.base
	if base == nil {
		base = http.DefaultTransport
	}
	return base.RoundTrip(req)
}
