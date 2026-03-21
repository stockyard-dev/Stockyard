// Package main implements a Terraform provider for Stockyard.
//
// Usage in Terraform:
//
//	terraform {
//	  required_providers {
//	    stockyard = {
//	      source = "stockyard-dev/stockyard"
//	    }
//	  }
//	}
//
//	provider "stockyard" {
//	  base_url  = "http://localhost:4200"
//	  admin_key = var.stockyard_admin_key
//	}
//
//	resource "stockyard_module" "costcap" {
//	  name    = "costcap"
//	  enabled = true
//	}
//
//	resource "stockyard_webhook" "slack" {
//	  url    = "https://hooks.slack.com/services/..."
//	  secret = var.webhook_secret
//	  events = "alert.fired,cost.threshold"
//	}
//
//	resource "stockyard_trust_policy" "block_pii" {
//	  name    = "block-pii"
//	  type    = "content"
//	  action  = "block"
//	  pattern = "\\b\\d{3}-\\d{2}-\\d{4}\\b"
//	}
//
//	data "stockyard_status" "current" {}
//
//	output "uptime" {
//	  value = data.stockyard_status.current.uptime
//	}
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client wraps HTTP calls to the Stockyard API.
type Client struct {
	BaseURL   string
	AdminKey  string
	HTTP      *http.Client
}

func NewClient(baseURL, adminKey string) *Client {
	return &Client{
		BaseURL:  baseURL,
		AdminKey: adminKey,
		HTTP:     &http.Client{Timeout: 10 * time.Second},
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
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request to %s: %w", path, err)
	}
	defer resp.Body.Close()

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return result, fmt.Errorf("API error %d: %v", resp.StatusCode, result["error"])
	}
	return result, nil
}

// GetModule returns the state of a middleware module.
func (c *Client) GetModule(name string) (map[string]any, error) {
	modules, err := c.do("GET", "/api/proxy/modules", nil)
	if err != nil {
		return nil, err
	}
	for _, m := range modules["modules"].([]any) {
		mod := m.(map[string]any)
		if mod["name"] == name {
			return mod, nil
		}
	}
	return nil, fmt.Errorf("module %q not found", name)
}

// SetModule enables or disables a module.
func (c *Client) SetModule(name string, enabled bool) error {
	_, err := c.do("PUT", "/api/proxy/modules/"+name, map[string]bool{"enabled": enabled})
	return err
}

// GetStatus returns the system status.
func (c *Client) GetStatus() (map[string]any, error) {
	return c.do("GET", "/api/status", nil)
}

// CreateWebhook registers a webhook endpoint.
func (c *Client) CreateWebhook(url, secret, events string) (map[string]any, error) {
	return c.do("POST", "/api/webhooks", map[string]string{
		"url": url, "secret": secret, "events": events,
	})
}

// DeleteWebhook removes a webhook.
func (c *Client) DeleteWebhook(id string) error {
	_, err := c.do("DELETE", "/api/webhooks/"+id, nil)
	return err
}

// ExportConfig downloads the full config snapshot.
func (c *Client) ExportConfig() (map[string]any, error) {
	return c.do("GET", "/api/config/export", nil)
}

// ImportConfig applies a config snapshot.
func (c *Client) ImportConfig(config map[string]any) (map[string]any, error) {
	return c.do("POST", "/api/config/import", config)
}

// --- Provider resources ---

// GetProvider returns a configured LLM provider.
func (c *Client) GetProvider(name string) (map[string]any, error) {
	result, err := c.do("GET", "/api/proxy/providers", nil)
	if err != nil {
		return nil, err
	}
	for _, p := range result["providers"].([]any) {
		prov := p.(map[string]any)
		if prov["name"] == name {
			return prov, nil
		}
	}
	return nil, fmt.Errorf("provider %q not found", name)
}

// --- Routing rule resources ---

// CreateRoutingRule creates a smart routing rule.
func (c *Client) CreateRoutingRule(name string, priority int, condition, action any) (map[string]any, error) {
	return c.do("POST", "/api/proxy/routing/rules", map[string]any{
		"name": name, "priority": priority, "condition": condition, "action": action,
	})
}

