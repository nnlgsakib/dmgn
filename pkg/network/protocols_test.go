package network

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"

	dmgnpb "github.com/nnlgsakib/dmgn/proto/dmgn/v1"
	"github.com/nnlgsakib/dmgn/pkg/sharding"
)

// mockStorage implements StorageBackend for testing.
type mockStorage struct {
	shards map[string]*sharding.Shard
}

func newMockStorage() *mockStorage {
	return &mockStorage{shards: make(map[string]*sharding.Shard)}
}

func (m *mockStorage) shardKey(memoryID string, shardIndex int) string {
	return fmt.Sprintf("%s:%d", memoryID, shardIndex)
}

func (m *mockStorage) SaveShard(shard *sharding.Shard) error {
	key := m.shardKey(shard.MemoryID, shard.ShardIndex)
	m.shards[key] = shard
	return nil
}

func (m *mockStorage) GetShard(memoryID string, shardIndex int) (*sharding.Shard, error) {
	key := m.shardKey(memoryID, shardIndex)
	s, ok := m.shards[key]
	if !ok {
		return nil, fmt.Errorf("shard not found")
	}
	return s, nil
}

func connectHosts(t *testing.T, a, b *Host) {
	t.Helper()
	bInfo := peer.AddrInfo{
		ID:    b.host.ID(),
		Addrs: b.host.Addrs(),
	}
	if err := a.host.Connect(context.Background(), bInfo); err != nil {
		t.Fatalf("failed to connect hosts: %v", err)
	}
	time.Sleep(100 * time.Millisecond)
}

func TestStoreProtocolRoundTrip(t *testing.T) {
	hostA := createTestHost(t, nil)
	defer hostA.Stop()
	hostB := createTestHost(t, nil)
	defer hostB.Stop()

	storeB := newMockStorage()
	hostB.RegisterStoreHandler(storeB)

	connectHosts(t, hostA, hostB)

	data := []byte("test-shard-data-for-protocol")
	checksum := sha256.Sum256(data)
	shard := &sharding.Shard{
		MemoryID:    "mem001",
		ShardIndex:  0,
		TotalShards: 5,
		Threshold:   3,
		Data:        data,
		Checksum:    hex.EncodeToString(checksum[:]),
	}

	ctx := context.Background()
	err := hostA.SendShard(ctx, hostB.ID(), shard)
	if err != nil {
		t.Fatalf("SendShard failed: %v", err)
	}

	// Verify shard was stored on host B
	stored, err := storeB.GetShard("mem001", 0)
	if err != nil {
		t.Fatalf("shard not found on host B: %v", err)
	}
	if string(stored.Data) != string(data) {
		t.Error("stored shard data mismatch")
	}
	if stored.MemoryID != "mem001" {
		t.Errorf("expected memory_id mem001, got %s", stored.MemoryID)
	}
}

func TestFetchProtocolRoundTrip(t *testing.T) {
	hostA := createTestHost(t, nil)
	defer hostA.Stop()
	hostB := createTestHost(t, nil)
	defer hostB.Stop()

	data := []byte("fetch-this-shard-data")
	checksum := sha256.Sum256(data)

	storeB := newMockStorage()
	storeB.SaveShard(&sharding.Shard{
		MemoryID:    "mem002",
		ShardIndex:  1,
		TotalShards: 5,
		Threshold:   3,
		Data:        data,
		Checksum:    hex.EncodeToString(checksum[:]),
	})
	hostB.RegisterFetchHandler(storeB)

	connectHosts(t, hostA, hostB)

	ctx := context.Background()
	fetched, err := hostA.FetchShard(ctx, hostB.ID(), "mem002", 1)
	if err != nil {
		t.Fatalf("FetchShard failed: %v", err)
	}

	if string(fetched.Data) != string(data) {
		t.Error("fetched shard data mismatch")
	}
	if fetched.ShardIndex != 1 {
		t.Errorf("expected shard_index 1, got %d", fetched.ShardIndex)
	}
}

func TestStoreToPeerWithoutHandler(t *testing.T) {
	hostA := createTestHost(t, nil)
	defer hostA.Stop()
	hostB := createTestHost(t, nil)
	defer hostB.Stop()

	// hostB has no store handler registered
	connectHosts(t, hostA, hostB)

	data := []byte("some data")
	checksum := sha256.Sum256(data)
	shard := &sharding.Shard{
		MemoryID:    "mem003",
		ShardIndex:  0,
		TotalShards: 5,
		Threshold:   3,
		Data:        data,
		Checksum:    hex.EncodeToString(checksum[:]),
	}

	ctx := context.Background()
	err := hostA.SendShard(ctx, hostB.ID(), shard)
	if err == nil {
		t.Error("expected error when peer has no store handler")
	}
}

func TestFetchNonexistentShard(t *testing.T) {
	hostA := createTestHost(t, nil)
	defer hostA.Stop()
	hostB := createTestHost(t, nil)
	defer hostB.Stop()

	storeB := newMockStorage()
	hostB.RegisterFetchHandler(storeB)

	connectHosts(t, hostA, hostB)

	ctx := context.Background()
	_, err := hostA.FetchShard(ctx, hostB.ID(), "nonexistent", 0)
	if err == nil {
		t.Error("expected error for nonexistent shard")
	}
}

func TestProtoFrameRoundtrip(t *testing.T) {
	original := &dmgnpb.StoreRequest{
		MemoryId:    "mem-roundtrip-test",
		ShardIndex:  3,
		TotalShards: 7,
		Threshold:   4,
		Checksum:    "abcdef0123456789",
		DataLen:     1024,
	}

	trailingData := []byte("trailing-shard-bytes")

	// Write
	var buf bytes.Buffer
	if err := writeProtoFrame(&buf, original, trailingData); err != nil {
		t.Fatalf("writeProtoFrame: %v", err)
	}

	// Read
	decoded := &dmgnpb.StoreRequest{}
	data, err := readProtoFrame(&buf, decoded, len(trailingData))
	if err != nil {
		t.Fatalf("readProtoFrame: %v", err)
	}

	if decoded.MemoryId != original.MemoryId {
		t.Errorf("MemoryId: got %q, want %q", decoded.MemoryId, original.MemoryId)
	}
	if decoded.ShardIndex != original.ShardIndex {
		t.Errorf("ShardIndex: got %d, want %d", decoded.ShardIndex, original.ShardIndex)
	}
	if decoded.TotalShards != original.TotalShards {
		t.Errorf("TotalShards: got %d, want %d", decoded.TotalShards, original.TotalShards)
	}
	if decoded.DataLen != original.DataLen {
		t.Errorf("DataLen: got %d, want %d", decoded.DataLen, original.DataLen)
	}
	if string(data) != string(trailingData) {
		t.Errorf("trailing data: got %q, want %q", data, trailingData)
	}
}

func BenchmarkProtoFrameStoreRequest(b *testing.B) {
	req := &dmgnpb.StoreRequest{
		MemoryId:    "mem-bench-001",
		ShardIndex:  0,
		TotalShards: 5,
		Threshold:   3,
		Checksum:    "cafebabe12345678cafebabe12345678cafebabe12345678cafebabe12345678",
		DataLen:     65536,
	}
	data := make([]byte, 128)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var buf bytes.Buffer
		writeProtoFrame(&buf, req, data)
		decoded := &dmgnpb.StoreRequest{}
		readProtoFrame(&buf, decoded, len(data))
	}
}
