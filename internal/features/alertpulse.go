package features

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/stockyard-dev/stockyard/internal/config"
	"github.com/stockyard-dev/stockyard/internal/provider"
	"github.com/stockyard-dev/stockyard/internal/proxy"
)

// AlertPulseEvent records a triggered alert for the dashboard.
type AlertPulseEvent struct {
	Timestamp time.Time `json:"timestamp"`
	RuleName  string    `json:"rule_name"`
	Metric    string    `json:"metric"`
	Value     float64   `json:"value"`
	Threshold float64   `json:"threshold"`
	Channel   string    `json:"channel"`
	Status    string    `json:"status"`
	Model     string    `json:"model"`
}

type apTimeBool struct {
	t time.Time
	v bool
}

type apTimeFloat struct {
	t time.Time
	v float64
}

// AlertPulseState holds runtime state for the alerting engine.
type AlertPulseState struct {
	mu           sync.Mutex
	cfg          config.AlertPulseConfig
	recentAlerts []AlertPulseEvent
	client       *http.Client

	errorWindow   []apTimeBool
	latencyWindow []apTimeFloat
	costWindow    []apTimeFloat
	activeAlerts  map[string]time.Time

	requestsMonitored atomic.Int64
	alertsFired       atomic.Int64
	alertsResolved    atomic.Int64
	alertErrors       atomic.Int64
	webhooksSent      atomic.Int64
}

// NewAlertPulse creates a new alerting engine from config.
func NewAlertPulse(cfg config.AlertPulseConfig) *AlertPulseState {
	return &AlertPulseState{
		cfg:          cfg,
		recentAlerts: make([]AlertPulseEvent, 0, 64),
		client:       &http.Client{Timeout: 5 * time.Second},
		activeAlerts: make(map[string]time.Time),
	}
}

// Stats returns current metrics for the SSE dashboard.
func (a *AlertPulseState) Stats() map[string]any {
	a.mu.Lock()
	recent := make([]AlertPulseEvent, len(a.recentAlerts))
	copy(recent, a.recentAlerts)
	activeCount := len(a.activeAlerts)
	a.mu.Unlock()

	return map[string]any{
		"requests_monitored": a.requestsMonitored.Load(),
		"alerts_fired":       a.alertsFired.Load(),
		"alerts_resolved":    a.alertsResolved.Load(),
		"alert_errors":       a.alertErrors.Load(),
		"webhooks_sent":      a.webhooksSent.Load(),
		"active_alerts":      activeCount,
		"recent_alerts":      recent,
	}
}

// AlertPulseMiddleware returns middleware that monitors metrics and fires alerts.
func AlertPulseMiddleware(state *AlertPulseState) proxy.Middleware {
	return func(next proxy.Handler) proxy.Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
			state.requestsMonitored.Add(1)
			start := time.Now()

			resp, err := next(ctx, req)

			latency := time.Since(start)
			now := time.Now()

			state.mu.Lock()
			defer state.mu.Unlock()

			isError := err != nil
			state.errorWindow = append(state.errorWindow, apTimeBool{t: now, v: isError})
			state.latencyWindow = append(state.latencyWindow, apTimeFloat{t: now, v: float64(latency.Milliseconds())})

			if resp != nil && resp.Usage.TotalTokens > 0 {
				cost := float64(resp.Usage.TotalTokens) * 0.00001
				state.costWindow = append(state.costWindow, apTimeFloat{t: now, v: cost})
			}

			windowDur := state.cfg.WindowDuration.Duration
			if windowDur == 0 {
				windowDur = 5 * time.Minute
			}
			cutoff := now.Add(-windowDur)
			state.errorWindow = apPruneBool(state.errorWindow, cutoff)
			state.latencyWindow = apPruneFloat(state.latencyWindow, cutoff)
			state.costWindow = apPruneFloat(state.costWindow, cutoff)

			for _, rule := range state.cfg.Rules {
				triggered := false
				var actualValue float64

				switch rule.Metric {
				case "error_rate":
					if len(state.errorWindow) >= 10 {
						errors := 0
						for _, e := range state.errorWindow {
							if e.v {
								errors++
							}
						}
						actualValue = float64(errors) / float64(len(state.errorWindow)) * 100
						triggered = actualValue >= rule.Threshold
					}
				case "latency_p95":
					if len(state.latencyWindow) >= 10 {
						actualValue = apPercentile(state.latencyWindow, 95)
						triggered = actualValue >= rule.Threshold
					}
				case "latency_p50":
					if len(state.latencyWindow) >= 10 {
						actualValue = apPercentile(state.latencyWindow, 50)
						triggered = actualValue >= rule.Threshold
					}
				case "cost_per_min":
					var total float64
					for _, c := range state.costWindow {
						total += c.v
					}
					if mins := windowDur.Minutes(); mins > 0 {
						actualValue = total / mins
					}
					triggered = actualValue >= rule.Threshold
				}

				if triggered {
					cooldown := state.cfg.Cooldown.Duration
					if cooldown == 0 {
						cooldown = 5 * time.Minute
					}
					if last, ok := state.activeAlerts[rule.Name]; ok && now.Sub(last) < cooldown {
						continue
					}
					state.activeAlerts[rule.Name] = now
					state.alertsFired.Add(1)

					evt := AlertPulseEvent{
						Timestamp: now, RuleName: rule.Name, Metric: rule.Metric,
						Value: actualValue, Threshold: rule.Threshold,
						Channel: rule.Channel, Status: "fired", Model: req.Model,
					}
					state.apAddEvent(evt)
					go state.apFireWebhook(rule, evt)

				} else if _, wasActive := state.activeAlerts[rule.Name]; wasActive {
					delete(state.activeAlerts, rule.Name)
					state.alertsResolved.Add(1)
					state.apAddEvent(AlertPulseEvent{
						Timestamp: now, RuleName: rule.Name, Metric: rule.Metric,
						Value: actualValue, Threshold: rule.Threshold, Status: "resolved", Model: req.Model,
					})
				}
			}

			return resp, err
		}
	}
}

