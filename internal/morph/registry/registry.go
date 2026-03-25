// Package registry maintains knowledge of external service APIs and how to call them.
// Each service has an adapter that translates structured operations into real API calls.
package registry

import (
	"fmt"
	"strings"
	"sync"
)

// Service describes an external API that Morph can interact with.
type Service struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	BaseURL     string            `json:"base_url"`
	AuthType    string            `json:"auth_type"`    // bearer, basic, api_key, header
	AuthHeader  string            `json:"auth_header"`  // Authorization, X-API-Key, etc.
	Actions     map[string]Action `json:"actions"`
	DocsURL     string            `json:"docs_url,omitempty"`
}

// Action describes a single operation on a service.
type Action struct {
	Name        string            `json:"name"`
	Method      string            `json:"method"`      // GET, POST, PUT, DELETE, PATCH
	Path        string            `json:"path"`         // /v1/charges, /repos/{owner}/{repo}/pulls
	Headers     map[string]string `json:"headers,omitempty"`
	BodyTemplate string           `json:"body_template,omitempty"` // JSON template with {{param}} placeholders
	Description string            `json:"description"`
}

// Registry holds all known service definitions.
type Registry struct {
	mu       sync.RWMutex
	services map[string]*Service
}

// New creates a registry pre-populated with common services.
func New() *Registry {
	r := &Registry{services: map[string]*Service{}}
	r.loadBuiltins()
	return r
}

// Get returns a service by ID.
func (r *Registry) Get(id string) *Service {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.services[strings.ToLower(id)]
}

// Register adds or updates a service.
func (r *Registry) Register(s *Service) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.services[strings.ToLower(s.ID)] = s
}

// List returns all registered services.
func (r *Registry) List() []*Service {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []*Service
	for _, s := range r.services {
		out = append(out, s)
	}
	return out
}

// ServiceNames returns names of all registered services.
func (r *Registry) ServiceNames() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var names []string
	for id := range r.services {
		names = append(names, id)
	}
	return names
}

// ResolveAction finds the best matching action for an operation.
func (r *Registry) ResolveAction(serviceID, actionName string) (*Service, *Action, error) {
	svc := r.Get(serviceID)
	if svc == nil {
		return nil, nil, fmt.Errorf("service %q not found", serviceID)
	}

	// Exact match
	if a, ok := svc.Actions[actionName]; ok {
		return svc, &a, nil
	}

	// Fuzzy match (contains)
	actionLower := strings.ToLower(actionName)
	for name, a := range svc.Actions {
		if strings.Contains(strings.ToLower(name), actionLower) ||
			strings.Contains(actionLower, strings.ToLower(name)) {
			return svc, &a, nil
		}
	}

	return svc, nil, fmt.Errorf("action %q not found in service %s (available: %s)",
		actionName, serviceID, actionList(svc))
}

func actionList(s *Service) string {
	var names []string
	for name := range s.Actions {
		names = append(names, name)
	}
	return strings.Join(names, ", ")
}

// =========================================================================
// Built-in service definitions
// =========================================================================

