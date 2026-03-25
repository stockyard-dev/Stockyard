// Package keyvault manages encrypted API keys for external services.
package keyvault

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"golang.org/x/crypto/pbkdf2"
)

const (
	// pbkdf2Iterations is the number of PBKDF2 iterations for key derivation.
	pbkdf2Iterations = 100_000
	// saltSize is the byte length of the random salt.
	saltSize = 32
	// nonceSize is the byte length of the AES-GCM nonce.
	nonceSize = 12
)

// encryptedPayload is the on-disk encrypted format.
type encryptedPayload struct {
	Version int    `json:"version"`    // 1 = AES-256-GCM
	Salt    string `json:"salt"`       // base64 encoded
	Nonce   string `json:"nonce"`      // base64 encoded
	Data    string `json:"ciphertext"` // base64 encoded
}

// Vault stores API keys for external services.
type Vault struct {
	mu   sync.RWMutex
	keys map[string]string // service ID -> API key
	path string            // file path for persistence
	pass string            // passphrase for encryption (empty = no encryption)
}

// New creates or loads a vault.
func New(dataDir string) *Vault {
	v := &Vault{
		keys: map[string]string{},
		path: filepath.Join(dataDir, "keys.json"),
		pass: os.Getenv("MORPH_VAULT_PASS"),
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

// load reads keys from disk, handling both plaintext and encrypted formats.
func (v *Vault) load() {
	data, err := os.ReadFile(v.path)
	if err != nil {
		return // No file yet, that's fine
	}

	// Try plaintext JSON first (backward compat)
	plainKeys := map[string]string{}
	if err := json.Unmarshal(data, &plainKeys); err == nil {
		// Check if it's actually our encrypted payload by looking for "version"
		var probe map[string]json.RawMessage
		json.Unmarshal(data, &probe)
		if _, hasVersion := probe["version"]; hasVersion {
			// This is an encrypted payload, try to decrypt
			v.loadEncrypted(data)
			return
		}
		// It's plaintext JSON — load and auto-migrate on next save
		v.keys = plainKeys
		// If we have a passphrase, auto-migrate to encrypted format
		if v.pass != "" {
			v.save()
		}
		return
	}

	// Try encrypted format
	v.loadEncrypted(data)
}

// loadEncrypted decrypts an encrypted payload.
func (v *Vault) loadEncrypted(data []byte) {
	if v.pass == "" {
		return // can't decrypt without a passphrase
	}

	var payload encryptedPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return
	}
	if payload.Version != 1 {
		return
	}

	salt, err := base64.StdEncoding.DecodeString(payload.Salt)
	if err != nil {
		return
	}
	nonce, err := base64.StdEncoding.DecodeString(payload.Nonce)
	if err != nil {
		return
	}
	ciphertext, err := base64.StdEncoding.DecodeString(payload.Data)
	if err != nil {
		return
	}

	key := deriveKey(v.pass, salt)
	block, err := aes.NewCipher(key)
	if err != nil {
		return
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return
	}

	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return
	}

	json.Unmarshal(plaintext, &v.keys)
}

// save persists keys to disk, encrypted if a passphrase is set.
func (v *Vault) save() error {
	v.mu.RLock()
	plaintext, err := json.MarshalIndent(v.keys, "", "  ")
	v.mu.RUnlock()
	if err != nil {
		return err
	}

	os.MkdirAll(filepath.Dir(v.path), 0755)

	// If no passphrase, save plaintext (backward compat)
	if v.pass == "" {
		return os.WriteFile(v.path, plaintext, 0600)
	}

	// Encrypt with AES-256-GCM
	salt := make([]byte, saltSize)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return fmt.Errorf("generating salt: %w", err)
	}

	key := deriveKey(v.pass, salt)
	block, err := aes.NewCipher(key)
	if err != nil {
		return fmt.Errorf("creating cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return fmt.Errorf("creating GCM: %w", err)
	}

	nonce := make([]byte, nonceSize)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return fmt.Errorf("generating nonce: %w", err)
	}

	ciphertext := gcm.Seal(nil, nonce, plaintext, nil)

	payload := encryptedPayload{
		Version: 1,
		Salt:    base64.StdEncoding.EncodeToString(salt),
		Nonce:   base64.StdEncoding.EncodeToString(nonce),
		Data:    base64.StdEncoding.EncodeToString(ciphertext),
	}

	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(v.path, data, 0600)
}

// deriveKey uses PBKDF2 to derive a 32-byte AES-256 key from a passphrase.
func deriveKey(passphrase string, salt []byte) []byte {
	return pbkdf2.Key([]byte(passphrase), salt, pbkdf2Iterations, 32, sha256.New)
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
