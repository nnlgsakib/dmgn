package query

import (
	"sort"
	"strings"

	"github.com/nnlgsakib/dmgn/pkg/memory"
	"github.com/nnlgsakib/dmgn/pkg/storage"
	"github.com/nnlgsakib/dmgn/pkg/vectorindex"
)

// QueryEngine handles local memory search with hybrid vector + text scoring.
type QueryEngine struct {
	index     *vectorindex.VectorIndex
	store     *storage.Store
	decryptFn func([]byte) ([]byte, error)
	alpha     float64 // hybrid score weight: alpha*vector + (1-alpha)*text
}

// NewQueryEngine creates a query engine with a vector index and storage backend.
func NewQueryEngine(index *vectorindex.VectorIndex, store *storage.Store,
	decryptFn func([]byte) ([]byte, error), alpha float64) *QueryEngine {
	if alpha <= 0 || alpha > 1 {
		alpha = 0.7
	}
	return &QueryEngine{
		index:     index,
		store:     store,
		decryptFn: decryptFn,
		alpha:     alpha,
	}
}

// QueryRequest holds parameters for a query.
type QueryRequest struct {
	Embedding []float32    `json:"embedding,omitempty"`
	TextQuery string       `json:"text_query,omitempty"`
	Limit     int          `json:"limit"`
	Filters   QueryFilters `json:"filters,omitempty"`
}

// QueryFilters holds optional query filters.
type QueryFilters struct {
	Type   string `json:"type,omitempty"`
	After  int64  `json:"after,omitempty"`
	Before int64  `json:"before,omitempty"`
}

// QueryResult holds a single search result.
type QueryResult struct {
	MemoryID   string  `json:"memory_id"`
	Score      float64 `json:"score"`
	Type       string  `json:"type"`
	Timestamp  int64   `json:"timestamp"`
	Snippet    string  `json:"snippet"`
	SourcePeer string  `json:"source_peer,omitempty"`
}

// BuildRequest creates a QueryRequest from common parameters.
func (qe *QueryEngine) BuildRequest(textQuery string, embedding []float32, limit int) QueryRequest {
	if limit <= 0 {
		limit = 10
	}
	return QueryRequest{
		TextQuery: textQuery,
		Embedding: embedding,
		Limit:     limit,
	}
}

// SearchLocal performs a local hybrid search (vector + text).
func (qe *QueryEngine) SearchLocal(req QueryRequest) ([]QueryResult, error) {
	if req.Limit <= 0 {
		req.Limit = 10
	}

	var vectorResults map[string]float64
	if len(req.Embedding) > 0 && qe.index != nil && qe.index.Count() > 0 {
		// Get more results than needed for hybrid scoring
		candidates := qe.index.Search(req.Embedding, req.Limit*3)
		vectorResults = make(map[string]float64, len(candidates))
		for _, c := range candidates {
			vectorResults[c.MemoryID] = c.Score
		}
	}

	// Get candidate memories for text scoring
	allMemories, err := qe.store.GetRecentMemories(1000)
	if err != nil {
		return nil, err
	}

	queryLower := strings.ToLower(req.TextQuery)
	queryWords := strings.Fields(queryLower)

	var results []QueryResult
	for _, mem := range allMemories {
		// Apply filters
		if !passesFilters(mem, req.Filters) {
			continue
		}

		// Compute text score
		plain, err := mem.Decrypt(qe.decryptFn)
		if err != nil {
			continue
		}

		textScore := 0.0
		if req.TextQuery != "" {
			textScore = scoreMatch(plain.Content, queryLower, queryWords)
		}

		// Compute hybrid score
		vecScore, hasVec := vectorResults[mem.ID]
		var finalScore float64
		if hasVec && textScore > 0 {
			finalScore = qe.alpha*vecScore + (1-qe.alpha)*textScore
		} else if hasVec {
			finalScore = vecScore
		} else if textScore > 0 {
			finalScore = textScore
		} else if len(req.Embedding) == 0 && req.TextQuery == "" {
			// No query — return all (recent listing)
			finalScore = 0
		} else {
			continue
		}

		snippet := plain.Content
		if len(snippet) > 100 {
			snippet = snippet[:100]
		}

		results = append(results, QueryResult{
			MemoryID:  mem.ID,
			Score:     finalScore,
			Type:      string(mem.Type),
			Timestamp: mem.Timestamp,
			Snippet:   snippet,
		})
	}

	// Sort by score descending
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	if len(results) > req.Limit {
		results = results[:req.Limit]
	}

	return results, nil
}

// passesFilters checks if a memory passes the query filters.
func passesFilters(mem *memory.Memory, filters QueryFilters) bool {
	if filters.Type != "" && string(mem.Type) != filters.Type {
		return false
	}
	if filters.After > 0 && mem.Timestamp < filters.After {
		return false
	}
	if filters.Before > 0 && mem.Timestamp > filters.Before {
		return false
	}
	return true
}

// scoreMatch computes a text similarity score between content and query.
// Reuses the proven scoring logic from the existing CLI query.
func scoreMatch(content string, queryLower string, queryWords []string) float64 {
	contentLower := strings.ToLower(content)

	if contentLower == queryLower {
		return 1.0
	}

	if strings.Contains(contentLower, queryLower) {
		return 0.8
	}

	if len(queryWords) > 0 {
		matchCount := 0
		contentWords := strings.Fields(contentLower)
		contentSet := make(map[string]bool, len(contentWords))
		for _, w := range contentWords {
			contentSet[w] = true
		}

		for _, qw := range queryWords {
			if contentSet[qw] {
				matchCount++
			}
		}

		ratio := float64(matchCount) / float64(len(queryWords))
		if ratio > 0.5 {
			return 0.5
		}

		for _, qw := range queryWords {
			for _, cw := range contentWords {
				if strings.Contains(cw, qw) || strings.Contains(qw, cw) {
					return 0.3
				}
			}
		}
	}

	return 0
}
