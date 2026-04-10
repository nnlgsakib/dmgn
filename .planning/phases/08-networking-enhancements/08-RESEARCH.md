# Phase 8: Networking Enhancements — Research

**Researched:** 2026-04-10
**Phase Goal:** Add QUIC transport and NAT traversal for improved connectivity behind NAT
**Requirements:** NETW-02, NETW-04

## Executive Summary

Phase 8 is a focused enhancement to the existing libp2p networking stack (Phase 3). The current codebase already has most dependencies needed — `quic-go v0.59.0` is an indirect dependency, and `libp2p.DefaultTransports` already includes QUIC transport. The main work is: (1) adding QUIC listen addresses, (2) enabling Circuit Relay v2 service + hole punching + AutoRelay via go-libp2p options, and (3) migrating the config from single `ListenAddr` to `ListenAddrs` array with new NAT boolean fields.

## 1. QUIC Transport Analysis

### Current State

```go
// pkg/network/host.go lines 74-83
opts := []libp2p.Option{
    libp2p.Identity(cfg.PrivateKey),
    libp2p.ListenAddrStrings(cfg.ListenAddrs...),
    libp2p.ConnectionManager(cm),
    libp2p.NATPortMap(),
    libp2p.EnableRelay(),
    libp2p.DefaultTransports,  // Already includes QUIC transport!
    libp2p.DefaultSecurity,
    libp2p.DefaultMuxers,
}
```

**Key finding:** `libp2p.DefaultTransports` in go-libp2p v0.48.0 already registers TCP, QUIC, and WebSocket transports. The node can already _dial_ QUIC peers. It just doesn't _listen_ on QUIC because the config only provides a TCP listen address (`/ip4/0.0.0.0/tcp/0`).

### What's Needed

Simply add `/ip4/0.0.0.0/udp/0/quic-v1` to the listen addresses. No new imports or transport constructors needed.

### QUIC v1 Multiaddr Format

- **TCP:** `/ip4/0.0.0.0/tcp/0`
- **QUIC v1:** `/ip4/0.0.0.0/udp/0/quic-v1`
- Both can use port 0 (auto-assign) or a fixed port

### Dependencies

Already present in `go.mod`:
- `github.com/quic-go/quic-go v0.59.0` (indirect, pulled by go-libp2p)
- `github.com/quic-go/qpack v0.6.0` (indirect)

No new dependencies required for QUIC.

## 2. Circuit Relay v2 Analysis

### go-libp2p API

```go
import "github.com/libp2p/go-libp2p/p2p/protocol/circuitv2/relay"

// Enable this node to act as a relay for other peers
libp2p.EnableRelayService(relay.WithResources(relay.Resources{...}))
```

**Current code has:** `libp2p.EnableRelay()` — this enables the node as a relay _client_ (can connect through relays). It does NOT enable the node to _be_ a relay for others.

**What to add:** `libp2p.EnableRelayService()` — makes the node act as a relay server for NAT'd peers. Should be conditional on `Config.EnableRelayService`.

### Resource Limits (defaults from go-libp2p)

```go
relay.DefaultResources() = relay.Resources{
    Limit: &relay.RelayLimit{
        Data:     1 << 17,  // 128 KiB per connection
        Duration: 2 * time.Minute,
    },
    MaxCircuits:           16,
    BufferSize:            2048,
    ReservationTTL:        time.Hour,
    MaxReservations:       128,
    MaxReservationsPerIP:  4,
    MaxReservationsPerASN: 32,
}
```

For DMGN, defaults are adequate. No need to customize initially.

### Import Required

```go
import "github.com/libp2p/go-libp2p/p2p/protocol/circuitv2/relay"
```

This package is already available in go-libp2p v0.48.0 — no new `go get` needed.

## 3. Hole Punching Analysis

### go-libp2p API

```go
import "github.com/libp2p/go-libp2p/p2p/protocol/holepunch"

libp2p.EnableHolePunching()
```

**Dependencies:**
- Relay must be enabled (it is: `libp2p.EnableRelay()`)
- Works best with AutoRelay so the node can advertise relay addresses

