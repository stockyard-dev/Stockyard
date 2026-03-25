// Package dag provides decision composition — a directed acyclic graph of
// decisions where one decision's outcome can feed into another decision's
// evaluation context. This lets teams model multi-step decision chains
// (e.g. "pick auth method, then pick rate limit based on auth method").
package dag

import (
	"context"
	"fmt"

	"github.com/stockyard-dev/stockyard/internal/grain/tree"
)

// DAG is a directed acyclic graph of decisions.
type DAG struct {
	Nodes map[string]*tree.Decision `json:"nodes"`
	Edges map[string][]string       `json:"edges"` // child -> parents (dependencies)
}

// New creates an empty DAG.
func New() *DAG {
	return &DAG{
		Nodes: make(map[string]*tree.Decision),
		Edges: make(map[string][]string),
	}
}

// AddDecision registers a decision node and its dependency edges.
func (d *DAG) AddDecision(dec *tree.Decision, dependsOn []string) {
	d.Nodes[dec.ID] = dec
	if len(dependsOn) > 0 {
		d.Edges[dec.ID] = dependsOn
	}
}

// Validate checks the graph for cycles using depth-first search.
// Returns an error describing the cycle if one is found.
func (d *DAG) Validate() error {
	const (
		white = 0 // unvisited
		gray  = 1 // in current path
		black = 2 // fully explored
	)
	color := make(map[string]int)
	var visit func(id string, path []string) error
	visit = func(id string, path []string) error {
		color[id] = gray
		for _, dep := range d.Edges[id] {
			if color[dep] == gray {
				return fmt.Errorf("cycle detected: %s -> %s (path: %v)", id, dep, append(path, id))
			}
			if color[dep] == white {
				if _, ok := d.Nodes[dep]; !ok {
					return fmt.Errorf("unknown decision %q referenced by %q", dep, id)
				}
				if err := visit(dep, append(path, id)); err != nil {
					return err
				}
			}
		}
		color[id] = black
		return nil
	}
	for id := range d.Nodes {
		if color[id] == white {
			if err := visit(id, nil); err != nil {
				return err
			}
		}
	}
	return nil
}

// Resolve evaluates the full DAG starting from rootDecisionID, resolving
// dependencies from leaves to root. Each decision's outcome is injected into
// its dependents' evaluation context under the key "dep.<decision_id>".
// Returns a map of decision_id -> resolved value for every node reachable
// from rootDecisionID.
func (d *DAG) Resolve(rootDecisionID string, baseContext map[string]string, registry *tree.Registry) (map[string]string, error) {
	if err := d.Validate(); err != nil {
		return nil, err
	}
	if _, ok := d.Nodes[rootDecisionID]; !ok {
		return nil, fmt.Errorf("root decision %q not found in DAG", rootDecisionID)
	}

	// Collect all nodes reachable from root (BFS upward through deps).
	needed := map[string]bool{}
	var collect func(id string)
	collect = func(id string) {
		if needed[id] {
			return
		}
		needed[id] = true
		for _, dep := range d.Edges[id] {
			collect(dep)
		}
	}
	collect(rootDecisionID)

	// Topological order — leaves first.
	order, err := d.topoSort(needed)
	if err != nil {
		return nil, err
	}

	results := make(map[string]string, len(order))
	for _, id := range order {
		ec := &tree.EvalContext{Custom: make(map[string]string)}
		// Copy base context.
		for k, v := range baseContext {
			ec.Custom[k] = v
		}
		// Inject dependency results.
		for _, dep := range d.Edges[id] {
			ec.Custom["dep."+dep] = results[dep]
		}
		outcome := registry.Evaluate(context.Background(), id, ec)
		results[id] = outcome.Value
	}

	return results, nil
}

// topoSort returns a topological ordering of the subset of nodes indicated
// by the needed set (leaves first).
func (d *DAG) topoSort(needed map[string]bool) ([]string, error) {
	inDegree := make(map[string]int)
	// Initialise in-degree for needed nodes.
	for id := range needed {
		if _, ok := inDegree[id]; !ok {
			inDegree[id] = 0
		}
		for _, dep := range d.Edges[id] {
			if needed[dep] {
				// dep is a dependency OF id, so dep must come first.
				// This means id has an incoming edge from dep.
				inDegree[id]++
				if _, ok := inDegree[dep]; !ok {
					inDegree[dep] = 0
				}
			}
		}
	}

	// Kahn's algorithm.
	var queue []string
	for id, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, id)
		}
	}

	var order []string
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		order = append(order, cur)

		// For each node that depends on cur, decrement in-degree.
		for id := range needed {
			for _, dep := range d.Edges[id] {
				if dep == cur {
					inDegree[id]--
					if inDegree[id] == 0 {
						queue = append(queue, id)
					}
				}
			}
		}
	}

	if len(order) != len(inDegree) {
		return nil, fmt.Errorf("cycle detected during topological sort")
	}
	return order, nil
}

// BuildFromRegistry constructs a DAG from all decisions in a registry that
// have DependsOn fields set.
func BuildFromRegistry(reg *tree.Registry) *DAG {
	d := New()
	for _, dec := range reg.ListDecisions() {
		cp := dec
		d.AddDecision(&cp, dec.DependsOn)
	}
	return d
}
