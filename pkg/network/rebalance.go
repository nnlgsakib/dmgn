package network

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p/core/network"
	"github.com/multiformats/go-multiaddr"
)

// RebalanceDistributor abstracts the distributor's drain operation.
type RebalanceDistributor interface {
	DrainPending(ctx context.Context) int
	PendingCount() int
}

// RebalanceNotifiee listens for peer connect/disconnect events and triggers rebalancing.
type RebalanceNotifiee struct {
	distributor RebalanceDistributor
	host        *Host
	mu          sync.Mutex
}

// NewRebalanceNotifiee creates a new notifiee that triggers rebalancing on peer events.
func NewRebalanceNotifiee(h *Host, d RebalanceDistributor) *RebalanceNotifiee {
	return &RebalanceNotifiee{
		distributor: d,
		host:        h,
	}
}

// Register attaches the notifiee to the host's network.
func (n *RebalanceNotifiee) Register() {
	n.host.host.Network().Notify(n)
}

// Connected is called when a new peer connects. Triggers pending shard distribution.
func (n *RebalanceNotifiee) Connected(_ network.Network, conn network.Conn) {
	n.mu.Lock()
	defer n.mu.Unlock()

	if n.distributor.PendingCount() > 0 {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			distributed := n.distributor.DrainPending(ctx)
			if distributed > 0 {
				fmt.Printf("Rebalance: distributed %d pending shards to new peer %s\n",
					distributed, conn.RemotePeer().String())
			}
		}()
	}
}

// Disconnected is called when a peer disconnects. Logs for awareness.
func (n *RebalanceNotifiee) Disconnected(_ network.Network, conn network.Conn) {
	fmt.Printf("Rebalance: peer disconnected %s, shard re-replication may be needed\n",
		conn.RemotePeer().String())
}

// Listen is required by the Notifiee interface.
func (n *RebalanceNotifiee) Listen(network.Network, multiaddr.Multiaddr) {}

// ListenClose is required by the Notifiee interface.
func (n *RebalanceNotifiee) ListenClose(network.Network, multiaddr.Multiaddr) {}

// ShardAuditor periodically checks shard distribution health.
type ShardAuditor struct {
	distributor RebalanceDistributor
	interval    time.Duration
	cancel      context.CancelFunc
	done        chan struct{}
}

// NewShardAuditor creates a new periodic shard auditor.
func NewShardAuditor(d RebalanceDistributor, interval time.Duration) *ShardAuditor {
	return &ShardAuditor{
		distributor: d,
		interval:    interval,
		done:        make(chan struct{}),
	}
}

// Start begins periodic shard auditing in a goroutine.
func (a *ShardAuditor) Start(ctx context.Context) {
	if a.distributor == nil {
		close(a.done)
		return
	}
	ctx, a.cancel = context.WithCancel(ctx)
	go func() {
		defer close(a.done)
		defer func() {
			if r := recover(); r != nil {
				fmt.Printf("ShardAuditor recovered from panic: %v\n", r)
			}
		}()
		ticker := time.NewTicker(a.interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				distributed := a.distributor.DrainPending(ctx)
				if distributed > 0 {
					fmt.Printf("Audit: distributed %d pending shards\n", distributed)
				}
			}
		}
	}()
}

// Stop cancels the periodic audit and waits for it to finish.
func (a *ShardAuditor) Stop() {
	if a.cancel != nil {
		a.cancel()
		<-a.done
	}
}
