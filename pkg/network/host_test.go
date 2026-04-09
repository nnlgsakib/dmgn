package network

import (
	"context"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/nnlgsakib/dmgn/pkg/identity"
)

func createTestIdentity(t *testing.T) *identity.Identity {
	t.Helper()
	dir := t.TempDir()
	id, err := identity.Generate("test-passphrase", dir)
	if err != nil {
		t.Fatalf("failed to generate identity: %v", err)
	}
	return id
}

func createTestHost(t *testing.T, privKey interface{}) *Host {
	t.Helper()

	id := createTestIdentity(t)
	key, err := DeriveLibp2pKey(id)
	if err != nil {
		t.Fatalf("failed to derive libp2p key: %v", err)
	}

	h, err := NewHost(HostConfig{
		ListenAddrs:  []string{"/ip4/127.0.0.1/tcp/0"},
		MDNSService:  "",
		MaxPeersLow:  5,
		MaxPeersHigh: 10,
		PrivateKey:   key,
	})
	if err != nil {
		t.Fatalf("failed to create host: %v", err)
	}
	return h
}

func TestDeriveLibp2pKey(t *testing.T) {
	id1 := createTestIdentity(t)
	id2 := createTestIdentity(t)

	// Same identity produces same key (deterministic)
	key1a, err := DeriveLibp2pKey(id1)
	if err != nil {
		t.Fatalf("DeriveLibp2pKey failed: %v", err)
	}
	key1b, err := DeriveLibp2pKey(id1)
	if err != nil {
		t.Fatalf("DeriveLibp2pKey failed: %v", err)
	}

	pid1a, _ := peer.IDFromPrivateKey(key1a)
	pid1b, _ := peer.IDFromPrivateKey(key1b)
	if pid1a != pid1b {
		t.Errorf("same identity should produce same peer ID, got %s vs %s", pid1a, pid1b)
	}

	// Different identities produce different keys
	key2, err := DeriveLibp2pKey(id2)
	if err != nil {
		t.Fatalf("DeriveLibp2pKey failed: %v", err)
	}

	pid2, _ := peer.IDFromPrivateKey(key2)
	if pid1a == pid2 {
		t.Error("different identities should produce different peer IDs")
	}
}

func TestNewHostAndStop(t *testing.T) {
	id := createTestIdentity(t)
	key, err := DeriveLibp2pKey(id)
	if err != nil {
		t.Fatalf("DeriveLibp2pKey failed: %v", err)
	}

	h, err := NewHost(HostConfig{
		ListenAddrs:  []string{"/ip4/127.0.0.1/tcp/0"},
		MDNSService:  "",
		MaxPeersLow:  5,
		MaxPeersHigh: 10,
		PrivateKey:   key,
	})
	if err != nil {
		t.Fatalf("NewHost failed: %v", err)
	}

	if h.ID() == "" {
		t.Error("host should have a non-empty peer ID")
	}

	addrs := h.Addrs()
	if len(addrs) == 0 {
		t.Error("host should have at least one listen address")
	}

	if err := h.Stop(); err != nil {
		t.Errorf("Stop failed: %v", err)
	}
}

func TestPeerCount(t *testing.T) {
	id := createTestIdentity(t)
	key, err := DeriveLibp2pKey(id)
	if err != nil {
		t.Fatalf("DeriveLibp2pKey failed: %v", err)
	}

	h, err := NewHost(HostConfig{
		ListenAddrs:  []string{"/ip4/127.0.0.1/tcp/0"},
		MDNSService:  "",
		MaxPeersLow:  5,
		MaxPeersHigh: 10,
		PrivateKey:   key,
	})
	if err != nil {
		t.Fatalf("NewHost failed: %v", err)
	}
	defer h.Stop()

	if count := h.PeerCount(); count != 0 {
		t.Errorf("expected 0 peers, got %d", count)
	}
}

func TestNetworkStats(t *testing.T) {
	id := createTestIdentity(t)
	key, err := DeriveLibp2pKey(id)
	if err != nil {
		t.Fatalf("DeriveLibp2pKey failed: %v", err)
	}

	h, err := NewHost(HostConfig{
		ListenAddrs:  []string{"/ip4/127.0.0.1/tcp/0"},
		MDNSService:  "",
		MaxPeersLow:  5,
		MaxPeersHigh: 10,
		PrivateKey:   key,
	})
	if err != nil {
		t.Fatalf("NewHost failed: %v", err)
	}
	defer h.Stop()

	stats := h.NetworkStats()

	if _, ok := stats["peer_id"]; !ok {
		t.Error("stats should contain peer_id")
	}
	if _, ok := stats["listen_addrs"]; !ok {
		t.Error("stats should contain listen_addrs")
	}
	if _, ok := stats["connected_peers"]; !ok {
		t.Error("stats should contain connected_peers")
	}
	if stats["connected_peers"].(int) != 0 {
		t.Errorf("expected 0 connected peers, got %v", stats["connected_peers"])
	}
	if stats["dht_mode"].(string) != "disabled" {
		t.Errorf("expected dht_mode disabled without Start(), got %v", stats["dht_mode"])
	}
}

func TestTwoHostsConnect(t *testing.T) {
	id1 := createTestIdentity(t)
	key1, _ := DeriveLibp2pKey(id1)
	id2 := createTestIdentity(t)
	key2, _ := DeriveLibp2pKey(id2)

	h1, err := NewHost(HostConfig{
		ListenAddrs:  []string{"/ip4/127.0.0.1/tcp/0"},
		MDNSService:  "",
		MaxPeersLow:  5,
		MaxPeersHigh: 10,
		PrivateKey:   key1,
	})
	if err != nil {
		t.Fatalf("NewHost h1 failed: %v", err)
	}
	defer h1.Stop()

	h2, err := NewHost(HostConfig{
		ListenAddrs:  []string{"/ip4/127.0.0.1/tcp/0"},
		MDNSService:  "",
		MaxPeersLow:  5,
		MaxPeersHigh: 10,
		PrivateKey:   key2,
	})
	if err != nil {
		t.Fatalf("NewHost h2 failed: %v", err)
	}
	defer h2.Stop()

	// Connect h2 to h1
	h1Addrs := h1.Addrs()
	h1Info := peer.AddrInfo{
		ID:    h1.ID(),
		Addrs: h1Addrs,
	}

	if err := h2.LibP2PHost().Connect(context.Background(), h1Info); err != nil {
		t.Fatalf("failed to connect h2 to h1: %v", err)
	}

	// Give connection time to establish
	time.Sleep(200 * time.Millisecond)

	if h1.PeerCount() != 1 {
		t.Errorf("h1 expected 1 peer, got %d", h1.PeerCount())
	}
	if h2.PeerCount() != 1 {
		t.Errorf("h2 expected 1 peer, got %d", h2.PeerCount())
	}

	peers1 := h1.ConnectedPeers()
	if len(peers1) != 1 {
		t.Errorf("h1 ConnectedPeers expected 1, got %d", len(peers1))
	} else if peers1[0].ID != h2.ID().String() {
		t.Errorf("h1 peer ID mismatch: got %s, want %s", peers1[0].ID, h2.ID().String())
	}
}
