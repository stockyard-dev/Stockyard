package agent

import (
	"testing"
)

func TestLoadCatalog(t *testing.T) {
	catalog, err := LoadCatalog()
	if err != nil {
		t.Fatalf("LoadCatalog failed: %v", err)
	}
	if len(catalog) == 0 {
		t.Fatal("catalog is empty")
	}
	if len(catalog) < 100 {
		t.Errorf("expected 100+ products, got %d", len(catalog))
	}

	// Check a known product
	costcap, ok := catalog["costcap"]
	if !ok {
		t.Fatal("costcap not found in catalog")
	}
	if costcap.DisplayName != "CostCap" {
		t.Errorf("expected CostCap, got %s", costcap.DisplayName)
	}
	if len(costcap.Tools) == 0 {
		t.Error("costcap has no tools")
	}
}

func TestAgentNew(t *testing.T) {
	catalog, _ := LoadCatalog()
	a := New(catalog, nil, "http://127.0.0.1:4200")

	if err := a.AddTool("costcap", "127.0.0.1", 4100); err != nil {
		t.Fatalf("AddTool failed: %v", err)
	}

	tools := a.ConnectedTools()
	if len(tools) != 1 {
		t.Fatalf("expected 1 connected tool, got %d", len(tools))
	}
	if tools[0].Product != "costcap" {
		t.Errorf("expected costcap, got %s", tools[0].Product)
	}
}

func TestAgentAddToolUnknown(t *testing.T) {
	catalog, _ := LoadCatalog()
	a := New(catalog, nil, "http://127.0.0.1:4200")

	err := a.AddTool("nonexistent", "127.0.0.1", 9999)
	if err == nil {
		t.Fatal("expected error for unknown product")
	}
}

func TestAddToolsFromSpec(t *testing.T) {
	catalog, _ := LoadCatalog()
	a := New(catalog, nil, "http://127.0.0.1:4200")

	a.AddToolsFromSpec([]string{"costcap:4100", "llmcache:4200"})

	tools := a.ConnectedTools()
	if len(tools) != 2 {
		t.Fatalf("expected 2 connected tools, got %d", len(tools))
	}
}
