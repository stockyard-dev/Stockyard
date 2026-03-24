// Stockyard Spine — Infrastructure that operates itself.
//
// Define what you want (latency, cost, availability). Spine observes,
// decides, and acts to deliver it.
//
// Usage:
//
//	spine run <objectives.yaml>     Start the observe-decide-act loop
//	spine check <objectives.yaml>   One-shot: evaluate current state against objectives
//	spine score <objectives.yaml>   Show objective scores without acting
//	spine init [dir]                Create example objectives file
//	spine version                   Show version
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/stockyard-dev/stockyard/internal/spine/act"
	"github.com/stockyard-dev/stockyard/internal/spine/decide"
	"github.com/stockyard-dev/stockyard/internal/spine/metrics"
	"github.com/stockyard-dev/stockyard/internal/spine/objectives"
	"github.com/stockyard-dev/stockyard/internal/spine/observe"
	"github.com/stockyard-dev/stockyard/internal/spine/safety"
	"github.com/stockyard-dev/stockyard/internal/spine/server"
	"github.com/stockyard-dev/stockyard/internal/spine/store"
)

var version = "dev"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "run":
		if len(os.Args) < 3 {
			fmt.Println("Usage: spine run <objectives.yaml>")
			os.Exit(1)
		}
		cmdRun(os.Args[2], os.Args[3:])
	case "check":
		if len(os.Args) < 3 {
			fmt.Println("Usage: spine check <objectives.yaml>")
			os.Exit(1)
		}
		cmdCheck(os.Args[2])
	case "score":
		if len(os.Args) < 3 {
			fmt.Println("Usage: spine score <objectives.yaml>")
			os.Exit(1)
		}
		cmdScore(os.Args[2])
	case "init":
		dir := "."
		if len(os.Args) > 2 {
			dir = os.Args[2]
		}
		cmdInit(dir)
	case "version":
		fmt.Printf("spine %s\n", version)
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Printf("Unknown command: %s\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func cmdRun(objFile string, args []string) {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	port := fs.Int("port", 9700, "HTTP dashboard port")
	interval := fs.String("interval", "30s", "Observation interval")
	dryRun := fs.Bool("dry-run", false, "Log decisions without executing")
	dbPath := fs.String("db", "spine.db", "SQLite database path")
	maxActions := fs.Int("max-actions", 20, "Max actions per hour")
	requireApproval := fs.Bool("require-approval", false, "Require approval for critical/high decisions")
	prometheusURL := fs.String("prometheus", "", "Prometheus endpoint URL")
	fs.Parse(args)

	if envPort := os.Getenv("PORT"); envPort != "" {
		if p, err := strconv.Atoi(envPort); err == nil {
			*port = p
		}
	}

	spec, err := objectives.Load(objFile)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	observeInterval, _ := time.ParseDuration(*interval)
	if observeInterval <= 0 {
		observeInterval = 30 * time.Second
	}

	target := fmt.Sprintf("http://localhost:%d", spec.Application.Port)
	if spec.Application.Port == 0 {
		target = "http://localhost:8080"
	}

	// Open SQLite store
	db, err := store.New(*dbPath)
	if err != nil {
		fmt.Printf("Warning: could not open database: %v (continuing without persistence)\n", err)
		db = nil
	}
	if db != nil {
		defer db.Close()
	}

	// Set up metrics collector
	var collector metrics.MetricsCollector
	if *prometheusURL != "" {
		collector = metrics.NewPrometheusSource(*prometheusURL)
	}

	// Create observer with multi-target support
	obs := observe.New(observe.Config{
		Target:     target,
		HealthPath: spec.Application.HealthPath,
		Interval:   observeInterval,
		Collector:  collector,
		Targets:    spec.Targets,
	})

	decider := decide.New(&spec.Objectives)

	// Safety guardrails
	rails := safety.NewPendingQueue(safety.GuardRails{
		MaxActionsPerHour: *maxActions,
		RequireApproval:   *requireApproval,
		DryRunMode:        *dryRun,
	})

	executor := act.NewExecutor(act.Config{
		DryRun: *dryRun,
		OnAction: func(entry act.ActionLog) {
			if entry.Executed {
				fmt.Printf("  [ACT] %s: %s\n", entry.Decision.Action, entry.Decision.Reason)
			}
			// Persist action to database
			if db != nil {
				db.InsertAction(&store.Action{
					DecisionID: entry.Decision.ID,
					Handler:    string(entry.Decision.Action),
					Success:    entry.Executed,
					Output:     entry.Result + entry.Error,
					ExecutedAt: entry.Timestamp,
				})
			}
			// Record for rate limiting
			if entry.Executed {
				rails.RecordAction()
			}
		},
	})
	// Register built-in handlers
	executor.RegisterHandler(decide.ActionAlert, &act.LogHandler{})

	fmt.Printf("\n  Stockyard Spine -- Self-Operating Infrastructure\n\n")
	fmt.Printf("    Application: %s\n", spec.Application.Name)
	fmt.Printf("    Target:      %s\n", target)
	fmt.Printf("    Interval:    %s\n", observeInterval)
	fmt.Printf("    Dry run:     %t\n", *dryRun)
	fmt.Printf("    Database:    %s\n", *dbPath)
	fmt.Printf("    Dashboard:   http://localhost:%d/ui\n", *port)
	fmt.Printf("    Rate limit:  %d actions/hour\n", *maxActions)

	if len(spec.Targets) > 0 {
		fmt.Printf("    Targets:     %d additional\n", len(spec.Targets))
	}
	if *prometheusURL != "" {
		fmt.Printf("    Prometheus:  %s\n", *prometheusURL)
	}

	if spec.Objectives.Latency != nil {
		fmt.Printf("    Latency:     P95 < %s\n", spec.Objectives.Latency.P95)
	}
	if spec.Objectives.Availability != nil {
		fmt.Printf("    Availability: %.2f%%\n", spec.Objectives.Availability.Target)
	}
	if spec.Objectives.Cost != nil {
		fmt.Printf("    Cost budget: $%.2f/month\n", spec.Objectives.Cost.MonthlyMax)
	}
	fmt.Println()

	// Start observing
	obs.Start()
	defer obs.Stop()

	// Start the observe-decide-act loop
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Println("\n  Shutting down...")
		cancel()
	}()

	// Start the control loop in background
	go func() {
		// Wait for initial observations
		time.Sleep(observeInterval + 2*time.Second)

		ticker := time.NewTicker(observeInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				m := obs.Metrics()
				score := objectives.Evaluate(&spec.Objectives, m)

				// Persist metrics snapshot
				if db != nil {
					db.InsertMetrics(&store.MetricsSnapshot{
						CPUPct:     m.CPUUsage,
						MemoryPct:  m.MemoryUsage,
						Instances:  m.Instances,
						RPS:        float64(m.CurrentRPS),
						ErrorRate:  m.ErrorRate,
						RecordedAt: time.Now(),
					})
				}

				// Persist observations
				if db != nil {
					for _, s := range obs.RecentSamples(1) {
						db.InsertObservation(&store.Observation{
							Target:     target,
							LatencyMs:  float64(s.LatencyMs),
							StatusCode: s.StatusCode,
							Available:  s.Healthy,
							Error:      s.Error,
							RecordedAt: s.Timestamp,
						})
					}
				}

				// Print current status
				fmt.Printf("  [SCORE] overall=%.0f%% latency=%.0f%% avail=%.0f%% cost=%.0f%%  (P95=%s, avail=%.2f%%)\n",
					score.Overall*100, score.Latency*100, score.Availability*100, score.Cost*100,
					m.LatencyP95, m.Availability)

				// Make decisions
				decisions := decider.Evaluate(m)
				if len(decisions) > 0 {
					// Run decisions through safety checks
					var approved []decide.Decision
					for _, d := range decisions {
						// Persist decision
						if db != nil {
							db.InsertDecision(&store.Decision{
								ID:         d.ID,
								ActionType: string(d.Action),
								Urgency:    string(d.Urgency),
								Reason:     d.Reason,
								Impact:     d.Impact,
								Rollback:   d.Rollback,
								CreatedAt:  d.Timestamp,
								Executed:   false,
							})
						}

						check := rails.Check(d)
						if check.Allowed {
							approved = append(approved, d)
						} else if check.RequiresApproval {
							fmt.Printf("  [SAFETY] %s queued for approval: %s\n", d.Action, check.Reason)
						} else {
							fmt.Printf("  [SAFETY] %s blocked: %s\n", d.Action, check.Reason)
						}
					}

					if len(approved) > 0 {
						executor.Execute(ctx, approved)
						// Mark executed in DB
						if db != nil {
							for _, d := range approved {
								db.MarkDecisionExecuted(d.ID)
							}
						}
					}

					// Set cooldowns to prevent thrashing
					for _, d := range decisions {
						decider.SetCooldown(d.Action, 5*time.Minute)
					}
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	// Start HTTP dashboard
	srv := server.New(server.Config{
		Port:       *port,
		Observer:   obs,
		Decider:    decider,
		Executor:   executor,
		Objectives: spec,
		Store:      db,
		Safety:     rails,
	})
	srv.ListenAndServe()
}

func cmdCheck(objFile string) {
	spec, err := objectives.Load(objFile)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	target := fmt.Sprintf("http://localhost:%d", spec.Application.Port)
	if spec.Application.Port == 0 {
		target = "http://localhost:8080"
	}

	fmt.Printf("\n  Checking %s against objectives...\n\n", spec.Application.Name)

	obs := observe.New(observe.Config{
		Target:     target,
		HealthPath: spec.Application.HealthPath,
		Interval:   2 * time.Second,
		MaxSamples: 10,
	})
	obs.Start()
	time.Sleep(12 * time.Second) // collect ~5 samples
	obs.Stop()

	m := obs.Metrics()
	score := objectives.Evaluate(&spec.Objectives, m)

	decider := decide.New(&spec.Objectives)
	decisions := decider.Evaluate(m)

	fmt.Printf("  Metrics:\n")
	fmt.Printf("    P95 Latency:    %s\n", m.LatencyP95)
	fmt.Printf("    Availability:   %.2f%%\n", m.Availability)
	fmt.Printf("    Error Rate:     %.1f%%\n", m.ErrorRate)
	fmt.Println()

	fmt.Printf("  Scores:\n")
	fmt.Printf("    Overall:        %s\n", scoreLabel(score.Overall))
	fmt.Printf("    Latency:        %s\n", scoreLabel(score.Latency))
	fmt.Printf("    Availability:   %s\n", scoreLabel(score.Availability))
	fmt.Printf("    Cost:           %s\n", scoreLabel(score.Cost))
	fmt.Printf("    Throughput:     %s\n", scoreLabel(score.Throughput))
	fmt.Println()

	if len(decisions) > 0 {
		fmt.Printf("  Recommendations:\n")
		for _, d := range decisions {
			fmt.Printf("    [%s] %s: %s\n", d.Urgency, d.Action, d.Reason)
		}
	} else {
		fmt.Printf("  All objectives met.\n")
	}
	fmt.Println()

	if score.Overall < 0.7 {
		os.Exit(1) // CI/CD gate
	}
}

func cmdScore(objFile string) {
	spec, err := objectives.Load(objFile)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	data, _ := json.MarshalIndent(spec, "", "  ")
	fmt.Printf("\n  Objectives:\n%s\n\n", string(data))
}

func cmdInit(dir string) {
	os.MkdirAll(dir, 0755)
	path := filepath.Join(dir, "objectives.yaml")
	if _, err := os.Stat(path); err == nil {
		fmt.Printf("  %s already exists\n", path)
		os.Exit(1)
	}
	if err := os.WriteFile(path, []byte(exampleObjectives), 0644); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("  Created %s\n", path)
	fmt.Printf("    Edit objectives, then run: spine run %s\n", path)
}

func scoreLabel(v float64) string {
	pct := v * 100
	if pct >= 95 {
		return fmt.Sprintf("%.0f%% PASS", pct)
	} else if pct >= 70 {
		return fmt.Sprintf("%.0f%% WARN", pct)
	}
	return fmt.Sprintf("%.0f%% FAIL", pct)
}

func printUsage() {
	fmt.Println(`
  Stockyard Spine -- Infrastructure that operates itself.

  You define objectives. Spine observes your infrastructure,
  decides what to do, and acts automatically.

  Usage:
    spine run <objectives.yaml>     Start the observe-decide-act loop
    spine check <objectives.yaml>   One-shot check against objectives
    spine score <objectives.yaml>   Show parsed objectives
    spine init [dir]                Create example objectives file
    spine version                   Show version

  Run flags:
    --port <n>              Dashboard port (default: 9700)
    --interval <dur>        Observation interval (default: 30s)
    --dry-run               Log decisions without executing
    --db <path>             SQLite database path (default: spine.db)
    --max-actions <n>       Max actions per hour (default: 20)
    --require-approval      Require approval for critical/high urgency
    --prometheus <url>      Prometheus endpoint for metrics collection

  Environment:
    PORT                    Override dashboard port

  Example:
    spine init myapp
    # edit myapp/objectives.yaml
    spine run myapp/objectives.yaml --dry-run`)
}

const exampleObjectives = `# Stockyard Spine -- Infrastructure Objectives
# Define what you want. Spine delivers it.

application:
  name: my-api
  port: 8080
  health_path: /health

objectives:
  latency:
    p95: 200ms
    p99: 500ms

  availability:
    target: 99.9

  cost:
    monthly_max: 50.00
    optimize: true

  throughput:
    min_rps: 100

# Additional targets to monitor
# targets:
#   - name: api-staging
#     url: http://staging.example.com
#     health_path: /health
#   - name: api-eu
#     url: http://eu.example.com
#     health_path: /health
#     prometheus: http://prometheus-eu:9090

constraints:
  - type: region
    value: us-east-1
    rule: prefer
`
