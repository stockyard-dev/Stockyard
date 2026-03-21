# Stockyard WASM Plugin SDK

Build custom proxy middleware as WebAssembly plugins.

## Plugin Interface

A WASM plugin must export these functions:

```
name() -> string           // Plugin name
process(request) -> response  // Process a proxy request
config(json) -> error      // Apply configuration
```

## Writing a Plugin (Go)

```go
package main

import "encoding/json"

//export name
func name() string {
    return "my-plugin"
}

//export process
func process(requestJSON []byte) []byte {
    var req map[string]any
    json.Unmarshal(requestJSON, &req)

    // Modify request or return response
    // Return nil to pass through to next middleware

    result, _ := json.Marshal(req)
    return result
}

//export config
func config(configJSON []byte) {
    // Apply plugin-specific configuration
}

func main() {}
```

## Building

```bash
GOOS=wasip1 GOARCH=wasm go build -o my-plugin.wasm .
```

## Installing

Place `.wasm` files in the `plugins/` directory. Stockyard loads them on boot and registers each as a toggleable proxy module.

```
plugins/
├── my-plugin.wasm
├── custom-filter.wasm
└── rate-limiter-v2.wasm
```

## Toggle

Each WASM plugin appears in the module list and can be enabled/disabled via:
- Dashboard console
- API: `PUT /api/proxy/modules/{plugin-name}`
- CLI: `sy modules enable {plugin-name}`
- Terraform: `stockyard_module` resource

## Lifecycle

1. **Boot**: Stockyard scans `plugins/` for `.wasm` files
2. **Load**: Each WASM binary is instantiated
3. **Register**: Plugin's `name()` is called, registered in toggle system
4. **Process**: On each request (if enabled), `process()` is called
5. **Config**: `config()` is called when module config is updated via API
