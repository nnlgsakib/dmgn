package vectorindex

import (
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"sort"
	"sync"
)

// SearchResult holds a search result with memory ID and similarity score.
type SearchResult struct {
	MemoryID string
	Score    float64 // cosine similarity, 0.0 to 1.0
}

// vectorEntry stores a single indexed vector.
type vectorEntry struct {
	MemoryID  string
	Embedding []float32
}

// VectorIndex provides vector similarity search with encrypted persistence.
// Uses brute-force cosine similarity, optimized for the expected DMGN dataset
// size (hundreds to low thousands of vectors). Pure Go, no CGo, cross-platform.
type VectorIndex struct {
	vectors   []vectorEntry
	lookup    map[string]int // memoryID -> index in vectors slice
	dimension int
	encryptFn func([]byte) ([]byte, error)
	decryptFn func([]byte) ([]byte, error)
	indexPath string
	dirty     bool
	mu        sync.RWMutex
}

// NewVectorIndex creates a new empty vector index.
// encryptFn/decryptFn are for persisting the index encrypted.
// indexPath is the file path for encrypted index storage.
func NewVectorIndex(indexPath string, encryptFn, decryptFn func([]byte) ([]byte, error)) *VectorIndex {
	return &VectorIndex{
		vectors:   make([]vectorEntry, 0),
		lookup:    make(map[string]int),
		encryptFn: encryptFn,
		decryptFn: decryptFn,
		indexPath: indexPath,
	}
}

// Load loads an encrypted index from disk. Returns nil if file doesn't exist.
func (vi *VectorIndex) Load() error {
	vi.mu.Lock()
	defer vi.mu.Unlock()

	data, err := os.ReadFile(vi.indexPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // no index yet — start fresh
		}
		return fmt.Errorf("read index file: %w", err)
	}

	plaintext, err := vi.decryptFn(data)
	if err != nil {
		return fmt.Errorf("decrypt index: %w", err)
	}

	vectors, dim, err := decodeIndex(plaintext)
	if err != nil {
		return fmt.Errorf("decode index: %w", err)
	}

	vi.vectors = vectors
	vi.dimension = dim
	vi.lookup = make(map[string]int, len(vectors))
	for i, v := range vectors {
		vi.lookup[v.MemoryID] = i
	}
	vi.dirty = false
	return nil
}

// Save encrypts and persists the index to disk.
func (vi *VectorIndex) Save() error {
	vi.mu.RLock()
	defer vi.mu.RUnlock()

	encoded := encodeIndex(vi.vectors, vi.dimension)

	encrypted, err := vi.encryptFn(encoded)
	if err != nil {
		return fmt.Errorf("encrypt index: %w", err)
	}

	if err := os.WriteFile(vi.indexPath, encrypted, 0600); err != nil {
		return fmt.Errorf("write index file: %w", err)
	}

	vi.dirty = false
	return nil
}

// Add inserts a vector for a memory ID. Auto-detects dimension from first vector.
func (vi *VectorIndex) Add(memoryID string, embedding []float32) error {
	vi.mu.Lock()
	defer vi.mu.Unlock()

	if len(embedding) == 0 {
		return fmt.Errorf("embedding must not be empty")
	}

	if vi.dimension == 0 {
		vi.dimension = len(embedding)
	} else if len(embedding) != vi.dimension {
		return fmt.Errorf("embedding dimension mismatch: expected %d, got %d", vi.dimension, len(embedding))
	}

	// Update existing entry or append new
	if idx, exists := vi.lookup[memoryID]; exists {
		vi.vectors[idx].Embedding = embedding
	} else {
		vi.lookup[memoryID] = len(vi.vectors)
		vi.vectors = append(vi.vectors, vectorEntry{
			MemoryID:  memoryID,
			Embedding: embedding,
		})
	}
	vi.dirty = true
	return nil
}

// Search returns the top-k most similar memory IDs with scores.
func (vi *VectorIndex) Search(query []float32, k int) []SearchResult {
	vi.mu.RLock()
	defer vi.mu.RUnlock()

	if len(vi.vectors) == 0 || k <= 0 {
		return nil
	}

	// Compute cosine similarity for all vectors
	type scored struct {
		idx   int
		score float64
	}
	scores := make([]scored, 0, len(vi.vectors))
	for i, v := range vi.vectors {
		s := cosineSimilarity(query, v.Embedding)
		scores = append(scores, scored{idx: i, score: s})
	}

	// Sort by score descending
	sort.Slice(scores, func(i, j int) bool {
		return scores[i].score > scores[j].score
	})

	// Take top-k
	if k > len(scores) {
		k = len(scores)
	}
	results := make([]SearchResult, k)
	for i := 0; i < k; i++ {
		results[i] = SearchResult{
			MemoryID: vi.vectors[scores[i].idx].MemoryID,
			Score:    scores[i].score,
		}
	}

	return results
}

