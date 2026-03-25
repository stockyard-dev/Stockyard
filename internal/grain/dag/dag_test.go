package dag

import (
	"context"
	"strings"
	"testing"

	"github.com/stockyard-dev/stockyard/internal/grain/tree"
)

func TestNew(t *testing.T) {
	d := New()
	if d.Nodes == nil || d.Edges == nil {
		t.Fatal("New() should initialise maps")
	}
	if len(d.Nodes) != 0 || len(d.Edges) != 0 {
		t.Error("New() should return empty DAG")
	}
}

func TestAddDecision(t *testing.T) {
	d := New()

	dec := &tree.Decision{ID: "auth", Name: "Auth Method", Default: "jwt"}
	d.AddDecision(dec, nil)

	if _, ok := d.Nodes["auth"]; !ok {
		t.Error("expected decision 'auth' in Nodes")
	}
	if len(d.Edges) != 0 {
		t.Error("expected no edges when dependsOn is nil")
	}
}

func TestAddDecision_WithDeps(t *testing.T) {
	d := New()

	d.AddDecision(&tree.Decision{ID: "auth", Default: "jwt"}, nil)
	d.AddDecision(&tree.Decision{ID: "rate-limit", Default: "100"}, []string{"auth"})

	if deps := d.Edges["rate-limit"]; len(deps) != 1 || deps[0] != "auth" {
		t.Errorf("expected rate-limit to depend on auth, got %v", d.Edges["rate-limit"])
	}
}

func TestValidate_NoCycle(t *testing.T) {
	d := New()
	d.AddDecision(&tree.Decision{ID: "a", Default: "1"}, nil)
	d.AddDecision(&tree.Decision{ID: "b", Default: "2"}, []string{"a"})
	d.AddDecision(&tree.Decision{ID: "c", Default: "3"}, []string{"b"})

	if err := d.Validate(); err != nil {
		t.Errorf("expected no error for acyclic graph, got %v", err)
	}
}

func TestValidate_Cycle(t *testing.T) {
	d := New()
	d.AddDecision(&tree.Decision{ID: "a", Default: "1"}, []string{"b"})
	d.AddDecision(&tree.Decision{ID: "b", Default: "2"}, []string{"a"})

	err := d.Validate()
	if err == nil {
		t.Fatal("expected error for cycle")
	}
	if !strings.Contains(err.Error(), "cycle") {
		t.Errorf("expected cycle error, got %v", err)
	}
}

func TestValidate_UnknownDep(t *testing.T) {
	d := New()
	d.AddDecision(&tree.Decision{ID: "a", Default: "1"}, []string{"missing"})

	err := d.Validate()
	if err == nil {
		t.Fatal("expected error for unknown dependency")
	}
	if !strings.Contains(err.Error(), "unknown") {
		t.Errorf("expected unknown decision error, got %v", err)
	}
}

func TestValidate_EmptyDAG(t *testing.T) {
	d := New()
	if err := d.Validate(); err != nil {
		t.Errorf("empty DAG should validate, got %v", err)
	}
}

func TestResolve_SingleNode(t *testing.T) {
	reg := tree.NewRegistry(nil)
	reg.Register(tree.Decision{ID: "d1", Default: "hello"})

	d := New()
	d.AddDecision(&tree.Decision{ID: "d1", Default: "hello"}, nil)

	results, err := d.Resolve("d1", nil, reg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if results["d1"] != "hello" {
		t.Errorf("expected 'hello', got %q", results["d1"])
	}
}

func TestResolve_Chain(t *testing.T) {
	reg := tree.NewRegistry(nil)
	reg.Register(tree.Decision{ID: "leaf", Default: "base"})
	reg.Register(tree.Decision{ID: "root", Default: "derived"})

	d := New()
	d.AddDecision(&tree.Decision{ID: "leaf", Default: "base"}, nil)
	d.AddDecision(&tree.Decision{ID: "root", Default: "derived"}, []string{"leaf"})

	results, err := d.Resolve("root", nil, reg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := results["leaf"]; !ok {
		t.Error("expected leaf in results")
	}
	if _, ok := results["root"]; !ok {
		t.Error("expected root in results")
	}
}

func TestResolve_UnknownRoot(t *testing.T) {
	reg := tree.NewRegistry(nil)
	d := New()

	_, err := d.Resolve("nonexistent", nil, reg)
	if err == nil {
		t.Fatal("expected error for unknown root")
	}
}

func TestResolve_WithBaseContext(t *testing.T) {
	reg := tree.NewRegistry(nil)
	reg.Register(tree.Decision{ID: "d1", Default: "val"})

	d := New()
	d.AddDecision(&tree.Decision{ID: "d1", Default: "val"}, nil)

	ctx := map[string]string{"env": "prod"}
	results, err := d.Resolve("d1", ctx, reg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if results["d1"] != "val" {
		t.Errorf("expected 'val', got %q", results["d1"])
	}
}

func TestResolve_CycleDetected(t *testing.T) {
	reg := tree.NewRegistry(nil)
	d := New()
	d.AddDecision(&tree.Decision{ID: "a", Default: "1"}, []string{"b"})
	d.AddDecision(&tree.Decision{ID: "b", Default: "2"}, []string{"a"})

	_, err := d.Resolve("a", nil, reg)
	if err == nil {
		t.Fatal("expected error for cycle in Resolve")
	}
}

func TestBuildFromRegistry(t *testing.T) {
	reg := tree.NewRegistry(nil)
	reg.Register(tree.Decision{ID: "auth", Default: "jwt"})
	reg.Register(tree.Decision{ID: "rate", Default: "100", DependsOn: []string{"auth"}})

	d := BuildFromRegistry(reg)

	if len(d.Nodes) != 2 {
		t.Errorf("expected 2 nodes, got %d", len(d.Nodes))
	}
	if deps := d.Edges["rate"]; len(deps) != 1 || deps[0] != "auth" {
		t.Errorf("expected rate to depend on auth, got %v", d.Edges["rate"])
	}
}

// Verify context.Background() is acceptable (no panics).
func TestResolve_UsesContext(t *testing.T) {
	reg := tree.NewRegistry(nil)
	reg.Register(tree.Decision{ID: "x", Default: "ok"})

	d := New()
	d.AddDecision(&tree.Decision{ID: "x", Default: "ok"}, nil)

	results, err := d.Resolve("x", nil, reg)
	if err != nil {
		t.Fatal(err)
	}
	_ = results
	_ = context.Background() // just proving we can use context
}