// GetRoutingRules lists all routing rules.
func (c *Client) GetRoutingRules() ([]any, error) {
	result, err := c.do("GET", "/api/proxy/routing/rules", nil)
	if err != nil {
		return nil, err
	}
	rules, _ := result["rules"].([]any)
	return rules, nil
}

// UpdateRoutingRule updates a routing rule.
func (c *Client) UpdateRoutingRule(id string, updates map[string]any) error {
	_, err := c.do("PUT", "/api/proxy/routing/rules/"+id, updates)
	return err
}

// DeleteRoutingRule deletes a routing rule.
func (c *Client) DeleteRoutingRule(id string) error {
	_, err := c.do("DELETE", "/api/proxy/routing/rules/"+id, nil)
	return err
}

// --- Alert resources ---

// CreateAlert creates an observe alert rule.
func (c *Client) CreateAlert(name, metric, condition string, threshold float64) (map[string]any, error) {
	return c.do("POST", "/api/observe/alerts", map[string]any{
		"name": name, "metric": metric, "condition": condition, "threshold": threshold,
	})
}

// GetAlerts lists all alert rules.
func (c *Client) GetAlerts() ([]any, error) {
	result, err := c.do("GET", "/api/observe/alerts", nil)
	if err != nil {
		return nil, err
	}
	alerts, _ := result["alerts"].([]any)
	return alerts, nil
}

// DeleteAlert deletes an alert rule.
func (c *Client) DeleteAlert(id string) error {
	_, err := c.do("DELETE", "/api/observe/alerts/"+id, nil)
	return err
}

// --- Team member resources ---

// InviteTeamMember sends a team invite.
func (c *Client) InviteTeamMember(email, name, role string) (map[string]any, error) {
	return c.do("POST", "/api/team/members", map[string]string{
		"email": email, "name": name, "role": role,
	})
}

// GetTeamMembers lists all team members.
func (c *Client) GetTeamMembers() ([]any, error) {
	result, err := c.do("GET", "/api/team/members", nil)
	if err != nil {
		return nil, err
	}
	// The team endpoint returns an array directly.
	return nil, fmt.Errorf("parse members: %v", result)
}

// UpdateTeamMember updates a member's role.
func (c *Client) UpdateTeamMember(id, role string) error {
	_, err := c.do("PUT", "/api/team/members/"+id, map[string]string{"role": role})
	return err
}

// DeleteTeamMember removes a team member.
func (c *Client) DeleteTeamMember(id string) error {
	_, err := c.do("DELETE", "/api/team/members/"+id, nil)
	return err
}

// --- Data sources ---

// ListModules returns all proxy modules.
func (c *Client) ListModules() ([]any, error) {
	result, err := c.do("GET", "/api/proxy/modules", nil)
	if err != nil {
		return nil, err
	}
	modules, _ := result["modules"].([]any)
	return modules, nil
}

// ListProviders returns all configured providers.
func (c *Client) ListProviders() ([]any, error) {
	result, err := c.do("GET", "/api/proxy/providers", nil)
	if err != nil {
		return nil, err
	}
	providers, _ := result["providers"].([]any)
	return providers, nil
}

// ListTraces returns recent traces.
func (c *Client) ListTraces(limit int) ([]any, error) {
	result, err := c.do("GET", fmt.Sprintf("/api/observe/traces?limit=%d", limit), nil)
	if err != nil {
		return nil, err
	}
	traces, _ := result["traces"].([]any)
	return traces, nil
}

func main() {
	// This would normally call terraform-plugin-sdk/v2.
	// Stub for now — full implementation requires the Terraform SDK.
	fmt.Println("stockyard terraform provider v1.0")
	fmt.Println()
	fmt.Println("Resources:")
	fmt.Println("  stockyard_module        — manage proxy middleware modules")
	fmt.Println("  stockyard_provider      — manage LLM provider configs")
	fmt.Println("  stockyard_routing_rule  — manage smart routing rules")
	fmt.Println("  stockyard_alert         — manage observe alert rules")
	fmt.Println("  stockyard_team_member   — manage team member invites")
	fmt.Println()
	fmt.Println("Data sources:")
	fmt.Println("  stockyard_modules       — list all modules")
	fmt.Println("  stockyard_providers     — list all providers")
	fmt.Println("  stockyard_traces        — query recent traces")
	fmt.Println()
	fmt.Println("See https://stockyard.dev/docs/ for configuration reference.")
}