// Remove removes a memory from the index.
func (vi *VectorIndex) Remove(memoryID string) {
	vi.mu.Lock()
	defer vi.mu.Unlock()

	idx, exists := vi.lookup[memoryID]
	if !exists {
		return
	}

	// Swap with last element and shrink
	last := len(vi.vectors) - 1
	if idx != last {
		vi.vectors[idx] = vi.vectors[last]
		vi.lookup[vi.vectors[idx].MemoryID] = idx
	}
	vi.vectors = vi.vectors[:last]
	delete(vi.lookup, memoryID)
	vi.dirty = true
}

// Count returns the number of indexed vectors.
func (vi *VectorIndex) Count() int {
	vi.mu.RLock()
	defer vi.mu.RUnlock()
	return len(vi.vectors)
}

// Dimension returns the detected embedding dimension (0 if empty).
func (vi *VectorIndex) Dimension() int {
	vi.mu.RLock()
	defer vi.mu.RUnlock()
	return vi.dimension
}

// Dirty returns true if the index has been modified since last save/load.
func (vi *VectorIndex) Dirty() bool {
	vi.mu.RLock()
	defer vi.mu.RUnlock()
	return vi.dirty
}

// cosineSimilarity computes cosine similarity between two float32 vectors.
func cosineSimilarity(a, b []float32) float64 {
	if len(a) != len(b) {
		return 0
	}
	var dotProduct, normA, normB float64
	for i := range a {
		dotProduct += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dotProduct / (math.Sqrt(normA) * math.Sqrt(normB))
}

// Binary encoding format:
// [4 bytes: dimension][4 bytes: count]
// For each entry: [4 bytes: id_len][id_bytes][dim*4 bytes: float32 values]

func encodeIndex(vectors []vectorEntry, dim int) []byte {
	// Calculate size
	size := 8 // dimension + count
	for _, v := range vectors {
		size += 4 + len(v.MemoryID) + dim*4
	}

	buf := make([]byte, size)
	offset := 0

	binary.LittleEndian.PutUint32(buf[offset:], uint32(dim))
	offset += 4
	binary.LittleEndian.PutUint32(buf[offset:], uint32(len(vectors)))
	offset += 4

	for _, v := range vectors {
		binary.LittleEndian.PutUint32(buf[offset:], uint32(len(v.MemoryID)))
		offset += 4
		copy(buf[offset:], v.MemoryID)
		offset += len(v.MemoryID)
		for _, f := range v.Embedding {
			binary.LittleEndian.PutUint32(buf[offset:], math.Float32bits(f))
			offset += 4
		}
	}

	return buf
}

func decodeIndex(data []byte) ([]vectorEntry, int, error) {
	if len(data) < 8 {
		return nil, 0, fmt.Errorf("index data too short")
	}

	dim := int(binary.LittleEndian.Uint32(data[0:4]))
	count := int(binary.LittleEndian.Uint32(data[4:8]))
	offset := 8

	vectors := make([]vectorEntry, 0, count)
	for i := 0; i < count; i++ {
		if offset+4 > len(data) {
			return nil, 0, fmt.Errorf("truncated index at entry %d", i)
		}
		idLen := int(binary.LittleEndian.Uint32(data[offset:]))
		offset += 4

		if offset+idLen > len(data) {
			return nil, 0, fmt.Errorf("truncated id at entry %d", i)
		}
		memID := string(data[offset : offset+idLen])
		offset += idLen

		embSize := dim * 4
		if offset+embSize > len(data) {
			return nil, 0, fmt.Errorf("truncated embedding at entry %d", i)
		}
		emb := make([]float32, dim)
		for j := 0; j < dim; j++ {
			emb[j] = math.Float32frombits(binary.LittleEndian.Uint32(data[offset:]))
			offset += 4
		}

		vectors = append(vectors, vectorEntry{MemoryID: memID, Embedding: emb})
	}

	return vectors, dim, nil
}
