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

// genTestPrivKey returns a hex-encoded Ed25519 private key suitable
// for the priv-key arguments to issueDesktopLicenseKey and
// verifyDesktopLicenseKey. Generated fresh per test so tests can
// run in parallel without sharing key material.
func genTestPrivKey(t *testing.T) string {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("generate ed25519 key: %v", err)
	}
	return hex.EncodeToString(priv)
}

// TestVerifyDesktopLicenseKey_RoundTrip: issue a key with
// issueDesktopLicenseKey, verify it with verifyDesktopLicenseKey,
// confirm the claims round-trip cleanly.
func TestVerifyDesktopLicenseKey_RoundTrip(t *testing.T) {
	priv := genTestPrivKey(t)

	key, err := issueDesktopLicenseKey(priv, "cloud-single", "cus_ABC", "founder@example.com", 0)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	if !strings.HasPrefix(key, "SY-") {
		t.Errorf("expected SY- prefix, got %q", key[:8])
	}

	claims, err := verifyDesktopLicenseKey(key, priv)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if claims.Tier != "cloud-single" {
		t.Errorf("tier: got %q, want cloud-single", claims.Tier)
	}
	if claims.CustomerID != "cus_ABC" {
		t.Errorf("customer: got %q, want cus_ABC", claims.CustomerID)
	}
	if claims.Email != "founder@example.com" {
		t.Errorf("email: got %q", claims.Email)
	}
	if claims.Product != "stockyard" {
		t.Errorf("product: got %q, want stockyard", claims.Product)
	}
	// IssuedAt should be within the last few seconds.
	if d := time.Since(claims.IssuedAt); d > 5*time.Second || d < 0 {
		t.Errorf("issued_at unrealistic: %v ago", d)
	}
	// ExpiresAt was passed as 0 → permanent license, zero time.
	if !claims.ExpiresAt.IsZero() {
		t.Errorf("expected zero expiry for permanent license, got %v", claims.ExpiresAt)
	}
	if claims.IsExpired() {
		t.Error("permanent license should not be expired")
	}
}

// TestVerifyDesktopLicenseKey_WrongKey: a key signed with one private
// key fails verification against a different private key. Critical for
// the security model — without this, anyone with the public key spec
// could mint working license keys.
func TestVerifyDesktopLicenseKey_WrongKey(t *testing.T) {
	privA := genTestPrivKey(t)
	privB := genTestPrivKey(t)

	key, err := issueDesktopLicenseKey(privA, "local", "cus_X", "x@example.com", 0)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}

	_, err = verifyDesktopLicenseKey(key, privB)
	if err == nil {
		t.Fatal("expected verification failure with wrong key, got nil")
	}
	if !strings.Contains(err.Error(), "signature") {
		t.Errorf("expected 'signature' in error, got: %v", err)
	}
}

// TestVerifyDesktopLicenseKey_TamperedPayload: flip a bit in the
// payload portion and verify rejects.
func TestVerifyDesktopLicenseKey_TamperedPayload(t *testing.T) {
	priv := genTestPrivKey(t)
	key, err := issueDesktopLicenseKey(priv, "cloud-multi", "cus_T", "t@example.com", 0)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	// Take "SY-<payload>.<sig>", forge a different payload that
	// claims a higher tier, keep the original signature.
	body := strings.TrimPrefix(key, "SY-")
	parts := strings.SplitN(body, ".", 2)
	forged := map[string]any{
		"p": "stockyard", "t": "cloud-multi",
		"c": "cus_T", "e": "t@example.com",
		"i": time.Now().Unix(), "x": 0,
		"forged": true,
	}
	forgedBytes, _ := json.Marshal(forged)
	tampered := "SY-" + base64.RawURLEncoding.EncodeToString(forgedBytes) + "." + parts[1]

	_, err = verifyDesktopLicenseKey(tampered, priv)
	if err == nil {
		t.Fatal("expected tampered payload to fail verification")
	}
}