**How it works:**
1. NAT'd peer connects to a relay and gets a relay address
2. When another peer dials the relay address, the relay notifies both sides
3. Both sides attempt direct connections (hole punch) via the relay's coordination
4. If successful, traffic flows directly; relay connection is closed after grace period

### Important Note from go-libp2p docs

> It is not mandatory but nice to also enable the `AutoRelay` option so the peer can discover and connect to Relay servers if it discovers that it is NATT'd.

This means we should enable AutoRelay alongside hole punching.

## 4. AutoRelay Analysis

### go-libp2p API

Two modes:

```go
import "github.com/libp2p/go-libp2p/p2p/host/autorelay"

// Mode 1: Static relay list (for known relay servers)
libp2p.EnableAutoRelayWithStaticRelays([]peer.AddrInfo{...})

// Mode 2: Peer source (discover relays from DHT/routing)
libp2p.EnableAutoRelayWithPeerSource(peerSourceFunc)
```

**Recommendation:** Use static relays from config when `TurnServers` (effectively "relay servers") are provided. Fall back to peer-source-based discovery from DHT otherwise.

### AutoRelay Behavior

- Detects if node is behind NAT via AutoNAT
- If behind NAT, connects to relay servers and advertises relay addresses
- Other peers can reach this node via the relay
- Combined with hole punching, the relay is just the initial coordination channel

## 5. TURN Server Mapping

**Key insight:** libp2p doesn't have native TURN protocol support. The `TurnServers` config field in CONTEXT.md maps to **static relay peers** in libp2p terminology. These are well-known relay nodes that NAT'd peers can always connect to.

The `TurnServers` config field should be renamed or documented as "relay server" addresses in libp2p multiaddr format. For the implementation, `TurnServers []string` will contain multiaddr strings of known relay servers (e.g., `/ip4/relay.example.com/tcp/4001/p2p/QmRelay...`).

These are passed to `libp2p.EnableAutoRelayWithStaticRelays()`.

## 6. Config Migration Strategy

### Current Config

```go
type Config struct {
    ListenAddr string `json:"listen_addr"`  // Single address
    // ... other fields
}
```

### Target Config

```go
type Config struct {
    ListenAddr          string   `json:"listen_addr"`           // DEPRECATED: kept for backward compat
    ListenAddrs         []string `json:"listen_addrs"`          // New: array of listen addresses
    EnableHolePunching  bool     `json:"enable_hole_punching"`  // New: default true
    EnableRelayService  bool     `json:"enable_relay_service"`  // New: default false (opt-in)
    RelayServers        []string `json:"relay_servers"`         // New: static relay multiaddrs
    // ... other fields
}
```

### Backward Compatibility

```go
func (c *Config) GetListenAddrs() []string {
    if len(c.ListenAddrs) > 0 {
        return c.ListenAddrs
    }
    if c.ListenAddr != "" {
        return []string{c.ListenAddr}
    }
    return []string{"/ip4/0.0.0.0/tcp/0", "/ip4/0.0.0.0/udp/0/quic-v1"}
}
```

### Default Listen Addresses

```go
ListenAddrs: []string{
    "/ip4/0.0.0.0/tcp/0",
    "/ip4/0.0.0.0/udp/0/quic-v1",
}
```

## 7. Integration Points

### Files to Modify

| File | Changes |
|------|---------|
| `internal/config/config.go` | Add `ListenAddrs`, `EnableHolePunching`, `EnableRelayService`, `RelayServers` fields; add `GetListenAddrs()` method; update defaults |
| `pkg/network/host.go` | Add `EnableHolePunching`, `EnableRelayService`, `RelayServers` to `HostConfig`; conditionally add libp2p options |
| `internal/daemon/daemon.go` | Use `GetListenAddrs()` instead of single `ListenAddr`; update `persistMultiaddrs` to handle QUIC addresses |
| `pkg/network/host_test.go` | Update test hosts to use both TCP and QUIC listen addresses |
| `tests/integration_test.go` | Verify QUIC connectivity if applicable |

### host.go Changes

