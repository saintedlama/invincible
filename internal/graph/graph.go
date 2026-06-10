package graph

type Node struct {
	Name   string
	Deps   []string
	DepsBy []string
}

type Graph struct {
	nodes map[string]*Node
}

// Edge describes one process in the dependency graph.
type Edge struct {
	Name string
	Deps []string
}

func New(edges []Edge) *Graph {
	g := &Graph{nodes: make(map[string]*Node, len(edges))}
	for _, e := range edges {
		g.nodes[e.Name] = &Node{Name: e.Name, Deps: e.Deps}
	}
	for _, n := range g.nodes {
		for _, dep := range n.Deps {
			if nd := g.nodes[dep]; nd != nil {
				nd.DepsBy = append(nd.DepsBy, n.Name)
			}
		}
	}
	return g
}

// Node returns the graph node for the given name, or nil.
func (g *Graph) Node(name string) *Node {
	return g.nodes[name]
}

// Len returns the number of nodes in the graph.
func (g *Graph) Len() int {
	return len(g.nodes)
}

// WalkDependents does a BFS from start following DepsBy edges (forward through
// the graph: start → dependents → transitive dependents). fn receives each
// node name. Return false from fn to stop walking deeper from that node.
func (g *Graph) WalkDependents(start string, fn func(name string) bool) {
	seen := map[string]bool{start: true}
	queue := g.nodeNames(g.Node(start))

	for len(queue) > 0 {
		name := queue[0]
		queue = queue[1:]

		if seen[name] {
			continue
		}
		seen[name] = true

		if !fn(name) {
			continue
		}

		nd := g.nodes[name]
		if nd != nil {
			queue = append(queue, nd.DepsBy...)
		}
	}
}

// StopLevels returns the graph partitioned into levels for ordered shutdown.
// Level 0 contains processes with no dependents — stop these first. Each
// subsequent level can be stopped after all previous levels finish. Processes
// within the same level can be stopped concurrently.
func (g *Graph) StopLevels() [][]string {
	return g.levelOrder(func(n *Node) int { return len(n.DepsBy) }, func(n *Node) []string { return n.Deps })
}

// StartLevels returns the graph partitioned into levels for ordered startup.
// Level 0 contains processes with no dependencies — start these first. Each
// subsequent level can be started after all previous levels are running.
// Processes within the same level can be started concurrently.
func (g *Graph) StartLevels() [][]string {
	return g.levelOrder(func(n *Node) int { return len(n.Deps) }, func(n *Node) []string { return n.DepsBy })
}

// levelOrder is Kahn's algorithm parameterised by the "blocking edge" direction.
// inCount extracts the incoming edge count for a node (edges from dependents
// for StopLevels, edges from dependencies for StartLevels). resolve returns
// the outgoing edges to propagate once a node is resolved.
func (g *Graph) levelOrder(inCount func(*Node) int, resolve func(*Node) []string) [][]string {
	indeg := make(map[string]int, len(g.nodes))
	for name, n := range g.nodes {
		indeg[name] = inCount(n)
	}

	var levels [][]string
	var current []string
	for name, d := range indeg {
		if d == 0 {
			current = append(current, name)
		}
	}

	for len(current) > 0 {
		levels = append(levels, current)
		var next []string
		for _, name := range current {
			nd := g.nodes[name]
			if nd == nil {
				continue
			}
			for _, target := range resolve(nd) {
				indeg[target]--
				if indeg[target] == 0 {
					next = append(next, target)
				}
			}
		}
		current = next
	}

	return levels
}

func (g *Graph) nodeNames(n *Node) []string {
	if n == nil {
		return nil
	}
	return n.DepsBy
}
