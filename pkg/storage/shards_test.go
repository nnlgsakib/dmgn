package storage

import (
	"testing"

	"github.com/nnlgsakib/dmgn/pkg/sharding"
)

func TestSaveGetShard(t *testing.T) {
	dir := t.TempDir()
	store, err := New(Options{DataDir: dir})
	if err != nil {
		t.Fatalf("New store failed: %v", err)
	}
	defer store.Close()

	shard := &sharding.Shard{
		MemoryID:    "mem001",
		ShardIndex:  0,
		TotalShards: 5,
		Threshold:   3,
		Data:        []byte("shard-data-bytes"),
		Checksum:    "abc123checksum",
	}

	if err := store.SaveShard(shard); err != nil {
		t.Fatalf("SaveShard failed: %v", err)
	}

	got, err := store.GetShard("mem001", 0)
	if err != nil {
		t.Fatalf("GetShard failed: %v", err)
	}

	if got.MemoryID != "mem001" {
		t.Errorf("expected memory_id mem001, got %s", got.MemoryID)
	}
	if got.ShardIndex != 0 {
		t.Errorf("expected shard_index 0, got %d", got.ShardIndex)
	}
	if string(got.Data) != "shard-data-bytes" {
		t.Errorf("data mismatch")
	}
}

func TestGetShardsForMemory(t *testing.T) {
	dir := t.TempDir()
	store, err := New(Options{DataDir: dir})
	if err != nil {
		t.Fatalf("New store failed: %v", err)
	}
	defer store.Close()

	for i := 0; i < 5; i++ {
		shard := &sharding.Shard{
			MemoryID:    "mem002",
			ShardIndex:  i,
			TotalShards: 5,
			Threshold:   3,
			Data:        []byte("data"),
			Checksum:    "chk",
		}
		if err := store.SaveShard(shard); err != nil {
			t.Fatalf("SaveShard %d failed: %v", i, err)
		}
	}

	// Also save a shard for a different memory
	other := &sharding.Shard{
		MemoryID:    "mem003",
		ShardIndex:  0,
		TotalShards: 5,
		Threshold:   3,
		Data:        []byte("other"),
		Checksum:    "chk2",
	}
	if err := store.SaveShard(other); err != nil {
		t.Fatalf("SaveShard other failed: %v", err)
	}

	shards, err := store.GetShardsForMemory("mem002")
	if err != nil {
		t.Fatalf("GetShardsForMemory failed: %v", err)
	}

	if len(shards) != 5 {
		t.Errorf("expected 5 shards for mem002, got %d", len(shards))
	}
}

func TestDeleteShard(t *testing.T) {
	dir := t.TempDir()
	store, err := New(Options{DataDir: dir})
	if err != nil {
		t.Fatalf("New store failed: %v", err)
	}
	defer store.Close()

	shard := &sharding.Shard{
		MemoryID:    "mem004",
		ShardIndex:  0,
		TotalShards: 5,
		Threshold:   3,
		Data:        []byte("delete-me"),
		Checksum:    "chk",
	}
	if err := store.SaveShard(shard); err != nil {
		t.Fatalf("SaveShard failed: %v", err)
	}

	if err := store.DeleteShard("mem004", 0); err != nil {
		t.Fatalf("DeleteShard failed: %v", err)
	}

	_, err = store.GetShard("mem004", 0)
	if err == nil {
		t.Error("expected error after delete, got nil")
	}
}

func TestShardStats(t *testing.T) {
	dir := t.TempDir()
	store, err := New(Options{DataDir: dir})
	if err != nil {
		t.Fatalf("New store failed: %v", err)
	}
	defer store.Close()

	for i := 0; i < 3; i++ {
		shard := &sharding.Shard{
			MemoryID:    "mem005",
			ShardIndex:  i,
			TotalShards: 5,
			Threshold:   3,
			Data:        []byte("stats-data"),
			Checksum:    "chk",
		}
		if err := store.SaveShard(shard); err != nil {
			t.Fatalf("SaveShard %d failed: %v", i, err)
		}
	}

	stats, err := store.GetShardStats()
	if err != nil {
		t.Fatalf("GetShardStats failed: %v", err)
	}

	if stats["shard_count"] != 3 {
		t.Errorf("expected shard_count 3, got %d", stats["shard_count"])
	}
}
