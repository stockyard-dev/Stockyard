package features

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/stockyard-dev/stockyard/internal/config"
	"github.com/stockyard-dev/stockyard/internal/provider"
	"github.com/stockyard-dev/stockyard/internal/proxy"
)

// IPFenceEvent records a block/allow action for the dashboard.
type IPFenceEvent struct {
	Timestamp time.Time `json:"timestamp"`
	IP        string    `json:"ip"`
	Action    string    `json:"action"` // blocked, allowed, warned
	Reason    string    `json:"reason"` // not_in_allowlist, in_denylist, etc.
	Model     string    `json:"model"`
	Project   string    `json:"project"`
}

// IPFenceState holds runtime state for IP access control.
type IPFenceState struct {
	mu           sync.Mutex
	cfg          config.IPFenceConfig
	allowNets    []*net.IPNet
	allowIPs     []net.IP
	denyNets     []*net.IPNet
	denyIPs      []net.IP
	recentEvents []IPFenceEvent
	client       *http.Client

	requestsChecked atomic.Int64
	requestsBlocked atomic.Int64
	requestsAllowed atomic.Int64
	requestsWarned  atomic.Int64
	uniqueIPs       sync.Map // IP string → struct{}
	uniqueIPCount   atomic.Int64
}

// NewIPFence creates a new IP fence from config.
func NewIPFence(cfg config.IPFenceConfig) *IPFenceState {
	ipf := &IPFenceState{
		cfg:          cfg,
		recentEvents: make([]IPFenceEvent, 0, 200),
		client:       &http.Client{Timeout: 10 * time.Second},
	}

	// Parse allowlist
	for _, entry := range cfg.Allowlist {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		if ipNet, ip := parseCIDROrIP(entry); ipNet != nil {
			ipf.allowNets = append(ipf.allowNets, ipNet)
		} else if ip != nil {
			ipf.allowIPs = append(ipf.allowIPs, ip)
		} else {
			log.Printf("ipfence: invalid allowlist entry %q (skipped)", entry)
		}
	}

	// Parse denylist
	for _, entry := range cfg.Denylist {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		if ipNet, ip := parseCIDROrIP(entry); ipNet != nil {
			ipf.denyNets = append(ipf.denyNets, ipNet)
		} else if ip != nil {
			ipf.denyIPs = append(ipf.denyIPs, ip)
		} else {
			log.Printf("ipfence: invalid denylist entry %q (skipped)", entry)
		}
	}

	return ipf
}

// parseCIDROrIP parses a string as either a CIDR range or a single IP.
// Returns (net, nil) for CIDR, (nil, ip) for plain IP, (nil, nil) on error.
func parseCIDROrIP(s string) (*net.IPNet, net.IP) {
	if strings.Contains(s, "/") {
		_, ipNet, err := net.ParseCIDR(s)
		if err != nil {
			return nil, nil
		}
		return ipNet, nil
	}
	ip := net.ParseIP(s)
	if ip != nil {
		return nil, ip
	}
	return nil, nil
}

// CheckIP determines whether an IP should be allowed or denied.
// Returns (allowed bool, reason string).
func (ipf *IPFenceState) CheckIP(ipStr string) (bool, string) {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false, "invalid_ip"
	}

	switch ipf.cfg.Mode {
	case "allowlist":
		// Only allow if in allowlist
		if ipf.matchesAny(ip, ipf.allowNets, ipf.allowIPs) {
			return true, "in_allowlist"
		}
		return false, "not_in_allowlist"

	case "denylist":
		// Block if in denylist, allow otherwise
		if ipf.matchesAny(ip, ipf.denyNets, ipf.denyIPs) {
			return false, "in_denylist"
		}
		return true, "not_in_denylist"

	case "mixed":
		// Check denylist first (deny wins over allow)
		if ipf.matchesAny(ip, ipf.denyNets, ipf.denyIPs) {
			return false, "in_denylist"
		}
		// If allowlist has entries, must be in it
		if len(ipf.allowNets) > 0 || len(ipf.allowIPs) > 0 {
			if ipf.matchesAny(ip, ipf.allowNets, ipf.allowIPs) {
				return true, "in_allowlist"
			}
			return false, "not_in_allowlist"
		}
		return true, "default_allow"

	default:
		// Default to denylist mode
		if ipf.matchesAny(ip, ipf.denyNets, ipf.denyIPs) {
			return false, "in_denylist"
		}
		return true, "default_allow"
	}
}

// matchesAny checks if an IP matches any network or individual IP in the lists.
func (ipf *IPFenceState) matchesAny(ip net.IP, nets []*net.IPNet, ips []net.IP) bool {
	for _, n := range nets {
		if n.Contains(ip) {
			return true
		}
	}
	for _, allowIP := range ips {
		if ip.Equal(allowIP) {
			return true
		}
	}
	return false
}

