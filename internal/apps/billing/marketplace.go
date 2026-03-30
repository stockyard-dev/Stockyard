package billing

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
)

const marketplaceSchema = `
CREATE TABLE IF NOT EXISTS marketplace_accounts (
    id TEXT PRIMARY KEY,
    email TEXT UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    api_key_hash TEXT NOT NULL,
    api_key_prefix TEXT NOT NULL,
    created_at TEXT DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_mp_accounts_email ON marketplace_accounts(email);
CREATE INDEX IF NOT EXISTS idx_mp_accounts_key ON marketplace_accounts(api_key_hash);
`

func (a *App) migrateMarketplace() {
	if _, err := a.conn.Exec(marketplaceSchema); err != nil {
		log.Printf("[schema] migration error: %v", err)
	}
}

func (a *App) registerMarketplaceRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/marketplace/signup", a.handleMarketplaceSignup)
	mux.HandleFunc("GET /api/marketplace/usage", a.handleMarketplaceUsage)
	mux.HandleFunc("GET /api/marketplace/balance", a.handleMarketplaceBalance)
	log.Printf("[billing] marketplace routes registered")
}

func (a *App) handleMarketplaceSignup(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	if req.Email == "" || req.Password == "" {
		w.WriteHeader(400)
		writeJSON(w, map[string]string{"error": "email and password required"})
		return
	}
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))

	// Check if already exists.
	var existing string
	if err := a.conn.QueryRow("SELECT id FROM marketplace_accounts WHERE email = ?", req.Email).Scan(&existing); err == nil {
		w.WriteHeader(409)
		writeJSON(w, map[string]string{"error": "account already exists"})
		return
	}

	// Create account.
	accountID := genID("mp_")
	passwordHash := hashPassword(req.Password)

	// Generate API key.
	keyBytes := make([]byte, 24)
	rand.Read(keyBytes)
	apiKey := "sk-sy-" + hex.EncodeToString(keyBytes)
	apiKeyHash := hashString(apiKey)
	apiKeyPrefix := apiKey[:12]

	a.conn.Exec(`INSERT INTO marketplace_accounts (id, email, password_hash, api_key_hash, api_key_prefix, created_at) VALUES (?, ?, ?, ?, ?, datetime('now'))`,
		accountID, req.Email, passwordHash, apiKeyHash, apiKeyPrefix)

	// Create a billing customer for this marketplace account.
	customerID := genID("bc_")
	a.conn.Exec(`INSERT INTO billing_customers (id, account_id, external_id, name, email, created_at) VALUES (?, ?, ?, ?, ?, datetime('now'))`,
		customerID, "marketplace", accountID, req.Email, req.Email)

	// Initialize credits at 0.
	a.addCredits(customerID, 0, "signup", "Account created")

	writeJSON(w, map[string]any{
		"account_id":  accountID,
		"customer_id": customerID,
		"api_key":     apiKey, // Only shown once
		"email":       req.Email,
		"status":      "created",
	})
}

func (a *App) handleMarketplaceUsage(w http.ResponseWriter, r *http.Request) {
	accountID := r.URL.Query().Get("account_id")
	if accountID == "" {
		w.WriteHeader(400)
		writeJSON(w, map[string]string{"error": "account_id required"})
		return
	}

	// Get customer ID for this account.
	var customerID string
	a.conn.QueryRow("SELECT id FROM billing_customers WHERE external_id = ? AND account_id = 'marketplace'", accountID).Scan(&customerID)
	if customerID == "" {
		w.WriteHeader(404)
		writeJSON(w, map[string]string{"error": "account not found"})
		return
	}

	// Get usage breakdown.
	rows, err := a.conn.Query(`SELECT model, COUNT(*) as requests,
		SUM(input_tokens) as tin, SUM(output_tokens) as tout, SUM(cost_cents) as cost
		FROM billing_usage WHERE customer_id = ? AND created_at >= datetime('now', '-30 days')
		GROUP BY model ORDER BY cost DESC`, customerID)
	if err != nil {
		writeJSON(w, map[string]any{"usage": []any{}})
		return
	}
	defer rows.Close()

	var usage []map[string]any
	var totalCents int64
	for rows.Next() {
		var model string
		var requests, tin, tout, cost int64
		if err := rows.Scan(&model, &requests, &tin, &tout, &cost); err != nil {
			continue
		}
		totalCents += cost
		usage = append(usage, map[string]any{
			"model": model, "requests": requests,
			"input_tokens": tin, "output_tokens": tout,
			"cost_cents": cost,
		})
	}
	if err := rows.Err(); err != nil {
		log.Printf("[db] rows iteration error: %v", err)
	}
	if usage == nil {
		usage = []map[string]any{}
	}

	// Get markup percentage.
	markupPct := 15
	if env := os.Getenv("MARKETPLACE_MARKUP_PCT"); env != "" {
		fmt.Sscanf(env, "%d", &markupPct)
	}

	writeJSON(w, map[string]any{
		"customer_id": customerID,
		"usage":       usage,
		"total_cents": totalCents,
		"markup_pct":  markupPct,
		"period":      "last_30_days",
	})
}

