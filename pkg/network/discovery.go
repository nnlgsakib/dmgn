package network

import (
	"context"
	"fmt"
	"sync"

	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/discovery/mdns"
	"github.com/multiformats/go-multiaddr"
)

const (
	// DHTProtocolPrefix is the custom DMGN DHT protocol prefix for private namespace.
	DHTProtocolPrefix = "/dmgn/kad/1.0.0"
)

// setupDHT creates and bootstraps a Kademlia DHT with a custom DMGN protocol prefix.
func setupDHT(ctx context.Context, h host.Host, bootstrapPeers []string) (*dht.IpfsDHT, error) {
	kademliaDHT, err := dht.New(ctx, h,
		dht.Mode(dht.ModeAutoServer),
		dht.ProtocolPrefix(DHTProtocolPrefix),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create DHT: %w", err)
	}

	if err := kademliaDHT.Bootstrap(ctx); err != nil {
		return nil, fmt.Errorf("failed to bootstrap DHT: %w", err)
	}

	if len(bootstrapPeers) > 0 {
		var wg sync.WaitGroup
		for _, addrStr := range bootstrapPeers {
			ma, err := multiaddr.NewMultiaddr(addrStr)
			if err != nil {
				fmt.Printf("Warning: invalid bootstrap peer address %s: %v\n", addrStr, err)
				continue
			}

			peerInfo, err := peer.AddrInfoFromP2pAddr(ma)
			if err != nil {
				fmt.Printf("Warning: failed to parse bootstrap peer %s: %v\n", addrStr, err)
				continue
			}

			wg.Add(1)
			go func(pi peer.AddrInfo) {
				defer wg.Done()
				if err := h.Connect(ctx, pi); err != nil {
					fmt.Printf("Warning: failed to connect to bootstrap peer %s: %v\n", pi.ID, err)
				} else {
					fmt.Printf("Connected to bootstrap peer: %s\n", pi.ID)
				}
			}(*peerInfo)
		}
		wg.Wait()
	}

	return kademliaDHT, nil
}

// discoveryNotifee handles mDNS peer discovery events.
type discoveryNotifee struct {
	h host.Host
}

// HandlePeerFound connects to newly discovered peers via mDNS.
func (n *discoveryNotifee) HandlePeerFound(pi peer.AddrInfo) {
	if pi.ID == n.h.ID() {
		return
	}

	if err := n.h.Connect(context.Background(), pi); err != nil {
		fmt.Printf("Warning: failed to connect to mDNS peer %s: %v\n", pi.ID, err)
	} else {
		fmt.Printf("Connected to mDNS peer: %s\n", pi.ID)
	}
}

// setupMDNS creates an mDNS discovery service with the given service tag.
func setupMDNS(h host.Host, serviceTag string) (mdns.Service, error) {
	notifee := &discoveryNotifee{h: h}
	svc := mdns.NewMdnsService(h, serviceTag, notifee)
	if err := svc.Start(); err != nil {
		return nil, fmt.Errorf("failed to start mDNS service: %w", err)
	}
	return svc, nil
}
