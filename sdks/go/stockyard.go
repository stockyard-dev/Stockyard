// Package stockyard provides helpers for using the Stockyard proxy with the Go OpenAI SDK.
package stockyard

import (
	"net/http"
	"os"
)

// BaseURL returns the Stockyard proxy base URL.
func BaseURL() string {
	if u := os.Getenv("STOCKYARD_URL"); u != "" {
		return u + "/v1"
	}
	return "http://localhost:8080/v1"
}

// Transport returns an http.RoundTripper that adds the X-Stockyard-Source header.
func Transport() http.RoundTripper {
	return &stockyardTransport{base: http.DefaultTransport}
}

type stockyardTransport struct {
	base http.RoundTripper
}

func (t *stockyardTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("X-Stockyard-Source", "go-sdk")
	return t.base.RoundTrip(req)
}
