package server

import (
	"bufio"
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

// ─── Probe Types ────────────────────────────────────────────────────────────

// canaryProbe defines a single test request to send through the proxy.
type canaryProbe struct {
	Name        string        // human label for the probe
	Model       string        // model to request
	Prompt      string        // message to send
	MaxTokens   int           // max tokens in response
	MaxLatency  time.Duration // anomaly if slower than this
	ExpectWords []string      // response should contain at least one of these (empty = any non-empty)
	Stream      bool          // send as streaming request
}

// multiTurnProbe defines a multi-turn conversation test.
type multiTurnProbe struct {
	Name       string
	Model      string
	MaxLatency time.Duration
	Turns      []multiTurnStep
}

type multiTurnStep struct {
	Prompt      string
	MaxTokens   int
	ExpectWords []string // at least one must appear in response
}

// edgeCaseProbe defines a probe that tests error handling and boundary conditions.
type edgeCaseProbe struct {
	Name         string
	Model        string
	Messages     []map[string]string // raw messages array
	MaxTokens    int
	ExpectStatus int           // expected HTTP status (0 = 200)
	MaxLatency   time.Duration
	Description  string        // what we're testing
}

// probeResult captures what happened when we ran a probe.
type probeResult struct {
	Probe     canaryProbe
	StatusCode int
	Latency    time.Duration
	Content    string
	Model      string
	Error      string
	Anomalies  []store.Anomaly
}

// ─── Probe Generation ───────────────────────────────────────────────────────

// personaProbes generates the single-turn test probes for a persona.
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
			Name:        "basic-completion",
			Model:       "gpt-4o-mini",
			Prompt:      "Reply with exactly: canary alive",
			MaxTokens:   10,
			MaxLatency:  10 * time.Second,
			ExpectWords: []string{"canary", "alive"},
		},
		{
			Name:        "anthropic-routing",
			Model:       "claude-sonnet-4-20250514",
			Prompt:      "Reply with exactly: claude routed",
			MaxTokens:   10,
			MaxLatency:  10 * time.Second,
			ExpectWords: []string{"claude", "routed"},
		},
		// Streaming probe for every persona
		{
			Name:        "streaming-basic",
			Model:       "gpt-4o-mini",
			Prompt:      "Count from 1 to 5, separated by commas.",
			MaxTokens:   20,
			MaxLatency:  15 * time.Second,
			ExpectWords: []string{"1", "3", "5"},
			Stream:      true,
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
			probes = append(probes, canaryProbe{
				Name:        "streaming-anthropic",
				Model:       "claude-sonnet-4-20250514",
				Prompt:      "Reply with exactly: stream ok",
				MaxTokens:   10,
				MaxLatency:  15 * time.Second,
				ExpectWords: []string{"stream", "ok"},
				Stream:      true,
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

// personaMultiTurnProbes generates multi-turn conversation tests.
func personaMultiTurnProbes(p *store.Persona) []multiTurnProbe {
	probes := []multiTurnProbe{
		{
			Name:       "context-retention",
			Model:      "gpt-4o-mini",
			MaxLatency: 12 * time.Second,
			Turns: []multiTurnStep{
				{
					Prompt:      "My favorite color is cerulean blue. Remember this. Reply with: understood.",
					MaxTokens:   10,
					ExpectWords: []string{"understood"},
				},
				{
					Prompt:      "What is my favorite color? Reply with just the color.",
					MaxTokens:   10,
					ExpectWords: []string{"cerulean", "blue"},
				},
			},
		},
		{
			Name:       "role-adherence",
			Model:      "gpt-4o-mini",
			MaxLatency: 12 * time.Second,
			Turns: []multiTurnStep{
				{
					Prompt:      "You are a pirate. From now on reply like a pirate. Say ahoy.",
					MaxTokens:   20,
					ExpectWords: []string{"ahoy", "Ahoy"},
				},
				{
					Prompt:      "What is 2+2?",
					MaxTokens:   30,
					ExpectWords: []string{"4", "four"},
				},
			},
		},
	}

	// Enterprise evaluators get a harder multi-turn test
	var profile map[string]any
	json.Unmarshal([]byte(p.BehaviorProfile), &profile)
	if level, _ := profile["technical_level"].(string); level == "expert" {
		probes = append(probes, multiTurnProbe{
			Name:       "cross-model-handoff",
			Model:      "gpt-4o-mini",
			MaxLatency: 15 * time.Second,
			Turns: []multiTurnStep{
				{
					Prompt:      "The secret code is PHANTOM-7742. Confirm by repeating the code.",
					MaxTokens:   20,
					ExpectWords: []string{"PHANTOM-7742", "phantom-7742"},
				},
				{
					Prompt:      "What was the secret code I gave you?",
					MaxTokens:   20,
					ExpectWords: []string{"PHANTOM-7742", "phantom-7742", "7742"},
				},
			},
		})
	}

	return probes
}

// personaEdgeCaseProbes generates boundary and error-handling tests.
func personaEdgeCaseProbes(p *store.Persona) []edgeCaseProbe {
	probes := []edgeCaseProbe{
		{
			Name:        "empty-message",
			Model:       "gpt-4o-mini",
			Messages:    []map[string]string{{"role": "user", "content": ""}},
			MaxTokens:   10,
			ExpectStatus: 0, // provider-dependent, just shouldn't crash
			MaxLatency:  10 * time.Second,
			Description: "Empty user message should not crash the proxy",
		},
		{
			Name:        "max-tokens-one",
			Model:       "gpt-4o-mini",
			Messages:    []map[string]string{{"role": "user", "content": "Tell me a long story about dragons."}},
			MaxTokens:   1,
			ExpectStatus: 200,
			MaxLatency:  10 * time.Second,
			Description: "max_tokens=1 should return a valid truncated response",
		},
		{
			Name:        "special-characters",
			Model:       "gpt-4o-mini",
			Messages:    []map[string]string{{"role": "user", "content": "Reply with: <script>alert('xss')</script> & \"quotes\" → émojis 🤠"}},
			MaxTokens:   30,
			ExpectStatus: 200,
			MaxLatency:  10 * time.Second,
			Description: "Special characters, HTML, unicode should pass through cleanly",
		},
		{
			Name:        "system-message-handling",
			Model:       "gpt-4o-mini",
			Messages: []map[string]string{
				{"role": "system", "content": "You are a helpful calculator. Only respond with numbers."},
				{"role": "user", "content": "What is 7 times 6?"},
			},
			MaxTokens:   10,
			ExpectStatus: 200,
			MaxLatency:  10 * time.Second,
			Description: "System messages should be forwarded correctly",
		},
		{
			Name:        "unknown-model-graceful",
			Model:       "nonexistent-model-xyz-999",
			Messages:    []map[string]string{{"role": "user", "content": "Hello"}},
			MaxTokens:   10,
			ExpectStatus: 0, // expect error but not a crash
			MaxLatency:  10 * time.Second,
			Description: "Unknown model should return a clean error, not a panic",
		},
	}

	return probes
}

// ─── Session Runner ─────────────────────────────────────────────────────────

// runSession executes a full canary session for a persona.
// It sends single-turn probes, multi-turn conversations, streaming tests,
// and edge case probes through the proxy, recording results and anomalies.
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

	var allAnomalies []store.Anomaly
	turns := 0

	// Phase 1: Single-turn probes (including streaming)
	probes := personaProbes(p)
	for _, probe := range probes {
		if probe.Stream {
			result := s.executeStreamProbe(p, sessionID, probe, apiKey)
			turns++
			allAnomalies = append(allAnomalies, result.Anomalies...)
			log.Printf("[phantom] %s/%s (stream): %dms status=%d anomalies=%d content=%q",
				p.Name, probe.Name, result.Latency.Milliseconds(), result.StatusCode,
				len(result.Anomalies), truncate(result.Content, 60))
		} else {
			result := s.executeProbe(p, sessionID, probe, apiKey)
			turns++
			allAnomalies = append(allAnomalies, result.Anomalies...)
			log.Printf("[phantom] %s/%s: %dms status=%d anomalies=%d content=%q",
				p.Name, probe.Name, result.Latency.Milliseconds(), result.StatusCode,
				len(result.Anomalies), truncate(result.Content, 60))
		}
	}

	// Phase 2: Multi-turn probes
	multiProbes := personaMultiTurnProbes(p)
	for _, mp := range multiProbes {
		anomalies, stepCount := s.executeMultiTurnProbe(p, sessionID, mp, apiKey)
		turns += stepCount
		allAnomalies = append(allAnomalies, anomalies...)
	}

	// Phase 3: Edge case probes
	edgeProbes := personaEdgeCaseProbes(p)
	for _, ep := range edgeProbes {
		anomalies := s.executeEdgeCaseProbe(p, sessionID, ep, apiKey)
		turns++
		allAnomalies = append(allAnomalies, anomalies...)
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

// ─── Single-Turn Executor ───────────────────────────────────────────────────

// executeProbe sends a single non-streaming request through the proxy.
func (s *Server) executeProbe(p *store.Persona, sessionID string, probe canaryProbe, apiKey string) probeResult {
	result := probeResult{Probe: probe}

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

	if resp.StatusCode != 200 {
		result.Error = string(respBody)
		severity := "high"
		if resp.StatusCode >= 500 { severity = "critical" }
		result.Anomalies = append(result.Anomalies, store.Anomaly{
			ID: genID("pa"), PersonaID: p.ID, SessionID: sessionID,
			Severity: severity, Category: "http_error",
			Description: fmt.Sprintf("Probe %s returned HTTP %d", probe.Name, resp.StatusCode),
			Expected: "200", Actual: fmt.Sprintf("%d", resp.StatusCode),
			Evidence: truncate(string(respBody), 500),
		})
		return result
	}

	content, model := parseOAIResponse(respBody)
	result.Model = model
	result.Content = content

	result.Anomalies = append(result.Anomalies, checkContent(p.ID, sessionID, probe.Name, content, probe.ExpectWords)...)
	result.Anomalies = append(result.Anomalies, checkLatency(p.ID, sessionID, probe.Name, result.Latency, probe.MaxLatency)...)

	return result
}

// ─── Streaming Executor ─────────────────────────────────────────────────────

// executeStreamProbe sends a streaming request and validates SSE chunks.
func (s *Server) executeStreamProbe(p *store.Persona, sessionID string, probe canaryProbe, apiKey string) probeResult {
	result := probeResult{Probe: probe}

	body, _ := json.Marshal(map[string]any{
		"model":      probe.Model,
		"messages":   []map[string]string{{"role": "user", "content": probe.Prompt}},
		"max_tokens": probe.MaxTokens,
		"stream":     true,
	})

	req, err := http.NewRequest("POST", p.TargetURL, bytes.NewReader(body))
	if err != nil {
		result.Error = err.Error()
		result.Anomalies = append(result.Anomalies, store.Anomaly{
			ID: genID("pa"), PersonaID: p.ID, SessionID: sessionID,
			Severity: "critical", Category: "request_build",
			Description: fmt.Sprintf("Stream probe %s: failed to build request: %s", probe.Name, err.Error()),
		})
		return result
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("X-Stockyard-Tags", `{"source":"phantom","persona":"`+p.Name+`","probe_type":"stream"}`)

	client := &http.Client{Timeout: 60 * time.Second}
	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		result.Latency = time.Since(start)
		result.Error = err.Error()
		result.Anomalies = append(result.Anomalies, store.Anomaly{
			ID: genID("pa"), PersonaID: p.ID, SessionID: sessionID,
			Severity: "critical", Category: "stream_connection",
			Description: fmt.Sprintf("Stream probe %s: connection failed: %s", probe.Name, err.Error()),
		})
		return result
	}
	defer resp.Body.Close()
	result.StatusCode = resp.StatusCode

	if resp.StatusCode != 200 {
		result.Latency = time.Since(start)
		respBody, _ := io.ReadAll(resp.Body)
		result.Error = string(respBody)
		result.Anomalies = append(result.Anomalies, store.Anomaly{
			ID: genID("pa"), PersonaID: p.ID, SessionID: sessionID,
			Severity: "high", Category: "stream_http_error",
			Description: fmt.Sprintf("Stream probe %s: HTTP %d", probe.Name, resp.StatusCode),
			Expected: "200", Actual: fmt.Sprintf("%d", resp.StatusCode),
			Evidence: truncate(string(respBody), 500),
		})
		return result
	}

	// Validate SSE stream
	var contentParts []string
	chunks := 0
	gotDone := false
	ttfb := time.Duration(0)

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")

		if chunks == 0 {
			ttfb = time.Since(start)
		}
		chunks++

		if data == "[DONE]" {
			gotDone = true
			break
		}

		// Parse SSE chunk for content delta
		var chunk struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
			} `json:"choices"`
			Model string `json:"model"`
		}
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			result.Anomalies = append(result.Anomalies, store.Anomaly{
				ID: genID("pa"), PersonaID: p.ID, SessionID: sessionID,
				Severity: "medium", Category: "stream_parse",
				Description: fmt.Sprintf("Stream probe %s: unparseable chunk #%d", probe.Name, chunks),
				Expected: "valid JSON SSE chunk", Actual: truncate(data, 200),
			})
			continue
		}
		if chunk.Model != "" {
			result.Model = chunk.Model
		}
		if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != "" {
			contentParts = append(contentParts, chunk.Choices[0].Delta.Content)
		}
	}
	result.Latency = time.Since(start)
	result.Content = strings.Join(contentParts, "")

	// Stream-specific validations
	if chunks == 0 {
		result.Anomalies = append(result.Anomalies, store.Anomaly{
			ID: genID("pa"), PersonaID: p.ID, SessionID: sessionID,
			Severity: "critical", Category: "stream_empty",
			Description: fmt.Sprintf("Stream probe %s: received zero SSE chunks", probe.Name),
			Expected: ">0 chunks", Actual: "0",
		})
	}

	if !gotDone && chunks > 0 {
		result.Anomalies = append(result.Anomalies, store.Anomaly{
			ID: genID("pa"), PersonaID: p.ID, SessionID: sessionID,
			Severity: "medium", Category: "stream_no_done",
			Description: fmt.Sprintf("Stream probe %s: stream ended without [DONE] sentinel", probe.Name),
			Expected: "data: [DONE]", Actual: fmt.Sprintf("%d chunks, no DONE", chunks),
		})
	}

	if ttfb > 5*time.Second && chunks > 0 {
		result.Anomalies = append(result.Anomalies, store.Anomaly{
			ID: genID("pa"), PersonaID: p.ID, SessionID: sessionID,
			Severity: "medium", Category: "stream_ttfb",
			Description: fmt.Sprintf("Stream probe %s: TTFB %dms is slow", probe.Name, ttfb.Milliseconds()),
			Expected: "<5000ms TTFB", Actual: fmt.Sprintf("%dms", ttfb.Milliseconds()),
		})
	}

	log.Printf("[phantom] %s/%s (stream): %d chunks, TTFB=%dms, total=%dms, done=%v",
		p.Name, probe.Name, chunks, ttfb.Milliseconds(), result.Latency.Milliseconds(), gotDone)

	result.Anomalies = append(result.Anomalies, checkContent(p.ID, sessionID, probe.Name, result.Content, probe.ExpectWords)...)
	result.Anomalies = append(result.Anomalies, checkLatency(p.ID, sessionID, probe.Name, result.Latency, probe.MaxLatency)...)

	return result
}

