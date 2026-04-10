# Phase 8: Networking Enhancements - Context

**Gathered:** 2026-04-10
**Status:** Ready for planning
**Source:** User requested update - replace TCP with QUIC, add NAT traversal for nodes behind NAT

<domain>
## Phase Boundary

Enhance libp2p networking to support QUIC transport and NAT traversal. This phase adds:
- QUIC transport alongside existing TCP
- Multiple NAT traversal mechanisms (Circuit Relay v2, hole punching, TURN fallback)
- Updated listen address configuration

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

### Agent's Discretion
- Exact QUIC tuning parameters (conn IDs, flow control)
- TURN server configuration format
- Logging verbosity for NAT traversal events
- Test strategy for NAT scenarios

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Networking
- `pkg/network/host.go` — Current libp2p host creation (lines 74-83 are the libp2p options)
- `internal/config/config.go` — Config struct, needs new fields for multiaddr array and NAT options
- `.planning/phases/03-networking-core/03-CONTEXT.md` — Prior networking decisions (HKDF-derived key, custom DHT)

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
- libp2p.DefaultTransports // Includes QUIC by default since v0.30
```

### Config Changes Required
```go
// Before:
ListenAddr string `json:"listen_addr"`

// After:
ListenAddrs         []string `json:"listen_addrs"`
EnableHolePunching  bool     `json:"enable_hole_punching"`
EnableRelayService  bool     `json:"enable_relay_service"`
TurnServers         []string `json:"turn_servers"`
```

</specifics>

<deferred>
## Deferred Ideas

- WebTransport transport (QUIC over HTTP/3) — future enhancement
- WebRTC transport — explicitly not wanted (PROJECT.md specifies TCP+QUIC only, no WebRTC)
- QUIC-only mode (remove TCP) — user chose to keep both

---

*Phase: 08-networking-enhancements*
*Context gathered: 2026-04-10*