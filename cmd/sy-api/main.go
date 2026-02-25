// sy-api — Stockyard API backend.
//
// Handles Stripe checkout sessions, webhook processing, license key generation,
// email delivery, and admin operations.
//
// Required environment variables:
//
//	STRIPE_SECRET_KEY       Stripe secret key (sk_live_... or sk_test_...)
//	STRIPE_WEBHOOK_SECRET   Stripe webhook signing secret (whsec_...)
//	STOCKYARD_PUBLIC_KEY    Ed25519 public key for license signing
//	STOCKYARD_SIGNING_KEY   Ed25519 private key for license signing
//
// Optional:
//
//	PORT                    HTTP port (default: 8080)
//	DATABASE_PATH           SQLite database path (default: ./stockyard-api.sqlite)
//	STOCKYARD_ADMIN_KEY     Admin API key for protected endpoints
//	SMTP_HOST               SMTP server for email delivery
//	SMTP_PORT               SMTP port (default: 587)
//	SMTP_USERNAME           SMTP username
//	SMTP_PASSWORD           SMTP password
//	SMTP_FROM               From address (default: hello@stockyard.dev)
//	RESEND_API_KEY          Resend API key (alternative to SMTP)
//	STRIPE_SUCCESS_URL      Checkout success redirect
//	STRIPE_CANCEL_URL       Checkout cancel redirect
//	STRIPE_PRICE_{PRODUCT}_{TIER}  Stripe price IDs per product/tier
//
// Stripe price ID examples:
//
//	STRIPE_PRICE_DEFAULT_STANDARD=price_1abc...   (Individual product $9.99/mo)
//	STRIPE_PRICE_STOCKYARD_PRO=price_2def...      (Pro — all products $29.99/mo)
package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"

	"github.com/stockyard-dev/stockyard/internal/apiserver"
	"github.com/stockyard-dev/stockyard/internal/license"
)

func main() {
	log.SetFlags(log.Ltime | log.Lshortfile)

	// Handle --help
	if len(os.Args) > 1 && (os.Args[1] == "--help" || os.Args[1] == "-h" || os.Args[1] == "help") {
		fmt.Println("sy-api — Stockyard API backend")
		fmt.Println()
		fmt.Println("Handles Stripe checkout, webhook processing, license key generation, and email delivery.")
		fmt.Println()
		fmt.Println("Environment variables:")
		fmt.Println("  STRIPE_SECRET_KEY       Stripe secret key (required)")
		fmt.Println("  STRIPE_WEBHOOK_SECRET   Stripe webhook secret (required)")
		fmt.Println("  STOCKYARD_PUBLIC_KEY    Ed25519 public key (required)")
		fmt.Println("  STOCKYARD_SIGNING_KEY   Ed25519 private key (required)")
		fmt.Println("  PORT                    HTTP port (default: 8080)")
		fmt.Println("  DATABASE_PATH           SQLite path (default: ./stockyard-api.db)")
		fmt.Println("  STOCKYARD_ADMIN_KEY     Admin API key")
		fmt.Println("  SMTP_HOST / RESEND_API_KEY   Email provider")
		fmt.Println()
		fmt.Println("Quick start:")
		fmt.Println("  1. sy-keygen init > keypair.json")
		fmt.Println("  2. Export STOCKYARD_PUBLIC_KEY and STOCKYARD_SIGNING_KEY from keypair.json")
		fmt.Println("  3. Export STRIPE_SECRET_KEY and STRIPE_WEBHOOK_SECRET")
		fmt.Println("  4. Set STRIPE_PRICE_STOCKYARD_PRO=price_... for each product/tier")
		fmt.Println("  5. sy-api")
		os.Exit(0)
	}

	// Load config from environment
	port := 8080
	if p := os.Getenv("PORT"); p != "" {
		if n, err := strconv.Atoi(p); err == nil {
			port = n
		}
	}

	dbPath := os.Getenv("DATABASE_PATH")
	if dbPath == "" {
		dbPath = "./stockyard-api.sqlite"
	}

	// Validate required env vars
	pubKey := os.Getenv("STOCKYARD_PUBLIC_KEY")
	privKey := os.Getenv("STOCKYARD_SIGNING_KEY")
	if pubKey == "" || privKey == "" {
		log.Println("⚠️  STOCKYARD_PUBLIC_KEY and STOCKYARD_SIGNING_KEY not set.")
		log.Println("   Generating ephemeral keypair for development...")
		kp, err := license.GenerateKeyPair()
		if err != nil {
			log.Fatalf("generate keypair: %v", err)
		}
		pubKey = kp.PublicKeyB64()
		privKey = kp.PrivateKeyB64()
		log.Printf("   Public key:  %s", pubKey)
		log.Printf("   Private key: %s", privKey)
		log.Println("   ⚠️  Keys are ephemeral — set env vars for production!")
	}

	// Set the production public key so license.Validate() works
	license.ProductionPublicKey = pubKey

	kp, err := license.LoadKeyPair(pubKey, privKey)
	if err != nil {
		log.Fatalf("load keypair: %v", err)
	}

	stripeKey := os.Getenv("STRIPE_SECRET_KEY")
	if stripeKey == "" {
		log.Println("⚠️  STRIPE_SECRET_KEY not set — checkout/portal endpoints will fail")
	}

	// Open database
	db, err := apiserver.OpenSqliteDB(dbPath)
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer db.Close()

	// Auto-import any legacy JSON files from the same directory
	if err := db.ImportLegacyJSON(filepath.Dir(dbPath)); err != nil {
		log.Printf("legacy import: %v", err)
	}

	// Initialize components
	stripeCfg := apiserver.GetStripeConfigFromEnv()
	stripe := apiserver.NewStripeClient(stripeCfg)
	mailer := apiserver.NewMailer()

	// Create and start server
	srv := apiserver.NewServer(
		apiserver.ServerConfig{
			Port:     port,
			DBPath:   dbPath,
			AdminKey: os.Getenv("STOCKYARD_ADMIN_KEY"),
		},
		db, stripe, kp, mailer,
	)

	log.Printf("══════════════════════════════════════")
	log.Printf("  Stockyard API is running")
	log.Printf("  Port:     http://localhost:%d", port)
	log.Printf("  Health:   http://localhost:%d/health", port)
	log.Printf("  Webhook:  POST /webhooks/stripe")
	log.Printf("  Checkout: POST /api/checkout")
	log.Printf("  Products: GET /api/products")
	if os.Getenv("STOCKYARD_ADMIN_KEY") != "" {
		log.Printf("  Admin:    GET /api/admin/stats")
	}
	log.Printf("  Database: %s", dbPath)
	log.Printf("══════════════════════════════════════")

	if err := srv.Start(); err != nil {
		log.Fatalf("server: %v", err)
	}
}
