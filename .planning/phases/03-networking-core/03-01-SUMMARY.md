# Plan 03-01 Summary: Core Network Package + Config Extensions

**Status:** Complete
**Wave:** 1

## What Was Built

### Config Extensions (`internal/config/config.go`)
- Added `BootstrapPeers []string` — custom DHT bootstrap peer list
- Added `MDNSService string` — mDNS service tag (default: `_dmgn._tcp`)
- Added `MaxPeersLow int` / `MaxPeersHigh int` — connection manager watermarks (default: 15/25)

### Network Package (`pkg/network/`)

**host.go** — libp2p host lifecycle:
- `DeriveLibp2pKey(id)` — HKDF-derived ed25519 key with purpose `"libp2p-host"`
- `NewHost(cfg)` — creates host with identity, transports (TCP+QUIC), NAT traversal, connection manager
- `Start()` / `Stop()` — DHT bootstrap + mDNS discovery lifecycle
- `ID()` / `Addrs()` / `LibP2PHost()` — accessors

**discovery.go** — peer discovery:
- `setupDHT()` — Kademlia DHT with `ModeAutoServer` and custom prefix `/dmgn/kad/1.0.0`
- `setupMDNS()` — mDNS local discovery with configurable service tag
- `discoveryNotifee` — auto-connects to discovered peers

**peers.go** — peer info:
- `ConnectedPeers()` — returns `[]PeerInfo` with ID, addresses, latency
- `PeerCount()` — connected peer count
- `NetworkStats()` — map with peer_id, listen_addrs, connected_peers, dht_mode

### Tests (`pkg/network/host_test.go`)
- `TestDeriveLibp2pKey` — deterministic key derivation, different identities → different peer IDs
- `TestNewHostAndStop` — host creation and clean shutdown
- `TestPeerCount` — zero peers initially
- `TestNetworkStats` — stats map structure
- `TestTwoHostsConnect` — two hosts connect and see each other

## Dependencies Added
- `github.com/libp2p/go-libp2p@v0.48.0`
- `github.com/libp2p/go-libp2p-kad-dht@v0.39.0`
- Plus transitive dependencies (multiformats, quic-go, pion, etc.)

## Requirements Addressed
- **NETW-01:** libp2p host with DHT and mDNS discovery
- **NETW-02:** TCP and QUIC transports via DefaultTransports
- **NETW-04:** NAT traversal via NATPortMap + EnableRelay
