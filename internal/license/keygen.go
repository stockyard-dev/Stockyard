package license

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"
)

// KeyPair holds an Ed25519 signing keypair.
type KeyPair struct {
	PublicKey  ed25519.PublicKey
	PrivateKey ed25519.PrivateKey
}

// GenerateKeyPair creates a new Ed25519 keypair for license signing.
func GenerateKeyPair() (*KeyPair, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("license: generate keypair: %w", err)
	}
	return &KeyPair{PublicKey: pub, PrivateKey: priv}, nil
}

// LoadKeyPair loads a keypair from base64-encoded strings.
func LoadKeyPair(pubB64, privB64 string) (*KeyPair, error) {
	pub, err := base64.RawURLEncoding.DecodeString(pubB64)
	if err != nil {
		return nil, fmt.Errorf("license: decode public key: %w", err)
	}
	priv, err := base64.RawURLEncoding.DecodeString(privB64)
	if err != nil {
		return nil, fmt.Errorf("license: decode private key: %w", err)
	}
	if len(pub) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("license: invalid public key size %d", len(pub))
	}
	if len(priv) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("license: invalid private key size %d", len(priv))
	}
	return &KeyPair{PublicKey: ed25519.PublicKey(pub), PrivateKey: ed25519.PrivateKey(priv)}, nil
}

// PublicKeyB64 returns the base64url-encoded public key (for embedding in binaries).
func (kp *KeyPair) PublicKeyB64() string {
	return base64.RawURLEncoding.EncodeToString(kp.PublicKey)
}

// PrivateKeyB64 returns the base64url-encoded private key (store securely in API backend).
func (kp *KeyPair) PrivateKeyB64() string {
	return base64.RawURLEncoding.EncodeToString(kp.PrivateKey)
}

// IssueRequest contains the parameters for issuing a new license key.
type IssueRequest struct {
	Product    string        // product slug or "stockyard" for suite or "*" for any
	Tier       Tier          // pricing tier
	CustomerID string        // Stripe customer ID
	Email      string        // customer email
	Duration   time.Duration // 0 = no expiry
	MaxSeats   int           // team/enterprise only
}

// Issue creates a signed license key string.
func (kp *KeyPair) Issue(req IssueRequest) (string, error) {
	if kp.PrivateKey == nil {
		return "", fmt.Errorf("license: cannot issue without private key")
	}

	now := time.Now()
	payload := Payload{
		Product:    req.Product,
		Tier:       req.Tier,
		CustomerID: req.CustomerID,
		Email:      req.Email,
		IssuedAt:   now.Unix(),
		MaxSeats:   req.MaxSeats,
	}
	if req.Duration > 0 {
		payload.ExpiresAt = now.Add(req.Duration).Unix()
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("license: marshal payload: %w", err)
	}

	sig := ed25519.Sign(kp.PrivateKey, payloadBytes)

	key := fmt.Sprintf("SY-%s.%s",
		base64.RawURLEncoding.EncodeToString(payloadBytes),
		base64.RawURLEncoding.EncodeToString(sig),
	)

	return key, nil
}

// IssuePro creates a pro-tier key for the given product.
func (kp *KeyPair) IssuePro(product, customerID, email string) (string, error) {
	return kp.Issue(IssueRequest{
		Product:    product,
		Tier:       TierPro,
		CustomerID: customerID,
		Email:      email,
		Duration:   365 * 24 * time.Hour, // 1 year
	})
}

// IssueCloud creates a cloud-tier key.
func (kp *KeyPair) IssueCloud(customerID, email string) (string, error) {
	return kp.Issue(IssueRequest{
		Product:    "stockyard",
		Tier:       TierCloud,
		CustomerID: customerID,
		Email:      email,
		Duration:   365 * 24 * time.Hour,
	})
}

// IssueEnterprise creates an enterprise-tier key with seat count.
func (kp *KeyPair) IssueEnterprise(customerID, email string, seats int) (string, error) {
	return kp.Issue(IssueRequest{
		Product:    "stockyard",
		Tier:       TierEnterprise,
		CustomerID: customerID,
		Email:      email,
		Duration:   365 * 24 * time.Hour,
		MaxSeats:   seats,
	})
}
