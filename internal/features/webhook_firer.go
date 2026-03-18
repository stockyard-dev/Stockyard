package features

import "sync"

// WebhookFireFunc is called by feature middlewares to fire webhook events.
// Args: eventType string, data any
type WebhookFireFunc func(string, any)

var (
	webhookMu   sync.RWMutex
	webhookFire WebhookFireFunc
)

// SetWebhookFirer installs the global webhook firer (called from engine wiring).
func SetWebhookFirer(fn WebhookFireFunc) {
	webhookMu.Lock()
	webhookFire = fn
	webhookMu.Unlock()
}

// fireWebhook dispatches a webhook event if the firer is wired.
func fireWebhook(eventType string, data any) {
	webhookMu.RLock()
	fn := webhookFire
	webhookMu.RUnlock()
	if fn != nil {
		go fn(eventType, data)
	}
}
