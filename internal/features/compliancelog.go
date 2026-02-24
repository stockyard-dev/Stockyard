package features

import (
	"context"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/stockyard-dev/stockyard/internal/config"
	"github.com/stockyard-dev/stockyard/internal/provider"
	"github.com/stockyard-dev/stockyard/internal/proxy"
)

// ComplianceEntry represents a single immutable audit log entry.
type ComplianceEntry struct {
	Sequence    int64     `json:"sequence"`
	Timestamp   time.Time `json:"timestamp"`
	Hash        string    `json:"hash"`         // SHA256 of this entry
	PrevHash    string    `json:"prev_hash"`    // SHA256 of previous entry (chain)
	RequestID   string    `json:"request_id"`
	Model       string    `json:"model"`
	Provider    string    `json:"provider"`
	Project     string    `json:"project"`
	UserID      string    `json:"user_id"`
	InputTokens int       `json:"input_tokens"`
	OutputTokens int      `json:"output_tokens"`
	Latency     int64     `json:"latency_ms"`
	Status      string    `json:"status"` // success, error
	ErrorMsg    string    `json:"error_msg,omitempty"`
	InputBody   string    `json:"input_body,omitempty"`
	OutputBody  string    `json:"output_body,omitempty"`
}

// ComplianceLogState holds runtime state for the compliance logger.
type ComplianceLogState struct {
	mu           sync.Mutex
	cfg          config.ComplianceLogConfig
	entries      []ComplianceEntry
	maxEntries   int
	lastHash     string
	sequence     atomic.Int64
	chainValid   atomic.Bool

	totalEntries atomic.Int64
	totalErrors  atomic.Int64
}

// NewComplianceLogger creates a new compliance logger from config.
func NewComplianceLogger(cfg config.ComplianceLogConfig) *ComplianceLogState {
	cl := &ComplianceLogState{
		cfg:        cfg,
		entries:    make([]ComplianceEntry, 0, 10000),
		maxEntries: 100000,
		lastHash:   "0000000000000000000000000000000000000000000000000000000000000000", // genesis
	}
	cl.chainValid.Store(true)
	return cl
}

// Append adds an entry to the immutable log with hash chain linking.
func (cl *ComplianceLogState) Append(entry ComplianceEntry) ComplianceEntry {
	cl.mu.Lock()
	defer cl.mu.Unlock()

	seq := cl.sequence.Add(1)
	entry.Sequence = seq
	entry.PrevHash = cl.lastHash

	// Compute hash of this entry (covers all fields except Hash itself)
	entry.Hash = cl.computeHash(entry)
	cl.lastHash = entry.Hash

	// Ring buffer to prevent unbounded growth
	if len(cl.entries) >= cl.maxEntries {
		cutoff := cl.maxEntries / 10
		cl.entries = cl.entries[cutoff:]
	}
	cl.entries = append(cl.entries, entry)
	cl.totalEntries.Add(1)

	return entry
}

// computeHash generates a SHA256 hash of the entry's content (excluding the hash field).
func (cl *ComplianceLogState) computeHash(entry ComplianceEntry) string {
	h := sha256.New()
	fmt.Fprintf(h, "%d|%s|%s|%s|%s|%s|%s|%d|%d|%d|%s|%s|%s|%s",
		entry.Sequence,
		entry.Timestamp.UTC().Format(time.RFC3339Nano),
		entry.PrevHash,
		entry.RequestID,
		entry.Model,
		entry.Provider,
		entry.Project,
		entry.InputTokens,
		entry.OutputTokens,
		entry.Latency,
		entry.Status,
		entry.ErrorMsg,
		entry.InputBody,
		entry.OutputBody,
	)
	return hex.EncodeToString(h.Sum(nil))
}

// VerifyChain validates the integrity of the hash chain.
// Returns the number of valid entries, total entries, and any integrity error.
func (cl *ComplianceLogState) VerifyChain() (valid int, total int, err error) {
	cl.mu.Lock()
	defer cl.mu.Unlock()

	total = len(cl.entries)
	if total == 0 {
		return 0, 0, nil
	}

	for i, entry := range cl.entries {
		// Verify this entry's hash
		expected := cl.computeHash(entry)
		if entry.Hash != expected {
			cl.chainValid.Store(false)
			return i, total, fmt.Errorf("hash mismatch at sequence %d: expected %s, got %s",
				entry.Sequence, expected, entry.Hash)
		}

		// Verify chain linkage (skip first entry)
		if i > 0 {
			if entry.PrevHash != cl.entries[i-1].Hash {
				cl.chainValid.Store(false)
				return i, total, fmt.Errorf("chain break at sequence %d: prev_hash %s != previous entry hash %s",
					entry.Sequence, entry.PrevHash, cl.entries[i-1].Hash)
			}
		}

		valid++
	}

	cl.chainValid.Store(true)
	return valid, total, nil
}

// ExportJSON returns all entries as a JSON byte slice.
func (cl *ComplianceLogState) ExportJSON() ([]byte, error) {
	cl.mu.Lock()
	entries := make([]ComplianceEntry, len(cl.entries))
	copy(entries, cl.entries)
	cl.mu.Unlock()

	return json.MarshalIndent(entries, "", "  ")
}

