// licensegen issues signed license keys for Stockyard tools.
// Run this on a trusted machine with access to the private key.
// The private key is NEVER embedded in product binaries.
//
// Usage:
//   STOCKYARD_PRIVATE_KEY=<hex> go run ./cmd/licensegen \
//     -product=corral -customer=acme-inc -days=365
//
// Output: a license key string to give to the customer.
// Customer sets it as: CORRAL_LICENSE_KEY=stockyard_...
package main

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"
)

var validProducts = map[string]bool{
	"corral": true,
	"gate":   true,
	"trough": true,
	"fence":  true,
	"brand":  true,
}

func main() {
	product  := flag.String("product",  "",  "Product slug: corral, gate, trough, fence, brand")
	customer := flag.String("customer", "",  "Customer identifier (e.g. acme-inc, user@example.com)")
	days     := flag.Int("days",        365, "License duration in days (0 = never expires)")
	flag.Parse()

	if *product == "" || *customer == "" {
		fmt.Fprintln(os.Stderr, "usage: licensegen -product=<slug> -customer=<id> [-days=365]")
		fmt.Fprintln(os.Stderr, "set STOCKYARD_PRIVATE_KEY env var to the hex-encoded private key")
		os.Exit(1)
	}
	if !validProducts[*product] {
		fmt.Fprintf(os.Stderr, "unknown product %q — valid: corral, gate, trough, fence, brand\n", *product)
		os.Exit(1)
	}

	privKeyHex := os.Getenv("STOCKYARD_PRIVATE_KEY")
	if privKeyHex == "" {
		fmt.Fprintln(os.Stderr, "error: STOCKYARD_PRIVATE_KEY not set")
		os.Exit(1)
	}
	privKeyBytes, err := hex.DecodeString(privKeyHex)
	if err != nil || len(privKeyBytes) != ed25519.PrivateKeySize {
		fmt.Fprintln(os.Stderr, "error: STOCKYARD_PRIVATE_KEY must be 64-byte hex-encoded Ed25519 private key")
		os.Exit(1)
	}
	privKey := ed25519.PrivateKey(privKeyBytes)

	now := time.Now().UTC()
	var expiresAt int64
	if *days > 0 {
		expiresAt = now.Add(time.Duration(*days) * 24 * time.Hour).Unix()
	}

	type payload struct {
		Product    string `json:"p"`
		Tier       string `json:"t"`
		ExpiresAt  int64  `json:"e"`
		CustomerID string `json:"c"`
		IssuedAt   int64  `json:"i"`
	}

	p := payload{
		Product:    *product,
		Tier:       "pro",
		ExpiresAt:  expiresAt,
		CustomerID: *customer,
		IssuedAt:   now.Unix(),
	}

	payloadBytes, _ := json.Marshal(p)
	sig := ed25519.Sign(privKey, payloadBytes)

	keyStr := "stockyard_" +
		base64.RawURLEncoding.EncodeToString(payloadBytes) + "." +
		base64.RawURLEncoding.EncodeToString(sig)

	expStr := "never"
	if expiresAt > 0 {
		expStr = time.Unix(expiresAt, 0).Format("2006-01-02")
	}

	fmt.Printf("\n  License key for %s/%s (expires: %s)\n\n", *product, *customer, expStr)
	fmt.Printf("  %s\n\n", keyStr)
	fmt.Printf("  Customer sets: %s_LICENSE_KEY=%s\n\n",
		strings.ToUpper(*product), keyStr)
}
