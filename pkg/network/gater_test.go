package network

import (
	"testing"

	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
)

func TestReputationGater_BlockedPeer(t *testing.T) {
	blocked := "12D3KooWDpJ7As7BWAwRMfu1VU2WCqNjvq387JEYKDBj4kx6nXTN"
	gater := NewReputationGater(nil, []string{blocked}, nil, 0.2)

	pid, err := peer.Decode(blocked)
	if err != nil {
		t.Fatalf("failed to decode peer ID: %v", err)
	}

	if gater.InterceptPeerDial(pid) {
		t.Error("blocked peer should be rejected at InterceptPeerDial")
	}
	if gater.InterceptSecured(network.DirInbound, pid, nil) {
		t.Error("blocked peer should be rejected at InterceptSecured")
	}
}

func TestReputationGater_AllowlistMode(t *testing.T) {
	allowed := "12D3KooWDpJ7As7BWAwRMfu1VU2WCqNjvq387JEYKDBj4kx6nXTN"
	other := "12D3KooWRby1HHKZAG4V57obTk6aHmFfBTVrB2GYmGXS6k8bNuf8"
	gater := NewReputationGater(nil, nil, []string{allowed}, 0.2)

	pidAllowed, err := peer.Decode(allowed)
	if err != nil {
		t.Fatalf("failed to decode allowed peer: %v", err)
	}
	pidOther, err := peer.Decode(other)
	if err != nil {
		t.Fatalf("failed to decode other peer: %v", err)
	}

	if !gater.InterceptSecured(network.DirInbound, pidAllowed, nil) {
		t.Error("allowed peer should pass InterceptSecured")
	}
	if gater.InterceptSecured(network.DirInbound, pidOther, nil) {
		t.Error("non-allowed peer should be rejected in allowlist mode")
	}
}

func TestReputationGater_ReputationThreshold(t *testing.T) {
	rm := NewReputationManager(nil)

	// Unknown peer (default 0.5 > threshold 0.3) should pass
	pid, err := peer.Decode("12D3KooWDpJ7As7BWAwRMfu1VU2WCqNjvq387JEYKDBj4kx6nXTN")
	if err != nil {
		t.Fatalf("failed to decode peer: %v", err)
	}

	gater := NewReputationGater(rm, nil, nil, 0.3)
	if !gater.InterceptSecured(network.DirInbound, pid, nil) {
		t.Error("unknown peer with neutral reputation should pass")
	}
}

func TestReputationGater_BlockUnblock(t *testing.T) {
	gater := NewReputationGater(nil, nil, nil, 0.2)
	pid, err := peer.Decode("12D3KooWDpJ7As7BWAwRMfu1VU2WCqNjvq387JEYKDBj4kx6nXTN")
	if err != nil {
		t.Fatalf("failed to decode peer: %v", err)
	}

	if !gater.InterceptPeerDial(pid) {
		t.Error("unblocked peer should pass")
	}

	gater.BlockPeer(pid)
	if gater.InterceptPeerDial(pid) {
		t.Error("blocked peer should be rejected")
	}

	gater.UnblockPeer(pid)
	if !gater.InterceptPeerDial(pid) {
		t.Error("unblocked peer should pass again")
	}
}

func TestReputationGater_PassthroughMethods(t *testing.T) {
	gater := NewReputationGater(nil, nil, nil, 0.2)

	if !gater.InterceptAccept(nil) {
		t.Error("InterceptAccept should always return true")
	}
	pid, _ := peer.Decode("12D3KooWDpJ7As7BWAwRMfu1VU2WCqNjvq387JEYKDBj4kx6nXTN")
	if !gater.InterceptAddrDial(pid, nil) {
		t.Error("InterceptAddrDial should always return true")
	}
	allow, _ := gater.InterceptUpgraded(nil)
	if !allow {
		t.Error("InterceptUpgraded should always return true")
	}
}
