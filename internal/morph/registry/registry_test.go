package registry

import (
	"sort"
	"testing"
)

func TestNew_LoadsBuiltins(t *testing.T) {
	r := New()
	names := r.ServiceNames()
	if len(names) == 0 {
		t.Fatal("expected builtins to be loaded, got 0 services")
	}
	// Spot-check a few known builtins.
	for _, id := range []string{"stripe", "github", "slack"} {
		if svc := r.Get(id); svc == nil {
			t.Errorf("expected builtin service %q to exist", id)
		}
	}
}

func TestRegister_And_Get(t *testing.T) {
	r := &Registry{services: map[string]*Service{}}
	svc := &Service{
		ID:   "TestSvc",
		Name: "Test Service",
		Actions: map[string]Action{
			"ping": {Name: "ping", Method: "GET", Path: "/ping"},
		},
	}
	r.Register(svc)

	// Get should be case-insensitive.
	tests := []struct {
		lookup string
		want   bool
	}{
		{"testsvc", true},
		{"TestSvc", true},
		{"TESTSVC", true},
		{"missing", false},
	}
	for _, tt := range tests {
		got := r.Get(tt.lookup)
		if (got != nil) != tt.want {
			t.Errorf("Get(%q): got %v, want found=%v", tt.lookup, got, tt.want)
		}
	}
}

func TestRegister_Overwrites(t *testing.T) {
	r := &Registry{services: map[string]*Service{}}
	r.Register(&Service{ID: "svc", Name: "Original"})
	r.Register(&Service{ID: "svc", Name: "Updated"})

	svc := r.Get("svc")
	if svc == nil {
		t.Fatal("expected service to exist")
	}
	if svc.Name != "Updated" {
		t.Errorf("expected name Updated, got %q", svc.Name)
	}
}

func TestList(t *testing.T) {
	r := &Registry{services: map[string]*Service{}}
	r.Register(&Service{ID: "a", Name: "A"})
	r.Register(&Service{ID: "b", Name: "B"})

	list := r.List()
	if len(list) != 2 {
		t.Fatalf("expected 2 services, got %d", len(list))
	}
}

func TestServiceNames(t *testing.T) {
	r := &Registry{services: map[string]*Service{}}
	r.Register(&Service{ID: "Zulu", Name: "Zulu"})
	r.Register(&Service{ID: "Alpha", Name: "Alpha"})

	names := r.ServiceNames()
	sort.Strings(names)
	if len(names) != 2 || names[0] != "alpha" || names[1] != "zulu" {
		t.Errorf("unexpected names: %v", names)
	}
}

func TestResolveAction(t *testing.T) {
	r := &Registry{services: map[string]*Service{}}
	r.Register(&Service{
		ID:   "myapi",
		Name: "My API",
		Actions: map[string]Action{
			"create_user":  {Name: "create_user", Method: "POST", Path: "/users"},
			"delete_user":  {Name: "delete_user", Method: "DELETE", Path: "/users/{{id}}"},
			"list_users":   {Name: "list_users", Method: "GET", Path: "/users"},
		},
	})

	tests := []struct {
		name       string
		serviceID  string
		actionName string
		wantAction string
		wantErr    bool
	}{
		{
			name:       "exact match",
			serviceID:  "myapi",
			actionName: "create_user",
			wantAction: "create_user",
		},
		{
			name:       "fuzzy match - partial name contained in action",
			serviceID:  "myapi",
			actionName: "create",
			wantAction: "create_user",
		},
		{
			name:       "service not found",
			serviceID:  "nosvc",
			actionName: "anything",
			wantErr:    true,
		},
		{
			name:       "action not found",
			serviceID:  "myapi",
			actionName: "deploy",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, action, err := r.ResolveAction(tt.serviceID, tt.actionName)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if svc == nil {
				t.Fatal("expected service, got nil")
			}
			if action == nil {
				t.Fatal("expected action, got nil")
			}
			if action.Name != tt.wantAction {
				t.Errorf("expected action %q, got %q", tt.wantAction, action.Name)
			}
		})
	}
}

func TestResolveAction_FuzzyCaseInsensitive(t *testing.T) {
	r := &Registry{services: map[string]*Service{}}
	r.Register(&Service{
		ID:   "svc",
		Name: "Svc",
		Actions: map[string]Action{
			"SendEmail": {Name: "SendEmail", Method: "POST", Path: "/send"},
		},
	})

	_, action, err := r.ResolveAction("svc", "sendemail")
	if err != nil {
		t.Fatalf("expected fuzzy case-insensitive match, got error: %v", err)
	}
	if action.Name != "SendEmail" {
		t.Errorf("expected SendEmail, got %q", action.Name)
	}
}

func TestResolveAction_ActionNotFound_ListsAvailable(t *testing.T) {
	r := &Registry{services: map[string]*Service{}}
	r.Register(&Service{
		ID:   "svc",
		Name: "Svc",
		Actions: map[string]Action{
			"alpha": {Name: "alpha"},
		},
	})

	_, _, err := r.ResolveAction("svc", "nonexistent")
	if err == nil {
		t.Fatal("expected error")
	}
	errStr := err.Error()
	if !contains(errStr, "alpha") {
		t.Errorf("error should list available actions, got: %s", errStr)
	}
}

func TestBuiltin_StripeActions(t *testing.T) {
	r := New()
	svc := r.Get("stripe")
	if svc == nil {
		t.Fatal("stripe service not found")
	}
	expectedActions := []string{"charge", "create_customer", "list_charges", "refund", "create_invoice"}
	for _, name := range expectedActions {
		if _, ok := svc.Actions[name]; !ok {
			t.Errorf("expected stripe action %q", name)
		}
	}
	// Verify charge action details.
	charge := svc.Actions["charge"]
	if charge.Method != "POST" {
		t.Errorf("charge method: got %q, want POST", charge.Method)
	}
	if charge.Path != "/v1/charges" {
		t.Errorf("charge path: got %q, want /v1/charges", charge.Path)
	}
	if charge.BodyTemplate == "" {
		t.Error("charge should have a body template")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
