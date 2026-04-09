package storage

import (
	"fmt"
	"testing"
	"time"

	"github.com/nnlgsakib/dmgn/pkg/memory"
)

func createTestMemory(t *testing.T, content string, ts int64) *memory.Memory {
	t.Helper()
	plain := &memory.PlaintextMemory{
		Content:  content,
		Type:     memory.TypeText,
		Metadata: map[string]string{},
	}

	// Use a no-op encrypt that just returns the JSON as-is for testing
	encryptFn := func(data []byte) ([]byte, error) {
		return data, nil
	}

	mem, err := memory.Create(plain, nil, encryptFn)
	if err != nil {
		t.Fatalf("Failed to create test memory: %v", err)
	}
	// Override timestamp for deterministic ordering
	mem.Timestamp = ts
	return mem
}

func TestEnforceRetentionUnlimited(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := New(Options{
		DataDir:      tmpDir,
		MaxRetention: 0, // unlimited
	})
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	for i := 0; i < 20; i++ {
		mem := createTestMemory(t, fmt.Sprintf("memory-%d", i), time.Now().UnixNano()+int64(i))
		if err := store.SaveMemory(mem); err != nil {
			t.Fatalf("SaveMemory failed: %v", err)
		}
	}

	stats, err := store.GetStats()
	if err != nil {
		t.Fatalf("GetStats failed: %v", err)
	}

	if stats["memory_count"] != 20 {
		t.Errorf("Expected 20 memories with unlimited retention, got %d", stats["memory_count"])
	}
}

func TestEnforceRetentionLimit(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := New(Options{
		DataDir:      tmpDir,
		MaxRetention: 5,
	})
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	for i := 0; i < 10; i++ {
		mem := createTestMemory(t, fmt.Sprintf("memory-%d", i), time.Now().UnixNano()+int64(i*1000))
		if err := store.SaveMemory(mem); err != nil {
			t.Fatalf("SaveMemory failed: %v", err)
		}
	}

	stats, err := store.GetStats()
	if err != nil {
		t.Fatalf("GetStats failed: %v", err)
	}

	if stats["memory_count"] != 5 {
		t.Errorf("Expected 5 memories with retention=5, got %d", stats["memory_count"])
	}
}

func TestEnforceRetentionKeepsNewest(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := New(Options{
		DataDir:      tmpDir,
		MaxRetention: 5,
	})
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	baseTime := time.Now().UnixNano()
	var lastFiveIDs []string

	for i := 0; i < 10; i++ {
		mem := createTestMemory(t, fmt.Sprintf("memory-%d", i), baseTime+int64(i*1000000))
		if i >= 5 {
			lastFiveIDs = append(lastFiveIDs, mem.ID)
		}
		if err := store.SaveMemory(mem); err != nil {
			t.Fatalf("SaveMemory failed: %v", err)
		}
	}

	// Verify the 5 newest are kept
	for _, id := range lastFiveIDs {
		_, err := store.GetMemory(id)
		if err != nil {
			t.Errorf("Expected newest memory %s to be kept, but got error: %v", id[:16], err)
		}
	}
}

func TestRetentionOnSave(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := New(Options{
		DataDir:      tmpDir,
		MaxRetention: 3,
	})
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	baseTime := time.Now().UnixNano()

	// Add 3 memories — should all be kept
	for i := 0; i < 3; i++ {
		mem := createTestMemory(t, fmt.Sprintf("memory-%d", i), baseTime+int64(i*1000000))
		if err := store.SaveMemory(mem); err != nil {
			t.Fatalf("SaveMemory failed: %v", err)
		}
	}

	stats, err := store.GetStats()
	if err != nil {
		t.Fatalf("GetStats failed: %v", err)
	}
	if stats["memory_count"] != 3 {
		t.Errorf("Expected 3 memories, got %d", stats["memory_count"])
	}

	// Add 4th memory — should trigger retention, leaving 3
	mem := createTestMemory(t, "memory-trigger", baseTime+int64(3*1000000))
	if err := store.SaveMemory(mem); err != nil {
		t.Fatalf("SaveMemory failed: %v", err)
	}

	stats, err = store.GetStats()
	if err != nil {
		t.Fatalf("GetStats failed: %v", err)
	}
	if stats["memory_count"] != 3 {
		t.Errorf("Expected 3 memories after retention trigger, got %d", stats["memory_count"])
	}
}
