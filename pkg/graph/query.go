package graph

import (
	"strings"
)

type Query struct {
	NodeID    string
	Direction Direction
	EdgeType  string
	MaxDepth  int
}

type QueryResult struct {
	Node  *Node
	Edges []Edge
	Depth int
}

func FindIncoming(g *Graph, nodeID string, edgeType string) ([]Edge, error) {
	return g.GetIncoming(nodeID, edgeType)
}

func FindOutgoing(g *Graph, nodeID string, edgeType string) ([]Edge, error) {
	return g.GetOutgoing(nodeID, edgeType)
}

func FindPath(g *Graph, from, to string, maxDepth int) ([]Edge, error) {
	if maxDepth <= 0 {
		maxDepth = 5
	}

	visited := make(map[string]bool)
	var path []Edge

	var dfs func(current string, depth int) bool
	dfs = func(current string, depth int) bool {
		if depth > maxDepth {
			return false
		}
		if current == to {
			return true
		}

		visited[current] = true

		edges, err := g.GetOutgoing(current, "")
		if err != nil {
			return false
		}

		for _, edge := range edges {
			if visited[edge.To] {
				continue
			}

			path = append(path, edge)

			if edge.To == to {
				return true
			}

			if dfs(edge.To, depth+1) {
				return true
			}

			path = path[:len(path)-1]
		}

		return false
	}

	if dfs(from, 0) {
		return path, nil
	}

	return nil, nil
}

func FindAllConnected(g *Graph, nodeID string, maxDepth int) (map[string]bool, error) {
	if maxDepth <= 0 {
		maxDepth = 3
	}

	connected := make(map[string]bool)

	var bfs func(start string, depth int)
	bfs = func(start string, depth int) {
		if depth > maxDepth {
			return
		}

		outEdges, _ := g.GetOutgoing(start, "")
		for _, edge := range outEdges {
			if !connected[edge.To] {
				connected[edge.To] = true
				bfs(edge.To, depth+1)
			}
		}

		inEdges, _ := g.GetIncoming(start, "")
		for _, edge := range inEdges {
			if !connected[edge.From] {
				connected[edge.From] = true
				bfs(edge.From, depth+1)
			}
		}
	}

	connected[nodeID] = true
	bfs(nodeID, 0)

	return connected, nil
}

func FindByType(g *Graph, nodeType string) ([]Node, error) {
	allNodes, err := g.GetAllNodes()
	if err != nil {
		return nil, err
	}

	var filtered []Node
	for _, node := range allNodes {
		if node.Type == nodeType {
			filtered = append(filtered, node)
		}
	}

	return filtered, nil
}

func FindByLabel(g *Graph, labelPattern string) ([]Node, error) {
	allNodes, err := g.GetAllNodes()
	if err != nil {
		return nil, err
	}

	patternLower := strings.ToLower(labelPattern)
	var filtered []Node
	for _, node := range allNodes {
		if strings.Contains(strings.ToLower(node.Label), patternLower) {
			filtered = append(filtered, node)
		}
	}

	return filtered, nil
}

func FindByEdgeType(g *Graph, edgeType string) ([]Edge, error) {
	allEdges, err := g.GetAllEdges()
	if err != nil {
		return nil, err
	}

	var filtered []Edge
	for _, edge := range allEdges {
		if edge.Type == edgeType {
			filtered = append(filtered, edge)
		}
	}

	return filtered, nil
}

func GetGraphStats(g *Graph) (int, int, error) {
	nodeCount, err := g.NodeCount()
	if err != nil {
		return 0, 0, err
	}

	edgeCount, err := g.EdgeCount()
	if err != nil {
		return 0, 0, err
	}

	return nodeCount, edgeCount, nil
}
