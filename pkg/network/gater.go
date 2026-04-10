package network

import (
	"sync"

	"github.com/libp2p/go-libp2p/core/connmgr"
	"github.com/libp2p/go-libp2p/core/control"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
)

// Compile-time check that ReputationGater implements ConnectionGater.
var _ connmgr.ConnectionGater = (*ReputationGater)(nil)

// ReputationGater blocks peers based on reputation score and explicit blocklist/allowlist.
type ReputationGater struct {
	reputation *ReputationManager
	blocked    map[peer.ID]bool
	allowed    map[peer.ID]bool
	threshold  float64
	mu         sync.RWMutex
}

// NewReputationGater creates a new connection gater.
// If allowedPeers is non-empty, only those peers are accepted (allowlist mode).
// Otherwise, peers are checked against blocklist and reputation threshold.
func NewReputationGater(rm *ReputationManager, blockedPeers, allowedPeers []string, threshold float64) *ReputationGater {
	blocked := make(map[peer.ID]bool, len(blockedPeers))
	for _, p := range blockedPeers {
		if pid, err := peer.Decode(p); err == nil {
			blocked[pid] = true
		}
	}
	allowed := make(map[peer.ID]bool, len(allowedPeers))
	for _, p := range allowedPeers {
		if pid, err := peer.Decode(p); err == nil {
			allowed[pid] = true
		}
	}
	return &ReputationGater{
		reputation: rm,
		blocked:    blocked,
		allowed:    allowed,
		threshold:  threshold,
	}
}

// BlockPeer adds a peer to the blocklist at runtime.
func (g *ReputationGater) BlockPeer(p peer.ID) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.blocked[p] = true
}

// UnblockPeer removes a peer from the blocklist.
func (g *ReputationGater) UnblockPeer(p peer.ID) {
	g.mu.Lock()
	defer g.mu.Unlock()
	delete(g.blocked, p)
}

// isAllowed checks if a peer should be allowed based on blocklist/allowlist/reputation.
func (g *ReputationGater) isAllowed(p peer.ID) bool {
	g.mu.RLock()
	defer g.mu.RUnlock()

	// Explicit blocklist always wins
	if g.blocked[p] {
		return false
	}

	// Allowlist mode: only listed peers accepted
	if len(g.allowed) > 0 {
		return g.allowed[p]
	}

	// Reputation check (skip if no reputation manager)
	if g.reputation != nil {
		score := g.reputation.GetScore(p.String())
		if score < g.threshold {
			return false
		}
	}

	return true
}

func (g *ReputationGater) InterceptPeerDial(p peer.ID) bool {
	return g.isAllowed(p)
}

func (g *ReputationGater) InterceptAddrDial(_ peer.ID, _ multiaddr.Multiaddr) bool {
	return true
}

func (g *ReputationGater) InterceptAccept(_ network.ConnMultiaddrs) bool {
	return true
}

func (g *ReputationGater) InterceptSecured(_ network.Direction, p peer.ID, _ network.ConnMultiaddrs) bool {
	return g.isAllowed(p)
}

func (g *ReputationGater) InterceptUpgraded(_ network.Conn) (bool, control.DisconnectReason) {
	return true, 0
}