// ─── Multi-Turn Executor ────────────────────────────────────────────────────

// executeMultiTurnProbe runs a multi-turn conversation, carrying context forward.
func (s *Server) executeMultiTurnProbe(p *store.Persona, sessionID string, mp multiTurnProbe, apiKey string) ([]store.Anomaly, int) {
	var allAnomalies []store.Anomaly
	var messages []map[string]string

	for i, step := range mp.Turns {
		messages = append(messages, map[string]string{"role": "user", "content": step.Prompt})

		body, _ := json.Marshal(map[string]any{
			"model":      mp.Model,
			"messages":   messages,
			"max_tokens": step.MaxTokens,
		})

		req, err := http.NewRequest("POST", p.TargetURL, bytes.NewReader(body))
		if err != nil {
			allAnomalies = append(allAnomalies, store.Anomaly{
				ID: genID("pa"), PersonaID: p.ID, SessionID: sessionID,
				Severity: "critical", Category: "multiturn_build",
				Description: fmt.Sprintf("Multi-turn %s step %d: request build failed: %s", mp.Name, i+1, err.Error()),
			})
			return allAnomalies, i + 1
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+apiKey)
		req.Header.Set("X-Stockyard-Tags", fmt.Sprintf(`{"source":"phantom","persona":"%s","probe_type":"multiturn","probe":"%s","step":%d}`, p.Name, mp.Name, i+1))

		client := &http.Client{Timeout: 30 * time.Second}
		start := time.Now()
		resp, err := client.Do(req)
		latency := time.Since(start)

		if err != nil {
			allAnomalies = append(allAnomalies, store.Anomaly{
				ID: genID("pa"), PersonaID: p.ID, SessionID: sessionID,
				Severity: "critical", Category: "multiturn_connection",
				Description: fmt.Sprintf("Multi-turn %s step %d: connection failed: %s", mp.Name, i+1, err.Error()),
			})
			return allAnomalies, i + 1
		}

		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode != 200 {
			allAnomalies = append(allAnomalies, store.Anomaly{
				ID: genID("pa"), PersonaID: p.ID, SessionID: sessionID,
				Severity: "high", Category: "multiturn_http_error",
				Description: fmt.Sprintf("Multi-turn %s step %d: HTTP %d", mp.Name, i+1, resp.StatusCode),
				Expected: "200", Actual: fmt.Sprintf("%d", resp.StatusCode),
				Evidence: truncate(string(respBody), 500),
			})
			return allAnomalies, i + 1
		}

		content, _ := parseOAIResponse(respBody)

		// Add assistant response to conversation history
		messages = append(messages, map[string]string{"role": "assistant", "content": content})

		allAnomalies = append(allAnomalies, checkContent(p.ID, sessionID, fmt.Sprintf("%s/step%d", mp.Name, i+1), content, step.ExpectWords)...)
		allAnomalies = append(allAnomalies, checkLatency(p.ID, sessionID, fmt.Sprintf("%s/step%d", mp.Name, i+1), latency, mp.MaxLatency)...)

		log.Printf("[phantom] %s/%s step %d/%d: %dms anomalies=%d content=%q",
			p.Name, mp.Name, i+1, len(mp.Turns), latency.Milliseconds(), len(allAnomalies), truncate(content, 60))
	}

	return allAnomalies, len(mp.Turns)
}

