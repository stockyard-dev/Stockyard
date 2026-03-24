// Package reconstruct replays the delta stream to rebuild system state at any point in time.
// This is the core of Replay's time-travel capability.
package reconstruct

import (
	"fmt"
	"sort"
	"time"

	"github.com/stockyard-dev/stockyard/internal/replay/capture"
	"github.com/stockyard-dev/stockyard/internal/replay/store"
)

// State represents the reconstructed system state at a specific moment.
type State struct {
	Timestamp  time.Time         `json:"timestamp"`
	HTTP       RequestState      `json:"http"`       // in-flight requests
	Config     map[string]string `json:"config"`     // configuration values
	Flags      map[string]string `json:"flags"`      // feature flag values
	DB         map[string]string `json:"db"`         // recent DB state by key
	External   []ExternalCall    `json:"external"`   // recent external API calls
	Errors     []ErrorEvent      `json:"errors"`     // recent errors
	DeltaCount int               `json:"delta_count"`
}

// RequestState holds HTTP request state at reconstruction time.
type RequestState struct {
	ActiveRequests []ActiveRequest `json:"active_requests"`
	RecentComplete []CompletedReq  `json:"recent_completed"`
}

// ActiveRequest was in-flight at the reconstruction timestamp.
type ActiveRequest struct {
	RequestID string    `json:"request_id"`
	Method    string    `json:"method"`
	Path      string    `json:"path"`
	UserID    string    `json:"user_id"`
	StartedAt time.Time `json:"started_at"`
}

// CompletedReq finished near the reconstruction timestamp.
type CompletedReq struct {
	RequestID  string `json:"request_id"`
	Method     string `json:"method"`
	Path       string `json:"path"`
	StatusCode int    `json:"status_code"`
	LatencyMs  int    `json:"latency_ms"`
	UserID     string `json:"user_id"`
}

// ExternalCall is an outbound API call.
type ExternalCall struct {
	URL        string    `json:"url"`
	Method     string    `json:"method"`
	StatusCode int       `json:"status_code"`
	LatencyMs  int       `json:"latency_ms"`
	Timestamp  time.Time `json:"timestamp"`
}

// ErrorEvent is a captured error.
type ErrorEvent struct {
	Message   string    `json:"message"`
	Path      string    `json:"path"`
	RequestID string    `json:"request_id"`
	Timestamp time.Time `json:"timestamp"`
}

