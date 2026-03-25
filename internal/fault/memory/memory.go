// Package memory provides error pattern learning for Stockyard Fault.
// When a fix is successfully applied, its pattern is remembered. Future
// occurrences of the same (or similar) error can skip the LLM entirely
// and apply the known fix directly.
package memory

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/stockyard-dev/stockyard/internal/fault/store"
)

// Pattern is a remembered fix that worked for a specific error signature.
type Pattern struct {
	ID             int       `json:"id"`
	Fingerprint    string    `json:"fingerprint"`
	ErrorType      string    `json:"error_type"`
	MessagePattern string    `json:"message_pattern"`
	DiagnosisJSON  string    `json:"diagnosis_json"`
	SuccessCount   int       `json:"success_count"`
	LastUsed       time.Time `json:"last_used"`
}

// Memory stores and retrieves error fix patterns backed by SQLite.
type Memory struct {
	store *store.DB
}

// New creates a Memory instance backed by the given store.
func New(db *store.DB) *Memory {
	return &Memory{store: db}
}

// Remember stores a successful fix pattern so it can be reused.
func (m *Memory) Remember(fingerprint string, diagJSON []byte, errorType, message string, success bool) error {
	if !success {
		return nil // only remember successes
	}

	// Check if we already have this pattern
	existing, err := m.store.RecallPattern(fingerprint)
	if err != nil {
		return err
	}
	if existing != nil {
		// Update existing pattern: bump success count and last_used
		existing.SuccessCount++
		existing.LastUsed = time.Now()
		return m.store.SavePattern(*existing)
	}

	// New pattern
	p := store.Pattern{
		Fingerprint:    fingerprint,
		ErrorType:      errorType,
		MessagePattern: normalizeMessage(message),
		DiagnosisJSON:  string(diagJSON),
		SuccessCount:   1,
		LastUsed:       time.Now(),
	}
	return m.store.SavePattern(p)
}

// Recall checks if we have a known fix for this exact error fingerprint.
// Returns the diagnosis JSON and true if found, nil and false otherwise.
func (m *Memory) Recall(fingerprint string) (json.RawMessage, bool) {
	p, err := m.store.RecallPattern(fingerprint)
	if err != nil || p == nil {
		return nil, false
	}
	return json.RawMessage(p.DiagnosisJSON), true
}

// SimilarPatterns returns patterns that fuzzy-match the given error type and message.
func (m *Memory) SimilarPatterns(errorType, message string) ([]Pattern, error) {
	storePatterns, err := m.store.ListPatterns()
	if err != nil {
		return nil, err
	}

	normalized := normalizeMessage(message)
	var matches []Pattern
	for _, sp := range storePatterns {
		score := similarity(sp.ErrorType, errorType, sp.MessagePattern, normalized)
		if score > 0.5 {
			matches = append(matches, Pattern{
				ID:             sp.ID,
				Fingerprint:    sp.Fingerprint,
				ErrorType:      sp.ErrorType,
				MessagePattern: sp.MessagePattern,
				DiagnosisJSON:  sp.DiagnosisJSON,
				SuccessCount:   sp.SuccessCount,
				LastUsed:       sp.LastUsed,
			})
		}
	}
	return matches, nil
}

// normalizeMessage strips variable parts (timestamps, IDs, numbers) to create
// a pattern that can match across instances of the same error.
func normalizeMessage(msg string) string {
	// Lowercase and collapse whitespace
	msg = strings.ToLower(strings.TrimSpace(msg))
	// Replace common variable parts with placeholders
	var result strings.Builder
	for i := 0; i < len(msg); i++ {
		ch := msg[i]
		if ch >= '0' && ch <= '9' {
			// Collapse consecutive digits into a placeholder
			for i+1 < len(msg) && msg[i+1] >= '0' && msg[i+1] <= '9' {
				i++
			}
			result.WriteString("<N>")
		} else {
			result.WriteByte(ch)
		}
	}
	return result.String()
}

// similarity scores how close two error patterns are (0.0–1.0).
func similarity(typeA, typeB, msgA, msgB string) float64 {
	score := 0.0
	// Exact type match is worth 0.4
	if typeA == typeB {
		score += 0.4
	}
	// Message similarity via token overlap
	tokensA := strings.Fields(msgA)
	tokensB := strings.Fields(msgB)
	if len(tokensA) == 0 || len(tokensB) == 0 {
		return score
	}

	setA := make(map[string]bool, len(tokensA))
	for _, t := range tokensA {
		setA[t] = true
	}
	overlap := 0
	for _, t := range tokensB {
		if setA[t] {
			overlap++
		}
	}
	maxLen := len(tokensA)
	if len(tokensB) > maxLen {
		maxLen = len(tokensB)
	}
	score += 0.6 * float64(overlap) / float64(maxLen)
	return score
}
