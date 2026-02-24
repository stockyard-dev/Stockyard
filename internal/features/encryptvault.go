package features

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"io"
	"sync"
	"sync/atomic"
	"time"

	"github.com/stockyard-dev/stockyard/internal/config"
	"github.com/stockyard-dev/stockyard/internal/provider"
	"github.com/stockyard-dev/stockyard/internal/proxy"
)

type EncryptVaultEvent struct {
	Timestamp time.Time `json:"timestamp"`
	Action    string    `json:"action"`
	Fields    int       `json:"fields"`
	Model     string    `json:"model"`
}

type EncryptVaultState struct {
	mu           sync.Mutex
	cfg          config.EncryptVaultConfig
	gcm          cipher.AEAD
	recentEvents []EncryptVaultEvent
	fieldsEncrypted atomic.Int64
	fieldsDecrypted atomic.Int64
	requestsProcessed atomic.Int64
}

func NewEncryptVault(cfg config.EncryptVaultConfig) *EncryptVaultState {
	ev := &EncryptVaultState{cfg: cfg, recentEvents: make([]EncryptVaultEvent, 0, 200)}
	// Initialize AES-GCM if key provided
	if len(cfg.Key) >= 16 {
		key := []byte(cfg.Key)
		if len(key) > 32 { key = key[:32] }
		if len(key) > 16 && len(key) < 32 { key = key[:16] }
		block, err := aes.NewCipher(key)
		if err == nil {
			gcm, err := cipher.NewGCM(block)
			if err == nil { ev.gcm = gcm }
		}
	}
	return ev
}

func (ev *EncryptVaultState) encrypt(plaintext string) string {
	if ev.gcm == nil { return plaintext }
	nonce := make([]byte, ev.gcm.NonceSize())
	io.ReadFull(rand.Reader, nonce)
	sealed := ev.gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return "enc:" + base64.StdEncoding.EncodeToString(sealed)
}

func (ev *EncryptVaultState) Stats() map[string]any {
	ev.mu.Lock()
	events := make([]EncryptVaultEvent, len(ev.recentEvents))
	copy(events, ev.recentEvents)
	ev.mu.Unlock()
	return map[string]any{
		"fields_encrypted": ev.fieldsEncrypted.Load(), "fields_decrypted": ev.fieldsDecrypted.Load(),
		"requests_processed": ev.requestsProcessed.Load(), "encryption_active": ev.gcm != nil,
		"recent_events": events,
	}
}

func EncryptVaultMiddleware(ev *EncryptVaultState) proxy.Middleware {
	return func(next proxy.Handler) proxy.Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
			ev.requestsProcessed.Add(1)
			return next(ctx, req)
		}
	}
}
