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

func (g *Graph) nodeNames(n *Node) []string {
	if n == nil {
		return nil
	}
	return n.DepsBy
}
