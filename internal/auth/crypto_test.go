package auth

import (
	"crypto/rand"
	"io"
	"strings"
	"testing"
)

func TestEncryptDecrypt(t *testing.T) {
	key := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		t.Fatal(err)
	}

	tests := []string{
		"sk-proj-abc123",
		"sk-ant-api03-longkey",
		"gsk_shortkey",
		"",
	}

	for _, plaintext := range tests {
		encrypted, err := encrypt(plaintext, key)
		if err != nil {
			t.Fatalf("encrypt(%q): %v", plaintext, err)
		}
		if plaintext != "" && !strings.HasPrefix(encrypted, encPrefix) {
			t.Errorf("encrypted value should start with %q, got %q", encPrefix, encrypted[:10])
		}

		decrypted, err := decrypt(encrypted, key)
		if err != nil {
			t.Fatalf("decrypt: %v", err)
		}
		if decrypted != plaintext {
			t.Errorf("roundtrip failed: got %q, want %q", decrypted, plaintext)
		}
	}
}

func TestDecryptLegacyPlaintext(t *testing.T) {
	key := make([]byte, 32)
	io.ReadFull(rand.Reader, key)

	// Legacy plaintext (no "enc:" prefix) should pass through unchanged
	legacy := "sk-proj-plaintext-legacy-key"
	result, err := decrypt(legacy, key)
	if err != nil {
		t.Fatalf("decrypt legacy: %v", err)
	}
	if result != legacy {
		t.Errorf("legacy passthrough failed: got %q, want %q", result, legacy)
	}
}

func TestEncryptDifferentCiphertexts(t *testing.T) {
	key := make([]byte, 32)
	io.ReadFull(rand.Reader, key)

	// Same plaintext should produce different ciphertexts (random nonce)
	plain := "sk-proj-test123"
	enc1, _ := encrypt(plain, key)
	enc2, _ := encrypt(plain, key)
	if enc1 == enc2 {
		t.Error("same plaintext produced identical ciphertexts — nonce reuse")
	}

	// Both should decrypt to the same value
	dec1, _ := decrypt(enc1, key)
	dec2, _ := decrypt(enc2, key)
	if dec1 != plain || dec2 != plain {
		t.Error("decryption after different encryptions failed")
	}
}

func TestDecryptWrongKey(t *testing.T) {
	key1 := make([]byte, 32)
	key2 := make([]byte, 32)
	io.ReadFull(rand.Reader, key1)
	io.ReadFull(rand.Reader, key2)

	encrypted, _ := encrypt("sk-proj-secret", key1)
	_, err := decrypt(encrypted, key2)
	if err == nil {
		t.Error("decrypt with wrong key should fail")
	}
}
