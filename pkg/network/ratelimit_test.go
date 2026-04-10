package network

import (
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
)

func TestPeerRateLimiter_AllowBurst(t *testing.T) {
	rl := NewPeerRateLimiter(10, 5) // 10 req/s, burst 5
	pid := peer.ID("test-peer-1")

	// First 5 (burst) should be allowed
	for i := 0; i < 5; i++ {
		if !rl.Allow(pid) {
			t.Fatalf("request %d within burst should be allowed", i+1)
		}
	}

	// 6th should be rate limited
	if rl.Allow(pid) {
		t.Error("request beyond burst should be rate limited")
	}
}

func TestPeerRateLimiter_IndependentPeers(t *testing.T) {
	rl := NewPeerRateLimiter(10, 2) // 10 req/s, burst 2
	pid1 := peer.ID("peer-1")
	pid2 := peer.ID("peer-2")

	// Exhaust peer1's burst
	rl.Allow(pid1)
	rl.Allow(pid1)

	// Peer2 should still have its own burst
	if !rl.Allow(pid2) {
		t.Error("different peers should have independent rate limits")
	}
}

func TestPeerRateLimiter_Cleanup(t *testing.T) {
	rl := NewPeerRateLimiter(10, 5)
	pid := peer.ID("test-peer-cleanup")

	rl.Allow(pid)
	if rl.Count() != 1 {
		t.Fatalf("expected 1 tracked peer, got %d", rl.Count())
	}

	// Wait briefly so entry becomes older than cutoff
	time.Sleep(10 * time.Millisecond)

	// Cleanup entries older than 5ms (our entry is ~10ms old now)
	rl.Cleanup(5 * time.Millisecond)
	if rl.Count() != 0 {
		t.Errorf("expected 0 tracked peers after cleanup, got %d", rl.Count())
	}
}

func TestPeerRateLimiter_Recovery(t *testing.T) {
	rl := NewPeerRateLimiter(1000, 1) // high rate, burst 1
	pid := peer.ID("test-peer-recover")

	// Use up burst
	rl.Allow(pid)
	if rl.Allow(pid) {
		t.Skip("rate too high for meaningful test")
	}

	// Wait for token replenishment
	time.Sleep(5 * time.Millisecond)
	if !rl.Allow(pid) {
		t.Error("should recover after waiting")
	}
}
