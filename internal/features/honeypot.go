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

// honeypotPatterns defines known attack patterns and their categories.
var honeypotPatterns = []struct {
	attackType string
	patterns   []string
}{
	{
		attackType: "sql_injection",
		patterns: []string{
			"' or 1=1",
			"'; drop table",
			"union select",
			"' or ''='",
			"1; drop table",
			"' or 'a'='a",
			"'; exec",
			"--",
			"'; delete from",
		},
	},
	{
		attackType: "prompt_injection",
		patterns: []string{
			"ignore previous instructions",
			"ignore all previous",
			"disregard previous",
			"forget your instructions",
			"override your system prompt",
			"new instructions:",
			"system: you are now",
			"<|im_start|>",
			"[system]",
			"ignore the above",
		},
	},
}

// initHoneypotTable creates the honeypot_events table if it doesn't exist.
func initHoneypotTable(conn *sql.DB) {
	_, err := conn.Exec(`CREATE TABLE IF NOT EXISTS honeypot_events (
		id TEXT PRIMARY KEY,
		attack_type TEXT,
		pattern TEXT,
		source_ip TEXT,
		created_at TEXT
	)`)
	if err != nil {
		log.Printf("honeypot: failed to create table: %v", err)
	}
}

// genHoneypotID generates a random hex ID for honeypot events.
func genHoneypotID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "unknown"
	}
	return hex.EncodeToString(b)
}

// HoneypotMiddleware checks for common attack patterns in request content
// (SQL injection, prompt injection markers). If detected, the event is logged
// to the honeypot_events table. The request is always passed through to the
// next handler regardless of detection.
func HoneypotMiddleware(conn *sql.DB) proxy.Middleware {
	initHoneypotTable(conn)

	return func(next proxy.Handler) proxy.Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
			// Build combined content from all messages for scanning
			var content strings.Builder
			for _, msg := range req.Messages {
				content.WriteString(msg.Content)
				content.WriteString(" ")
			}
			lowerContent := strings.ToLower(content.String())

			// Check for attack patterns
			sourceIP := req.ClientIP
			for _, category := range honeypotPatterns {
				for _, pattern := range category.patterns {
					if strings.Contains(lowerContent, strings.ToLower(pattern)) {
						logHoneypotEvent(conn, category.attackType, pattern, sourceIP)
					}
				}
			}

			// Always pass through to next handler
			return next(ctx, req)
		}
	}
}

// logHoneypotEvent inserts a detected attack event into the honeypot_events table.
func logHoneypotEvent(conn *sql.DB, attackType, pattern, sourceIP string) {
	id := genHoneypotID()
	createdAt := time.Now().UTC().Format(time.RFC3339)
	_, err := conn.Exec(
		"INSERT INTO honeypot_events (id, attack_type, pattern, source_ip, created_at) VALUES (?, ?, ?, ?, ?)",
		id, attackType, pattern, sourceIP, createdAt,
	)
	if err != nil {
		log.Printf("honeypot: failed to log event: %v", err)
	}
}
