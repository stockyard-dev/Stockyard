package features

import (
	"log"
	"os"
	"path/filepath"
	"strings"
)

// WASMPlugin represents a loaded WASM plugin.
type WASMPlugin struct {
	Name     string
	Path     string
	Enabled  bool
}

// LoadWASMPlugins scans the plugins/ directory for .wasm files.
// Each plugin is registered as a toggleable proxy module.
func LoadWASMPlugins(pluginsDir string) []WASMPlugin {
	if pluginsDir == "" {
		pluginsDir = "plugins"
	}

	entries, err := os.ReadDir(pluginsDir)
	if err != nil {
		// plugins/ directory doesn't exist — that's fine.
		return nil
	}

	var plugins []WASMPlugin
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".wasm") {
			continue
		}

		name := strings.TrimSuffix(entry.Name(), ".wasm")
		path := filepath.Join(pluginsDir, entry.Name())

		plugins = append(plugins, WASMPlugin{
			Name:    "wasm:" + name,
			Path:    path,
			Enabled: true,
		})

		log.Printf("[wasm] loaded plugin: %s from %s", name, path)
	}

	if len(plugins) > 0 {
		log.Printf("[wasm] %d plugins loaded from %s", len(plugins), pluginsDir)
	}

	return plugins
}

// Note: Full WASM execution requires a WASM runtime (wazero or similar).
// This loader provides the plugin discovery and registration infrastructure.
// The actual WASM execution would use:
//   ctx := context.Background()
//   r := wazero.NewRuntime(ctx)
//   mod, _ := r.Instantiate(ctx, wasmBytes)
//   nameResult, _ := mod.ExportedFunction("name").Call(ctx)
