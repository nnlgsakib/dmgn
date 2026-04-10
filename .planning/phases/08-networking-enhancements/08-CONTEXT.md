# Phase 8: Networking Enhancements - Context

**Gathered:** 2026-04-10
**Status:** Ready for planning
**Source:** User requested update - replace TCP with QUIC, add NAT traversal for nodes behind NAT

<domain>
## Phase Boundary

Enhance libp2p networking to support QUIC transport, NAT traversal, and networking layer security. This phase adds:
- QUIC transport alongside existing TCP
- Multiple NAT traversal mechanisms (Circuit Relay v2, hole punching, TURN fallback)
- Updated listen address configuration
- Connection gater with reputation-based blocking and peer blocklist
- libp2p Resource Manager for connection/stream limits
- Per-peer rate limiting on protocol handlers
- Config-driven peer blocklist/allowlist

Requirements: NETW-02, NETW-04, NETW-06, NETW-07, NETW-08, NETW-09

This does NOT include: daemon architecture changes (Phase 7), protocol handlers (Phase 4), gossip (Phase 5).

</domain>

<decisions>
## Implementation Decisions

### Transport Configuration
- **D-01:** Add QUIC transport (`/ip4/.../udp/0/quic-v1`) alongside existing TCP
- **D-02:** Default listen addresses: `/ip4/0.0.0.0/tcp/0` AND `/ip4/0.0.0.0/udp/0/quic-v1`
- **D-03:** Config field: `ListenAddrs` (array) replacing single `ListenAddr` string
- **D-04:** Keep TCP transport — add QUIC as additional protocol, not replacement
- **D-05:** QUIC v1 (RFC 9000) as the QUIC version

### NAT Traversal
- **D-06:** Enable Circuit Relay v2 (`EnableRelayService`) for nodes behind NAT
- **D-07:** Enable direct hole punching (`EnableHolePunching`) with SRE/ENR support
- **D-08:** TURN fallback via config option for commercial relay as last resort
- **D-09:** Autorelay enabled by default — node finds relay peers automatically
- **D-10:** Config fields: `EnableHolePunching` (bool), `EnableRelayService` (bool), `TurnServers` ([]string)

### Networking Security
- **D-11:** Connection gater implementing `libp2p.ConnectionGater` interface — blocks peers based on ReputationManager score (below configurable threshold) and explicit blocklist
- **D-12:** Gater checks at `InterceptPeerDial`, `InterceptAccept`, and `InterceptSecured` stages — reject before resource allocation
- **D-13:** libp2p Resource Manager (`rcmgr`) with per-peer limits: max 16 streams, max 8 connections per peer; system-wide: max 256 connections, max 512 streams
- **D-14:** Per-peer protocol rate limiter: max 10 req/sec for `/memory/store`, max 20 req/sec for `/memory/fetch`, max 20 req/sec for `/memory/query`
- **D-15:** Config fields: `BlockedPeers []string`, `AllowedPeers []string`, `ReputationThreshold float64` (default 0.2), `MaxConnectionsPerPeer int`, `MaxStreamsPerPeer int`
- **D-16:** If `AllowedPeers` is non-empty, operate in allowlist mode — only those peers accepted. Otherwise, blocklist mode with reputation threshold.

### Agent's Discretion
- Exact QUIC tuning parameters (conn IDs, flow control)
- TURN server configuration format
- Logging verbosity for NAT traversal events
- Test strategy for NAT scenarios
- Resource manager limit values (within reasonable bounds)
- Rate limiter algorithm (token bucket vs sliding window)

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Networking
- `pkg/network/host.go` — Current libp2p host creation (lines 74-83 are the libp2p options)
- `internal/config/config.go` — Config struct, needs new fields for multiaddr array, NAT options, and security fields
- `.planning/phases/03-networking-core/03-CONTEXT.md` — Prior networking decisions (HKDF-derived key, custom DHT)

### Security
- `pkg/network/reputation.go` — Existing ReputationManager with peer scoring (no enforcement yet)
- `pkg/network/reputation_test.go` — Tests for reputation scoring
- `pkg/network/protocols.go` — Protocol handlers that need rate limiting
- `internal/daemon/daemon.go` — Daemon wiring where connection gater and resource manager must be integrated

### libp2p Documentation
- https://github.com/libp2p/go-libp2p/pull/3204 — QUIC-go v0.50.0 update (Feb 2025)
- https://github.com/libp2p/go-libp2p/pull/1128 — QUIC as default transport (merged 2021)
- https://docs.libp2p.io/blog/2023-09-13-quic-crypto-tls/ — Go 1.21 QUIC/TLS integration

</canonical_refs>

<specifics>
## Implementation Notes

### Current State (before)
```
libp2p options:
- ListenAddrStrings("/ip4/0.0.0.0/tcp/0")
- DefaultTransports (TCP only in current go-libp2p default)
- EnableRelay() // Circuit Relay v1 (client mode only)
```

### Desired State (after)
```
libp2p options:
- ListenAddrStrings("/ip4/0.0.0.0/tcp/0", "/ip4/0.0.0.0/udp/0/quic-v1")
- EnableRelay()        // Keep for backward compatibility
- EnableRelayService() // Act as relay for other peers
- EnableHolePunching() // Direct NAT traversal
- ConnectionGater(reputationGater) // Blocks bad/blocked peers
- ResourceManager(rcmgr)          // Per-peer resource limits
- libp2p.DefaultTransports // Includes QUIC by default since v0.30
```

### Config Changes Required
```go
// Before:
ListenAddr string `json:"listen_addr"`

// After (transport + NAT):
ListenAddrs         []string `json:"listen_addrs"`
EnableHolePunching  bool     `json:"enable_hole_punching"`
EnableRelayService  bool     `json:"enable_relay_service"`
RelayServers        []string `json:"relay_servers"`

// After (security):
BlockedPeers          []string `json:"blocked_peers"`
AllowedPeers          []string `json:"allowed_peers"`
ReputationThreshold   float64  `json:"reputation_threshold"`
MaxConnectionsPerPeer int      `json:"max_connections_per_peer"`
MaxStreamsPerPeer     int      `json:"max_streams_per_peer"`
```

</specifics>

<deferred>
## Deferred Ideas

- WebTransport transport (QUIC over HTTP/3) — future enhancement
- WebRTC transport — explicitly not wanted (PROJECT.md specifies TCP+QUIC only, no WebRTC)
- QUIC-only mode (remove TCP) — user chose to keep both
- Dynamic rate limit adjustment based on load — adds complexity, static limits sufficient for now
- Distributed reputation consensus — each node tracks independently for now
- IP-level blocking (beyond peer ID) — defer unless needed

---

*Phase: 08-networking-enhancements*
*Context gathered: 2026-04-10*