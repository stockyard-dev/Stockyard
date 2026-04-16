package apiserver

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// generateDesktopTestKey returns a fresh Ed25519 keypair as hex
// strings, suitable for round-tripping desktop license keys in tests.
func generateDesktopTestKey(t *testing.T) (privHex, pubHex string) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("generate ed25519: %v", err)
	}
	return hex.EncodeToString(priv), hex.EncodeToString(pub)
}

func TestIssueDesktopLicenseKey_TrialPayloadShape(t *testing.T) {
	privHex, pubHex := generateDesktopTestKey(t)

	trialEnd := time.Now().Add(7 * 24 * time.Hour).Unix()
	key, err := issueDesktopLicenseKey(privHex, "trial", "cus_test", "alice@example.com", trialEnd)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}

	if !strings.HasPrefix(key, "SY-") {
		t.Fatalf("expected SY- prefix, got %q", key[:10])
	}

	// Round-trip: split, decode, verify signature, parse payload.
	body := strings.TrimPrefix(key, "SY-")
	parts := strings.Split(body, ".")
	if len(parts) != 2 {
		t.Fatalf("expected 2 parts, got %d", len(parts))
	}
	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		t.Fatalf("decode sig: %v", err)
	}
	pubBytes, _ := hex.DecodeString(pubHex)
	if !ed25519.Verify(ed25519.PublicKey(pubBytes), payloadBytes, sig) {
		t.Fatal("signature failed verification")
	}

	// Field names match what stockyard-desktop/internal/licensing
	// expects — keep these in sync. Drift here = customers can't
	// activate their licenses.
	var p struct {
		Product    string `json:"p"`
		Tier       string `json:"t"`
		CustomerID string `json:"c"`
		Email      string `json:"e"`
		IssuedAt   int64  `json:"i"`
		ExpiresAt  int64  `json:"x"`
	}
	if err := json.Unmarshal(payloadBytes, &p); err != nil {
		t.Fatalf("parse payload: %v", err)
	}
	if p.Product != "stockyard" {
		t.Errorf("Product = %q, want stockyard", p.Product)
	}
	if p.Tier != "trial" {
		t.Errorf("Tier = %q, want trial", p.Tier)
	}
	if p.CustomerID != "cus_test" {
		t.Errorf("CustomerID = %q, want cus_test", p.CustomerID)
	}
	if p.Email != "alice@example.com" {
		t.Errorf("Email = %q, want alice@example.com", p.Email)
	}
	if p.ExpiresAt != trialEnd {
		t.Errorf("ExpiresAt = %d, want %d", p.ExpiresAt, trialEnd)
	}
	if p.IssuedAt == 0 {
		t.Error("IssuedAt should be non-zero")
	}
}

func TestIssueDesktopLicenseKey_LocalHasNoExpiry(t *testing.T) {
	// Local tier brand promise: no expiry, binary works forever.
	// This test guards against accidentally setting a non-zero
	// ExpiresAt on Local licenses (which would break the brand
	// promise even though desktop currently ignores expiry on
	// non-trial tiers).
	privHex, _ := generateDesktopTestKey(t)
	key, err := issueDesktopLicenseKey(privHex, "local", "cus_local", "bob@example.com", 0)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	body := strings.TrimPrefix(key, "SY-")
	parts := strings.Split(body, ".")
	payloadBytes, _ := base64.RawURLEncoding.DecodeString(parts[0])
	var p struct {
		Tier      string `json:"t"`
		ExpiresAt int64  `json:"x"`
	}
	json.Unmarshal(payloadBytes, &p)
	if p.Tier != "local" {
		t.Errorf("Tier = %q, want local", p.Tier)
	}
	if p.ExpiresAt != 0 {
		t.Errorf("Local tier ExpiresAt = %d, want 0 (no expiry)", p.ExpiresAt)
	}
}

func TestIssueDesktopLicenseKey_CloudHasFarFutureExpiry(t *testing.T) {
	// Cloud tiers: expiry is set ~10 years out. Desktop ignores
	// expiry on cloud per docs/TIERS.md, but we set a non-zero value
	// so the field isn't suspiciously empty.
	privHex, _ := generateDesktopTestKey(t)
	tenYears := time.Now().Add(10 * 365 * 24 * time.Hour).Unix()
	key, err := issueDesktopLicenseKey(privHex, "cloud-single", "cus_cloud", "c@example.com", tenYears)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	body := strings.TrimPrefix(key, "SY-")
	parts := strings.Split(body, ".")
	payloadBytes, _ := base64.RawURLEncoding.DecodeString(parts[0])
	var p struct {
		ExpiresAt int64 `json:"x"`
	}
	json.Unmarshal(payloadBytes, &p)
	if p.ExpiresAt == 0 {
		t.Error("Cloud tier should have non-zero ExpiresAt")
	}
	// Should be within a minute of tenYears (clock drift tolerance).
	if abs(p.ExpiresAt-tenYears) > 60 {
		t.Errorf("ExpiresAt drift: got %d, want ~%d", p.ExpiresAt, tenYears)
	}
}

func TestIssueDesktopLicenseKey_BadKeyRejected(t *testing.T) {
	cases := map[string]string{
		"empty":           "",
		"too short":       "deadbeef",
		"odd hex":         "abc",
		"non-hex chars":   "zzzz" + strings.Repeat("00", 62),
		"wrong byte len":  strings.Repeat("00", 32), // 32 bytes, want 64
	}
	for name, badKey := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := issueDesktopLicenseKey(badKey, "trial", "cus_x", "a@b.c", time.Now().Unix())
			if err == nil {
				t.Errorf("expected error for %q, got nil", name)
			}
		})
	}
}

func TestDesktopTierDisplayName(t *testing.T) {
	cases := map[string]string{
		"local":         "Local (one-time, $99)",
		"cloud-single":  "Cloud Single Site",
		"cloud-multi":   "Cloud Multi-Site",
		"unknown-tier":  "unknown-tier", // pass-through
	}
	for tier, want := range cases {
		got := desktopTierDisplayName(tier)
		if got != want {
			t.Errorf("desktopTierDisplayName(%q) = %q, want %q", tier, got, want)
		}
	}
}

func abs(x int64) int64 {
	if x < 0 {
		return -x
	}
	return x
}
