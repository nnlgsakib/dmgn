package memory

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"
)

type Type string

const (
	TypeText         Type = "text"
	TypeConversation Type = "conversation"
	TypeObservation  Type = "observation"
	TypeDocument     Type = "document"
)

type Memory struct {
	ID               string            `json:"id"`
	Timestamp        int64             `json:"timestamp"`
	Type             Type              `json:"type"`
	Embedding        []float32         `json:"embedding,omitempty"`
	EncryptedPayload []byte            `json:"encrypted_payload"`
	Links            []string          `json:"links"`
	MerkleProof      string            `json:"merkle_proof"`
	Metadata         map[string]string `json:"metadata,omitempty"`
}

type PlaintextMemory struct {
	Content  string            `json:"content"`
	Type     Type              `json:"type"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

type Node struct {
	Memory   *Memory
	Children []*Edge
	Parents  []*Edge
}

type Edge struct {
	From   string
	To     string
	Weight float32
	Type   string
}

type Graph struct {
	nodes map[string]*Node
	edges map[string]*Edge
}

func NewGraph() *Graph {
	return &Graph{
		nodes: make(map[string]*Node),
		edges: make(map[string]*Edge),
	}
}

func (g *Graph) AddNode(m *Memory) *Node {
	if existing, ok := g.nodes[m.ID]; ok {
		existing.Memory = m
		return existing
	}

	node := &Node{
		Memory:   m,
		Children: make([]*Edge, 0),
		Parents:  make([]*Edge, 0),
	}
	g.nodes[m.ID] = node
	return node
}

func (g *Graph) AddEdge(from, to string, weight float32, edgeType string) error {
	fromNode, ok := g.nodes[from]
	if !ok {
		return fmt.Errorf("source node not found: %s", from)
	}

	toNode, ok := g.nodes[to]
	if !ok {
		return fmt.Errorf("target node not found: %s", to)
	}

	edgeID := fmt.Sprintf("%s->%s", from, to)
	edge := &Edge{
		From:   from,
		To:     to,
		Weight: weight,
		Type:   edgeType,
	}
	g.edges[edgeID] = edge

	fromNode.Children = append(fromNode.Children, edge)
	toNode.Parents = append(toNode.Parents, edge)

	return nil
}

func (g *Graph) GetNode(id string) (*Node, bool) {
	node, ok := g.nodes[id]
	return node, ok
}

func (g *Graph) GetAllNodes() []*Node {
	nodes := make([]*Node, 0, len(g.nodes))
	for _, node := range g.nodes {
		nodes = append(nodes, node)
	}
	return nodes
}

func (g *Graph) TraverseFrom(startID string, maxDepth int) []*Node {
	visited := make(map[string]bool)
	result := make([]*Node, 0)

	var traverse func(nodeID string, depth int)
	traverse = func(nodeID string, depth int) {
		if depth > maxDepth || visited[nodeID] {
			return
		}

		node, ok := g.nodes[nodeID]
		if !ok {
			return
		}

		visited[nodeID] = true
		result = append(result, node)

		for _, edge := range node.Children {
			traverse(edge.To, depth+1)
		}
	}

	traverse(startID, 0)
	return result
}

func Create(plaintext *PlaintextMemory, links []string, encryptFn func([]byte) ([]byte, error)) (*Memory, error) {
	plaintextJSON, err := json.Marshal(plaintext)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal plaintext: %w", err)
	}

	encryptedPayload, err := encryptFn(plaintextJSON)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt payload: %w", err)
	}

	hash := sha256.Sum256(encryptedPayload)
	id := hex.EncodeToString(hash[:])

	merkleHash := calculateMerkleProof(id, encryptedPayload)

	memory := &Memory{
		ID:               id,
		Timestamp:        time.Now().UnixNano(),
		Type:             plaintext.Type,
		EncryptedPayload: encryptedPayload,
		Links:            links,
		MerkleProof:      merkleHash,
		Metadata:         plaintext.Metadata,
	}

	return memory, nil
}

func (m *Memory) VerifyIntegrity() bool {
	hash := sha256.Sum256(m.EncryptedPayload)
	calculatedID := hex.EncodeToString(hash[:])

	if calculatedID != m.ID {
		return false
	}

	calculatedMerkle := calculateMerkleProof(m.ID, m.EncryptedPayload)
	return calculatedMerkle == m.MerkleProof
}

func (m *Memory) Decrypt(decryptFn func([]byte) ([]byte, error)) (*PlaintextMemory, error) {
	plaintextJSON, err := decryptFn(m.EncryptedPayload)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt payload: %w", err)
	}

	var plaintext PlaintextMemory
	if err := json.Unmarshal(plaintextJSON, &plaintext); err != nil {
		return nil, fmt.Errorf("failed to unmarshal plaintext: %w", err)
	}

	return &plaintext, nil
}

func (m *Memory) ToJSON() ([]byte, error) {
	return json.Marshal(m)
}

func FromJSON(data []byte) (*Memory, error) {
	var m Memory
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("failed to unmarshal memory: %w", err)
	}
	return &m, nil
}

func calculateMerkleProof(id string, encryptedPayload []byte) string {
	h := sha256.New()
	h.Write([]byte(id))
	h.Write(encryptedPayload)
	return hex.EncodeToString(h.Sum(nil))
}

func GenerateID(encryptedPayload []byte) string {
	hash := sha256.Sum256(encryptedPayload)
	return hex.EncodeToString(hash[:])
}

func LinkExists(links []string, targetID string) bool {
	for _, link := range links {
		if link == targetID {
			return true
		}
	}
	return false
}

func AddLink(links []string, targetID string) []string {
	if LinkExists(links, targetID) {
		return links
	}
	return append(links, targetID)
}

func RemoveLink(links []string, targetID string) []string {
	result := make([]string, 0, len(links))
	for _, link := range links {
		if link != targetID {
			result = append(result, link)
		}
	}
	return result
}

func SortByTimestamp(memories []*Memory, ascending bool) {
	if ascending {
		for i := 0; i < len(memories)-1; i++ {
			for j := i + 1; j < len(memories); j++ {
				if memories[i].Timestamp > memories[j].Timestamp {
					memories[i], memories[j] = memories[j], memories[i]
				}
			}
		}
	} else {
		for i := 0; i < len(memories)-1; i++ {
			for j := i + 1; j < len(memories); j++ {
				if memories[i].Timestamp < memories[j].Timestamp {
					memories[i], memories[j] = memories[j], memories[i]
				}
			}
		}
	}
}
