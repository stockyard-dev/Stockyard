// Package discovery provides a file-based service registry for Stockyard tools.
// When a tool starts, it writes a JSON file to ~/.stockyard/discovery/.
// Hub reads these files to find running tools without configuration.
package discovery

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
)

// ServiceInfo represents a running Stockyard tool.
type ServiceInfo struct {
	Slug      string `json:"slug"`
	Name      string `json:"name"`
	Port      int    `json:"port"`
	PID       int    `json:"pid"`
	Health    string `json:"health_url"`
	Dashboard string `json:"dashboard_url"`
	StartedAt string `json:"started_at"`
	Version   string `json:"version,omitempty"`
}

// DiscoveryDir returns the path to the discovery directory.
func DiscoveryDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "/tmp"
	}
	dir := filepath.Join(home, ".stockyard", "discovery")
	os.MkdirAll(dir, 0755)
	return dir
}

// Register writes a service info file and sets up cleanup on exit.
func Register(slug, name string, port int, version string) {
	info := ServiceInfo{
		Slug:      slug,
		Name:      name,
		Port:      port,
		PID:       os.Getpid(),
		Health:    fmt.Sprintf("http://localhost:%d/api/health", port),
		Dashboard: fmt.Sprintf("http://localhost:%d/ui", port),
		StartedAt: time.Now().UTC().Format(time.RFC3339),
		Version:   version,
	}

	dir := DiscoveryDir()
	path := filepath.Join(dir, slug+".json")

	data, _ := json.MarshalIndent(info, "", "  ")
	if err := os.WriteFile(path, data, 0644); err != nil {
		log.Printf("[discovery] failed to register %s: %v", slug, err)
		return
	}
	log.Printf("[discovery] registered %s at :%d (pid %d)", slug, port, info.PID)

	// Clean up on exit
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		<-sigCh
		Deregister(slug)
		os.Exit(0)
	}()
}

// Deregister removes the service info file.
func Deregister(slug string) {
	path := filepath.Join(DiscoveryDir(), slug+".json")
	os.Remove(path)
	log.Printf("[discovery] deregistered %s", slug)
}

// Discover reads all service info files and returns running services.
func Discover() ([]ServiceInfo, error) {
	dir := DiscoveryDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var services []ServiceInfo
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			continue
		}
		var info ServiceInfo
		if err := json.Unmarshal(data, &info); err != nil {
			continue
		}

		// Check if process is still running
		p, err := os.FindProcess(info.PID)
		if err != nil {
			os.Remove(filepath.Join(dir, entry.Name()))
			continue
		}
		if err := p.Signal(syscall.Signal(0)); err != nil {
			// Process not running, clean up stale file
			os.Remove(filepath.Join(dir, entry.Name()))
			continue
		}

		services = append(services, info)
	}
	return services, nil
}
