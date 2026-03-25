// Action handlers that perform real infrastructure operations.
package act

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"github.com/stockyard-dev/stockyard/internal/spine/decide"
)

// ScriptHandler executes user-defined shell scripts per action type.
// Scripts are mapped from action type to file path:
//
//	handlers:
//	  scale_up: ./scripts/scale-up.sh
//	  scale_down: ./scripts/scale-down.sh
//	  restart: ./scripts/restart.sh
type ScriptHandler struct {
	Scripts map[string]string // action_type -> script path
	Timeout time.Duration
}

// NewScriptHandler creates a script handler with the given action-to-script mapping.
func NewScriptHandler(scripts map[string]string) *ScriptHandler {
	return &ScriptHandler{
		Scripts: scripts,
		Timeout: 60 * time.Second,
	}
}

// Execute runs the script associated with the decision's action type.
func (h *ScriptHandler) Execute(ctx context.Context, d decide.Decision) error {
	script, ok := h.Scripts[string(d.Action)]
	if !ok {
		return fmt.Errorf("no script configured for action %s", d.Action)
	}

	timeout := h.Timeout
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(execCtx, "/bin/sh", "-c", script)

	// Pass decision details as environment variables
	cmd.Env = append(cmd.Environ(),
		"SPINE_ACTION="+string(d.Action),
		"SPINE_URGENCY="+string(d.Urgency),
		"SPINE_REASON="+d.Reason,
		"SPINE_DECISION_ID="+d.ID,
	)
	if d.Params != nil {
		for k, v := range d.Params {
			cmd.Env = append(cmd.Env, fmt.Sprintf("SPINE_PARAM_%s=%v", strings.ToUpper(k), v))
		}
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("script %s failed: %w (output: %s)", script, err, strings.TrimSpace(string(output)))
	}
	return nil
}

// Rollback runs the rollback script if configured (script path with _rollback suffix).
func (h *ScriptHandler) Rollback(ctx context.Context, d decide.Decision) error {
	rollbackKey := string(d.Action) + "_rollback"
	script, ok := h.Scripts[rollbackKey]
	if !ok {
		return fmt.Errorf("no rollback script for action %s", d.Action)
	}
	return h.executeScript(ctx, script, d)
}

func (h *ScriptHandler) executeScript(ctx context.Context, script string, d decide.Decision) error {
	execCtx, cancel := context.WithTimeout(ctx, h.Timeout)
	defer cancel()

	cmd := exec.CommandContext(execCtx, "/bin/sh", "-c", script)
	cmd.Env = append(cmd.Environ(),
		"SPINE_ACTION="+string(d.Action),
		"SPINE_DECISION_ID="+d.ID,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("script %s failed: %w (output: %s)", script, err, strings.TrimSpace(string(output)))
	}
	return nil
}

// DockerHandler uses docker compose to scale services up or down.
type DockerHandler struct {
	ComposeFile string // path to docker-compose.yml (optional, uses default if empty)
	ServiceName string // the service to scale
}

// NewDockerHandler creates a handler that uses docker compose for scaling.
func NewDockerHandler(service string, composeFile string) *DockerHandler {
	return &DockerHandler{
		ComposeFile: composeFile,
		ServiceName: service,
	}
}

// Execute runs docker compose scale for the configured service.
func (h *DockerHandler) Execute(ctx context.Context, d decide.Decision) error {
	delta := 0
	if d.Params != nil {
		if v, ok := d.Params["instances_delta"]; ok {
			switch val := v.(type) {
			case float64:
				delta = int(val)
			case int:
				delta = val
			}
		}
	}

	switch d.Action {
	case decide.ActionScaleUp:
		if delta <= 0 {
			delta = 1
		}
		return h.scale(ctx, delta)
	case decide.ActionScaleDown:
		if delta >= 0 {
			delta = -1
		}
		return h.scale(ctx, delta)
	case decide.ActionRestart:
		return h.restart(ctx)
	default:
		return fmt.Errorf("docker handler does not support action %s", d.Action)
	}
}

