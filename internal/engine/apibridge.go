// Package engine — apibridge mounts the sy-api billing/licensing/cloud/exchange
// routes onto the unified stockyard binary's HTTP mux. When STRIPE_SECRET_KEY
// (or DATABASE_PATH) is set, all apiserver endpoints become available alongside
// the proxy and 6 flagship app endpoints on the same port.
package engine

import (
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/stockyard-dev/stockyard/internal/apiserver"
	"github.com/stockyard-dev/stockyard/internal/license"
)

// mountAPIServer checks for apiserver env vars and, if present, initializes
// the billing/licensing/cloud/exchange server and mounts its routes on mux.
// Returns true if mounted, false if skipped (env vars not set).
func mountAPIServer(mux *http.ServeMux, dataDir string) bool {
	// The apiserver needs at minimum a database path. Stripe keys are optional
	// (endpoints will fail gracefully without them). This lets the unified
	// binary always serve /api/products, /api/exchange, etc.
	dbPath := os.Getenv("DATABASE_PATH")
	if dbPath == "" {
		// Default: put the API database alongside the proxy database
		dbPath = filepath.Join(dataDir, "stockyard-api.sqlite")
	}

	// License keypair — generate ephemeral if not configured
	pubKey := os.Getenv("STOCKYARD_PUBLIC_KEY")
	privKey := os.Getenv("STOCKYARD_SIGNING_KEY")
	if pubKey == "" || privKey == "" {
		log.Println("[apibridge] STOCKYARD_PUBLIC_KEY / STOCKYARD_SIGNING_KEY not set — generating ephemeral keypair")
		kp, err := license.GenerateKeyPair()
		if err != nil {
			log.Printf("[apibridge] failed to generate keypair: %v", err)
			return false
		}
		pubKey = kp.PublicKeyB64()
		privKey = kp.PrivateKeyB64()
	}
	license.ProductionPublicKey = pubKey

	kp, err := license.LoadKeyPair(pubKey, privKey)
	if err != nil {
		log.Printf("[apibridge] load keypair: %v", err)
		return false
	}

	// Open apiserver database (separate from proxy's storage.DB)
	db, err := apiserver.OpenSqliteDB(dbPath)
	if err != nil {
		log.Printf("[apibridge] database: %v", err)
		return false
	}

	// Import legacy JSON if present
	if err := db.ImportLegacyJSON(filepath.Dir(dbPath)); err != nil {
		log.Printf("[apibridge] legacy import: %v", err)
	}

	// Initialize Stripe + mailer (both degrade gracefully if unconfigured)
	stripeCfg := apiserver.GetStripeConfigFromEnv()
	stripe := apiserver.NewStripeClient(stripeCfg)
	mailer := apiserver.NewMailer()

	srv := apiserver.NewServer(
		apiserver.ServerConfig{
			DBPath:   dbPath,
			AdminKey: os.Getenv("STOCKYARD_ADMIN_KEY"),
		},
		db, stripe, kp, mailer,
	)

	// Mount all apiserver routes onto the shared mux
	srv.RegisterOnMux(mux)

	// CORS preflight for apiserver paths (stockyard.dev frontend calls these)
	corsHandler := func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin == "" {
			origin = "*"
		}
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Admin-Key")
		w.Header().Set("Access-Control-Max-Age", "86400")
		w.WriteHeader(http.StatusNoContent)
	}
	mux.HandleFunc("OPTIONS /api/checkout", corsHandler)
	mux.HandleFunc("OPTIONS /api/portal", corsHandler)
	mux.HandleFunc("OPTIONS /api/cloud/{path...}", corsHandler)
	mux.HandleFunc("OPTIONS /api/exchange/{path...}", corsHandler)
	mux.HandleFunc("OPTIONS /api/license/{path...}", corsHandler)
	mux.HandleFunc("OPTIONS /api/admin/{path...}", corsHandler)
	mux.HandleFunc("OPTIONS /webhooks/stripe", corsHandler)

	stripeStatus := "not configured"
	if os.Getenv("STRIPE_SECRET_KEY") != "" {
		stripeStatus = "configured"
	}
	log.Printf("[apibridge] mounted: billing, licensing, cloud, exchange (stripe: %s, db: %s)", stripeStatus, dbPath)
	return true
}
