package billing

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"sync"
	"time"
)

// MeterReporter sends usage events to Stripe Billing Meters.
// This replaces the prepaid credit wallet for cloud customers —
// Stripe bills usage at end of billing cycle instead of us holding money.
//
// Two meters:
//   - llm_usage: LLM token costs in cents (reported per request)
//   - marketplace_fee: App Builder + Knowledge query fees in cents
//
// For self-hosted (no Stripe), falls back to local credit deduction.
type MeterReporter struct {
	mu          sync.Mutex
	queue       []meterEvent
	flushTicker *time.Ticker
	stopCh      chan struct{}
}

type meterEvent struct {
	EventName  string // "llm_usage" or "marketplace_fee"
	CustomerID string // Stripe customer ID (cus_...)
	Value      int64  // amount in cents
	Timestamp  int64  // unix seconds
}

// Global meter reporter — initialized on boot if Stripe is configured.
var GlobalMeter *MeterReporter

// StartMeterReporter creates and starts the background meter event flusher.
// Events are batched and sent every 30 seconds to avoid per-request API calls.
func StartMeterReporter() *MeterReporter {
	if !stripeEnabled() {
		log.Printf("[metering] Stripe not configured — using local credit deduction")
		return nil
	}

	// Check meter IDs are configured
	if os.Getenv("STRIPE_METER_LLM_USAGE") == "" {
		log.Printf("[metering] STRIPE_METER_LLM_USAGE not set — metered billing disabled")
		return nil
	}

	m := &MeterReporter{
		queue:       make([]meterEvent, 0, 100),
		flushTicker: time.NewTicker(30 * time.Second),
		stopCh:      make(chan struct{}),
	}

	go m.flushLoop()
	log.Printf("[metering] Stripe Billing Meter reporter started (flush every 30s)")
	GlobalMeter = m
	return m
}

// ReportLLMUsage queues a usage event for LLM token costs.
func (m *MeterReporter) ReportLLMUsage(stripeCustomerID string, costCents int64) {
	if costCents <= 0 || stripeCustomerID == "" {
		return
	}
	m.mu.Lock()
	m.queue = append(m.queue, meterEvent{
		EventName:  "llm_usage",
		CustomerID: stripeCustomerID,
		Value:      costCents,
		Timestamp:  time.Now().Unix(),
	})
	m.mu.Unlock()
}

// ReportMarketplaceFee queues a usage event for app/knowledge fees.
func (m *MeterReporter) ReportMarketplaceFee(stripeCustomerID string, feeCents int64) {
	if feeCents <= 0 || stripeCustomerID == "" {
		return
	}
	m.mu.Lock()
	m.queue = append(m.queue, meterEvent{
		EventName:  "marketplace_fee",
		CustomerID: stripeCustomerID,
		Value:      feeCents,
		Timestamp:  time.Now().Unix(),
	})
	m.mu.Unlock()
}

func (m *MeterReporter) flushLoop() {
	for {
		select {
		case <-m.flushTicker.C:
			m.flush()
		case <-m.stopCh:
			m.flush() // final flush
			return
		}
	}
}

func (m *MeterReporter) flush() {
	m.mu.Lock()
	if len(m.queue) == 0 {
		m.mu.Unlock()
		return
	}
	batch := m.queue
	m.queue = make([]meterEvent, 0, 100)
	m.mu.Unlock()

	sent := 0
	failed := 0
	for _, evt := range batch {
		if err := sendMeterEvent(evt); err != nil {
			log.Printf("[metering] failed to send %s event: %v", evt.EventName, err)
			failed++
		} else {
			sent++
		}
	}
	if sent > 0 || failed > 0 {
		log.Printf("[metering] flushed %d events (%d failed)", sent, failed)
	}
}

// sendMeterEvent posts a single event to the Stripe Billing Meter API.
func sendMeterEvent(evt meterEvent) error {
	params := url.Values{
		"event_name":                    {evt.EventName},
		"timestamp":                     {fmt.Sprintf("%d", evt.Timestamp)},
		"payload[value]":                {fmt.Sprintf("%d", evt.Value)},
		"payload[stripe_customer_id]":   {evt.CustomerID},
	}

	_, err := stripeRequest("POST", "/billing/meter_events", params)
	return err
}

// Stop gracefully shuts down the meter reporter.
func (m *MeterReporter) Stop() {
	m.flushTicker.Stop()
	close(m.stopCh)
}

// registerMeteringRoutes adds metering status/debug endpoints.
func (a *App) registerMeteringRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/billing/metering/status", a.handleMeteringStatus)
	mux.HandleFunc("POST /api/billing/metering/flush", a.handleMeteringFlush)
}

func (a *App) handleMeteringStatus(w http.ResponseWriter, r *http.Request) {
	active := GlobalMeter != nil
	queueSize := 0
	if active {
		GlobalMeter.mu.Lock()
		queueSize = len(GlobalMeter.queue)
		GlobalMeter.mu.Unlock()
	}

	writeJSON(w, map[string]any{
		"active":                  active,
		"queue_size":              queueSize,
		"stripe_configured":       stripeEnabled(),
		"llm_meter":              os.Getenv("STRIPE_METER_LLM_USAGE") != "",
		"marketplace_meter":       os.Getenv("STRIPE_METER_MARKETPLACE_FEE") != "",
		"llm_price":              os.Getenv("STRIPE_PRICE_LLM_USAGE_METERED") != "",
		"marketplace_price":       os.Getenv("STRIPE_PRICE_MARKETPLACE_METERED") != "",
	})
}

func (a *App) handleMeteringFlush(w http.ResponseWriter, r *http.Request) {
	if GlobalMeter == nil {
		writeJSON(w, map[string]string{"status": "metering not active"})
		return
	}
	GlobalMeter.flush()
	writeJSON(w, map[string]string{"status": "flushed"})
}
