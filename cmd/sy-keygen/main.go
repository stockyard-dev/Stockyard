// sy-keygen — License key management for Stockyard products.
//
// Usage:
//
//	sy-keygen init                          Generate new Ed25519 keypair
//	sy-keygen issue [flags]                 Issue a license key
//	sy-keygen validate <key>                Validate a license key
//	sy-keygen info <key>                    Show license key details
//
// Examples:
//
//	sy-keygen init > keypair.json
//	export STOCKYARD_SIGNING_KEY=$(cat keypair.json | jq -r .private_key)
//	sy-keygen issue --product costcap --tier pro --customer cus_abc123 --email dev@example.com
//	sy-keygen validate SY-eyJw...
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/stockyard-dev/stockyard/internal/license"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "init":
		cmdInit()
	case "issue":
		cmdIssue()
	case "validate":
		cmdValidate()
	case "info":
		cmdInfo()
	case "help", "--help", "-h":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		usage()
		os.Exit(1)
	}
}

func usage() {
	fmt.Println(`sy-keygen — Stockyard license key management

Commands:
  init                Generate new Ed25519 keypair (JSON to stdout)
  issue [flags]       Issue a signed license key
  validate <key>      Validate a license key
  info <key>          Show license key details

Issue flags:
  --product <slug>    Product slug or "stockyard" for suite (default: "stockyard")
  --tier <tier>       free|starter|pro|team|enterprise (default: "pro")
  --customer <id>     Customer ID (required)
  --email <email>     Customer email (optional)
  --duration <days>   Key validity in days (default: 365, 0 = forever)
  --seats <n>         Max concurrent instances (team/enterprise only)

Environment:
  STOCKYARD_SIGNING_KEY    Base64 private key (from init output)
  STOCKYARD_PUBLIC_KEY     Base64 public key (for validate/info)`)
}

func cmdInit() {
	kp, err := license.GenerateKeyPair()
	if err != nil {
		fatal("generate keypair: %v", err)
	}

	out := map[string]string{
		"public_key":  kp.PublicKeyB64(),
		"private_key": kp.PrivateKeyB64(),
		"note":        "Store private_key securely (API backend). Embed public_key in binaries via -ldflags.",
		"ldflags":     fmt.Sprintf(`-X github.com/stockyard-dev/stockyard/internal/license.ProductionPublicKey=%s`, kp.PublicKeyB64()),
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.Encode(out)
}

func cmdIssue() {
	// Parse flags manually (no flag package dependency for simplicity)
	args := os.Args[2:]
	product := "stockyard"
	tier := "pro"
	customer := ""
	email := ""
	days := 365
	seats := 0

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--product":
			i++; product = args[i]
		case "--tier":
			i++; tier = args[i]
		case "--customer":
			i++; customer = args[i]
		case "--email":
			i++; email = args[i]
		case "--duration":
			i++; fmt.Sscanf(args[i], "%d", &days)
		case "--seats":
			i++; fmt.Sscanf(args[i], "%d", &seats)
		default:
			fatal("unknown flag: %s", args[i])
		}
	}

	if customer == "" {
		fatal("--customer is required")
	}

	// Load signing key from environment
	privB64 := os.Getenv("STOCKYARD_SIGNING_KEY")
	if privB64 == "" {
		fatal("STOCKYARD_SIGNING_KEY not set. Run 'sy-keygen init' first.")
	}

	// We need the public key too to construct the keypair
	pubB64 := os.Getenv("STOCKYARD_PUBLIC_KEY")
	if pubB64 == "" {
		fatal("STOCKYARD_PUBLIC_KEY not set. Run 'sy-keygen init' first.")
	}

	kp, err := license.LoadKeyPair(pubB64, privB64)
	if err != nil {
		fatal("load keypair: %v", err)
	}

	var duration time.Duration
	if days > 0 {
		duration = time.Duration(days) * 24 * time.Hour
	}

	key, err := kp.Issue(license.IssueRequest{
		Product:    product,
		Tier:       license.TierFromString(tier),
		CustomerID: customer,
		Email:      email,
		Duration:   duration,
		MaxSeats:   seats,
	})
	if err != nil {
		fatal("issue key: %v", err)
	}

	// Output
	fmt.Println(key)
	fmt.Fprintf(os.Stderr, "\n  Product:  %s\n", product)
	fmt.Fprintf(os.Stderr, "  Tier:     %s\n", tier)
	fmt.Fprintf(os.Stderr, "  Customer: %s\n", customer)
	if email != "" {
		fmt.Fprintf(os.Stderr, "  Email:    %s\n", email)
	}
	if days > 0 {
		fmt.Fprintf(os.Stderr, "  Expires:  %s (%d days)\n", time.Now().Add(duration).Format("2006-01-02"), days)
	} else {
		fmt.Fprintf(os.Stderr, "  Expires:  never\n")
	}
	if seats > 0 {
		fmt.Fprintf(os.Stderr, "  Seats:    %d\n", seats)
	}
	fmt.Fprintf(os.Stderr, "\n  Set STOCKYARD_LICENSE_KEY=%s\n", key)
}

