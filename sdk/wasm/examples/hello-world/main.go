// Hello World WASM plugin for Stockyard.
// Logs each request and passes it through unchanged.
//
// Build: GOOS=wasip1 GOARCH=wasm go build -o hello-world.wasm .
package main

import (
	"encoding/json"
	"fmt"
)

//export name
func name() string {
	return "hello-world"
}

//export process
func process(requestJSON []byte) []byte {
	var req map[string]any
	if err := json.Unmarshal(requestJSON, &req); err != nil {
		return requestJSON // Pass through on error.
	}

	// Log the request model.
	if model, ok := req["model"].(string); ok {
		fmt.Printf("[hello-world] processing request for model: %s\n", model)
	}

	// Count messages.
	if messages, ok := req["messages"].([]any); ok {
		fmt.Printf("[hello-world] message count: %d\n", len(messages))
	}

	// Pass through unchanged.
	return requestJSON
}

//export config
func config(configJSON []byte) {
	fmt.Printf("[hello-world] config updated: %s\n", string(configJSON))
}

func main() {}
