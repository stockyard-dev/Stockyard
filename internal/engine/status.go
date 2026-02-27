package engine

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

// StatusCollector tracks real-time system metrics.
type StatusCollector struct {
	startTime      time.Time
	requestCount   atomic.Int64
	errorCount     atomic.Int64
	totalLatencyNs atomic.Int64

	mu          sync.RWMutex
	lastChecks  map[string]healthCheck
}

type healthCheck struct {
	Status    string    `json:"status"` // healthy, degraded, down
	Latency   float64   `json:"latency_ms"`
	CheckedAt time.Time `json:"checked_at"`
}

// StatusResponse is the JSON shape for GET /api/status.
type StatusResponse struct {
	Status       string               `json:"status"` // healthy, degraded, down
	Uptime       string               `json:"uptime"`
	UptimeSeconds float64             `json:"uptime_seconds"`
	Version      string               `json:"version"`
	Go           string               `json:"go_version"`
	Requests     int64                `json:"total_requests"`
	Errors       int64                `json:"total_errors"`
	ErrorRate    float64              `json:"error_rate"`
	AvgLatencyMs float64             `json:"avg_latency_ms"`
	Memory       memStats             `json:"memory"`
	Goroutines   int                  `json:"goroutines"`
	Components   map[string]compStatus `json:"components"`
}

type memStats struct {
	AllocMB   float64 `json:"alloc_mb"`
	SysMB     float64 `json:"sys_mb"`
	NumGC     uint32  `json:"num_gc"`
}

type compStatus struct {
	Status  string `json:"status"`
	Detail  string `json:"detail,omitempty"`
}

func NewStatusCollector() *StatusCollector {
	sc := &StatusCollector{
		startTime:  time.Now(),
		lastChecks: make(map[string]healthCheck),
	}
	GlobalStatus = sc
	return sc
}

// GlobalStatus is the package-level status collector, set at boot.
// Used by appHooksMiddleware to record per-request metrics.
var GlobalStatus *StatusCollector

// RecordRequest records a proxy request for status tracking.
func (sc *StatusCollector) RecordRequest(latency time.Duration, isError bool) {
	sc.requestCount.Add(1)
	sc.totalLatencyNs.Add(int64(latency))
	if isError {
		sc.errorCount.Add(1)
	}
}

// RegisterStatusRoutes mounts GET /api/status.
func RegisterStatusRoutes(mux *http.ServeMux, sc *StatusCollector, conn *sql.DB, version string) {
	mux.HandleFunc("GET /api/status", func(w http.ResponseWriter, r *http.Request) {
		uptime := time.Since(sc.startTime)
		reqs := sc.requestCount.Load()
		errs := sc.errorCount.Load()
		totalNs := sc.totalLatencyNs.Load()

		var errRate float64
		var avgLatency float64
		if reqs > 0 {
			errRate = float64(errs) / float64(reqs)
			avgLatency = float64(totalNs) / float64(reqs) / 1e6
		}

		var m runtime.MemStats
		runtime.ReadMemStats(&m)

		components := checkComponents(conn)

		overall := "healthy"
		for _, c := range components {
			if c.Status == "down" {
				overall = "down"
				break
			}
			if c.Status == "degraded" {
				overall = "degraded"
			}
		}

		resp := StatusResponse{
			Status:       overall,
			Uptime:       formatDuration(uptime),
			UptimeSeconds: uptime.Seconds(),
			Version:      version,
			Go:           runtime.Version(),
			Requests:     reqs,
			Errors:       errs,
			ErrorRate:    errRate,
			AvgLatencyMs: avgLatency,
			Memory: memStats{
				AllocMB: float64(m.Alloc) / 1024 / 1024,
				SysMB:   float64(m.Sys) / 1024 / 1024,
				NumGC:   m.NumGC,
			},
			Goroutines: runtime.NumGoroutine(),
			Components: components,
		}

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-cache")
		json.NewEncoder(w).Encode(resp)
	})
}

func checkComponents(conn *sql.DB) map[string]compStatus {
	comps := make(map[string]compStatus)

	// Database
	start := time.Now()
	if err := conn.Ping(); err != nil {
		comps["database"] = compStatus{Status: "down", Detail: err.Error()}
	} else {
		latency := time.Since(start)
		comps["database"] = compStatus{Status: "healthy", Detail: latency.String()}
	}

	// Modules count
	var moduleCount int
	if err := conn.QueryRow(`SELECT COUNT(*) FROM proxy_modules`).Scan(&moduleCount); err == nil {
		comps["modules"] = compStatus{Status: "healthy", Detail: formatInt(moduleCount) + " registered"}
	} else {
		comps["modules"] = compStatus{Status: "degraded", Detail: "count unavailable"}
	}

	// Traces
	var traceCount int
	if err := conn.QueryRow(`SELECT COUNT(*) FROM observe_traces`).Scan(&traceCount); err == nil {
		comps["observe"] = compStatus{Status: "healthy", Detail: formatInt(traceCount) + " traces"}
	} else {
		comps["observe"] = compStatus{Status: "degraded"}
	}

	// Trust ledger
	var ledgerCount int
	if err := conn.QueryRow(`SELECT COUNT(*) FROM trust_ledger`).Scan(&ledgerCount); err == nil {
		comps["trust"] = compStatus{Status: "healthy", Detail: formatInt(ledgerCount) + " events"}
	} else {
		comps["trust"] = compStatus{Status: "degraded"}
	}

	return comps
}

func formatDuration(d time.Duration) string {
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	mins := int(d.Minutes()) % 60

	if days > 0 {
		return formatInt(days) + "d " + formatInt(hours) + "h " + formatInt(mins) + "m"
	}
	if hours > 0 {
		return formatInt(hours) + "h " + formatInt(mins) + "m"
	}
	return formatInt(mins) + "m"
}

func formatInt(n int) string {
	if n < 1000 {
		s := ""
		if n == 0 {
			return "0"
		}
		for n > 0 {
			s = string(rune('0'+n%10)) + s
			n /= 10
		}
		return s
	}
	// Simple comma formatting
	s := formatInt(n % 1000)
	for len(s) < 3 {
		s = "0" + s
	}
	return formatInt(n/1000) + "," + s
}