func (a *AlertPulseState) apAddEvent(evt AlertPulseEvent) {
	a.recentAlerts = append(a.recentAlerts, evt)
	if len(a.recentAlerts) > 64 {
		a.recentAlerts = a.recentAlerts[len(a.recentAlerts)-64:]
	}
}

func (a *AlertPulseState) apFireWebhook(rule config.AlertRule, evt AlertPulseEvent) {
	url := rule.WebhookURL
	if url == "" {
		url = a.cfg.DefaultWebhook
	}
	if url == "" {
		return
	}
	payload, _ := json.Marshal(map[string]any{
		"event": "alert_" + evt.Status, "rule": evt.RuleName,
		"metric": evt.Metric, "value": evt.Value, "threshold": evt.Threshold,
		"model": evt.Model, "timestamp": evt.Timestamp.Format(time.RFC3339),
	})
	r, err := http.NewRequest("POST", url, bytes.NewReader(payload))
	if err != nil {
		a.alertErrors.Add(1)
		return
	}
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("User-Agent", "Stockyard-AlertPulse/1.0")
	resp, err := a.client.Do(r)
	if err != nil {
		a.alertErrors.Add(1)
		log.Printf("alertpulse: webhook failed for %s: %v", rule.Name, err)
		return
	}
	resp.Body.Close()
	a.webhooksSent.Add(1)
	log.Printf("alertpulse: %s alert %s (%.2f >= %.2f) → %d",
		evt.Status, rule.Name, evt.Value, rule.Threshold, resp.StatusCode)
}

func apPruneBool(s []apTimeBool, cutoff time.Time) []apTimeBool {
	i := 0
	for _, v := range s {
		if v.t.After(cutoff) {
			s[i] = v
			i++
		}
	}
	return s[:i]
}

func apPruneFloat(s []apTimeFloat, cutoff time.Time) []apTimeFloat {
	i := 0
	for _, v := range s {
		if v.t.After(cutoff) {
			s[i] = v
			i++
		}
	}
	return s[:i]
}

func apPercentile(data []apTimeFloat, pct int) float64 {
	if len(data) == 0 {
		return 0
	}
	vals := make([]float64, len(data))
	for i, d := range data {
		vals[i] = d.v
	}
	for i := 1; i < len(vals); i++ {
		for j := i; j > 0 && vals[j] < vals[j-1]; j-- {
			vals[j], vals[j-1] = vals[j-1], vals[j]
		}
	}
	idx := len(vals) * pct / 100
	if idx >= len(vals) {
		idx = len(vals) - 1
	}
	return vals[idx]
}