func (r *Registry) loadBuiltins() {
	r.Register(&Service{
		ID: "stripe", Name: "Stripe", BaseURL: "https://api.stripe.com",
		AuthType: "bearer", AuthHeader: "Authorization",
		DocsURL: "https://stripe.com/docs/api",
		Actions: map[string]Action{
			"charge": {Name: "charge", Method: "POST", Path: "/v1/charges",
				BodyTemplate: `{"amount":{{amount_cents}},"currency":"{{currency}}","customer":"{{customer_id}}","description":"{{description}}"}`,
				Description: "Create a charge"},
			"create_customer": {Name: "create_customer", Method: "POST", Path: "/v1/customers",
				BodyTemplate: `{"email":"{{email}}","name":"{{name}}"}`,
				Description: "Create a customer"},
			"list_charges": {Name: "list_charges", Method: "GET", Path: "/v1/charges?limit={{limit}}",
				Description: "List recent charges"},
			"refund": {Name: "refund", Method: "POST", Path: "/v1/refunds",
				BodyTemplate: `{"charge":"{{charge_id}}","amount":{{amount_cents}}}`,
				Description: "Create a refund"},
			"create_invoice": {Name: "create_invoice", Method: "POST", Path: "/v1/invoices",
				BodyTemplate: `{"customer":"{{customer_id}}","auto_advance":true}`,
				Description: "Create an invoice"},
		},
	})

	r.Register(&Service{
		ID: "github", Name: "GitHub", BaseURL: "https://api.github.com",
		AuthType: "bearer", AuthHeader: "Authorization",
		DocsURL: "https://docs.github.com/en/rest",
		Actions: map[string]Action{
			"create_issue": {Name: "create_issue", Method: "POST", Path: "/repos/{{owner}}/{{repo}}/issues",
				BodyTemplate: `{"title":"{{title}}","body":"{{body}}","labels":[{{labels}}]}`,
				Description: "Create an issue"},
			"create_pr": {Name: "create_pr", Method: "POST", Path: "/repos/{{owner}}/{{repo}}/pulls",
				BodyTemplate: `{"title":"{{title}}","body":"{{body}}","head":"{{from_branch}}","base":"{{to_branch}}"}`,
				Description: "Create a pull request"},
			"list_repos": {Name: "list_repos", Method: "GET", Path: "/user/repos?sort=updated&per_page={{limit}}",
				Description: "List user repositories"},
			"create_comment": {Name: "create_comment", Method: "POST", Path: "/repos/{{owner}}/{{repo}}/issues/{{number}}/comments",
				BodyTemplate: `{"body":"{{body}}"}`,
				Description: "Comment on an issue or PR"},
		},
	})

	r.Register(&Service{
		ID: "slack", Name: "Slack", BaseURL: "https://slack.com/api",
		AuthType: "bearer", AuthHeader: "Authorization",
		DocsURL: "https://api.slack.com/methods",
		Actions: map[string]Action{
			"send_message": {Name: "send_message", Method: "POST", Path: "/chat.postMessage",
				BodyTemplate: `{"channel":"{{channel}}","text":"{{text}}"}`,
				Description: "Send a message to a channel"},
			"list_channels": {Name: "list_channels", Method: "GET", Path: "/conversations.list?limit={{limit}}",
				Description: "List channels"},
			"set_topic": {Name: "set_topic", Method: "POST", Path: "/conversations.setTopic",
				BodyTemplate: `{"channel":"{{channel}}","topic":"{{topic}}"}`,
				Description: "Set channel topic"},
		},
	})

	r.Register(&Service{
		ID: "sendgrid", Name: "SendGrid", BaseURL: "https://api.sendgrid.com",
		AuthType: "bearer", AuthHeader: "Authorization",
		Actions: map[string]Action{
			"send_email": {Name: "send_email", Method: "POST", Path: "/v3/mail/send",
				BodyTemplate: `{"personalizations":[{"to":[{"email":"{{to}}"}]}],"from":{"email":"{{from}}"},"subject":"{{subject}}","content":[{"type":"text/plain","value":"{{body}}"}]}`,
				Description: "Send an email"},
		},
	})

	r.Register(&Service{
		ID: "twilio", Name: "Twilio", BaseURL: "https://api.twilio.com",
		AuthType: "basic", AuthHeader: "Authorization",
		Actions: map[string]Action{
			"send_sms": {Name: "send_sms", Method: "POST", Path: "/2010-04-01/Accounts/{{account_sid}}/Messages.json",
				BodyTemplate: `To={{to}}&From={{from}}&Body={{body}}`,
				Description: "Send an SMS"},
		},
	})

	r.Register(&Service{
		ID: "linear", Name: "Linear", BaseURL: "https://api.linear.app",
		AuthType: "bearer", AuthHeader: "Authorization",
		Actions: map[string]Action{
			"create_issue": {Name: "create_issue", Method: "POST", Path: "/graphql",
				BodyTemplate: `{"query":"mutation{issueCreate(input:{title:\"{{title}}\",description:\"{{description}}\",teamId:\"{{team_id}}\"}){success issue{id identifier title}}}"}`,
				Description: "Create a Linear issue"},
		},
	})

	r.Register(&Service{
		ID: "notion", Name: "Notion", BaseURL: "https://api.notion.com",
		AuthType: "bearer", AuthHeader: "Authorization",
		Actions: map[string]Action{
			"create_page": {Name: "create_page", Method: "POST", Path: "/v1/pages",
				Description: "Create a page in a database"},
			"search": {Name: "search", Method: "POST", Path: "/v1/search",
				BodyTemplate: `{"query":"{{query}}"}`,
				Description: "Search pages and databases"},
		},
	})

	r.Register(&Service{
		ID: "jira", Name: "Jira", BaseURL: "https://{{domain}}.atlassian.net",
		AuthType: "basic", AuthHeader: "Authorization",
		Actions: map[string]Action{
			"create_issue": {Name: "create_issue", Method: "POST", Path: "/rest/api/3/issue",
				BodyTemplate: `{"fields":{"project":{"key":"{{project}}"},"summary":"{{summary}}","description":{"type":"doc","version":1,"content":[{"type":"paragraph","content":[{"type":"text","text":"{{description}}"}]}]},"issuetype":{"name":"{{type}}"}}}`,
				Description: "Create a Jira issue"},
		},
	})

	r.Register(&Service{
		ID: "discord", Name: "Discord", BaseURL: "https://discord.com/api/v10",
		AuthType: "header", AuthHeader: "Authorization",
		Actions: map[string]Action{
			"send_message": {Name: "send_message", Method: "POST", Path: "/channels/{{channel_id}}/messages",
				BodyTemplate: `{"content":"{{content}}"}`,
				Description: "Send a message to a channel"},
		},
	})

	r.Register(&Service{
		ID: "resend", Name: "Resend", BaseURL: "https://api.resend.com",
		AuthType: "bearer", AuthHeader: "Authorization",
		Actions: map[string]Action{
			"send_email": {Name: "send_email", Method: "POST", Path: "/emails",
				BodyTemplate: `{"from":"{{from}}","to":["{{to}}"],"subject":"{{subject}}","text":"{{body}}"}`,
				Description: "Send an email via Resend"},
		},
	})
}
