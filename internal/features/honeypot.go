package features

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"log"
	"strings"
	"time"

	"github.com/stockyard-dev/stockyard/internal/provider"
	"github.com/stockyard-dev/stockyard/internal/proxy"
)

// HoneypotMiddleware deploys fake vulnerabilities to capture attack techniques.
func HoneypotMiddleware(conn *sql.DB) proxy.Middleware {
	// Ensure table exists
	conn.Exec(`CREATE TABLE IF NOT EXISTS honeypot_events (
		id TEXT PRIMARY KEY,
		attack_type TEXT NOT NULL,
		pattern TEXT NOT NULL,
		client_ip TEXT,
		created_at TEXT NOT NULL
	)`)

	traps := []struct {
		pattern    string
		attackType string
	}{
		{"ignore previous instructions", "prompt_injection"},
		{"you are now", "jailbreak"},
		{"system prompt:", "prompt_extraction"},
		{"reveal your instructions", "prompt_extraction"},
		{"act as root", "privilege_escalation"},
		{"sudo", "privilege_escalation"},
		{"<script>", "xss_attempt"},
		{"DROP TABLE", "sql_injection"},
	}

	return func(next proxy.Handler) proxy.Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
			for _, msg := range req.Messages {
				lower := strings.ToLower(msg.Content)
				for _, trap := range traps {
					if strings.Contains(lower, trap.pattern) {
						now := time.Now().UTC().Format(time.RFC3339)
						id := genHoneypotID()
						conn.Exec(`INSERT INTO honeypot_events (id, attack_type, pattern, client_ip, created_at) VALUES (?, ?, ?, ?, ?)`,
							id, trap.attackType, trap.pattern, req.ClientIP, now)
						log.Printf("[honeypot] trapped %s attack from %s", trap.attackType, req.ClientIP)

						// Fire webhook
						if webhookFire != nil {
							webhookFire("honeypot_triggered", map[string]any{
								"attack_type": trap.attackType,
								"client_ip":   req.ClientIP,
								"pattern":     trap.pattern,
							})
						}
					}
				}
			}
			return next(ctx, req)
		}
	}
}

func genHoneypotID() string {
	b := make([]byte, 6)
	rand.Read(b)
	return "hp_" + hex.EncodeToString(b)
}
