// Package main implements a Pulumi provider for Stockyard.
//
// This is a simplified provider that wraps the Stockyard REST API
// for infrastructure-as-code management of modules, providers,
// routing rules, alerts, and team members.
package main

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

// NewClient creates a client from environment variables.
func NewClient() *Client {
	url := os.Getenv("STOCKYARD_URL")
	if url == "" {
		url = "http://localhost:7749"
	}
	return &Client{
		BaseURL:  url,
		AdminKey: os.Getenv("STOCKYARD_ADMIN_KEY"),
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

// --- Resource CRUD operations ---

// Module manages a proxy middleware module.
type Module struct {
	Name       string `json:"name"`
	Enabled    bool   `json:"enabled"`
	ConfigJSON string `json:"config_json,omitempty"`
}

func (c *Client) CreateModule(m Module) error {
	_, err := c.do("PUT", "/api/proxy/modules/"+m.Name, map[string]any{
		"enabled": m.Enabled,
	})
	return err
}

func (c *Client) ReadModule(name string) (*Module, error) {
	result, err := c.do("GET", "/api/proxy/modules/"+name, nil)
	if err != nil {
		return nil, err
	}
	enabled, _ := result["enabled"].(bool)
	return &Module{Name: name, Enabled: enabled}, nil
}

func (c *Client) UpdateModule(m Module) error {
	return c.CreateModule(m)
}

func (c *Client) DeleteModule(name string) error {
	_, err := c.do("PUT", "/api/proxy/modules/"+name, map[string]any{"enabled": false})
	return err
}

// RoutingRule manages a smart routing rule.
type RoutingRule struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Priority  int    `json:"priority"`
	Condition any    `json:"condition"`
	Action    any    `json:"action"`
}

func (c *Client) CreateRoutingRule(r RoutingRule) (string, error) {
	result, err := c.do("POST", "/api/proxy/routing/rules", map[string]any{
		"name": r.Name, "priority": r.Priority, "condition": r.Condition, "action": r.Action,
	})
	if err != nil {
		return "", err
	}
	id, _ := result["id"].(string)
	return id, nil
}

func (c *Client) ReadRoutingRule(id string) (*RoutingRule, error) {
	result, err := c.do("GET", "/api/proxy/routing/rules", nil)
	if err != nil {
		return nil, err
	}
	rules, _ := result["rules"].([]any)
	for _, r := range rules {
		rule, _ := r.(map[string]any)
		if rule["id"] == id {
			return &RoutingRule{
				ID:   id,
				Name: rule["name"].(string),
			}, nil
		}
	}
	return nil, fmt.Errorf("rule %s not found", id)
}

func (c *Client) DeleteRoutingRule(id string) error {
	_, err := c.do("DELETE", "/api/proxy/routing/rules/"+id, nil)
	return err
}

// Alert manages an observe alert rule.
type Alert struct {
	Name      string  `json:"name"`
	Metric    string  `json:"metric"`
	Condition string  `json:"condition"`
	Threshold float64 `json:"threshold"`
}

func (c *Client) CreateAlert(a Alert) error {
	_, err := c.do("POST", "/api/observe/alerts", map[string]any{
		"name": a.Name, "metric": a.Metric, "condition": a.Condition, "threshold": a.Threshold,
	})
	return err
}

func (c *Client) DeleteAlert(name string) error {
	_, err := c.do("DELETE", "/api/observe/alerts/"+name, nil)
	return err
}

// TeamMember manages a team member invite.
type TeamMember struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Name  string `json:"name"`
	Role  string `json:"role"`
}

func (c *Client) InviteTeamMember(m TeamMember) (string, error) {
	result, err := c.do("POST", "/api/team/members", map[string]string{
		"email": m.Email, "name": m.Name, "role": m.Role,
	})
	if err != nil {
		return "", err
	}
	id, _ := result["id"].(string)
	return id, nil
}

func (c *Client) UpdateTeamMember(id, role string) error {
	_, err := c.do("PUT", "/api/team/members/"+id, map[string]string{"role": role})
	return err
}

func (c *Client) DeleteTeamMember(id string) error {
	_, err := c.do("DELETE", "/api/team/members/"+id, nil)
	return err
}

func main() {
	// This would normally integrate with the Pulumi SDK.
	// Stub for now — demonstrates the resource CRUD pattern.
	fmt.Println("stockyard pulumi provider v1.0")
	fmt.Println()
	fmt.Println("Resources:")
	fmt.Println("  stockyard:Module       — manage proxy middleware modules")
	fmt.Println("  stockyard:RoutingRule  — manage smart routing rules")
	fmt.Println("  stockyard:Alert        — manage observe alert rules")
	fmt.Println("  stockyard:TeamMember   — manage team member invites")
	fmt.Println()
	fmt.Println("See https://stockyard.dev/docs/ for usage.")
}