// ExtractIP extracts the client IP from request context.
// If TrustProxy is enabled, checks X-Forwarded-For and X-Real-IP.
func (ipf *IPFenceState) ExtractIP(req *provider.Request) string {
	// Provider requests don't directly carry HTTP headers,
	// so we use the UserID field or Project field as a proxy for IP.
	// In the actual HTTP handler, the IP is extracted and injected into
	// the request's metadata. We look for it in the provider.Request.
	if req.ClientIP != "" {
		return req.ClientIP
	}
	// Fallback: try UserID as IP (some configurations use IP as user ID)
	if req.UserID != "" && net.ParseIP(req.UserID) != nil {
		return req.UserID
	}
	return "127.0.0.1" // localhost fallback
}

// Stats returns fence statistics for the dashboard.
func (ipf *IPFenceState) Stats() map[string]any {
	ipf.mu.Lock()
	events := make([]IPFenceEvent, len(ipf.recentEvents))
	copy(events, ipf.recentEvents)
	ipf.mu.Unlock()

	return map[string]any{
		"requests_checked": ipf.requestsChecked.Load(),
		"requests_blocked": ipf.requestsBlocked.Load(),
		"requests_allowed": ipf.requestsAllowed.Load(),
		"requests_warned":  ipf.requestsWarned.Load(),
		"unique_ips":       ipf.uniqueIPCount.Load(),
		"allowlist_size":   len(ipf.allowNets) + len(ipf.allowIPs),
		"denylist_size":    len(ipf.denyNets) + len(ipf.denyIPs),
		"mode":             ipf.cfg.Mode,
		"recent_events":    events,
	}
}

// recordEvent adds a fence event to the ring buffer.
func (ipf *IPFenceState) recordEvent(ev IPFenceEvent) {
	ipf.mu.Lock()
	defer ipf.mu.Unlock()
	if len(ipf.recentEvents) >= 200 {
		ipf.recentEvents = ipf.recentEvents[1:]
	}
	ipf.recentEvents = append(ipf.recentEvents, ev)
}

// trackUniqueIP records a unique IP.
func (ipf *IPFenceState) trackUniqueIP(ip string) {
	if _, loaded := ipf.uniqueIPs.LoadOrStore(ip, struct{}{}); !loaded {
		ipf.uniqueIPCount.Add(1)
	}
}

// sendWebhook fires an alert webhook for blocked requests.
func (ipf *IPFenceState) sendWebhook(ev IPFenceEvent) {
	if ipf.cfg.Webhook == "" {
		return
	}
	go func() {
		payload := map[string]any{
			"event":     "ip_blocked",
			"ip":        ev.IP,
			"reason":    ev.Reason,
			"model":     ev.Model,
			"project":   ev.Project,
			"timestamp": ev.Timestamp.Format(time.RFC3339),
		}
		body, _ := json.Marshal(payload)
		resp, err := ipf.client.Post(ipf.cfg.Webhook, "application/json", bytes.NewReader(body))
		if err != nil {
			log.Printf("ipfence: webhook failed: %v", err)
			return
		}
		resp.Body.Close()
	}()
}

// IPFenceMiddleware returns middleware that enforces IP access control.
func IPFenceMiddleware(ipf *IPFenceState) proxy.Middleware {
	return func(next proxy.Handler) proxy.Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
			ipf.requestsChecked.Add(1)

			clientIP := ipf.ExtractIP(req)
			ipf.trackUniqueIP(clientIP)

			allowed, reason := ipf.CheckIP(clientIP)

			if allowed {
				ipf.requestsAllowed.Add(1)
				return next(ctx, req)
			}

			// IP is not allowed
			ev := IPFenceEvent{
				Timestamp: time.Now(),
				IP:        clientIP,
				Reason:    reason,
				Model:     req.Model,
				Project:   req.Project,
			}

			switch ipf.cfg.Action {
			case "log":
				// Log-only mode: record but allow through
				ev.Action = "logged"
				ipf.recordEvent(ev)
				if ipf.cfg.LogBlocked {
					log.Printf("ipfence: [LOG] ip=%s reason=%s model=%s project=%s",
						clientIP, reason, req.Model, req.Project)
				}
				return next(ctx, req)

			case "warn":
				// Warn mode: allow through but record and log
				ev.Action = "warned"
				ipf.requestsWarned.Add(1)
				ipf.recordEvent(ev)
				if ipf.cfg.LogBlocked {
					log.Printf("ipfence: [WARN] ip=%s reason=%s model=%s project=%s",
						clientIP, reason, req.Model, req.Project)
				}
				return next(ctx, req)

			default: // "block"
				ev.Action = "blocked"
				ipf.requestsBlocked.Add(1)
				ipf.recordEvent(ev)
				ipf.sendWebhook(ev)
				if ipf.cfg.LogBlocked {
					log.Printf("ipfence: [BLOCK] ip=%s reason=%s model=%s project=%s",
						clientIP, reason, req.Model, req.Project)
				}
				return nil, fmt.Errorf("ipfence: access denied for IP %s (%s)", clientIP, reason)
			}
		}
	}
}