// AtTime reconstructs the system state at a specific moment.
func AtTime(db *store.DB, target time.Time) (*State, error) {
	// Get a window of deltas around the target time
	windowStart := target.Add(-5 * time.Minute)
	windowEnd := target.Add(1 * time.Second) // slight buffer for concurrent writes

	deltas, err := db.GetTimeRange(windowStart, windowEnd)
	if err != nil {
		return nil, fmt.Errorf("querying deltas: %w", err)
	}

	state := &State{
		Timestamp:  target,
		Config:     map[string]string{},
		Flags:      map[string]string{},
		DB:         map[string]string{},
		DeltaCount: len(deltas),
	}

	// Try loading a snapshot as baseline
	snapTime, snapState, snapErr := db.LoadSnapshot(target)
	if snapErr == nil && !snapTime.IsZero() {
		// Apply snapshot as baseline
		if cfg, ok := snapState["config"].(map[string]any); ok {
			for k, v := range cfg {
				state.Config[k] = fmt.Sprintf("%v", v)
			}
		}
		if flags, ok := snapState["flags"].(map[string]any); ok {
			for k, v := range flags {
				state.Flags[k] = fmt.Sprintf("%v", v)
			}
		}
	}

	// Replay deltas up to the target time
	requestStarts := map[string]ActiveRequest{}

	for _, d := range deltas {
		if d.Timestamp.After(target) {
			break
		}

		switch d.Type {
		case capture.DeltaHTTPRequest:
			// Track request lifecycle
			if d.Metadata.StatusCode == 0 {
				// Request started but no response yet at target time = in-flight
				requestStarts[d.Metadata.RequestID] = ActiveRequest{
					RequestID: d.Metadata.RequestID,
					Method:    d.Metadata.Method,
					Path:      d.Metadata.Path,
					UserID:    d.Metadata.UserID,
					StartedAt: d.Timestamp,
				}
			} else {
				// Request completed
				delete(requestStarts, d.Metadata.RequestID)
				state.HTTP.RecentComplete = append(state.HTTP.RecentComplete, CompletedReq{
					RequestID:  d.Metadata.RequestID,
					Method:     d.Metadata.Method,
					Path:       d.Metadata.Path,
					StatusCode: d.Metadata.StatusCode,
					LatencyMs:  d.Metadata.LatencyMs,
					UserID:     d.Metadata.UserID,
				})
			}

		case capture.DeltaConfigChange:
			state.Config[d.Key] = d.After

		case capture.DeltaFlagChange:
			state.Flags[d.Key] = d.After

		case capture.DeltaDBWrite:
			state.DB[d.Key] = d.After

		case capture.DeltaExternalCall:
			state.External = append(state.External, ExternalCall{
				URL:        d.Metadata.Path,
				Method:     d.Metadata.Method,
				StatusCode: d.Metadata.StatusCode,
				LatencyMs:  d.Metadata.LatencyMs,
				Timestamp:  d.Timestamp,
			})

		case capture.DeltaError:
			state.Errors = append(state.Errors, ErrorEvent{
				Message:   d.Metadata.ErrorMessage,
				Path:      d.Metadata.Path,
				RequestID: d.Metadata.RequestID,
				Timestamp: d.Timestamp,
			})

		case capture.DeltaStateSet:
			state.DB[d.Key] = d.After
		}
	}

	// Collect in-flight requests
	for _, req := range requestStarts {
		state.HTTP.ActiveRequests = append(state.HTTP.ActiveRequests, req)
	}

	// Keep only last 20 completed requests
	if len(state.HTTP.RecentComplete) > 20 {
		state.HTTP.RecentComplete = state.HTTP.RecentComplete[len(state.HTTP.RecentComplete)-20:]
	}
	// Keep only last 10 external calls
	if len(state.External) > 10 {
		state.External = state.External[len(state.External)-10:]
	}
	// Keep only last 10 errors
	if len(state.Errors) > 10 {
		state.Errors = state.Errors[len(state.Errors)-10:]
	}

	return state, nil
}

// Diff compares system state between two timestamps and identifies what changed.
type Diff struct {
	Before    time.Time    `json:"before"`
	After     time.Time    `json:"after"`
	Changes   []Change     `json:"changes"`
	Summary   DiffSummary  `json:"summary"`
}

// Change is a single difference between two states.
type Change struct {
	Category  string `json:"category"` // config, flag, db, http, error
	Key       string `json:"key"`
	Before    string `json:"before"`
	After     string `json:"after"`
	Timestamp time.Time `json:"timestamp"`
	Type      string `json:"type"` // added, removed, modified
}

// DiffSummary gives aggregate counts of changes.
type DiffSummary struct {
	ConfigChanges  int `json:"config_changes"`
	FlagChanges    int `json:"flag_changes"`
	DBChanges      int `json:"db_changes"`
	Errors         int `json:"errors"`
	ExternalCalls  int `json:"external_calls"`
	TotalDeltas    int `json:"total_deltas"`
}

