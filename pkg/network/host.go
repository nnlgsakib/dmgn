package network

import (
	"context"
	"crypto/ed25519"
	"fmt"
	"time"

	"github.com/libp2p/go-libp2p"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	libnet "github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/discovery/mdns"
	"github.com/libp2p/go-libp2p/p2p/net/connmgr"
	"github.com/multiformats/go-multiaddr"

	"github.com/nnlgsakib/dmgn/pkg/identity"
)

// HostConfig holds configuration for creating a libp2p host.
type HostConfig struct {
	ListenAddrs    []string
	BootstrapPeers []string
	MDNSService    string
	MaxPeersLow    int
	MaxPeersHigh   int
	PrivateKey     crypto.PrivKey
}

// Host wraps a libp2p host with DHT and mDNS discovery.
type Host struct {
	host        host.Host
	dht         *dht.IpfsDHT
	mdnsService mdns.Service
	ctx         context.Context
	cancel      context.CancelFunc
	cfg         HostConfig
}

// DeriveLibp2pKey derives a libp2p ed25519 private key from a DMGN identity
// using HKDF with purpose "libp2p-host" for domain separation.
func DeriveLibp2pKey(id *identity.Identity) (crypto.PrivKey, error) {
	seed, err := id.DeriveKey("libp2p-host", 32)
	if err != nil {
		return nil, fmt.Errorf("failed to derive libp2p key seed: %w", err)
	}

	stdKey := ed25519.NewKeyFromSeed(seed)

	privKey, err := crypto.UnmarshalEd25519PrivateKey(stdKey)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal ed25519 key for libp2p: %w", err)
	}

	return privKey, nil
}

// NewHost creates a new libp2p host with the given configuration.
func NewHost(cfg HostConfig) (*Host, error) {
	ctx, cancel := context.WithCancel(context.Background())

	cm, err := connmgr.NewConnManager(
		cfg.MaxPeersLow,
		cfg.MaxPeersHigh,
		connmgr.WithGracePeriod(time.Minute),
	)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create connection manager: %w", err)
	}

	opts := []libp2p.Option{
		libp2p.Identity(cfg.PrivateKey),
		libp2p.ListenAddrStrings(cfg.ListenAddrs...),
		libp2p.ConnectionManager(cm),
		libp2p.NATPortMap(),
		libp2p.EnableRelay(),
		libp2p.DefaultTransports,
		libp2p.DefaultSecurity,
		libp2p.DefaultMuxers,
	}

	h, err := libp2p.New(opts...)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create libp2p host: %w", err)
	}

	return &Host{
		host:   h,
		ctx:    ctx,
		cancel: cancel,
		cfg:    cfg,
	}, nil
}

// Start initializes DHT bootstrap and mDNS discovery.
func (h *Host) Start() error {
	kademliaDHT, err := setupDHT(h.ctx, h.host, h.cfg.BootstrapPeers)
	if err != nil {
		return fmt.Errorf("failed to setup DHT: %w", err)
	}
	h.dht = kademliaDHT

	if h.cfg.MDNSService != "" {
		svc, err := setupMDNS(h.host, h.cfg.MDNSService)
		if err != nil {
			fmt.Printf("Warning: mDNS setup failed: %v\n", err)
		} else {
			h.mdnsService = svc
		}
	}

	return nil
}

// Stop gracefully shuts down the host and all discovery services.
func (h *Host) Stop() error {
	h.cancel()

	if h.mdnsService != nil {
		h.mdnsService.Close()
	}

	if h.dht != nil {
		if err := h.dht.Close(); err != nil {
			fmt.Printf("Warning: DHT close error: %v\n", err)
		}
	}

	return h.host.Close()
}

// ID returns the host's peer ID.
func (h *Host) ID() peer.ID {
	return h.host.ID()
}

// Addrs returns the host's listen addresses.
func (h *Host) Addrs() []multiaddr.Multiaddr {
	return h.host.Addrs()
}

// LibP2PHost returns the underlying libp2p host for direct access.
func (h *Host) LibP2PHost() host.Host {
	return h.host
}

// RegisterConnectionNotifier registers a network notifiee for peer events.
func (h *Host) RegisterConnectionNotifier(n libnet.Notifiee) {
	h.host.Network().Notify(n)
}

// DHT returns the Kademlia DHT instance for provider operations.
func (h *Host) DHT() *dht.IpfsDHT {
	return h.dht
}