// TestVerifyDesktopLicenseKey_MissingPrefix: a token without SY-
// is rejected immediately so the caller can try other auth paths
// (sk-sy-, BYOK).
func TestVerifyDesktopLicenseKey_MissingPrefix(t *testing.T) {
	priv := genTestPrivKey(t)
	_, err := verifyDesktopLicenseKey("not-a-license-key", priv)
	if err == nil {
		t.Fatal("expected error for missing SY- prefix")
	}
	if !strings.Contains(err.Error(), "SY-") {
		t.Errorf("expected 'SY-' mention in error, got: %v", err)
	}
}

// TestVerifyDesktopLicenseKey_Malformed: missing the dot separator,
// bad base64, etc.
func TestVerifyDesktopLicenseKey_Malformed(t *testing.T) {
	priv := genTestPrivKey(t)
	cases := []string{
		"SY-",                    // just the prefix
		"SY-noseparator",         // no dot
		"SY-not!base64.alsobad!", // bad base64 in payload
	}
	for _, c := range cases {
		if _, err := verifyDesktopLicenseKey(c, priv); err == nil {
			t.Errorf("expected error for malformed key %q", c)
		}
	}
}

// TestDesktopLicenseClaims_IsExpired: zero ExpiresAt is permanent;
// past ExpiresAt is expired; future is not.
func TestDesktopLicenseClaims_IsExpired(t *testing.T) {
	zero := DesktopLicenseClaims{}
	if zero.IsExpired() {
		t.Error("zero ExpiresAt should be treated as permanent (not expired)")
	}
	past := DesktopLicenseClaims{ExpiresAt: time.Now().Add(-time.Hour)}
	if !past.IsExpired() {
		t.Error("past ExpiresAt should be expired")
	}
	future := DesktopLicenseClaims{ExpiresAt: time.Now().Add(time.Hour)}
	if future.IsExpired() {
		t.Error("future ExpiresAt should not be expired")
	}
}

// TestVerifyDesktopLicenseKey_ExpiringKey: issue a key with a
// past ExpiresAt and verify IsExpired() returns true.
func TestVerifyDesktopLicenseKey_ExpiringKey(t *testing.T) {
	priv := genTestPrivKey(t)
	pastExp := time.Now().Add(-time.Hour).Unix()
	key, err := issueDesktopLicenseKey(priv, "cloud-single", "cus_E", "e@example.com", pastExp)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	claims, err := verifyDesktopLicenseKey(key, priv)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if !claims.IsExpired() {
		t.Errorf("expected expired claims, IsExpired=false. ExpiresAt=%v", claims.ExpiresAt)
	}
}

// TestLicenseTierGrantsProxy: the gate function returns true ONLY
// for the three paying tiers. Trial tiers, empty strings, future
// unknown tiers all reject (fail-closed).
func TestLicenseTierGrantsProxy(t *testing.T) {
	allowed := []string{"local", "cloud-single", "cloud-multi"}
	for _, tier := range allowed {
		if !licenseTierGrantsProxy(tier) {
			t.Errorf("expected %q to grant proxy access", tier)
		}
	}
	denied := []string{"", "trial-cloud-single", "trial-local", "enterprise", "free", "unknown-future-tier"}
	for _, tier := range denied {
		if licenseTierGrantsProxy(tier) {
			t.Errorf("expected %q to be denied (fail-closed)", tier)
		}
	}
}

// TestVerifyDesktopLicenseKey_BadPrivKey: the priv-key arg must be
// valid 64-byte hex. Other shapes should error with a clear message.
func TestVerifyDesktopLicenseKey_BadPrivKey(t *testing.T) {
	cases := []struct {
		name, key string
	}{
		{"empty", ""},
		{"too short", "abcd"},
		{"not hex", "z" + strings.Repeat("a", 127)},
		{"wrong length but hex", strings.Repeat("ab", 30)}, // 60 bytes, not 64
	}
	// Use a real key to build a valid-shape token, then verify with bad keys.
	realPriv := genTestPrivKey(t)
	tok, err := issueDesktopLicenseKey(realPriv, "local", "cus_X", "", 0)
	if err != nil {
		t.Fatalf("issue with real key: %v", err)
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := verifyDesktopLicenseKey(tok, c.key); err == nil {
				t.Errorf("expected error for invalid priv key %q", c.name)
			}
		})
	}
}
