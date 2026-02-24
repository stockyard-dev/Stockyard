package features

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"
)

// AlertConfig defines alerting settings.
type AlertConfig struct {
	WebhookURL string
	Thresholds []float64 // Percentage of cap (e.g., 0.5, 0.8, 1.0)
}

// AlertPayload is the JSON sent to webhooks.
type AlertPayload struct {
	Event     string  `json:"event"`
	Threshold float64 `json:"threshold"`
	Project   string  `json:"project"`
	Spent     float64 `json:"spent"`
	Cap       float64 `json:"cap"`
	Timestamp string  `json:"timestamp"`
}

// Alerter manages webhook notifications with debouncing.
type Alerter struct {
	mu           sync.Mutex
	config       AlertConfig
	lastAlerted  map[string]time.Time // key: "project:threshold" → last alert time
	debounceTime time.Duration
	client       *http.Client
}

// NewAlerter creates a new alerter.
func NewAlerter(cfg AlertConfig) *Alerter {
	return &Alerter{
		config:       cfg,
		lastAlerted:  make(map[string]time.Time),
		debounceTime: 1 * time.Hour,
		client:       &http.Client{Timeout: 10 * time.Second},
	}
}

// CheckAndAlert checks if any thresholds are breached and sends alerts.
func (a *Alerter) CheckAndAlert(project string, spent, cap float64) {
	if a.config.WebhookURL == "" || cap == 0 {
		return
	}

	ratio := spent / cap
	for _, threshold := range a.config.Thresholds {
		if ratio >= threshold {
			a.sendAlert(project, threshold, spent, cap)
		}
	}
}

// sendAlert fires a webhook if not recently alerted for this threshold.
func (a *Alerter) sendAlert(project string, threshold, spent, cap float64) {
	a.mu.Lock()
	defer a.mu.Unlock()

	key := project + ":" + formatFloat(threshold)
	if last, ok := a.lastAlerted[key]; ok {
		if time.Since(last) < a.debounceTime {
			return // Already alerted recently
		}
	}

	payload := AlertPayload{
		Event:     "threshold_reached",
		Threshold: threshold,
		Project:   project,
		Spent:     spent,
		Cap:       cap,
		Timestamp: time.Now().Format(time.RFC3339),
	}

	body, _ := json.Marshal(payload)
	resp, err := a.client.Post(a.config.WebhookURL, "application/json", bytes.NewReader(body))
	if err != nil {
		log.Printf("alert webhook failed: %v", err)
		return
	}
	resp.Body.Close()

	a.lastAlerted[key] = time.Now()
	log.Printf("alert sent: project=%s threshold=%.0f%% spent=$%.2f cap=$%.2f",
		project, threshold*100, spent, cap)
}

func formatFloat(f float64) string {
	return fmt.Sprintf("%.2f", f)
}
