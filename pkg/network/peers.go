package network

import (
	"github.com/libp2p/go-libp2p/core/network"
)

// PeerInfo holds information about a connected peer.
type PeerInfo struct {
	ID        string   `json:"id"`
	Addrs     []string `json:"addrs"`
	Connected bool     `json:"connected"`
	Latency   string   `json:"latency,omitempty"`
}

// ConnectedPeers returns a list of currently connected peers with their addresses.
func (h *Host) ConnectedPeers() []PeerInfo {
	peers := h.host.Network().Peers()
	result := make([]PeerInfo, 0, len(peers))

	for _, p := range peers {
		connectedness := h.host.Network().Connectedness(p)
		if connectedness != network.Connected {
			continue
		}

		addrs := h.host.Peerstore().Addrs(p)
		addrStrs := make([]string, 0, len(addrs))
		for _, a := range addrs {
			addrStrs = append(addrStrs, a.String())
		}

		latency := h.host.Peerstore().LatencyEWMA(p)
		latStr := ""
		if latency > 0 {
			latStr = latency.String()
		}

		result = append(result, PeerInfo{
			ID:        p.String(),
			Addrs:     addrStrs,
			Connected: true,
			Latency:   latStr,
		})
	}

	return result
}

// PeerCount returns the number of currently connected peers.
func (h *Host) PeerCount() int {
	count := 0
	for _, p := range h.host.Network().Peers() {
		if h.host.Network().Connectedness(p) == network.Connected {
			count++
		}
	}
	return count
}

// NetworkStats returns a map of network statistics.
func (h *Host) NetworkStats() map[string]interface{} {
	addrs := h.host.Addrs()
	addrStrs := make([]string, 0, len(addrs))
	for _, a := range addrs {
		addrStrs = append(addrStrs, a.String())
	}

	stats := map[string]interface{}{
		"peer_id":         h.host.ID().String(),
		"listen_addrs":    addrStrs,
		"connected_peers": h.PeerCount(),
	}

	if h.dht != nil {
		stats["dht_mode"] = "active"
	} else {
		stats["dht_mode"] = "disabled"
	}

	return stats
}