```go
type HostConfig struct {
    ListenAddrs        []string
    BootstrapPeers     []string
    MDNSService        string
    MaxPeersLow        int
    MaxPeersHigh       int
    PrivateKey         crypto.PrivKey
    EnableHolePunching bool     // New
    EnableRelayService bool     // New
    RelayServers       []string // New
}

func NewHost(cfg HostConfig) (*Host, error) {
    // ... existing code ...
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

    if cfg.EnableRelayService {
        opts = append(opts, libp2p.EnableRelayService())
    }

    if cfg.EnableHolePunching {
        opts = append(opts, libp2p.EnableHolePunching())
    }

    if len(cfg.RelayServers) > 0 {
        // Parse relay server multiaddrs to peer.AddrInfo
        relayInfos := parseRelayAddrs(cfg.RelayServers)
        opts = append(opts, libp2p.EnableAutoRelayWithStaticRelays(relayInfos))
    } else {
        // Use DHT-based relay discovery
        opts = append(opts, libp2p.EnableAutoRelayWithPeerSource(peerSourceFromDHT))
    }

    // ...
}
```

### daemon.go Changes

```go
// Current (line 100-101):
hostCfg := network.HostConfig{
    ListenAddrs: []string{d.cfg.ListenAddr},
    // ...
}

// New:
hostCfg := network.HostConfig{
    ListenAddrs:        d.cfg.GetListenAddrs(),
    EnableHolePunching: d.cfg.EnableHolePunching,
    EnableRelayService: d.cfg.EnableRelayService,
    RelayServers:       d.cfg.RelayServers,
    // ...
}
```

### persistMultiaddrs Update

The current `persistMultiaddrs` only handles TCP port extraction. Needs to also handle UDP/QUIC ports:

```go
func (d *Daemon) persistMultiaddrs(peerID string) {
    addrs := d.host.Addrs()
    fullAddrs := make([]string, 0, len(addrs))
    for _, addr := range addrs {
        fullAddrs = append(fullAddrs, fmt.Sprintf("%s/p2p/%s", addr.String(), peerID))
    }

    // Extract bound addresses and update ListenAddrs
    listenAddrs := make([]string, 0, len(addrs))
    for _, addr := range addrs {
        parts := strings.Split(addr.String(), "/")
        for i, p := range parts {
            if p == "tcp" && i+1 < len(parts) {
                listenAddrs = append(listenAddrs, fmt.Sprintf("/ip4/0.0.0.0/tcp/%s", parts[i+1]))
            }
            if p == "udp" && i+1 < len(parts) {
                listenAddrs = append(listenAddrs, fmt.Sprintf("/ip4/0.0.0.0/udp/%s/quic-v1", parts[i+1]))
            }
        }
    }
    if len(listenAddrs) > 0 {
        d.cfg.ListenAddrs = listenAddrs
    }

    d.cfg.NodeMultiaddrs = fullAddrs
    d.cfg.Save()
}
```

## 8. Risk Assessment

| Risk | Severity | Mitigation |
|------|----------|------------|
| QUIC blocked by firewall | Medium | Keep TCP as fallback, QUIC is additive |
| Hole punching unreliable on symmetric NAT | Medium | AutoRelay provides fallback via relay |
| Config migration breaks existing setups | Low | `GetListenAddrs()` falls back to `ListenAddr` |
| AutoRelay overhead on public nodes | Low | Only activates when AutoNAT detects NAT |
| Relay service resource consumption | Medium | Use default resource limits, make opt-in |

## 9. Test Strategy

1. **Unit tests for config**: Verify `GetListenAddrs()` backward compatibility
2. **Unit tests for host creation**: Verify QUIC listen address appears in `host.Addrs()`
3. **Unit tests for relay/holepunch options**: Verify options are passed correctly
4. **Integration test**: Two-node connectivity over QUIC (localhost)
5. **Manual test**: Verify both TCP and QUIC addresses in `dmgn status` output

## Validation Architecture

### Critical Path Validation
- QUIC listen address appears in host addresses
- TCP still works (no regression)
- Config migration preserves existing single-address configs

### Sampling Points
- Relay service activates on public nodes
- Hole punching attempts logged on NAT'd nodes
- AutoRelay discovers relay peers from DHT

---

## RESEARCH COMPLETE

**Confidence:** High — all go-libp2p APIs are well-documented and the existing codebase already has most dependencies. This is primarily a configuration and wiring task.
