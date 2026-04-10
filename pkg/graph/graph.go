package graph

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/dgraph-io/badger/v4"
)

const (
	PrefixNode = "kg:node:"
	PrefixEdge = "kg:edge:"
)

type Direction string

const (
	DirectionIncoming Direction = "incoming"
	DirectionOutgoing Direction = "outgoing"
	DirectionBoth     Direction = "both"
)

type Node struct {
	ID        string         `json:"id"`
	Type      string         `json:"type"`
	Label     string         `json:"label"`
	Meta      map[string]any `json:"meta,omitempty"`
	CreatedAt int64          `json:"created_at"`
}

type Edge struct {
	ID        string  `json:"id"`
	From      string  `json:"from"`
	To        string  `json:"to"`
	Type      string  `json:"type"`
	Weight    float32 `json:"weight"`
	CreatedAt int64   `json:"created_at"`
}

type Graph struct {
	db *badger.DB
}

func NewGraph(db *badger.DB) *Graph {
	return &Graph{db: db}
}

func (g *Graph) AddNode(node *Node) error {
	if node.CreatedAt == 0 {
		node.CreatedAt = time.Now().UnixNano()
	}
	data, err := json.Marshal(node)
	if err != nil {
		return fmt.Errorf("failed to marshal node: %w", err)
	}
	return g.db.Update(func(txn *badger.Txn) error {
		return txn.Set([]byte(PrefixNode+node.ID), data)
	})
}

func (g *Graph) AddEdge(edge *Edge) error {
	if edge.CreatedAt == 0 {
		edge.CreatedAt = time.Now().UnixNano()
	}
	if edge.ID == "" {
		edge.ID = fmt.Sprintf("%s:%s:%s", edge.From, edge.Type, edge.To)
	}
	data, err := json.Marshal(edge)
	if err != nil {
		return fmt.Errorf("failed to marshal edge: %w", err)
	}
	return g.db.Update(func(txn *badger.Txn) error {
		return txn.Set([]byte(PrefixEdge+edge.ID), data)
	})
}

func (g *Graph) GetNode(id string) (*Node, error) {
	var node Node
	err := g.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(PrefixNode + id))
		if err != nil {
			return err
		}
		valCopy, err := item.ValueCopy(nil)
		if err != nil {
			return err
		}
		return json.Unmarshal(valCopy, &node)
	})
	if err != nil {
		return nil, fmt.Errorf("node not found: %s", id)
	}
	return &node, nil
}

func (g *Graph) GetEdges(nodeID string, dir Direction) ([]Edge, error) {
	var edges []Edge
	prefix := []byte(PrefixEdge)

	err := g.db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()

		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()
			valCopy, err := item.ValueCopy(nil)
			if err != nil {
				continue
			}
			var edge Edge
			if err := json.Unmarshal(valCopy, &edge); err != nil {
				continue
			}

			include := false
			switch dir {
			case DirectionIncoming:
				include = edge.To == nodeID
			case DirectionOutgoing:
				include = edge.From == nodeID
			case DirectionBoth:
				include = edge.From == nodeID || edge.To == nodeID
			}

			if include {
				edges = append(edges, edge)
			}
		}
		return nil
	})

	return edges, err
}

func (g *Graph) GetIncoming(nodeID string, edgeType string) ([]Edge, error) {
	allEdges, err := g.GetEdges(nodeID, DirectionIncoming)
	if err != nil {
		return nil, err
	}

	if edgeType == "" {
		return allEdges, nil
	}

	var filtered []Edge
	for _, e := range allEdges {
		if e.To == nodeID && e.Type == edgeType {
			filtered = append(filtered, e)
		}
	}

	return filtered, nil
}

func (g *Graph) GetOutgoing(nodeID string, edgeType string) ([]Edge, error) {
	allEdges, err := g.GetEdges(nodeID, DirectionOutgoing)
	if err != nil {
		return nil, err
	}

	if edgeType == "" {
		return allEdges, nil
	}

	var filtered []Edge
	for _, e := range allEdges {
		if e.From == nodeID && e.Type == edgeType {
			filtered = append(filtered, e)
		}
	}

	return filtered, nil
}

func (g *Graph) GetAllNodes() ([]Node, error) {
	var nodes []Node
	prefix := []byte(PrefixNode)

	err := g.db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()

		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()
			valCopy, err := item.ValueCopy(nil)
			if err != nil {
				continue
			}
			var node Node
			if err := json.Unmarshal(valCopy, &node); err != nil {
				continue
			}
			nodes = append(nodes, node)
		}
		return nil
	})

	return nodes, err
}

func (g *Graph) GetAllEdges() ([]Edge, error) {
	var edges []Edge
	prefix := []byte(PrefixEdge)

	err := g.db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()

		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()
			valCopy, err := item.ValueCopy(nil)
			if err != nil {
				continue
			}
			var edge Edge
			if err := json.Unmarshal(valCopy, &edge); err != nil {
				continue
			}
			edges = append(edges, edge)
		}
		return nil
	})

	return edges, err
}

func (g *Graph) NodeCount() (int, error) {
	count := 0
	prefix := []byte(PrefixNode)

	err := g.db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()

		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			count++
		}
		return nil
	})

	return count, err
}

func (g *Graph) EdgeCount() (int, error) {
	count := 0
	prefix := []byte(PrefixEdge)

	err := g.db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()

		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			count++
		}
		return nil
	})

	return count, err
}

func (g *Graph) Exists(id string) (bool, error) {
	_, err := g.GetNode(id)
	if err != nil {
		return false, nil
	}
	return true, nil
}

func (g *Graph) DeleteNode(id string) error {
	return g.db.Update(func(txn *badger.Txn) error {
		return txn.Delete([]byte(PrefixNode + id))
	})
}

func (g *Graph) DeleteEdge(id string) error {
	return g.db.Update(func(txn *badger.Txn) error {
		return txn.Delete([]byte(PrefixEdge + id))
	})
}