func (h *DockerHandler) scale(ctx context.Context, delta int) error {
	// Get current scale first — we compose the final count
	// For simplicity, we use an absolute count based on delta.
	// Positive delta = add instances, negative = remove.
	count := 1 + delta
	if count < 1 {
		count = 1
	}

	args := []string{"compose"}
	if h.ComposeFile != "" {
		args = append(args, "-f", h.ComposeFile)
	}
	args = append(args, "up", "--scale", fmt.Sprintf("%s=%d", h.ServiceName, count), "-d", "--no-recreate")

	cmd := exec.CommandContext(ctx, "docker", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker compose scale failed: %w (output: %s)", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func (h *DockerHandler) restart(ctx context.Context) error {
	args := []string{"compose"}
	if h.ComposeFile != "" {
		args = append(args, "-f", h.ComposeFile)
	}
	args = append(args, "restart", h.ServiceName)

	cmd := exec.CommandContext(ctx, "docker", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker compose restart failed: %w (output: %s)", err, strings.TrimSpace(string(output)))
	}
	return nil
}

// Rollback attempts to reverse a docker scaling action.
func (h *DockerHandler) Rollback(ctx context.Context, d decide.Decision) error {
	// Reverse the delta
	reversed := decide.Decision{
		ID:     d.ID,
		Action: d.Action,
		Params: map[string]any{},
	}
	if d.Params != nil {
		if v, ok := d.Params["instances_delta"]; ok {
			switch val := v.(type) {
			case float64:
				reversed.Params["instances_delta"] = -val
			case int:
				reversed.Params["instances_delta"] = -val
			}
		}
	}
	return h.Execute(ctx, reversed)
}

// HTTPHandler sends decision details as an HTTP POST to a webhook URL.
// Useful for integrating with PagerDuty, Slack, OpsGenie, etc.
type HTTPHandler struct {
	WebhookURL string
	Headers    map[string]string // additional headers (e.g. Authorization)
	client     *http.Client
}

// NewHTTPHandler creates a webhook-based action handler.
func NewHTTPHandler(webhookURL string, headers map[string]string) *HTTPHandler {
	return &HTTPHandler{
		WebhookURL: webhookURL,
		Headers:    headers,
		client: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

// Execute POSTs the decision details to the configured webhook URL.
func (h *HTTPHandler) Execute(ctx context.Context, d decide.Decision) error {
	payload := map[string]any{
		"event":     "spine.action",
		"action":    string(d.Action),
		"urgency":   string(d.Urgency),
		"reason":    d.Reason,
		"impact":    d.Impact,
		"rollback":  d.Rollback,
		"decision_id": d.ID,
		"timestamp": d.Timestamp.Format(time.RFC3339),
		"params":    d.Params,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshaling webhook payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", h.WebhookURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("creating webhook request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Stockyard-Spine/1.0")
	for k, v := range h.Headers {
		req.Header.Set(k, v)
	}

	resp, err := h.client.Do(req)
	if err != nil {
		return fmt.Errorf("webhook POST to %s failed: %w", h.WebhookURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("webhook returned error: status %d", resp.StatusCode)
	}
	return nil
}

// Rollback sends a rollback notification to the webhook.
func (h *HTTPHandler) Rollback(ctx context.Context, d decide.Decision) error {
	payload := map[string]any{
		"event":       "spine.rollback",
		"action":      string(d.Action),
		"decision_id": d.ID,
		"timestamp":   time.Now().Format(time.RFC3339),
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshaling rollback payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", h.WebhookURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("creating rollback request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range h.Headers {
		req.Header.Set(k, v)
	}

	resp, err := h.client.Do(req)
	if err != nil {
		return fmt.Errorf("webhook rollback POST failed: %w", err)
	}
	resp.Body.Close()
	return nil
}
