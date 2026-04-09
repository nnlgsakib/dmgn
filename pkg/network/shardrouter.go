package network

import (
	"context"
	"crypto/sha256"
	"fmt"

	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multihash"

	cid "github.com/ipfs/go-cid"
)

// ShardRouter handles DHT-based shard provider tracking.
type ShardRouter struct {
	dht *dht.IpfsDHT
}

// NewShardRouter creates a new ShardRouter from a DHT instance.
func NewShardRouter(d *dht.IpfsDHT) *ShardRouter {
	return &ShardRouter{dht: d}
}

// ShardCID generates a deterministic CID for a shard from memory ID and index.
func ShardCID(memoryID string, shardIndex int) cid.Cid {
	raw := fmt.Sprintf("%s:%d", memoryID, shardIndex)
	hash := sha256.Sum256([]byte(raw))
	mh, _ := multihash.Encode(hash[:], multihash.SHA2_256)
	return cid.NewCidV1(cid.Raw, mh)
}

// AnnounceShardProvider announces this peer as a provider for a shard.
func (r *ShardRouter) AnnounceShardProvider(ctx context.Context, memoryID string, shardIndex int) error {
	if r.dht == nil {
		return fmt.Errorf("DHT not initialized")
	}
	c := ShardCID(memoryID, shardIndex)
	return r.dht.Provide(ctx, c, true)
}

// FindShardProviders finds peers that hold a specific shard.
func (r *ShardRouter) FindShardProviders(ctx context.Context, memoryID string, shardIndex int, count int) ([]peer.AddrInfo, error) {
	if r.dht == nil {
		return nil, fmt.Errorf("DHT not initialized")
	}
	c := ShardCID(memoryID, shardIndex)

	providers := make([]peer.AddrInfo, 0, count)
	ch := r.dht.FindProvidersAsync(ctx, c, count)
	for p := range ch {
		if p.ID == "" {
			continue
		}
		providers = append(providers, p)
		if len(providers) >= count {
			break
		}
	}

	return providers, nil
}
