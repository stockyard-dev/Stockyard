package graph

import "testing"

func TestGraph(t *testing.T) {
	g := New()
	g.AddEdge("a", "b", "event", 1.0)
	g.AddEdge("b", "c", "event", 0.5)
	if g.NodeCount() != 3 { t.Errorf("expected 3 nodes, got %d", g.NodeCount()) }
}
