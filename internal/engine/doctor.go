package engine

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/stockyard-dev/stockyard/internal/config"
	_ "modernc.org/sqlite"
)

type checkResult struct {
	Name   string `json:"name"`
	Status string `json:"status"` // pass, warn, fail
	Detail string `json:"detail"`
}

// RunDoctor performs health checks and prints diagnostics.
func RunDoctor(pc ProductConfig) {
	fmt.Println()
	fmt.Printf("  Stockyard Doctor — %s %s\n", pc.Name, pc.Version)
	fmt.Printf("  %s/%s, %d CPUs\n", runtime.GOOS, runtime.GOARCH, runtime.NumCPU())
	fmt.Println()

	checks := []checkResult{}

	// 1. Go runtime
	checks = append(checks, checkResult{
		Name:   "Go runtime",
		Status: "pass",
		Detail: runtime.Version(),
	})

	// 2. Config file
	cfg, err := config.LoadOrDefault("", pc.Product)
	if err != nil {
		checks = append(checks, checkResult{"Config", "warn", "No config file (using defaults)"})
	} else {
		checks = append(checks, checkResult{"Config", "pass", fmt.Sprintf("Port %d, data dir: %s", cfg.Port, cfg.DataDir)})
	}

	// 3. Data directory
	dataDir := cfg.DataDir
	if dataDir == "" {
		home, _ := os.UserHomeDir()
		dataDir = filepath.Join(home, ".stockyard")
	}
	if info, err := os.Stat(dataDir); err == nil && info.IsDir() {
		checks = append(checks, checkResult{"Data directory", "pass", dataDir})
	} else {
		checks = append(checks, checkResult{"Data directory", "warn", fmt.Sprintf("%s (will be created on first run)", dataDir)})
	}

	// 4. SQLite database
	dbPath := filepath.Join(dataDir, "stockyard.db")
	if _, err := os.Stat(dbPath); err == nil {
		db, err := sql.Open("sqlite", dbPath)
		if err != nil {
			checks = append(checks, checkResult{"Database", "fail", err.Error()})
		} else {
			var count int
			_ = db.QueryRow("SELECT COUNT(*) FROM proxy_modules").Scan(&count)
			db.Close()
			checks = append(checks, checkResult{"Database", "pass", fmt.Sprintf("%s (%d modules registered)", dbPath, count)})
		}
	} else {
		checks = append(checks, checkResult{"Database", "warn", "Not yet created (run stockyard once to initialize)"})
	}

	// 5. Port availability
	port := cfg.Port
	if port == 0 {
		port = 4200
	}
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		// Check if stockyard is already running
		resp, httpErr := http.Get(fmt.Sprintf("http://localhost:%d/health", port))
		if httpErr == nil {
			resp.Body.Close()
			if resp.StatusCode == 200 {
				checks = append(checks, checkResult{"Port", "pass", fmt.Sprintf(":%d (Stockyard already running)", port)})
			} else {
				checks = append(checks, checkResult{"Port", "warn", fmt.Sprintf(":%d in use (not by Stockyard)", port)})
			}
		} else {
			checks = append(checks, checkResult{"Port", "warn", fmt.Sprintf(":%d in use by another process", port)})
		}
	} else {
		ln.Close()
		checks = append(checks, checkResult{"Port", "pass", fmt.Sprintf(":%d available", port)})
	}

	// 6. Provider API keys
	providers := map[string]string{
		"OpenAI":    "OPENAI_API_KEY",
		"Anthropic": "ANTHROPIC_API_KEY",
		"Gemini":    "GEMINI_API_KEY",
		"Groq":      "GROQ_API_KEY",
		"Mistral":   "MISTRAL_API_KEY",
		"DeepSeek":  "DEEPSEEK_API_KEY",
		"xAI":       "XAI_API_KEY",
	}
	found := []string{}
	for name, env := range providers {
		if v := os.Getenv(env); v != "" {
			prefix := v
			if len(prefix) > 8 {
				prefix = v[:8] + "..."
			}
			found = append(found, name)
			_ = prefix
		}
	}
	if len(found) > 0 {
		checks = append(checks, checkResult{"Provider keys", "pass", fmt.Sprintf("%d found: %s", len(found), strings.Join(found, ", "))})
	} else {
		checks = append(checks, checkResult{"Provider keys", "warn", "None found. Set OPENAI_API_KEY or other provider env vars."})
	}

	// 7. Ollama local
	if _, err := net.DialTimeout("tcp", "localhost:11434", 500*time.Millisecond); err == nil {
		checks = append(checks, checkResult{"Ollama", "pass", "Running at localhost:11434"})
	} else {
		checks = append(checks, checkResult{"Ollama", "warn", "Not detected (optional — for local models)"})
	}

	// 8. License
	if key := os.Getenv("STOCKYARD_LICENSE_KEY"); key != "" {
		checks = append(checks, checkResult{"License", "pass", "Key set (SY-...)"})
	} else {
		checks = append(checks, checkResult{"License", "warn", "No license key (Community tier — 10k reqs/mo)"})
	}

	// 9. Git (for version tracking)
	if _, err := exec.LookPath("git"); err == nil {
		checks = append(checks, checkResult{"Git", "pass", "Available"})
	} else {
		checks = append(checks, checkResult{"Git", "warn", "Not found (optional)"})
	}

	// 10. Disk space
	checks = append(checks, checkDiskSpace(dataDir))

	// Print results
	pass, warn, fail := 0, 0, 0
	for _, c := range checks {
		icon := "✓"
		switch c.Status {
		case "pass":
			icon = "  ✓"
			pass++
		case "warn":
			icon = "  !"
			warn++
		case "fail":
			icon = "  ✗"
			fail++
		}
		fmt.Printf("  %s %-18s %s\n", icon, c.Name, c.Detail)
	}

	fmt.Println()
	fmt.Printf("  %d passed, %d warnings, %d failures\n", pass, warn, fail)

	if fail > 0 {
		fmt.Println("\n  Fix failures above before running Stockyard.")
		os.Exit(1)
	} else if warn > 0 {
		fmt.Println("\n  Warnings are non-blocking. Stockyard will run fine.")
	} else {
		fmt.Println("\n  Everything looks good. Run `stockyard` to start.")
	}
	fmt.Println()

	// JSON output if requested
	if len(os.Args) > 2 && os.Args[2] == "--json" {
		json.NewEncoder(os.Stdout).Encode(map[string]any{
			"checks": checks,
			"pass":   pass,
			"warn":   warn,
			"fail":   fail,
		})
	}

	os.Exit(0)
}

func checkDiskSpace(dir string) checkResult {
	// Cross-platform: just check if we can write
	tmpFile := filepath.Join(dir, ".stockyard-doctor-test")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return checkResult{"Disk", "fail", "Cannot create data directory: " + err.Error()}
	}
	if err := os.WriteFile(tmpFile, []byte("test"), 0644); err != nil {
		return checkResult{"Disk", "fail", "Cannot write to data directory: " + err.Error()}
	}
	os.Remove(tmpFile)
	return checkResult{"Disk", "pass", "Writable"}
}
