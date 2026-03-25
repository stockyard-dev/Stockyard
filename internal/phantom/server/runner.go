package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/stockyard-dev/stockyard/internal/phantom/store"
)

// canaryProbe defines a single test request to send through the proxy.
type canaryProbe struct {
	Name        string  // human label for the probe
	Model       string  // model to request
	Prompt      string  // message to send
	MaxTokens   int     // max tokens in response
	MaxLatency  time.Duration // anomaly if slower than this
	ExpectWords []string // response should contain at least one of these (empty = any non-empty)
}

// probeResult captures what happened when we ran a probe.
type probeResult struct {
	Probe       canaryProbe
	StatusCode  int
	Latency     time.Duration
	Content     string
	Model       string
	Error       string
	Anomalies   []store.Anomaly
}

// personaProbes generates the test probes for a persona based on its goals.
func personaProbes(p *store.Persona) []canaryProbe {
	var goals []string
	var profile map[string]any
	json.Unmarshal([]byte(p.BehaviorProfile), &profile)
	if g, ok := profile["goals"].([]any); ok {
		for _, v := range g {
			if s, ok := v.(string); ok {
				goals = append(goals, s)
			}
		}
	}

	probes := []canaryProbe{
		// Every persona gets the basics
		{
			Name:       "basic-completion",
			Model:      "gpt-4o-mini",
			Prompt:     "Reply with exactly: canary alive",
			MaxTokens:  10,
			MaxLatency: 10 * time.Second,
			ExpectWords: []string{"canary", "alive"},
		},
		{
			Name:       "anthropic-routing",
			Model:      "claude-sonnet-4-20250514",
			Prompt:     "Reply with exactly: claude routed",
			MaxTokens:  10,
			MaxLatency: 10 * time.Second,
			ExpectWords: []string{"claude", "routed"},
		},
	}

	// Goal-specific probes
	for _, goal := range goals {
		switch {
		case strings.Contains(goal, "failover"):
			probes = append(probes, canaryProbe{
				Name:       "failover-model-routing",
				Model:      "claude-sonnet-4-20250514",
				Prompt:     "What provider are you? Reply in one word.",
				MaxTokens:  10,
				MaxLatency: 15 * time.Second,
			})
		case strings.Contains(goal, "cost"):
			probes = append(probes, canaryProbe{
				Name:       "cost-tracking-cheap",
				Model:      "gpt-4o-mini",
				Prompt:     "Say yes.",
				MaxTokens:  3,
				MaxLatency: 8 * time.Second,
			})
		case strings.Contains(goal, "setup") || strings.Contains(goal, "api"):
			probes = append(probes, canaryProbe{
				Name:       "first-request-experience",
				Model:      "gpt-4o-mini",
				Prompt:     "Hello! I'm testing the API for the first time. Reply with a short welcome.",
				MaxTokens:  30,
				MaxLatency: 10 * time.Second,
			})
		}
	}

	return probes
}

// runSession executes a full canary session for a persona.
// It sends probes through the proxy, records results and anomalies.
func (s *Server) runSession(p *store.Persona, apiKey string) {
	sessionID := genID("ps")
	session := &store.Session{
		ID:        sessionID,
		PersonaID: p.ID,
		StartedAt: time.Now(),
		Status:    "running",
	}

	if err := s.cfg.Store.CreateSession(session); err != nil {
		log.Printf("[phantom] failed to create session: %v", err)
		return
	}
	s.cfg.Store.SetPersonaStatus(p.ID, "running")

	probes := personaProbes(p)
	var allAnomalies []store.Anomaly
	turns := 0

	for _, probe := range probes {
		result := s.executeProbe(p, sessionID, probe, apiKey)
		turns++
		allAnomalies = append(allAnomalies, result.Anomalies...)

		log.Printf("[phantom] %s/%s: %dms status=%d anomalies=%d content=%q",
			p.Name, probe.Name, result.Latency.Milliseconds(), result.StatusCode,
			len(result.Anomalies), truncate(result.Content, 60))
	}

	// Persist anomalies
	for i := range allAnomalies {
		s.cfg.Store.CreateAnomaly(&allAnomalies[i])
	}

	// Complete session
	anomalySummary, _ := json.Marshal(allAnomalies)
	s.cfg.Store.CompleteSession(sessionID, turns, string(anomalySummary))
	s.cfg.Store.IncrementPersonaSessions(p.ID, len(allAnomalies))

	log.Printf("[phantom] session %s complete: %d turns, %d anomalies", sessionID, turns, len(allAnomalies))
}