func cmdValidate() {
	if len(os.Args) < 3 {
		fatal("usage: sy-keygen validate <key>")
	}
	key := os.Args[2]

	// Set public key if available
	if pub := os.Getenv("STOCKYARD_PUBLIC_KEY"); pub != "" {
		license.ProductionPublicKey = pub
	}

	lic := license.Validate(key)
	if !lic.Valid {
		fmt.Println("❌ INVALID — key is malformed or signature verification failed")
		os.Exit(1)
	}

	if lic.IsExpired() {
		fmt.Printf("⚠️  EXPIRED — key expired on %s\n", lic.ExpiresAt.Format("2006-01-02"))
		os.Exit(1)
	}

	fmt.Println("✅ VALID")
	os.Exit(0)
}

func cmdInfo() {
	if len(os.Args) < 3 {
		fatal("usage: sy-keygen info <key>")
	}
	key := os.Args[2]

	// Set public key if available
	if pub := os.Getenv("STOCKYARD_PUBLIC_KEY"); pub != "" {
		license.ProductionPublicKey = pub
	}

	lic := license.Validate(key)

	fmt.Printf("Valid:      %v\n", lic.Valid)
	if !lic.Valid {
		fmt.Println("(key is malformed or signature failed)")
		return
	}

	fmt.Printf("Product:    %s\n", lic.Payload.Product)
	fmt.Printf("Tier:       %s\n", lic.Payload.Tier)
	fmt.Printf("Customer:   %s\n", lic.Payload.CustomerID)
	if lic.Payload.Email != "" {
		fmt.Printf("Email:      %s\n", lic.Payload.Email)
	}
	fmt.Printf("Issued:     %s\n", lic.IssuedAt.Format("2006-01-02 15:04:05"))
	if lic.Payload.ExpiresAt > 0 {
		fmt.Printf("Expires:    %s\n", lic.ExpiresAt.Format("2006-01-02 15:04:05"))
		if lic.IsExpired() {
			fmt.Printf("Status:     EXPIRED\n")
		} else {
			days := int(time.Until(lic.ExpiresAt).Hours() / 24)
			fmt.Printf("Status:     active (%d days remaining)\n", days)
		}
	} else {
		fmt.Printf("Expires:    never\n")
		fmt.Printf("Status:     active (perpetual)\n")
	}
	if lic.Payload.MaxSeats > 0 {
		fmt.Printf("Max Seats:  %d\n", lic.Payload.MaxSeats)
	}

	// Show tier limits
	lim := license.Limits(lic.Payload.Tier)
	fmt.Println("\nTier Limits:")
	if lim.MaxRequestsPerDay > 0 {
		fmt.Printf("  Daily:    %d requests\n", lim.MaxRequestsPerDay)
	} else {
		fmt.Printf("  Daily:    unlimited\n")
	}
	if lim.MaxRequestsPerMonth > 0 {
		fmt.Printf("  Monthly:  %d requests\n", lim.MaxRequestsPerMonth)
	} else {
		fmt.Printf("  Monthly:  unlimited\n")
	}

	features := []string{}
	if lim.DashboardAccess { features = append(features, "dashboard") }
	if lim.APIAccess { features = append(features, "api") }
	if lim.ExportAccess { features = append(features, "export") }
	if lim.MultiInstance { features = append(features, "multi-instance") }
	if lim.WhiteLabel { features = append(features, "whitelabel") }
	fmt.Printf("  Features: %s\n", strings.Join(features, ", "))
}

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "error: "+format+"\n", args...)
	os.Exit(1)
}
