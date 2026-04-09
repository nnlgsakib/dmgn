package sharding

import (
	"bytes"
	"testing"

	"github.com/nnlgsakib/dmgn/pkg/memory"
)

func TestShardMemory(t *testing.T) {
	mem := &memory.Memory{
		ID:               "abc123",
		EncryptedPayload: []byte("encrypted-test-data-for-sharding"),
	}

	cfg := ShardConfig{Threshold: 3, TotalShards: 5}
	shards, err := ShardMemory(mem, cfg)
	if err != nil {
		t.Fatalf("ShardMemory failed: %v", err)
	}

	if len(shards) != 5 {
		t.Fatalf("expected 5 shards, got %d", len(shards))
	}

	for i, s := range shards {
		if s.MemoryID != "abc123" {
			t.Errorf("shard %d: wrong memory_id %q", i, s.MemoryID)
		}
		if s.ShardIndex != i {
			t.Errorf("shard %d: wrong index %d", i, s.ShardIndex)
		}
		if s.TotalShards != 5 {
			t.Errorf("shard %d: wrong total_shards %d", i, s.TotalShards)
		}
		if s.Threshold != 3 {
			t.Errorf("shard %d: wrong threshold %d", i, s.Threshold)
		}
		if len(s.Data) == 0 {
			t.Errorf("shard %d: empty data", i)
		}
		if s.Checksum == "" {
			t.Errorf("shard %d: empty checksum", i)
		}
	}
}

func TestReconstructPayload(t *testing.T) {
	payload := []byte("reconstruct this encrypted payload please")
	mem := &memory.Memory{
		ID:               "def456",
		EncryptedPayload: payload,
	}

	cfg := ShardConfig{Threshold: 3, TotalShards: 5}
	shards, err := ShardMemory(mem, cfg)
	if err != nil {
		t.Fatalf("ShardMemory failed: %v", err)
	}

	// Reconstruct from threshold shards
	result, err := ReconstructPayload(shards[:3])
	if err != nil {
		t.Fatalf("ReconstructPayload failed: %v", err)
	}

	if !bytes.Equal(payload, result) {
		t.Error("reconstructed payload does not match original")
	}
}

func TestReconstructInsufficientShards(t *testing.T) {
	mem := &memory.Memory{
		ID:               "ghi789",
		EncryptedPayload: []byte("need enough shards"),
	}

	cfg := ShardConfig{Threshold: 3, TotalShards: 5}
	shards, err := ShardMemory(mem, cfg)
	if err != nil {
		t.Fatalf("ShardMemory failed: %v", err)
	}

	_, err = ReconstructPayload(shards[:2])
	if err == nil {
		t.Error("expected error with insufficient shards")
	}
}

func TestShardChecksum(t *testing.T) {
	mem := &memory.Memory{
		ID:               "chk001",
		EncryptedPayload: []byte("checksum verification data"),
	}

	cfg := ShardConfig{Threshold: 3, TotalShards: 5}
	shards, err := ShardMemory(mem, cfg)
	if err != nil {
		t.Fatalf("ShardMemory failed: %v", err)
	}

	for _, s := range shards {
		if !VerifyShard(s) {
			t.Errorf("shard %d failed checksum verification", s.ShardIndex)
		}
	}

	// Corrupt a shard
	corrupted := shards[0]
	corrupted.Data[0] ^= 0xFF
	if VerifyShard(corrupted) {
		t.Error("corrupted shard should fail checksum verification")
	}
}

func TestShardMemoryNilMemory(t *testing.T) {
	_, err := ShardMemory(nil, DefaultShardConfig())
	if err == nil {
		t.Error("expected error for nil memory")
	}
}

func TestShardMemoryEmptyPayload(t *testing.T) {
	mem := &memory.Memory{
		ID:               "empty",
		EncryptedPayload: []byte{},
	}
	_, err := ShardMemory(mem, DefaultShardConfig())
	if err == nil {
		t.Error("expected error for empty payload")
	}
}
