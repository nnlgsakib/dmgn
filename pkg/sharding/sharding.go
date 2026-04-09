package sharding

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/nnlgsakib/dmgn/pkg/memory"
)

// ShardConfig holds configuration for memory sharding.
type ShardConfig struct {
	Threshold   int // k: minimum shares to reconstruct (default 3)
	TotalShards int // n: total shares to create (default 5)
}

// DefaultShardConfig returns the default shard configuration.
func DefaultShardConfig() ShardConfig {
	return ShardConfig{
		Threshold:   3,
		TotalShards: 5,
	}
}

// Shard represents a single shard of a memory's encrypted payload.
type Shard struct {
	MemoryID    string `json:"memory_id"`
	ShardIndex  int    `json:"shard_index"`
	TotalShards int    `json:"total_shards"`
	Threshold   int    `json:"threshold"`
	Data        []byte `json:"data"`
	Checksum    string `json:"checksum"`
	OwnerPeerID string `json:"owner_peer_id,omitempty"`
	ReceivedAt  int64  `json:"received_at,omitempty"`
}

// ShardMemory splits a memory's encrypted payload into shards using Shamir's Secret Sharing.
func ShardMemory(mem *memory.Memory, cfg ShardConfig) ([]Shard, error) {
	if mem == nil {
		return nil, fmt.Errorf("memory must not be nil")
	}
	if len(mem.EncryptedPayload) == 0 {
		return nil, fmt.Errorf("memory has no encrypted payload")
	}
	if cfg.TotalShards < cfg.Threshold {
		return nil, fmt.Errorf("total_shards (%d) must be >= threshold (%d)", cfg.TotalShards, cfg.Threshold)
	}

	shares, err := Split(mem.EncryptedPayload, cfg.TotalShards, cfg.Threshold)
	if err != nil {
		return nil, fmt.Errorf("failed to split payload: %w", err)
	}

	shards := make([]Shard, len(shares))
	for i, share := range shares {
		checksum := sha256.Sum256(share)
		shards[i] = Shard{
			MemoryID:    mem.ID,
			ShardIndex:  i,
			TotalShards: cfg.TotalShards,
			Threshold:   cfg.Threshold,
			Data:        share,
			Checksum:    hex.EncodeToString(checksum[:]),
		}
	}

	return shards, nil
}

// ReconstructPayload reconstructs the original encrypted payload from k or more shards.
func ReconstructPayload(shards []Shard) ([]byte, error) {
	if len(shards) == 0 {
		return nil, fmt.Errorf("no shards provided")
	}

	threshold := shards[0].Threshold
	if len(shards) < threshold {
		return nil, fmt.Errorf("need at least %d shards to reconstruct, got %d", threshold, len(shards))
	}

	shares := make([][]byte, len(shards))
	for i, s := range shards {
		shares[i] = s.Data
	}

	return Combine(shares)
}

// VerifyShard checks a shard's checksum integrity.
func VerifyShard(s Shard) bool {
	checksum := sha256.Sum256(s.Data)
	return hex.EncodeToString(checksum[:]) == s.Checksum
}