// Between computes the diff between two timestamps.
func Between(db *store.DB, before, after time.Time) (*Diff, error) {
	deltas, err := db.GetTimeRange(before, after)
	if err != nil {
		return nil, err
	}

	diff := &Diff{
		Before: before,
		After:  after,
	}

	for _, d := range deltas {
		change := Change{
			Category:  d.Category,
			Key:       d.Key,
			Before:    d.Before,
			After:     d.After,
			Timestamp: d.Timestamp,
		}

		if d.Before == "" && d.After != "" {
			change.Type = "added"
		} else if d.Before != "" && d.After == "" {
			change.Type = "removed"
		} else {
			change.Type = "modified"
		}

		diff.Changes = append(diff.Changes, change)

		switch d.Category {
		case "config":
			diff.Summary.ConfigChanges++
		case "flag":
			diff.Summary.FlagChanges++
		case "db":
			diff.Summary.DBChanges++
		case "error":
			diff.Summary.Errors++
		case "external":
			diff.Summary.ExternalCalls++
		}
	}

	diff.Summary.TotalDeltas = len(deltas)

	// Sort by timestamp
	sort.Slice(diff.Changes, func(i, j int) bool {
		return diff.Changes[i].Timestamp.Before(diff.Changes[j].Timestamp)
	})

	return diff, nil
}

// ForRequest reconstructs the full execution trace of a single request.
type RequestTrace struct {
	RequestID   string          `json:"request_id"`
	Method      string          `json:"method"`
	Path        string          `json:"path"`
	UserID      string          `json:"user_id"`
	StatusCode  int             `json:"status_code"`
	LatencyMs   int             `json:"latency_ms"`
	Steps       []TraceStep     `json:"steps"`
	Config      map[string]string `json:"config_at_time"`
	Flags       map[string]string `json:"flags_at_time"`
}

// TraceStep is a single event in a request's lifecycle.
type TraceStep struct {
	Timestamp   time.Time `json:"timestamp"`
	Type        string    `json:"type"`
	Description string    `json:"description"`
	Detail      string    `json:"detail,omitempty"`
	LatencyMs   int       `json:"latency_ms,omitempty"`
}

// ForRequest reconstructs the full trace of a single request.
func ForRequest(db *store.DB, requestID string) (*RequestTrace, error) {
	deltas, err := db.GetRequestTrace(requestID)
	if err != nil {
		return nil, err
	}

	if len(deltas) == 0 {
		return nil, fmt.Errorf("no deltas found for request %s", requestID)
	}

	trace := &RequestTrace{
		RequestID: requestID,
		Config:    map[string]string{},
		Flags:     map[string]string{},
	}

	for _, d := range deltas {
		step := TraceStep{
			Timestamp: d.Timestamp,
			Type:      string(d.Type),
		}

		switch d.Type {
		case capture.DeltaHTTPRequest:
			trace.Method = d.Metadata.Method
			trace.Path = d.Metadata.Path
			trace.UserID = d.Metadata.UserID
			trace.StatusCode = d.Metadata.StatusCode
			trace.LatencyMs = d.Metadata.LatencyMs
			step.Description = fmt.Sprintf("%s %s → %d", d.Metadata.Method, d.Metadata.Path, d.Metadata.StatusCode)
			step.LatencyMs = d.Metadata.LatencyMs

		case capture.DeltaDBWrite, capture.DeltaDBRead:
			step.Description = fmt.Sprintf("DB: %s %s", d.Metadata.Operation, d.Metadata.Table)
			step.Detail = d.After

		case capture.DeltaExternalCall:
			step.Description = fmt.Sprintf("External: %s %s → %d", d.Metadata.Method, d.Metadata.Path, d.Metadata.StatusCode)
			step.LatencyMs = d.Metadata.LatencyMs

		case capture.DeltaError:
			step.Description = "Error: " + d.Metadata.ErrorMessage
			step.Detail = d.After

		default:
			step.Description = fmt.Sprintf("%s: %s", d.Type, d.Key)
		}

		trace.Steps = append(trace.Steps, step)
	}

	// Get config/flags at the time of the request
	if len(deltas) > 0 {
		firstDelta := deltas[0]
		state, err := AtTime(db, firstDelta.Timestamp)
		if err == nil {
			trace.Config = state.Config
			trace.Flags = state.Flags
		}
	}

	return trace, nil
}
