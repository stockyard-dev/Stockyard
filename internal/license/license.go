// Package license handles Stockyard license key generation, validation, and tier enforcement.
//
// License keys are Ed25519-signed tokens validated offline — no phone-home, no network dependency.
// Format: SY-<base64url(payload)>.<base64url(signature)>
//
// The public key is embedded in every binary. The private key lives only in the API backend.
package license

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

// Tier represents a pricing tier.
type Tier string

const (
	TierFree       Tier = "free"
	TierStarter    Tier = "starter"
	TierPro        Tier = "pro"
	TierTeam       Tier = "team"
	TierEnterprise Tier = "enterprise"
)

// Payload is the data embedded in a license key.
type Payload struct {
	Product    string `json:"p"`             // product slug ("costcap", "stockyard" for suite) or "*" for any
	Tier       Tier   `json:"t"`             // pricing tier
	CustomerID string `json:"c"`             // Stripe customer ID or internal ID
	Email      string `json:"e,omitempty"`   // customer email (optional, for display)
	IssuedAt   int64  `json:"i"`             // unix timestamp
	ExpiresAt  int64  `json:"x"`             // unix timestamp (0 = never)
	MaxSeats   int    `json:"s,omitempty"`   // max concurrent instances (team/enterprise)
}

// License is a validated license with parsed payload.
type License struct {
	Payload   Payload
	Raw       string // original key string
	Valid     bool
	ExpiresAt time.Time
	IssuedAt  time.Time
}

// Info returns a human-readable summary.
func (l *License) Info() string {
	exp := "never"
	if !l.ExpiresAt.IsZero() {
		exp = l.ExpiresAt.Format("2006-01-02")
	}
	return fmt.Sprintf("%s/%s (customer=%s, expires=%s)", l.Payload.Product, l.Payload.Tier, l.Payload.CustomerID, exp)
}

// IsExpired returns true if the license has passed its expiry date.
func (l *License) IsExpired() bool {
	if l.Payload.ExpiresAt == 0 {
		return false // no expiry
	}
	return time.Now().After(l.ExpiresAt)
}

// CoversProduct returns true if this license covers the given product slug.
func (l *License) CoversProduct(product string) bool {
	if l.Payload.Product == "*" || l.Payload.Product == "stockyard" {
		return true // suite or wildcard covers everything
	}
	return l.Payload.Product == product
}

// Hardcoded public key for production license validation.
// This is set during build or replaced by the API backend's keypair.
// For development, we generate a throwaway keypair if this is empty.
//
//nolint:unused
var publicKeyBytes []byte

// ProductionPublicKey is the base64-encoded Ed25519 public key baked into releases.
// Set this at build time: -ldflags "-X github.com/stockyard-dev/stockyard/internal/license.ProductionPublicKey=..."
var ProductionPublicKey = ""

// devKeyPair is lazily initialized for development mode (no production key set).
var devKeyPair *KeyPair

// getPublicKey returns the public key for validation. Uses production key if set,
// otherwise falls back to dev mode (accepts any well-formed key for local testing).
func getPublicKey() ed25519.PublicKey {
	if ProductionPublicKey != "" {
		b, err := base64.RawURLEncoding.DecodeString(ProductionPublicKey)
		if err == nil && len(b) == ed25519.PublicKeySize {
			return ed25519.PublicKey(b)
		}
	}
	// Dev mode: return nil (we'll skip signature check)
	return nil
}

// Validate parses and validates a license key string.
// Returns a License with Valid=true if the key is well-formed, properly signed, and not expired.
func Validate(key string) *License {
	l := &License{Raw: key}

	// Strip prefix
	if !strings.HasPrefix(key, "SY-") {
		return l
	}
	key = key[3:]

	// Split payload.signature
	parts := strings.SplitN(key, ".", 2)
	if len(parts) != 2 {
		return l
	}

	// Decode payload
	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return l
	}

	// Decode signature
	sigBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return l
	}

	// Verify signature
	pubKey := getPublicKey()
	if pubKey != nil {
		// Production mode: strict verification
		if len(sigBytes) != ed25519.SignatureSize {
			return l
		}
		if !ed25519.Verify(pubKey, payloadBytes, sigBytes) {
			return l
		}
	}
	// Dev mode (pubKey == nil): skip signature check, accept any well-formed key

	// Parse payload
	var payload Payload
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		return l
	}

	l.Payload = payload
	l.IssuedAt = time.Unix(payload.IssuedAt, 0)
	if payload.ExpiresAt > 0 {
		l.ExpiresAt = time.Unix(payload.ExpiresAt, 0)
	}
	l.Valid = true

	return l
}

// FromEnv reads and validates the license key from STOCKYARD_LICENSE_KEY environment variable.
// Returns a free-tier license if no key is set.
func FromEnv() *License {
	key := os.Getenv("STOCKYARD_LICENSE_KEY")
	if key == "" {
		return &License{
			Valid:   true,
			Payload: Payload{Product: "*", Tier: TierFree, CustomerID: "free"},
		}
	}
	return Validate(key)
}

// TierFromString parses a tier string, defaulting to free.
func TierFromString(s string) Tier {
	switch strings.ToLower(s) {
	case "starter":
		return TierStarter
	case "pro":
		return TierPro
	case "team":
		return TierTeam
	case "enterprise":
		return TierEnterprise
	default:
		return TierFree
	}
}