// executeProbe sends a single request through the proxy and checks for anomalies.
func (s *Server) executeProbe(p *store.Persona, sessionID string, probe canaryProbe, apiKey string) probeResult {
	result := probeResult{Probe: probe}

	// Build request
	body, _ := json.Marshal(map[string]any{
		"model":      probe.Model,
		"messages":   []map[string]string{{"role": "user", "content": probe.Prompt}},
		"max_tokens": probe.MaxTokens,
	})

	req, err := http.NewRequest("POST", p.TargetURL, bytes.NewReader(body))
	if err != nil {
		result.Error = err.Error()
		result.Anomalies = append(result.Anomalies, store.Anomaly{
			ID: genID("pa"), PersonaID: p.ID, SessionID: sessionID,
			Severity: "critical", Category: "request_build",
			Description: "Failed to build request: " + err.Error(),
		})
		return result
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("X-Stockyard-Tags", `{"source":"phantom","persona":"`+p.Name+`"}`)

	// Send
	client := &http.Client{Timeout: 30 * time.Second}
	start := time.Now()
	resp, err := client.Do(req)
	result.Latency = time.Since(start)

	if err != nil {
		result.Error = err.Error()
		result.Anomalies = append(result.Anomalies, store.Anomaly{
			ID: genID("pa"), PersonaID: p.ID, SessionID: sessionID,
			Severity: "critical", Category: "connection",
			Description: fmt.Sprintf("Probe %s failed to connect: %s", probe.Name, err.Error()),
			Expected: "HTTP 200", Actual: "connection error",
		})
		return result
	}
	defer resp.Body.Close()
	result.StatusCode = resp.StatusCode

	respBody, _ := io.ReadAll(resp.Body)

	// Check HTTP status
	if resp.StatusCode != 200 {
		result.Error = string(respBody)
		severity := "high"
		if resp.StatusCode >= 500 {
			severity = "critical"
		}
		result.Anomalies = append(result.Anomalies, store.Anomaly{
			ID: genID("pa"), PersonaID: p.ID, SessionID: sessionID,
			Severity: severity, Category: "http_error",
			Description: fmt.Sprintf("Probe %s returned HTTP %d", probe.Name, resp.StatusCode),
			Expected: "200", Actual: fmt.Sprintf("%d", resp.StatusCode),
			Evidence: truncate(string(respBody), 500),
		})
		return result
	}

	// Parse response
	var oaiResp struct {
		Model   string `json:"model"`
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(respBody, &oaiResp); err != nil {
		result.Anomalies = append(result.Anomalies, store.Anomaly{
			ID: genID("pa"), PersonaID: p.ID, SessionID: sessionID,
			Severity: "high", Category: "response_parse",
			Description: fmt.Sprintf("Probe %s: unparseable response", probe.Name),
			Expected: "valid JSON", Actual: truncate(string(respBody), 200),
		})
		return result
	}

	result.Model = oaiResp.Model
	if len(oaiResp.Choices) > 0 {
		result.Content = oaiResp.Choices[0].Message.Content
	}

	// Check: empty response
	if result.Content == "" {
		result.Anomalies = append(result.Anomalies, store.Anomaly{
			ID: genID("pa"), PersonaID: p.ID, SessionID: sessionID,
			Severity: "high", Category: "empty_response",
			Description: fmt.Sprintf("Probe %s: empty content in response", probe.Name),
			Expected: "non-empty content", Actual: "empty",
		})
	}

	// Check: latency
	if probe.MaxLatency > 0 && result.Latency > probe.MaxLatency {
		result.Anomalies = append(result.Anomalies, store.Anomaly{
			ID: genID("pa"), PersonaID: p.ID, SessionID: sessionID,
			Severity: "medium", Category: "latency",
			Description: fmt.Sprintf("Probe %s: %dms exceeds %dms threshold", probe.Name, result.Latency.Milliseconds(), probe.MaxLatency.Milliseconds()),
			Expected: fmt.Sprintf("<%dms", probe.MaxLatency.Milliseconds()),
			Actual:   fmt.Sprintf("%dms", result.Latency.Milliseconds()),
		})
	}

	// Check: expected words
	if len(probe.ExpectWords) > 0 {
		lower := strings.ToLower(result.Content)
		found := false
		for _, word := range probe.ExpectWords {
			if strings.Contains(lower, strings.ToLower(word)) {
				found = true
				break
			}
		}
		if !found {
			result.Anomalies = append(result.Anomalies, store.Anomaly{
				ID: genID("pa"), PersonaID: p.ID, SessionID: sessionID,
				Severity: "low", Category: "content_mismatch",
				Description: fmt.Sprintf("Probe %s: response missing expected words", probe.Name),
				Expected: strings.Join(probe.ExpectWords, " | "),
				Actual:   truncate(result.Content, 200),
			})
		}
	}

	return result
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
