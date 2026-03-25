package lineage

import "testing"

func TestBuildTree(t *testing.T) {
	nodes := []Node{
		{ID: "a", ParentID: ""},
		{ID: "b", ParentID: "a"},
		{ID: "c", ParentID: "a"},
	}
	roots := BuildTree(nodes)
	if len(roots) != 1 { t.Fatalf("expected 1 root, got %d", len(roots)) }
	if len(roots[0].Children) != 2 { t.Errorf("expected 2 children, got %d", len(roots[0].Children)) }
}
