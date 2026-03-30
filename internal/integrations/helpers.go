package integrations

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// writeJSON writes a JSON response.
func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// mustJSON marshals to JSON string, ignoring errors.
func mustJSON(v any) string {
	b, marshalErr := json.Marshal(v)
	if marshalErr != nil {
		b = []byte("{}")
	}
	return string(b)
}

// postJSON sends a JSON POST request.
func postJSON(client *http.Client, url string, payload any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Stockyard/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("send: %w", err)
	}
	resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return nil
}

// splitCommand splits a slash command text into parts.
func splitCommand(text string) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	return strings.Fields(text)
}

// formatEventForHumans converts an event type + data into human-readable title and detail.
func formatEventForHumans(eventType string, data any) (string, string) {
	dataMap, _ := toMap(data)

	switch eventType {
	case "alert.fired":
		name, _ := dataMap["name"].(string)
		metric, _ := dataMap["metric"].(string)
		return "Alert Fired: " + name, fmt.Sprintf("Metric *%s* triggered an alert.", metric)

	case "cost.threshold":
		amount, _ := dataMap["amount"].(float64)
		return "Cost Threshold Reached", fmt.Sprintf("Spending has reached *$%.2f*.", amount)

	case "trust.violation":
		action, _ := dataMap["action"].(string)
		return "Trust Violation", fmt.Sprintf("Policy violation detected: *%s*.", action)

	case "provider.failure":
		provider, _ := dataMap["provider"].(string)
		errRate, _ := dataMap["error_rate"].(float64)
		return "Provider Failure: " + provider, fmt.Sprintf("Error rate: *%.1f%%*.", errRate*100)

	case "guardrail.violation":
		rule, _ := dataMap["rule"].(string)
		return "Guardrail Violation", fmt.Sprintf("Rule *%s* was triggered.", rule)

	case "spend.cap_breach":
		return "Spend Cap Breached", "A spending cap has been exceeded."

	default:
		return "Stockyard Event: " + eventType, fmt.Sprintf("Event data: %v", data)
	}
}

// eventSeverity returns the severity level for an event type.
func eventSeverity(eventType string) string {
	switch eventType {
	case "provider.failure", "spend.cap_breach":
		return "critical"
	case "cost.threshold", "trust.violation", "guardrail.violation":
		return "warning"
	default:
		return "info"
	}
}

// toMap converts any value to a map[string]any.
func toMap(v any) (map[string]any, bool) {
	if m, ok := v.(map[string]any); ok {
		return m, true
	}
	// Try via JSON round-trip.
	b, err := json.Marshal(v)
	if err != nil {
		return nil, false
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, false
	}
	return m, true
}
