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

func main() {
	// This would normally call terraform-plugin-sdk/v2.
	// Stub for now — full implementation requires the Terraform SDK.
	fmt.Println("stockyard terraform provider — use with: terraform init")
	fmt.Println("Resources: stockyard_module, stockyard_webhook, stockyard_trust_policy")
	fmt.Println("Data sources: stockyard_status, stockyard_config")
	fmt.Println()
	fmt.Println("See https://stockyard.dev/docs/ for configuration reference.")
}
