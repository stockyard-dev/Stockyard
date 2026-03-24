// Package keyvault manages encrypted API keys for external services.
package keyvault

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// Vault stores API keys for external services.
type Vault struct {
	mu   sync.RWMutex
	keys map[string]string // service ID -> API key
	path string            // file path for persistence
}

// New creates or loads a vault.
func New(dataDir string) *Vault {
	v := &Vault{
		keys: map[string]string{},
		path: filepath.Join(dataDir, "keys.json"),
	}
	v.load()
	v.loadFromEnv()
	return v
}

// Get returns the API key for a service.
func (v *Vault) Get(service string) string {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.keys[service]
}

// Set stores an API key for a service.
func (v *Vault) Set(service, key string) error {
	v.mu.Lock()
	v.keys[service] = key
	v.mu.Unlock()
	return v.save()
}

// Delete removes an API key.
func (v *Vault) Delete(service string) error {
	v.mu.Lock()
	delete(v.keys, service)
	v.mu.Unlock()
	return v.save()
}

// List returns all configured services (keys redacted).
func (v *Vault) List() map[string]string {
	v.mu.RLock()
	defer v.mu.RUnlock()
	out := map[string]string{}
	for svc, key := range v.keys {
		if len(key) > 8 {
			out[svc] = key[:4] + "..." + key[len(key)-4:]
		} else {
			out[svc] = "****"
		}
	}
	return out
}

// ConfiguredServices returns names of services with keys.
func (v *Vault) ConfiguredServices() []string {
	v.mu.RLock()
	defer v.mu.RUnlock()
	var out []string
	for svc := range v.keys {
		out = append(out, svc)
	}
	return out
}

func (v *Vault) load() {
	data, err := os.ReadFile(v.path)
	if err != nil {
		return // No file yet, that's fine
	}
	json.Unmarshal(data, &v.keys)
}

func (v *Vault) save() error {
	v.mu.RLock()
	data, err := json.MarshalIndent(v.keys, "", "  ")
	v.mu.RUnlock()
	if err != nil {
		return err
	}
	os.MkdirAll(filepath.Dir(v.path), 0755)
	return os.WriteFile(v.path, data, 0600) // 0600 = owner read/write only
}

// loadFromEnv loads API keys from environment variables.
func (v *Vault) loadFromEnv() {
	envMap := map[string]string{
		"STRIPE_API_KEY":    "stripe",
		"GITHUB_TOKEN":      "github",
		"SLACK_TOKEN":       "slack",
		"SLACK_BOT_TOKEN":   "slack",
		"SENDGRID_API_KEY":  "sendgrid",
		"TWILIO_AUTH_TOKEN": "twilio",
		"LINEAR_API_KEY":    "linear",
		"NOTION_API_KEY":    "notion",
		"JIRA_API_TOKEN":    "jira",
		"DISCORD_BOT_TOKEN": "discord",
		"RESEND_API_KEY":    "resend",
	}

	v.mu.Lock()
	defer v.mu.Unlock()
	for env, svc := range envMap {
		if key := os.Getenv(env); key != "" {
			if _, exists := v.keys[svc]; !exists {
				v.keys[svc] = key
			}
		}
	}
}

// Hint returns a help message about how to configure a service.
func Hint(service string) string {
	hints := map[string]string{
		"stripe":   "Set STRIPE_API_KEY or run: morph keys set stripe sk_live_...",
		"github":   "Set GITHUB_TOKEN or run: morph keys set github ghp_...",
		"slack":    "Set SLACK_BOT_TOKEN or run: morph keys set slack xoxb-...",
		"sendgrid": "Set SENDGRID_API_KEY or run: morph keys set sendgrid SG...",
		"twilio":   "Set TWILIO_AUTH_TOKEN or run: morph keys set twilio ...",
		"linear":   "Set LINEAR_API_KEY or run: morph keys set linear lin_api_...",
		"notion":   "Set NOTION_API_KEY or run: morph keys set notion ntn_...",
		"jira":     "Set JIRA_API_TOKEN or run: morph keys set jira ...",
		"discord":  "Set DISCORD_BOT_TOKEN or run: morph keys set discord ...",
		"resend":   "Set RESEND_API_KEY or run: morph keys set resend re_...",
	}
	if h, ok := hints[service]; ok {
		return h
	}
	return fmt.Sprintf("Run: morph keys set %s <your-api-key>", service)
}
