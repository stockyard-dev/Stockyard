// Package alert provides quality gates and alerting for Verdikt.
// A Gate watches a specific dimension over a rolling window and fires
// an Alert when the average drops below its threshold.
package alert

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/stockyard-dev/stockyard/internal/verdikt/store"
)

// Gate defines a quality threshold to monitor.
type Gate struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	Dimension  string  `json:"dimension"`  // relevance, accuracy, helpfulness, safety, tone, score
	Threshold  float64 `json:"threshold"`  // fire if avg drops below this
	Window     int     `json:"window"`     // last N evaluations to consider
	WebhookURL string  `json:"webhook_url,omitempty"`
	Enabled    bool    `json:"enabled"`
}

// Alert records a gate violation.
type Alert struct {
	GateID    string    `json:"gate_id"`
	GateName  string    `json:"gate_name"`
	Dimension string    `json:"dimension"`
	Threshold float64   `json:"threshold"`
	Actual    float64   `json:"actual"`
	Timestamp time.Time `json:"timestamp"`
}

// Check evaluates all provided gates against recent evaluations and returns
// any alerts for gates whose dimension average falls below the threshold.
func Check(gates []Gate, evals []store.Evaluation) []Alert {
	var alerts []Alert
	for _, g := range gates {
		if !g.Enabled {
			continue
		}
		window := evals
		if g.Window > 0 && g.Window < len(window) {
			window = window[:g.Window] // evals are newest-first
		}
		if len(window) == 0 {
			continue
		}

		avg := dimensionAvg(window, g.Dimension)
		if avg < g.Threshold {
			a := Alert{
				GateID:    g.ID,
				GateName:  g.Name,
				Dimension: g.Dimension,
				Threshold: g.Threshold,
				Actual:    avg,
				Timestamp: time.Now(),
			}
			alerts = append(alerts, a)
			if g.WebhookURL != "" {
				go fireWebhook(g.WebhookURL, a)
			}
		}
	}
	return alerts
}

func dimensionAvg(evals []store.Evaluation, dim string) float64 {
	var sum float64
	for _, e := range evals {
		switch dim {
		case "relevance":
			sum += e.Relevance
		case "accuracy":
			sum += e.Accuracy
		case "helpfulness":
			sum += e.Helpfulness
		case "safety":
			sum += e.Safety
		case "tone":
			sum += e.Tone
		case "score":
			sum += e.Score
		}
	}
	return sum / float64(len(evals))
}

func fireWebhook(url string, a Alert) {
	body, marshalErr := json.Marshal(a)
	if marshalErr != nil {
		body = []byte("{}")
	}
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return
	}
	resp.Body.Close()
}

// FormatAlert returns a human-readable description of the alert.
func FormatAlert(a Alert) string {
	return fmt.Sprintf("[%s] %s: %s avg %.2f < threshold %.2f",
		a.Timestamp.Format("15:04:05"), a.GateName, a.Dimension, a.Actual, a.Threshold)
}
