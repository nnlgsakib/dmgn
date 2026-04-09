package vectorindex

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

// Simple encrypt/decrypt for testing (XOR with key byte)
func testEncrypt(data []byte) ([]byte, error) {
	out := make([]byte, len(data))
	for i, b := range data {
		out[i] = b ^ 0x42
	}
	return out, nil
}

func testDecrypt(data []byte) ([]byte, error) {
	return testEncrypt(data) // XOR is symmetric
}

func TestAddAndSearch(t *testing.T) {
	dir := t.TempDir()
	idx := NewVectorIndex(filepath.Join(dir, "test.idx"), testEncrypt, testDecrypt)

	if err := idx.Add("mem1", []float32{1, 0, 0}); err != nil {
		t.Fatalf("Add mem1: %v", err)
	}
	if err := idx.Add("mem2", []float32{0, 1, 0}); err != nil {
		t.Fatalf("Add mem2: %v", err)
	}
	if err := idx.Add("mem3", []float32{0.9, 0.1, 0}); err != nil {
		t.Fatalf("Add mem3: %v", err)
	}

	results := idx.Search([]float32{1, 0, 0}, 2)
	if len(results) == 0 {
		t.Fatal("expected results")
	}
	if results[0].MemoryID != "mem1" && results[0].MemoryID != "mem3" {
		t.Errorf("expected mem1 or mem3 as top result, got %s", results[0].MemoryID)
	}
}

func TestAutoDetectDimension(t *testing.T) {
	dir := t.TempDir()
	idx := NewVectorIndex(filepath.Join(dir, "test.idx"), testEncrypt, testDecrypt)

	if err := idx.Add("mem1", []float32{1, 0, 0}); err != nil {
		t.Fatalf("first Add: %v", err)
	}

	if dim := idx.Dimension(); dim != 3 {
		t.Errorf("expected dimension 3, got %d", dim)
	}

	err := idx.Add("mem2", []float32{1, 0})
	if err == nil {
		t.Error("expected error for dimension mismatch")
	}
}

func TestSaveLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.idx")
	idx := NewVectorIndex(path, testEncrypt, testDecrypt)

	idx.Add("mem1", []float32{1, 0, 0})
	idx.Add("mem2", []float32{0, 1, 0})

	if err := idx.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("index file not created")
	}

	// Load into new index
	idx2 := NewVectorIndex(path, testEncrypt, testDecrypt)
	if err := idx2.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}

	if idx2.Count() != 2 {
		t.Errorf("expected 2 vectors after load, got %d", idx2.Count())
	}

	results := idx2.Search([]float32{1, 0, 0}, 1)
	if len(results) == 0 {
		t.Fatal("expected search results after load")
	}
	if results[0].MemoryID != "mem1" {
		t.Errorf("expected mem1, got %s", results[0].MemoryID)
	}
}

func TestEmptyIndex(t *testing.T) {
	dir := t.TempDir()
	idx := NewVectorIndex(filepath.Join(dir, "test.idx"), testEncrypt, testDecrypt)

	results := idx.Search([]float32{1, 0, 0}, 5)
	if len(results) != 0 {
		t.Errorf("expected empty results, got %d", len(results))
	}
}

func TestRemove(t *testing.T) {
	dir := t.TempDir()
	idx := NewVectorIndex(filepath.Join(dir, "test.idx"), testEncrypt, testDecrypt)

	idx.Add("mem1", []float32{1, 0, 0})
	idx.Add("mem2", []float32{0, 1, 0})

	idx.Remove("mem1")

	results := idx.Search([]float32{1, 0, 0}, 5)
	for _, r := range results {
		if r.MemoryID == "mem1" {
			t.Error("mem1 should have been removed")
		}
	}
}

func TestEmptyEmbeddingRejected(t *testing.T) {
	dir := t.TempDir()
	idx := NewVectorIndex(filepath.Join(dir, "test.idx"), testEncrypt, testDecrypt)

	err := idx.Add("mem1", []float32{})
	if err == nil {
		t.Error("expected error for empty embedding")
	}
}

func TestLoadNonexistentFile(t *testing.T) {
	dir := t.TempDir()
	idx := NewVectorIndex(filepath.Join(dir, "nonexistent.idx"), testEncrypt, testDecrypt)

	if err := idx.Load(); err != nil {
		t.Errorf("Load on non-existent file should return nil, got: %v", err)
	}
	if idx.Count() != 0 {
		t.Errorf("expected empty index")
	}
}

func TestConcurrentAccess(t *testing.T) {
	dir := t.TempDir()
	idx := NewVectorIndex(filepath.Join(dir, "test.idx"), testEncrypt, testDecrypt)

	// Pre-populate
	for i := 0; i < 10; i++ {
		vec := make([]float32, 8)
		vec[i%8] = 1.0
		idx.Add(fmt.Sprintf("mem%d", i), vec)
	}

	var wg sync.WaitGroup
	// Concurrent searches
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			idx.Search([]float32{1, 0, 0, 0, 0, 0, 0, 0}, 3)
		}()
	}
	// Concurrent adds
	for i := 10; i < 20; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			vec := make([]float32, 8)
			vec[id%8] = float32(id)
			idx.Add(fmt.Sprintf("mem%d", id), vec)
		}(i)
	}
	wg.Wait()
}

// ensure fmt is used
var _ = fmt.Sprintf
