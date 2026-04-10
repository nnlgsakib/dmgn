package sharding

import (
	"context"
	"fmt"
	"sync"
)

// PeerSender abstracts sending/fetching shards to/from peers.
type PeerSender interface {
	SendShardToPeer(ctx context.Context, peerID string, shard *Shard) error
	FetchShardFromPeer(ctx context.Context, peerID string, memoryID string, shardIndex int) (*Shard, error)
	ConnectedPeerIDs() []string
}

// ShardStore abstracts local shard persistence.
type ShardStore interface {
	SaveShard(shard *Shard) error
	GetShard(memoryID string, shardIndex int) (*Shard, error)
	GetShardsForMemory(memoryID string) ([]*Shard, error)
}

// ShardAnnouncer abstracts DHT provider announcements.
type ShardAnnouncer interface {
	AnnounceShardProvider(ctx context.Context, memoryID string, shardIndex int) error
}

// DistributionResult holds the outcome of distributing a memory's shards.
type DistributionResult struct {
	MemoryID    string
	TotalShards int
	Distributed int
	LocalOnly   int
	PeerMap     map[int]string // shard_index -> peer_id
}

// Distributor orchestrates sharding, distribution, and reconstruction.
type Distributor struct {
	sender    PeerSender
	store     ShardStore
	announcer ShardAnnouncer
	cfg       ShardConfig
	mu        sync.Mutex
	pending   []Shard // shards awaiting distribution
}

// NewDistributor creates a new Distributor.
func NewDistributor(sender PeerSender, store ShardStore, announcer ShardAnnouncer, cfg ShardConfig) *Distributor {
	return &Distributor{
		sender:    sender,
		store:     store,
		announcer: announcer,
		cfg:       cfg,
		pending:   make([]Shard, 0),
	}
}

// DistributeShards distributes pre-created shards to connected peers.
// Stores shards locally if insufficient peers are available.
func (d *Distributor) DistributeShards(ctx context.Context, shards []Shard) (*DistributionResult, error) {
	if len(shards) == 0 {
		return nil, fmt.Errorf("no shards to distribute")
	}

	result := &DistributionResult{
		MemoryID:    shards[0].MemoryID,
		TotalShards: len(shards),
		PeerMap:     make(map[int]string),
	}

	peers := d.sender.ConnectedPeerIDs()

	for i, shard := range shards {
		// Always save locally first
		if err := d.store.SaveShard(&shards[i]); err != nil {
			return nil, fmt.Errorf("failed to save shard %d locally: %w", i, err)
		}

		if i < len(peers) {
			err := d.sender.SendShardToPeer(ctx, peers[i], &shard)
			if err != nil {
				// Failed to send — stays local only
				result.LocalOnly++
				d.addPending(shard)
				continue
			}
			result.Distributed++
			result.PeerMap[shard.ShardIndex] = peers[i]

			// Announce as provider in DHT
			if d.announcer != nil {
				d.announcer.AnnounceShardProvider(ctx, shard.MemoryID, shard.ShardIndex)
			}
		} else {
			// No peer available for this shard
			result.LocalOnly++
			d.addPending(shard)
		}
	}

	return result, nil
}

// ReconstructFromLocal attempts to reconstruct from locally stored shards.
func (d *Distributor) ReconstructFromLocal(memoryID string) ([]byte, error) {
	shards, err := d.store.GetShardsForMemory(memoryID)
	if err != nil {
		return nil, fmt.Errorf("failed to get local shards: %w", err)
	}

	if len(shards) == 0 {
		return nil, fmt.Errorf("no local shards for memory %s", memoryID)
	}

	threshold := shards[0].Threshold
	if len(shards) < threshold {
		return nil, fmt.Errorf("only %d local shards, need %d to reconstruct", len(shards), threshold)
	}

	deref := make([]Shard, len(shards))
	for i, s := range shards {
		deref[i] = *s
	}

	return ReconstructPayload(deref)
}

// ReconstructFromPeers fetches shards from peers and reconstructs the payload.
func (d *Distributor) ReconstructFromPeers(ctx context.Context, memoryID string, totalShards, threshold int, peerMap map[int]string) ([]byte, error) {
	collected := make([]Shard, 0, threshold)

	// First try local shards
	localShards, _ := d.store.GetShardsForMemory(memoryID)
	for _, s := range localShards {
		collected = append(collected, *s)
		if len(collected) >= threshold {
			break
		}
	}

	// Fetch remaining from peers
	if len(collected) < threshold {
		for shardIdx, peerID := range peerMap {
			if len(collected) >= threshold {
				break
			}
			// Skip if we already have this shard locally
			alreadyHave := false
			for _, c := range collected {
				if c.ShardIndex == shardIdx {
					alreadyHave = true
					break
				}
			}
			if alreadyHave {
				continue
			}

			shard, err := d.sender.FetchShardFromPeer(ctx, peerID, memoryID, shardIdx)
			if err != nil {
				continue // try next peer
			}
			collected = append(collected, *shard)
		}
	}

	if len(collected) < threshold {
		return nil, fmt.Errorf("collected only %d shards, need %d", len(collected), threshold)
	}

	return ReconstructPayload(collected)
}

// DrainPending distributes any pending shards to newly available peers.
func (d *Distributor) DrainPending(ctx context.Context) int {
	if d.sender == nil {
		return 0
	}

	d.mu.Lock()
	pending := make([]Shard, len(d.pending))
	copy(pending, d.pending)
	d.pending = d.pending[:0]
	d.mu.Unlock()

	if len(pending) == 0 {
		return 0
	}

	peers := d.sender.ConnectedPeerIDs()
	distributed := 0

	for i, shard := range pending {
		if i < len(peers) {
			err := d.sender.SendShardToPeer(ctx, peers[i%len(peers)], &shard)
			if err != nil {
				d.addPending(shard) // re-queue
				continue
			}
			distributed++
			if d.announcer != nil {
				d.announcer.AnnounceShardProvider(ctx, shard.MemoryID, shard.ShardIndex)
			}
		} else {
			d.addPending(shard)
		}
	}

	return distributed
}

// PendingCount returns the number of shards awaiting distribution.
func (d *Distributor) PendingCount() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return len(d.pending)
}

func (d *Distributor) addPending(shard Shard) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.pending = append(d.pending, shard)
}
