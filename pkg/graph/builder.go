package graph

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

type SourceType string

const (
	SourceTypeMemory SourceType = "memory"
	SourceTypeCode   SourceType = "code"
	SourceTypeText   SourceType = "text"
	SourceTypeFile   SourceType = "file"
)

var DefaultEdgeTypes = []string{
	"CREATES",
	"USES",
	"BUILT_BY",
	"PART_OF",
	"RELATED_TO",
	"DEPENDS_ON",
	"CALLS",
	"IMPORTS",
	"EXTENDS",
	"IMPLEMENTS",
	"CONTAINS",
	"TAGGED_WITH",
}

func ExtractEntities(content string, sourceType SourceType) ([]Node, error) {
	var nodes []Node

	switch sourceType {
	case SourceTypeMemory:
		nodes = extractFromMemory(content)
	case SourceTypeCode:
		nodes = extractFromCode(content)
	case SourceTypeText:
		nodes = extractFromText(content)
	default:
		nodes = extractFromText(content)
	}

	for i := range nodes {
		if nodes[i].ID == "" {
			nodes[i].ID = uuid.New().String()
		}
		if nodes[i].CreatedAt == 0 {
			nodes[i].CreatedAt = time.Now().UnixNano()
		}
	}

	return nodes, nil
}

func extractFromMemory(content string) []Node {
	var nodes []Node

	words := strings.Fields(content)
	for _, word := range words {
		word = strings.Trim(word, ".,!?;:\"'()[]")
		if len(word) < 3 {
			continue
		}

		if isCapitalized(word) {
			capitalType := detectEntityType(word)
			nodes = append(nodes, Node{
				ID:    uuid.New().String(),
				Type:  capitalType,
				Label: word,
			})
		}
	}

	return nodes
}

func extractFromCode(code string) []Node {
	var nodes []Node

	lines := strings.Split(code, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)

		if strings.HasPrefix(line, "func ") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				funcName := strings.Trim(parts[1], "()")
				nodes = append(nodes, Node{
					ID:    uuid.New().String(),
					Type:  "function",
					Label: funcName,
				})
			}
		}

		if strings.HasPrefix(line, "type ") && strings.Contains(line, " struct") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				typeName := parts[1]
				nodes = append(nodes, Node{
					ID:    uuid.New().String(),
					Type:  "type",
					Label: typeName,
				})
			}
		}

		if strings.HasPrefix(line, "import ") {
			pkg := strings.TrimPrefix(line, "import ")
			pkg = strings.Trim(pkg, "\"")
			if pkg != "" && !strings.Contains(pkg, "(") {
				nodes = append(nodes, Node{
					ID:    uuid.New().String(),
					Type:  "package",
					Label: pkg,
				})
			}
		}
	}

	return nodes
}

func extractFromText(text string) []Node {
	var nodes []Node

	sentences := strings.Split(text, ".")
	for _, sentence := range sentences {
		sentence = strings.TrimSpace(sentence)
		if len(sentence) < 3 {
			continue
		}

		words := strings.Fields(sentence)
		for _, word := range words {
			word = strings.Trim(word, ".,!?;:\"'()[]")
			if len(word) < 3 {
				continue
			}

			if isCapitalized(word) {
				capitalType := detectEntityType(word)
				nodes = append(nodes, Node{
					ID:    uuid.New().String(),
					Type:  capitalType,
					Label: word,
				})
			}
		}
	}

	return nodes
}

func DiscoverRelations(entities []Node) ([]Edge, error) {
	var edges []Edge

	for i := 0; i < len(entities); i++ {
		for j := i + 1; j < len(entities); j++ {
			rel := findRelationship(entities[i], entities[j])
			if rel != nil {
				if rel.From == "" {
					rel.From = entities[i].ID
				}
				if rel.To == "" {
					rel.To = entities[j].ID
				}
				if rel.ID == "" {
					rel.ID = uuid.New().String()
				}
				if rel.CreatedAt == 0 {
					rel.CreatedAt = time.Now().UnixNano()
				}
				edges = append(edges, *rel)
			}
		}
	}

	return edges, nil
}

func findRelationship(from, to Node) *Edge {
	fromLabel := strings.ToLower(from.Label)
	toLabel := strings.ToLower(to.Label)

	if strings.Contains(fromLabel, "create") || strings.Contains(toLabel, "create") {
		return &Edge{From: from.ID, To: to.ID, Type: "CREATES", Weight: 1.0}
	}
	if strings.Contains(toLabel, "tool") || strings.Contains(toLabel, "library") {
		return &Edge{From: from.ID, To: to.ID, Type: "USES", Weight: 0.8}
	}
	if strings.Contains(toLabel, "part") {
		return &Edge{From: from.ID, To: to.ID, Type: "PART_OF", Weight: 0.7}
	}
	if from.Type == to.Type && from.Type != "" {
		return &Edge{From: from.ID, To: to.ID, Type: "RELATED_TO", Weight: 0.5}
	}

	return &Edge{From: from.ID, To: to.ID, Type: "RELATED_TO", Weight: 0.3}
}

func BuildGraph(g *Graph, content string, sourceType SourceType) error {
	nodes, err := ExtractEntities(content, sourceType)
	if err != nil {
		return err
	}

	for _, node := range nodes {
		if err := g.AddNode(&node); err != nil {
			continue
		}
	}

	edges, err := DiscoverRelations(nodes)
	if err != nil {
		return err
	}

	for _, edge := range edges {
		if err := g.AddEdge(&edge); err != nil {
			continue
		}
	}

	return nil
}

func isCapitalized(s string) bool {
	if s == "" {
		return false
	}
	first := rune(s[0])
	return first >= 'A' && first <= 'Z'
}

func detectEntityType(label string) string {
	labelLower := strings.ToLower(label)

	personKeywords := []string{"person", "user", "developer", "programmer", "engineer", "ai", "agent", "bot"}
	for _, kw := range personKeywords {
		if strings.Contains(labelLower, kw) {
			return "person"
		}
	}

	conceptKeywords := []string{"system", "platform", "network", "graph", "knowledge", "memory", "data"}
	for _, kw := range conceptKeywords {
		if strings.Contains(labelLower, kw) {
			return "concept"
		}
	}

	toolKeywords := []string{"tool", "library", "framework", "package", "module"}
	for _, kw := range toolKeywords {
		if strings.Contains(labelLower, kw) {
			return "tool"
		}
	}

	return "entity"
}
