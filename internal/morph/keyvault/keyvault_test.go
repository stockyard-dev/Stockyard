package keyvault

import (
	"os"
	"path/filepath"
	"testing"
)

func TestVault_SetGetDelete(t *testing.T) {
	dir := t.TempDir()
	v := &Vault{
		keys: map[string]string{},
		path: filepath.Join(dir, "keys.json"),
	}

	// Set
	if err := v.Set("stripe", "sk_test_123456789"); err != nil {
		t.Fatalf("Set: %v", err)
	}

	// Get
	if got := v.Get("stripe"); got != "sk_test_123456789" {
		t.Errorf("Get = %q, want sk_test_123456789", got)
	}

	// Get missing
	if got := v.Get("nonexistent"); got != "" {
		t.Errorf("Get nonexistent = %q, want empty", got)
	}

	// Delete
	if err := v.Delete("stripe"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if got := v.Get("stripe"); got != "" {
		t.Errorf("Get after delete = %q, want empty", got)
	}
}

func TestVault_List(t *testing.T) {
	v := &Vault{
		keys: map[string]string{
			"stripe": "sk_test_123456789",
			"github": "ghp_abc",
			"tiny":   "abc",
		},
		path: filepath.Join(t.TempDir(), "keys.json"),
	}

	list := v.List()

	// Long key should be redacted with first 4 and last 4
	if got := list["stripe"]; got != "sk_t...6789" {
		t.Errorf("stripe redacted = %q, want sk_t...6789", got)
	}
	// Short key should be fully masked
	if got := list["tiny"]; got != "****" {
		t.Errorf("tiny redacted = %q, want ****", got)
	}
}

func TestVault_ConfiguredServices(t *testing.T) {
	v := &Vault{
		keys: map[string]string{"stripe": "key1", "github": "key2"},
		path: filepath.Join(t.TempDir(), "keys.json"),
	}

	svcs := v.ConfiguredServices()
	if len(svcs) != 2 {
		t.Errorf("expected 2 services, got %d", len(svcs))
	}
	found := map[string]bool{}
	for _, s := range svcs {
		found[s] = true
	}
	if !found["stripe"] || !found["github"] {
		t.Errorf("expected stripe and github, got %v", svcs)
	}
}

func TestVault_SaveLoad_Plaintext(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "keys.json")

	// Create and save
	v1 := &Vault{
		keys: map[string]string{},
		path: path,
	}
	v1.Set("stripe", "sk_test_abc")

	// Load into new vault
	v2 := &Vault{
		keys: map[string]string{},
		path: path,
	}
	v2.load()

	if got := v2.Get("stripe"); got != "sk_test_abc" {
		t.Errorf("loaded value = %q, want sk_test_abc", got)
	}
}

func TestVault_SaveLoad_Encrypted(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "keys.json")

	// Create with passphrase and save
	v1 := &Vault{
		keys: map[string]string{},
		path: path,
		pass: "test-passphrase-123",
	}
	if err := v1.Set("github", "ghp_testtoken"); err != nil {
		t.Fatalf("Set: %v", err)
	}

	// Load into new vault with same passphrase
	v2 := &Vault{
		keys: map[string]string{},
		path: path,
		pass: "test-passphrase-123",
	}
	v2.load()

	if got := v2.Get("github"); got != "ghp_testtoken" {
		t.Errorf("loaded encrypted value = %q, want ghp_testtoken", got)
	}

	// Try loading with wrong passphrase should not load keys
	v3 := &Vault{
		keys: map[string]string{},
		path: path,
		pass: "wrong-passphrase",
	}
	v3.load()

	if got := v3.Get("github"); got != "" {
		t.Errorf("wrong passphrase should not decrypt, got %q", got)
	}
}

func TestVault_LoadMissing(t *testing.T) {
	v := &Vault{
		keys: map[string]string{},
		path: filepath.Join(t.TempDir(), "nonexistent", "keys.json"),
	}
	v.load() // should not panic
	if len(v.keys) != 0 {
		t.Error("expected empty keys after loading missing file")
	}
}

func TestVault_LoadFromEnv(t *testing.T) {
	dir := t.TempDir()
	v := &Vault{
		keys: map[string]string{},
		path: filepath.Join(dir, "keys.json"),
	}

	os.Setenv("STRIPE_API_KEY", "sk_env_test")
	defer os.Unsetenv("STRIPE_API_KEY")

	v.loadFromEnv()

	if got := v.Get("stripe"); got != "sk_env_test" {
		t.Errorf("env loaded value = %q, want sk_env_test", got)
	}
}

func TestVault_LoadFromEnv_NoOverride(t *testing.T) {
	dir := t.TempDir()
	v := &Vault{
		keys: map[string]string{"stripe": "existing_key"},
		path: filepath.Join(dir, "keys.json"),
	}

	os.Setenv("STRIPE_API_KEY", "sk_env_test")
	defer os.Unsetenv("STRIPE_API_KEY")

	v.loadFromEnv()

	// Should not override existing key
	if got := v.Get("stripe"); got != "existing_key" {
		t.Errorf("env should not override, got %q, want existing_key", got)
	}
}

func TestHint(t *testing.T) {
	tests := []struct {
		service string
		substr  string
	}{
		{"stripe", "STRIPE_API_KEY"},
		{"github", "GITHUB_TOKEN"},
		{"slack", "SLACK_BOT_TOKEN"},
		{"unknown", "morph keys set unknown"},
	}
	for _, tc := range tests {
		t.Run(tc.service, func(t *testing.T) {
			got := Hint(tc.service)
			if got == "" {
				t.Error("Hint returned empty string")
			}
			if len(tc.substr) > 0 && !contains(got, tc.substr) {
				t.Errorf("Hint(%q) = %q, missing %q", tc.service, got, tc.substr)
			}
		})
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsStr(s, sub))
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func TestDeriveKey(t *testing.T) {
	salt := []byte("test-salt-12345678901234567890ab")
	key := deriveKey("my-passphrase", salt)
	if len(key) != 32 {
		t.Errorf("key length = %d, want 32", len(key))
	}

	// Same inputs should produce same key
	key2 := deriveKey("my-passphrase", salt)
	for i := range key {
		if key[i] != key2[i] {
			t.Fatal("same inputs should produce same key")
		}
	}

	// Different passphrase should produce different key
	key3 := deriveKey("different-passphrase", salt)
	same := true
	for i := range key {
		if key[i] != key3[i] {
			same = false
			break
		}
	}
	if same {
		t.Error("different passphrases should produce different keys")
	}
}
