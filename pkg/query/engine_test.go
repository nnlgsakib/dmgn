package query

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/nnlgsakib/dmgn/pkg/memory"
	"github.com/nnlgsakib/dmgn/pkg/storage"
	"github.com/nnlgsakib/dmgn/pkg/vectorindex"
)

func setupTestStore(t *testing.T) (*storage.Store, func([]byte) ([]byte, error)) {
	t.Helper()
	dir := t.TempDir()
	store, err := storage.New(storage.Options{DataDir: dir})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })

	// Simple passthrough decrypt for testing
	decryptFn := func(ciphertext []byte) ([]byte, error) {
		return ciphertext, nil
	}

	return store, decryptFn
}

func addTestMemory(t *testing.T, store *storage.Store, content string, embedding []float32) *memory.Memory {
	t.Helper()
	// Create a memory with "encrypted" payload that is just the plaintext JSON
	plain := &memory.PlaintextMemory{
		Content:  content,
		Metadata: map[string]string{},
	}
	data, _ := json.Marshal(plain)

	// Use data as both payload (our test decryptFn returns as-is)
	mem := &memory.Memory{
		ID:               fmt.Sprintf("%x", data[:16]),
		Timestamp:        1700000000 + int64(len(content)),
		Type:             memory.TypeText,
		EncryptedPayload: data,
		Embedding:        embedding,
		Links:            []string{},
	}

	if err := store.SaveMemory(mem); err != nil {
		t.Fatal(err)
	}
	return mem
}

func TestSearchLocalTextOnly(t *testing.T) {
	store, decryptFn := setupTestStore(t)
	idx := vectorindex.NewVectorIndex(
		os.TempDir()+"/test.idx",
		func(b []byte) ([]byte, error) { return b, nil },
		func(b []byte) ([]byte, error) { return b, nil },
	)

	engine := NewQueryEngine(idx, store, decryptFn, 0.7)

	addTestMemory(t, store, "hello world", nil)
	addTestMemory(t, store, "goodbye world", nil)
	addTestMemory(t, store, "something else", nil)

	results, err := engine.SearchLocal(QueryRequest{
		TextQuery: "hello",
		Limit:     10,
	})
	if err != nil {
		t.Fatalf("SearchLocal: %v", err)
	}

	if len(results) == 0 {
		t.Fatal("expected results for 'hello'")
	}
	if results[0].Snippet != "hello world" {
		t.Errorf("expected 'hello world', got '%s'", results[0].Snippet)
	}
}

func TestSearchLocalVectorOnly(t *testing.T) {
	store, decryptFn := setupTestStore(t)
	dir := t.TempDir()
	idx := vectorindex.NewVectorIndex(
		dir+"/test.idx",
		func(b []byte) ([]byte, error) { return b, nil },
		func(b []byte) ([]byte, error) { return b, nil },
	)

	engine := NewQueryEngine(idx, store, decryptFn, 0.7)

	mem1 := addTestMemory(t, store, "neural networks", []float32{1, 0, 0})
	idx.Add(mem1.ID, []float32{1, 0, 0})

	mem2 := addTestMemory(t, store, "deep learning", []float32{0.9, 0.1, 0})
	idx.Add(mem2.ID, []float32{0.9, 0.1, 0})

	mem3 := addTestMemory(t, store, "cooking recipes", []float32{0, 0, 1})
	idx.Add(mem3.ID, []float32{0, 0, 1})

	results, err := engine.SearchLocal(QueryRequest{
		Embedding: []float32{1, 0, 0},
		Limit:     2,
	})
	if err != nil {
		t.Fatalf("SearchLocal: %v", err)
	}

	if len(results) < 2 {
		t.Fatalf("expected at least 2 results, got %d", len(results))
	}

	// First result should be neural networks (exact match) or deep learning (very close)
	if results[0].Score < 0.9 {
		t.Errorf("expected high score for first result, got %.4f", results[0].Score)
	}
	_ = mem3 // just to keep it used
}