func (a *App) handleMarketplaceBalance(w http.ResponseWriter, r *http.Request) {
	accountID := r.URL.Query().Get("account_id")
	if accountID == "" {
		w.WriteHeader(400)
		writeJSON(w, map[string]string{"error": "account_id required"})
		return
	}

	var customerID string
	a.conn.QueryRow("SELECT id FROM billing_customers WHERE external_id = ? AND account_id = 'marketplace'", accountID).Scan(&customerID)

	var balanceCents int64
	a.conn.QueryRow("SELECT balance_cents FROM billing_credits WHERE customer_id = ?", customerID).Scan(&balanceCents)

	writeJSON(w, map[string]any{
		"account_id":    accountID,
		"balance_cents": balanceCents,
		"balance_usd":   fmt.Sprintf("%.2f", float64(balanceCents)/100),
	})
}

// hashPassword creates a salted, iterated HMAC-SHA256 hash (100k iterations).
// Format: "pbkdf2:100000:salt_hex:hash_hex"
// This is resistant to GPU brute-force attacks unlike plain SHA-256.
func hashPassword(password string) string {
	salt := make([]byte, 16)
	rand.Read(salt)
	iterations := 100000
	hash := pbkdf2Hash([]byte(password), salt, iterations)
	return fmt.Sprintf("pbkdf2:%d:%s:%s", iterations, hex.EncodeToString(salt), hex.EncodeToString(hash))
}

// verifyPassword checks a password against a stored hash.
func verifyPassword(password, stored string) bool {
	parts := strings.SplitN(stored, ":", 4)
	if len(parts) == 4 && parts[0] == "pbkdf2" {
		var iterations int
		fmt.Sscanf(parts[1], "%d", &iterations)
		salt, _ := hex.DecodeString(parts[2])
		expected, _ := hex.DecodeString(parts[3])
		actual := pbkdf2Hash([]byte(password), salt, iterations)
		return subtle.ConstantTimeCompare(expected, actual) == 1
	}
	// Legacy: plain SHA-256 (migrate on next login)
	h := sha256.Sum256([]byte(password))
	legacy := hex.EncodeToString(h[:])
	return subtle.ConstantTimeCompare([]byte(legacy), []byte(stored)) == 1
}

// pbkdf2Hash implements PBKDF2-HMAC-SHA256 using only stdlib.
func pbkdf2Hash(password, salt []byte, iterations int) []byte {
	mac := hmac.New(sha256.New, password)
	mac.Write(salt)
	mac.Write([]byte{0, 0, 0, 1}) // block index
	u := mac.Sum(nil)
	result := make([]byte, len(u))
	copy(result, u)
	for i := 1; i < iterations; i++ {
		mac.Reset()
		mac.Write(u)
		u = mac.Sum(nil)[:sha256.Size]
		for j := range result {
			result[j] ^= u[j]
		}
	}
	return result
}

// hashString is used for API key hashing (high-entropy input, SHA-256 is fine).
func hashString(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}
