// Package graph provides system interaction graph construction.
package graph

// Edge represents a directed edge in the system interaction graph.
type Edge struct {
	Source    string  `json:"source"`
	Target   string  `json:"target"`
	Weight   float64 `json:"weight"`
	EventType string `json:"event_type"`
}

// Graph holds the system interaction graph.
type Graph struct {
	Edges []Edge `json:"edges"`
	Nodes map[string]bool `json:"nodes"`
}

// New creates an empty graph.
func New() *Graph { return &Graph{Nodes: map[string]bool{}} }

// AddEdge adds a directed edge to the graph.
func (g *Graph) AddEdge(source, target, eventType string, weight float64) {
	g.Edges = append(g.Edges, Edge{Source: source, Target: target, Weight: weight, EventType: eventType})
	g.Nodes[source] = true
	g.Nodes[target] = true
}

// NodeCount returns the number of unique nodes.
func (g *Graph) NodeCount() int { return len(g.Nodes) }