func TestSearchLocalHybrid(t *testing.T) {
	store, decryptFn := setupTestStore(t)
	dir := t.TempDir()
	idx := vectorindex.NewVectorIndex(
		dir+"/test.idx",
		func(b []byte) ([]byte, error) { return b, nil },
		func(b []byte) ([]byte, error) { return b, nil },
	)

	engine := NewQueryEngine(idx, store, decryptFn, 0.7)

	mem1 := addTestMemory(t, store, "machine learning basics", []float32{1, 0, 0})
	idx.Add(mem1.ID, []float32{1, 0, 0})

	results, err := engine.SearchLocal(QueryRequest{
		Embedding: []float32{1, 0, 0},
		TextQuery: "machine learning",
		Limit:     10,
	})
	if err != nil {
		t.Fatalf("SearchLocal: %v", err)
	}

	if len(results) == 0 {
		t.Fatal("expected results")
	}
	// Hybrid score should be higher than either alone
	if results[0].Score < 0.5 {
		t.Errorf("expected hybrid score > 0.5, got %.4f", results[0].Score)
	}
}

func TestSearchLocalWithFilters(t *testing.T) {
	store, decryptFn := setupTestStore(t)
	idx := vectorindex.NewVectorIndex(
		os.TempDir()+"/test.idx",
		func(b []byte) ([]byte, error) { return b, nil },
		func(b []byte) ([]byte, error) { return b, nil },
	)

	engine := NewQueryEngine(idx, store, decryptFn, 0.7)

	addTestMemory(t, store, "hello world", nil)
	addTestMemory(t, store, "hello earth", nil)

	results, err := engine.SearchLocal(QueryRequest{
		TextQuery: "hello",
		Limit:     10,
		Filters: QueryFilters{
			After: 1700000020, // filter out shorter content memories
		},
	})
	if err != nil {
		t.Fatalf("SearchLocal: %v", err)
	}

	// Should filter to only "hello earth" (longer content -> higher timestamp)
	for _, r := range results {
		if r.Timestamp < 1700000020 {
			t.Errorf("filter not applied: result has timestamp %d", r.Timestamp)
		}
	}
}

func TestSearchLocalEmptyIndex(t *testing.T) {
	store, decryptFn := setupTestStore(t)
	idx := vectorindex.NewVectorIndex(
		os.TempDir()+"/test.idx",
		func(b []byte) ([]byte, error) { return b, nil },
		func(b []byte) ([]byte, error) { return b, nil },
	)

	engine := NewQueryEngine(idx, store, decryptFn, 0.7)

	addTestMemory(t, store, "hello world", nil)

	// Vector query with empty index should still work (fallback)
	results, err := engine.SearchLocal(QueryRequest{
		Embedding: []float32{1, 0, 0},
		TextQuery: "hello",
		Limit:     10,
	})
	if err != nil {
		t.Fatalf("SearchLocal: %v", err)
	}

	// Should fall back to text search
	if len(results) == 0 {
		t.Fatal("expected text fallback results")
	}
}

func TestSnippetGeneration(t *testing.T) {
	store, decryptFn := setupTestStore(t)
	idx := vectorindex.NewVectorIndex(
		os.TempDir()+"/test.idx",
		func(b []byte) ([]byte, error) { return b, nil },
		func(b []byte) ([]byte, error) { return b, nil },
	)

	engine := NewQueryEngine(idx, store, decryptFn, 0.7)

	longContent := ""
	for i := 0; i < 200; i++ {
		longContent += "a"
	}
	addTestMemory(t, store, longContent, nil)

	results, err := engine.SearchLocal(QueryRequest{
		TextQuery: longContent[:10],
		Limit:     10,
	})
	if err != nil {
		t.Fatalf("SearchLocal: %v", err)
	}

	for _, r := range results {
		if len(r.Snippet) > 100 {
			t.Errorf("snippet too long: %d chars", len(r.Snippet))
		}
	}
}
