package network

import (
	"sync"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"golang.org/x/time/rate"
)

// PeerRateLimiter enforces per-peer rate limits using token bucket algorithm.
type PeerRateLimiter struct {
	limiters map[peer.ID]*rateLimiterEntry
	mu       sync.Mutex
	limit    rate.Limit
	burst    int
}

type rateLimiterEntry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// NewPeerRateLimiter creates a rate limiter with the given per-peer rate (requests/sec) and burst.
func NewPeerRateLimiter(rps float64, burst int) *PeerRateLimiter {
	return &PeerRateLimiter{
		limiters: make(map[peer.ID]*rateLimiterEntry),
		limit:    rate.Limit(rps),
		burst:    burst,
	}
}

// Allow checks if the peer is within their rate limit.
func (rl *PeerRateLimiter) Allow(p peer.ID) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	entry, ok := rl.limiters[p]
	if !ok {
		entry = &rateLimiterEntry{
			limiter: rate.NewLimiter(rl.limit, rl.burst),
		}
		rl.limiters[p] = entry
	}
	entry.lastSeen = time.Now()
	return entry.limiter.Allow()
}

// Cleanup removes limiter entries for peers not seen since the given duration.
func (rl *PeerRateLimiter) Cleanup(maxAge time.Duration) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	cutoff := time.Now().Add(-maxAge)
	for p, entry := range rl.limiters {
		if entry.lastSeen.Before(cutoff) {
			delete(rl.limiters, p)
		}
	}
}

// Count returns the number of tracked peers.
func (rl *PeerRateLimiter) Count() int {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	return len(rl.limiters)
}
