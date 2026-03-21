// Package stockyard provides a Go client for the Stockyard LLM proxy API.
//
// Usage:
//
//	client := stockyard.NewClient("http://localhost:4200", "your-admin-key")
//	status, _ := client.Status()
//	modules, _ := client.ListModules()
package stockyard

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// Client wraps HTTP calls to the Stockyard API.
type Client struct {
	BaseURL  string
	AdminKey string
	HTTP     *http.Client
}

// NewClient creates a Stockyard API client.
func NewClient(baseURL, adminKey string) *Client {
	if baseURL == "" {
		baseURL = os.Getenv("STOCKYARD_URL")
		if baseURL == "" {
			baseURL = "http://localhost:4200"
		}
	}
	if adminKey == "" {
		adminKey = os.Getenv("STOCKYARD_ADMIN_KEY")
	}
	return &Client{
		BaseURL:  baseURL,
		AdminKey: adminKey,
		HTTP:     &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *Client) do(method, path string, body any) (map[string]any, error) {
	var reader io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		reader = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, c.BaseURL+path, reader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.AdminKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.AdminKey)
		req.Header.Set("X-Admin-Key", c.AdminKey)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request to %s: %w", path, err)
	}
	defer resp.Body.Close()
	var result map[string]any
	json.NewDecoder(resp.Body).Decode(&result)
	if resp.StatusCode >= 400 {
		return result, fmt.Errorf("API error %d: %v", resp.StatusCode, result["error"])
	}
	return result, nil
}

// --- Status ---

// Status returns system status.
func (c *Client) Status() (map[string]any, error) { return c.do("GET", "/api/status", nil) }

// Apps returns the list of running apps.
func (c *Client) Apps() ([]any, error) {
	r, err := c.do("GET", "/api/apps", nil)
	if err != nil {
		return nil, err
	}
	apps, _ := r["apps"].([]any)
	return apps, nil
}

// --- Proxy ---

// ListModules returns all proxy modules.
func (c *Client) ListModules() ([]any, error) {
	r, err := c.do("GET", "/api/proxy/modules", nil)
	if err != nil {
		return nil, err
	}
	return r["modules"].([]any), nil
}

// ToggleModule enables or disables a module.
func (c *Client) ToggleModule(name string, enabled bool) error {
	_, err := c.do("PUT", "/api/proxy/modules/"+name, map[string]any{"enabled": enabled})
	return err
}

// ListProviders returns configured LLM providers.
func (c *Client) ListProviders() ([]any, error) {
	r, err := c.do("GET", "/api/proxy/providers", nil)
	if err != nil {
		return nil, err
	}
	return r["providers"].([]any), nil
}

// ProviderHealth checks all provider health.
func (c *Client) ProviderHealth() ([]any, error) {
	r, err := c.do("GET", "/api/proxy/providers/health", nil)
	if err != nil {
		return nil, err
	}
	return r["providers"].([]any), nil
}

// --- Observe ---

// ListTraces returns recent traces.
func (c *Client) ListTraces(limit int) ([]any, error) {
	r, err := c.do("GET", fmt.Sprintf("/api/observe/traces?limit=%d", limit), nil)
	if err != nil {
		return nil, err
	}
	return r["traces"].([]any), nil
}

// GetCosts returns cost breakdown.
func (c *Client) GetCosts(period string) (map[string]any, error) {
	return c.do("GET", "/api/observe/costs?period="+period, nil)
}

// GetDrift returns model drift detection.
func (c *Client) GetDrift() (map[string]any, error) {
	return c.do("GET", "/api/observe/drift", nil)
}

// --- Billing ---

// CreateCustomer creates a billing customer.
func (c *Client) CreateCustomer(name, email string) (map[string]any, error) {
	return c.do("POST", "/api/billing/customers", map[string]any{
		"name": name, "email": email, "account_id": "default",
	})
}

// QueryUsage returns usage summary.
func (c *Client) QueryUsage() (map[string]any, error) {
	return c.do("GET", "/api/billing/usage/summary", nil)
}

// CreditBalance returns credit balance for a customer.
func (c *Client) CreditBalance(customerID string) (map[string]any, error) {
	return c.do("GET", "/api/billing/credits/balance?customer_id="+customerID, nil)
}

// --- Team ---

// ListTeamMembers returns all team members.
func (c *Client) ListTeamMembers() (map[string]any, error) {
	return c.do("GET", "/api/team/members", nil)
}

// InviteMember sends a team invite.
func (c *Client) InviteMember(email, name, role string) (map[string]any, error) {
	return c.do("POST", "/api/team/members", map[string]string{
		"email": email, "name": name, "role": role,
	})
}

// --- Config ---

// ExportConfig exports the full configuration.
func (c *Client) ExportConfig() (map[string]any, error) {
	return c.do("GET", "/api/config/export", nil)
}

// ImportConfig imports a configuration.
func (c *Client) ImportConfig(config map[string]any) (map[string]any, error) {
	return c.do("POST", "/api/config/import", config)
}

// --- Routing ---

// ListRoutingRules returns smart routing rules.
func (c *Client) ListRoutingRules() ([]any, error) {
	r, err := c.do("GET", "/api/proxy/routing/rules", nil)
	if err != nil {
		return nil, err
	}
	return r["rules"].([]any), nil
}

// CreateRoutingRule creates a routing rule.
func (c *Client) CreateRoutingRule(name string, condition, action any, priority int) (map[string]any, error) {
	return c.do("POST", "/api/proxy/routing/rules", map[string]any{
		"name": name, "condition": condition, "action": action, "priority": priority,
	})
}

// --- Firewall ---

// ScanText scans text with the firewall.
func (c *Client) ScanText(text string) (map[string]any, error) {
	return c.do("POST", "/api/firewall/scan", map[string]string{"text": text})
}

// FirewallStats returns firewall detection stats.
func (c *Client) FirewallStats() (map[string]any, error) {
	return c.do("GET", "/api/firewall/stats", nil)
}

// --- Chat ---

// Chat sends a chat completion through the proxy.
func (c *Client) Chat(model string, messages []map[string]string) (map[string]any, error) {
	return c.do("POST", "/v1/chat/completions", map[string]any{
		"model": model, "messages": messages,
	})
}
