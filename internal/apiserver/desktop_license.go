package apiserver

// Desktop license verification.
//
// Counterpart to issueDesktopLicenseKey (stripe.go). The signed
// license keys we mint and email to customers are also their bearer
// tokens for the LLM proxy — verifying them here lets the proxy auth
// path accept "Authorization: Bearer SY-..." without a database
// round-trip on every request.
//
// Format (mirror of issueDesktopLicenseKey):
//   SY-<base64url(payload)>.<base64url(signature)>
// Where payload is JSON: {"p","t","c","e","i","x"} and signature is
// ed25519 over the raw payload bytes.
//
// Verification uses the public key derived from the same Ed25519
// private key the issuer signs with — single STOCKYARD_DESKTOP_
// PRIVATE_KEY env var serves both.

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// DesktopLicenseClaims is what verifyDesktopLicenseKey returns on a
// successful signature check. Field names mirror the short JSON keys
// in the on-the-wire payload but are spelled out for Go callers.
//
// Tier values seen in production: "local" (one-time $299),
// "cloud-single" ($49/mo), "cloud-multi" ($129/mo). Older "trial-*"
// values may appear on legacy keys; callers that care about gating
// should treat unknown tiers as "no access" rather than allowing.
type DesktopLicenseClaims struct {
	Product    string
	Tier       string
	CustomerID string
	Email      string
	IssuedAt   time.Time
	ExpiresAt  time.Time // zero value means "never expires" (Local)
}

// IsExpired reports whether ExpiresAt is set AND in the past.
// Permanent licenses (Local) have a zero ExpiresAt and are never
// expired by this check.
func (c DesktopLicenseClaims) IsExpired() bool {
	return !c.ExpiresAt.IsZero() && time.Now().After(c.ExpiresAt)
}

// verifyDesktopLicenseKey decodes a SY-prefixed license key,
// verifies its Ed25519 signature with the public key derived from
// privKeyHex, and returns the parsed claims. Returns an error on:
//   - missing SY- prefix (not a license key, caller should try other auth)
//   - malformed structure (missing dot, bad base64)
//   - signature verification failure (forged or wrong-key license)
//   - corrupt JSON payload
//
// Does NOT check expiry — callers that care should call IsExpired
// on the returned claims. Verifying signature without enforcing
// expiry is intentional: the same helper is used by debug/admin
// tools that may want to inspect expired keys.
func verifyDesktopLicenseKey(key, privKeyHex string) (*DesktopLicenseClaims, error) {
	if !strings.HasPrefix(key, "SY-") {
		return nil, fmt.Errorf("not a desktop license key (missing SY- prefix)")
	}
	body := strings.TrimPrefix(key, "SY-")

	parts := strings.SplitN(body, ".", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("malformed license key: expected payload.signature")
	}

	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, fmt.Errorf("decode payload: %w", err)
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("decode signature: %w", err)
	}

	privBytes, err := hex.DecodeString(privKeyHex)
	if err != nil || len(privBytes) != 64 {
		return nil, fmt.Errorf("invalid desktop private key: must be 64-byte hex")
	}
	privKey := ed25519.PrivateKey(privBytes)
	pubKey := privKey.Public().(ed25519.PublicKey)

	if !ed25519.Verify(pubKey, payloadBytes, sig) {
		return nil, fmt.Errorf("signature verification failed")
	}

	// Decode the short-key payload. Field names mirror
	// issueDesktopLicenseKey's payload struct exactly.
	var raw struct {
		Product    string `json:"p"`
		Tier       string `json:"t"`
		CustomerID string `json:"c"`
		Email      string `json:"e,omitempty"`
		IssuedAt   int64  `json:"i"`
		ExpiresAt  int64  `json:"x"`
	}
	if err := json.Unmarshal(payloadBytes, &raw); err != nil {
		return nil, fmt.Errorf("parse payload: %w", err)
	}

	out := &DesktopLicenseClaims{
		Product:    raw.Product,
		Tier:       raw.Tier,
		CustomerID: raw.CustomerID,
		Email:      raw.Email,
		IssuedAt:   time.Unix(raw.IssuedAt, 0),
	}
	if raw.ExpiresAt > 0 {
		out.ExpiresAt = time.Unix(raw.ExpiresAt, 0)
	}
	return out, nil
}

// VerifyDesktopLicenseKey is the exported entry point for license
// verification, used by engine wiring to construct the LicenseVerifier
// closure passed to auth.ProxyAuthMiddleware. Returns claims on a
// successful signature check; see verifyDesktopLicenseKey for details.
func VerifyDesktopLicenseKey(key, privKeyHex string) (*DesktopLicenseClaims, error) {
	return verifyDesktopLicenseKey(key, privKeyHex)
}

// LicenseTierGrantsProxy is the exported tier-gate check, used by
// engine wiring to filter out trial / unknown tiers from proxy access.
func LicenseTierGrantsProxy(tier string) bool {
	return licenseTierGrantsProxy(tier)
}

// licenseTierGrantsProxy reports whether the given tier is allowed
// to use the LLM proxy. Per the Apr 16 product decision: every
// PAID tier gets unlimited proxy access included with their
// purchase. Trial tiers do not (we'll convert them with the value
// of the bundle, not give them free LLM calls).
//
// Centralizing this here so the gating rule lives in one place
// rather than scattered across handlers.
func licenseTierGrantsProxy(tier string) bool {
	switch tier {
	case "local", "cloud-single", "cloud-multi":
		return true
	default:
		// Includes "trial-*" tiers and any unknown values. Fail
		// closed — paying customers shouldn't be punished by a
		// new tier name not being listed, but free trials
		// shouldn't get the bonus either.
		return false
	}
}