// ─── Edge Case Executor ─────────────────────────────────────────────────────

// executeEdgeCaseProbe tests boundary conditions and error handling.
func (s *Server) executeEdgeCaseProbe(p *store.Persona, sessionID string, ep edgeCaseProbe, apiKey string) []store.Anomaly {
	var anomalies []store.Anomaly

	body, _ := json.Marshal(map[string]any{
		"model":      ep.Model,
		"messages":   ep.Messages,
		"max_tokens": ep.MaxTokens,
	})

	req, err := http.NewRequest("POST", p.TargetURL, bytes.NewReader(body))
	if err != nil {
		anomalies = append(anomalies, store.Anomaly{
			ID: genID("pa"), PersonaID: p.ID, SessionID: sessionID,
			Severity: "critical", Category: "edge_build",
			Description: fmt.Sprintf("Edge case %s: request build failed: %s", ep.Name, err.Error()),
		})
		return anomalies
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("X-Stockyard-Tags", fmt.Sprintf(`{"source":"phantom","persona":"%s","probe_type":"edge","probe":"%s"}`, p.Name, ep.Name))

	client := &http.Client{Timeout: 30 * time.Second}
	start := time.Now()
	resp, err := client.Do(req)
	latency := time.Since(start)

	if err != nil {
		anomalies = append(anomalies, store.Anomaly{
			ID: genID("pa"), PersonaID: p.ID, SessionID: sessionID,
			Severity: "critical", Category: "edge_connection",
			Description: fmt.Sprintf("Edge case %s: connection failed (proxy may have crashed): %s", ep.Name, err.Error()),
		})
		return anomalies
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	// If we expected a specific status, check it
	if ep.ExpectStatus > 0 && resp.StatusCode != ep.ExpectStatus {
		anomalies = append(anomalies, store.Anomaly{
			ID: genID("pa"), PersonaID: p.ID, SessionID: sessionID,
			Severity: "medium", Category: "edge_status",
			Description: fmt.Sprintf("Edge case %s: unexpected status", ep.Name),
			Expected: fmt.Sprintf("%d", ep.ExpectStatus), Actual: fmt.Sprintf("%d", resp.StatusCode),
			Evidence: truncate(string(respBody), 300),
		})
	}

	// The key assertion: the proxy should ALWAYS return valid JSON, even for errors.
	var parsed map[string]any
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		anomalies = append(anomalies, store.Anomaly{
			ID: genID("pa"), PersonaID: p.ID, SessionID: sessionID,
			Severity: "high", Category: "edge_invalid_json",
			Description: fmt.Sprintf("Edge case %s: proxy returned non-JSON response (status %d)", ep.Name, resp.StatusCode),
			Expected: "valid JSON body", Actual: truncate(string(respBody), 300),
		})
	}

	anomalies = append(anomalies, checkLatency(p.ID, sessionID, ep.Name, latency, ep.MaxLatency)...)

	log.Printf("[phantom] %s/%s (edge): %dms status=%d anomalies=%d desc=%q",
		p.Name, ep.Name, latency.Milliseconds(), resp.StatusCode, len(anomalies), ep.Description)

	return anomalies
}

// ─── Shared Helpers ─────────────────────────────────────────────────────────

// parseOAIResponse extracts content and model from an OpenAI-compatible response.
func parseOAIResponse(body []byte) (content, model string) {
	var resp struct {
		Model   string `json:"model"`
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", ""
	}
	model = resp.Model
	if len(resp.Choices) > 0 {
		content = resp.Choices[0].Message.Content
	}
	return
}

// checkContent returns anomalies if the response is empty or missing expected words.
func checkContent(personaID, sessionID, probeName, content string, expectWords []string) []store.Anomaly {
	var anomalies []store.Anomaly

	if content == "" {
		anomalies = append(anomalies, store.Anomaly{
			ID: genID("pa"), PersonaID: personaID, SessionID: sessionID,
			Severity: "high", Category: "empty_response",
			Description: fmt.Sprintf("Probe %s: empty content in response", probeName),
			Expected: "non-empty content", Actual: "empty",
		})
		return anomalies
	}

	if len(expectWords) > 0 {
		lower := strings.ToLower(content)
		found := false
		for _, word := range expectWords {
			if strings.Contains(lower, strings.ToLower(word)) {
				found = true
				break
			}
		}
		if !found {
			anomalies = append(anomalies, store.Anomaly{
				ID: genID("pa"), PersonaID: personaID, SessionID: sessionID,
				Severity: "low", Category: "content_mismatch",
				Description: fmt.Sprintf("Probe %s: response missing expected words", probeName),
				Expected: strings.Join(expectWords, " | "),
				Actual:   truncate(content, 200),
			})
		}
	}

	return anomalies
}

// checkLatency returns an anomaly if latency exceeds the threshold.
func checkLatency(personaID, sessionID, probeName string, latency, maxLatency time.Duration) []store.Anomaly {
	if maxLatency > 0 && latency > maxLatency {
		return []store.Anomaly{{
			ID: genID("pa"), PersonaID: personaID, SessionID: sessionID,
			Severity: "medium", Category: "latency",
			Description: fmt.Sprintf("Probe %s: %dms exceeds %dms threshold", probeName, latency.Milliseconds(), maxLatency.Milliseconds()),
			Expected: fmt.Sprintf("<%dms", maxLatency.Milliseconds()),
			Actual:   fmt.Sprintf("%dms", latency.Milliseconds()),
		}}
	}
	return nil
}

func truncate(s string, n int) string {
	if len(s) <= n { return s }
	return s[:n] + "..."
}
