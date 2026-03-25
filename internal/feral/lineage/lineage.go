// Package lineage provides attack family tree tracking.
package lineage

// Node represents a node in the attack family tree.
type Node struct {
	ID       string  `json:"id"`
	ParentID string  `json:"parent_id"`
	Type     string  `json:"attack_type"`
	Success  bool    `json:"success"`
	Children []*Node `json:"children,omitempty"`
}

// BuildTree constructs a tree from a flat list of nodes.
func BuildTree(nodes []Node) []*Node {
	byID := map[string]*Node{}
	for i := range nodes { byID[nodes[i].ID] = &nodes[i] }
	var roots []*Node
	for i := range nodes {
		n := &nodes[i]
		if n.ParentID == "" { roots = append(roots, n); continue }
		if parent, ok := byID[n.ParentID]; ok { parent.Children = append(parent.Children, n) }
	}
	return roots
}
