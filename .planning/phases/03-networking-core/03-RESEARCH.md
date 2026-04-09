# Phase 3: Networking Core - Research

**Researched:** 2026-04-09
**Phase Goal:** Establish libp2p networking and peer management

## Key Packages

| Package | Import Path | Purpose |
|---------|-------------|---------|
| go-libp2p | `github.com/libp2p/go-libp2p` | Core host, options, transport |
| go-libp2p-kad-dht | `github.com/libp2p/go-libp2p-kad-dht` | Kademlia DHT for global discovery |
| connmgr | `github.com/libp2p/go-libp2p/p2p/net/connmgr` | Connection manager with watermarks |
| mdns | `github.com/libp2p/go-libp2p/p2p/discovery/mdns` | mDNS local network discovery |
| libp2p-crypto | `github.com/libp2p/go-libp2p/core/crypto` | Ed25519 key for peer ID |
| libp2p-peer | `github.com/libp2p/go-libp2p/core/peer` | Peer ID, AddrInfo |
| libp2p-host | `github.com/libp2p/go-libp2p/core/host` | Host interface |

## Architecture Decisions

### HKDF-Derived Peer Identity
- Use `Identity.DeriveKey("libp2p-host", 32)` to get 32-byte seed
- Feed seed into `ed25519.NewKeyFromSeed(seed)` to get deterministic ed25519 private key
- Convert to libp2p crypto key via `crypto.UnmarshalEd25519PrivateKey(privKeyBytes)`
- Peer ID is derived deterministically from identity — same node always has same peer ID

### Host Configuration
```go
libp2p.New(
    libp2p.Identity(privKey),
    libp2p.ListenAddrStrings(listenAddrs...),
    libp2p.ConnectionManager(connMgr),
    libp2p.NATPortMap(),           // UPnP NAT traversal
    libp2p.EnableAutoRelay(),      // Autorelay for NAT (NETW-04)
    libp2p.DefaultTransports,      // TCP + QUIC (NETW-02)
    libp2p.DefaultSecurity,        // TLS + Noise
    libp2p.DefaultMuxers,          // yamux + mplex
)
```

### DHT Bootstrap
- Custom DMGN bootstrap list stored in config
- DHT operates in `dht.ModeAutoServer` — server mode if publicly reachable, client mode behind NAT
- Use `/dmgn/kad/1.0.0` as protocol prefix for private DHT namespace

### mDNS Discovery
- Use `mdns.NewMdnsService(host, serviceTag, notifee)` from `p2p/discovery/mdns`
- Default service tag: `_dmgn._tcp`
- Notifee callback connects to discovered peers

### Connection Manager
- `connmgr.NewConnManager(lowWatermark, highWatermark, connmgr.WithGracePeriod(time.Minute))`
- Default: low=15, high=25
- Grace period allows newly connected peers time before trimming

## Package Structure

```
pkg/network/
├── host.go       # Host creation, start/stop lifecycle
├── discovery.go  # DHT + mDNS discovery management
├── peers.go      # Peer info, listing, connection status
└── host_test.go  # Tests with mock/real libp2p hosts
```

## Integration Points

- `internal/config/config.go` — Add BootstrapPeers, MDNSService, MaxPeersLow, MaxPeersHigh
- `internal/cli/start.go` — Replace stub with libp2p host + optional API server
- `internal/cli/status.go` — Wire in live peer count
- `internal/cli/peers.go` — New command listing connected peers
- `cmd/dmgn/main.go` — Register PeersCmd()

## Risk Mitigations

| Risk | Mitigation |
|------|------------|
| NAT traversal failures | Enable both UPnP (NATPortMap) and autorelay as fallback |
| DHT bootstrap with no peers | Graceful degradation — log warning, mDNS still works locally |
| libp2p version compatibility | Pin to latest stable release, avoid pre-release APIs |
| Test flakiness with real network | Use libp2p's in-memory transport for unit tests |

## RESEARCH COMPLETE