// ExportCSV returns all entries as CSV.
func (cl *ComplianceLogState) ExportCSV() (string, error) {
	cl.mu.Lock()
	entries := make([]ComplianceEntry, len(cl.entries))
	copy(entries, cl.entries)
	cl.mu.Unlock()

	var buf strings.Builder
	w := csv.NewWriter(&buf)

	// Header
	if err := w.Write([]string{
		"sequence", "timestamp", "hash", "prev_hash", "request_id",
		"model", "provider", "project", "user_id",
		"input_tokens", "output_tokens", "latency_ms", "status", "error_msg",
	}); err != nil {
		return "", fmt.Errorf("write csv header: %w", err)
	}

	for _, e := range entries {
		if err := w.Write([]string{
			fmt.Sprintf("%d", e.Sequence),
			e.Timestamp.UTC().Format(time.RFC3339),
			e.Hash,
			e.PrevHash,
			e.RequestID,
			e.Model,
			e.Provider,
			e.Project,
			e.UserID,
			fmt.Sprintf("%d", e.InputTokens),
			fmt.Sprintf("%d", e.OutputTokens),
			fmt.Sprintf("%d", e.Latency),
			e.Status,
			e.ErrorMsg,
		}); err != nil {
			return "", fmt.Errorf("write csv row: %w", err)
		}
	}

	w.Flush()
	return buf.String(), w.Error()
}

// ExportSOC2 returns a SOC2-formatted compliance report.
func (cl *ComplianceLogState) ExportSOC2() map[string]any {
	cl.mu.Lock()
	total := len(cl.entries)
	var earliest, latest time.Time
	successCount, errorCount := 0, 0
	models := make(map[string]int)
	providers := make(map[string]int)

	for i, e := range cl.entries {
		if i == 0 {
			earliest = e.Timestamp
		}
		latest = e.Timestamp
		if e.Status == "success" {
			successCount++
		} else {
			errorCount++
		}
		models[e.Model]++
		providers[e.Provider]++
	}
	cl.mu.Unlock()

	valid, _, verifyErr := cl.VerifyChain()
	integrityStatus := "PASS"
	if verifyErr != nil {
		integrityStatus = "FAIL: " + verifyErr.Error()
	}

	return map[string]any{
		"report_type":      "SOC2 AI Interaction Audit",
		"generated_at":     time.Now().UTC().Format(time.RFC3339),
		"period_start":     earliest.UTC().Format(time.RFC3339),
		"period_end":       latest.UTC().Format(time.RFC3339),
		"total_interactions": total,
		"successful":       successCount,
		"errors":           errorCount,
		"chain_integrity":  integrityStatus,
		"verified_entries": valid,
		"models_used":      models,
		"providers_used":   providers,
	}
}

// RecentEntries returns the N most recent log entries.
func (cl *ComplianceLogState) RecentEntries(n int) []ComplianceEntry {
	cl.mu.Lock()
	defer cl.mu.Unlock()

	if n > len(cl.entries) {
		n = len(cl.entries)
	}
	start := len(cl.entries) - n
	result := make([]ComplianceEntry, n)
	copy(result, cl.entries[start:])
	return result
}

// Stats returns compliance log statistics for the dashboard.
func (cl *ComplianceLogState) Stats() map[string]any {
	recent := cl.RecentEntries(20)
	return map[string]any{
		"total_entries": cl.totalEntries.Load(),
		"total_errors":  cl.totalErrors.Load(),
		"chain_valid":   cl.chainValid.Load(),
		"current_hash":  cl.lastHash,
		"recent":        recent,
	}
}

// ComplianceLogMiddleware returns middleware that creates immutable audit records.
func ComplianceLogMiddleware(cl *ComplianceLogState) proxy.Middleware {
	return func(next proxy.Handler) proxy.Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
			start := time.Now()

			// Build input body for logging
			inputBody := ""
			if cl.cfg.IncludeBodies {
				inputBody = formatMessages(req.Messages, cl.cfg.MaxBodySize)
			}

			// Execute the request
			resp, err := next(ctx, req)
			latency := time.Since(start)

			// Build the compliance entry
			entry := ComplianceEntry{
				Timestamp:   start,
				RequestID:   generateRequestID(),
				Model:       req.Model,
				Project:     req.Project,
				UserID:      req.UserID,
				Latency:     latency.Milliseconds(),
				InputBody:   inputBody,
			}

			if err != nil {
				entry.Status = "error"
				entry.ErrorMsg = err.Error()
				cl.totalErrors.Add(1)
			} else {
				entry.Status = "success"
				entry.Provider = resp.Provider
				entry.InputTokens = resp.Usage.PromptTokens
				entry.OutputTokens = resp.Usage.CompletionTokens
				if cl.cfg.IncludeBodies && len(resp.Choices) > 0 {
					entry.OutputBody = truncateBody(resp.Choices[0].Message.Content, cl.cfg.MaxBodySize)
				}
			}

			// Append to immutable log
			logged := cl.Append(entry)
			log.Printf("compliancelog: seq=%d hash=%s status=%s model=%s latency=%dms",
				logged.Sequence, logged.Hash[:16], entry.Status, entry.Model, entry.Latency)

			return resp, err
		}
	}
}

func formatMessages(msgs []provider.Message, maxSize int) string {
	var parts []string
	for _, m := range msgs {
		parts = append(parts, fmt.Sprintf("[%s] %s", m.Role, m.Content))
	}
	result := strings.Join(parts, "\n")
	return truncateBody(result, maxSize)
}

func truncateBody(s string, max int) string {
	if max <= 0 || len(s) <= max {
		return s
	}
	return s[:max] + "...[truncated]"
}

func generateRequestID() string {
	h := sha256.New()
	fmt.Fprintf(h, "%d-%d", time.Now().UnixNano(), time.Now().UnixMicro())
	return hex.EncodeToString(h.Sum(nil))[:16]
}
