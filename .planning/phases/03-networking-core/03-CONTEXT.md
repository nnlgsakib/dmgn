# Phase 3: Networking Core - Context

**Gathered:** 2026-04-09
**Status:** Ready for planning

<domain>
## Phase Boundary

Establish libp2p peer-to-peer networking for DMGN nodes. This phase delivers: a libp2p host with DHT and mDNS discovery, the `dmgn start` command that launches both networking and the API server, `dmgn peers` command, and updated `dmgn status` with live peer info. No protocol handlers for memory storage/fetch (Phase 4) or sync (Phase 5).

Requirements: CLI-02, CLI-05, CLI-06, NETW-01, NETW-02, NETW-04

</domain>

<decisions>
## Implementation Decisions

### Daemon Architecture
- **D-01:** `dmgn start` launches BOTH the libp2p host AND the REST API server in a single process. `--no-api` flag disables the API for headless/relay-only nodes.
- **D-02:** `dmgn serve` remains as API-only mode (no networking). `dmgn start` is the primary command for full operation.
- **D-03:** Foreground process by default (Ctrl+C to stop). `--daemon` flag is deferred — users can use systemd/launchd/nssm for background operation.

### libp2p Peer Identity
- **D-04:** Derive libp2p host key from the existing ed25519 identity using HKDF with purpose `'libp2p-host'`. This provides deterministic peer IDs with proper domain separation — the network key cannot be reverse-linked to the master identity key.
- **D-05:** The HKDF infrastructure from Phase 2 (`Identity.DeriveKey`) is reused directly. The derived 32-byte seed is used to construct an ed25519 key for libp2p.

### Bootstrap & Discovery
- **D-06:** Custom DMGN bootstrap node list — private DHT namespace, not shared with IPFS. At least one bootstrap node must be operated. Bootstrap peers stored in config as `bootstrap_peers` list.
- **D-07:** mDNS discovery uses `_dmgn._tcp` as default service name, overridable via config field `mdns_service`. All DMGN nodes on the same LAN auto-discover each other.
- **D-08:** DHT operates in server mode on publicly reachable nodes, client mode behind NAT. libp2p autorelay provides NAT traversal (NETW-04).

### Connection Management
- **D-09:** Use libp2p ConnManager with adaptive watermarks: low=15, high=25 by default. Configurable via `max_peers_low` and `max_peers_high` in config.
- **D-10:** Exponential backoff on reconnection failures. ConnManager trims least-useful connections when high watermark is exceeded.
- **D-11:** Basic peer health via libp2p's built-in ping protocol. Advanced peer reputation scoring (NETW-05) deferred to Phase 6.

### Claude's Discretion
- Network package internal structure (interfaces, file layout)
- libp2p option configuration details (security transport, muxer)
- Test strategy for networking (mock vs real libp2p hosts)
- Log output format and verbosity levels

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Project Specs
- `.planning/PROJECT.md` — Core value, constraints (TCP+QUIC only, offline-first, no WebRTC)
- `.planning/REQUIREMENTS.md` — NETW-01 through NETW-04, CLI-02/05/06
- `.planning/ROADMAP.md` §Phase 3 — Success criteria, key components

### Prior Phase Artifacts
- `.planning/phases/02-encryption-api/02-01-SUMMARY.md` — HKDF infrastructure used for D-04/D-05
- `pkg/identity/identity.go` — `DeriveKey(purpose, keyLen)` method reused for libp2p key derivation

### Existing Code
- `internal/cli/start.go` — Stub to be replaced with full libp2p host launch
- `internal/cli/status.go` — Stub to be updated with live peer stats
- `internal/cli/serve.go` — API server code to be integrated into `start`
- `internal/config/config.go` — Config struct needs new networking fields
- `internal/api/server.go` — API server to be optionally started from `dmgn start`

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- **`Identity.DeriveKey`** (`pkg/identity/identity.go`): HKDF-SHA256 key derivation — use with purpose `'libp2p-host'` to derive network key
- **`api.Server`** (`internal/api/server.go`): REST API server with auth — to be embedded in `dmgn start`
- **`config.Config`** (`internal/config/config.go`): Already has `ListenAddr` in multiaddr format (`/ip4/0.0.0.0/tcp/0`)
- **Cobra CLI framework**: All commands follow the same pattern (flags, RunE, config load)

### Established Patterns
- **Config-driven**: All tunables go in `config.Config` with JSON serialization
- **Passphrase-gated**: Operations requiring identity prompt for passphrase via `promptPassphraseOnce()`
- **Graceful shutdown**: Signal handling with `SIGINT/SIGTERM` channels (used in start.go, serve.go)

### Integration Points
- `cmd/dmgn/main.go` — Register `PeersCmd()`, update `StartCmd()`
- `internal/config/config.go` — Add `BootstrapPeers`, `MDNSService`, `MaxPeersLow`, `MaxPeersHigh` fields
- `internal/cli/start.go` — Replace stub with libp2p host + optional API server
- `internal/cli/status.go` — Wire in live peer count from network package

</code_context>

<specifics>
## Specific Ideas

No specific requirements — open to standard libp2p patterns (go-libp2p host, Kademlia DHT, mDNS discovery module).

</specifics>

<deferred>
## Deferred Ideas

- **`--daemon` flag** for background operation — add when user demand warrants it
- **Peer reputation scoring** (NETW-05) — Phase 6
- **Protocol handlers** (`/memory/store/1.0.0`, `/memory/fetch/1.0.0`) — Phase 4
- **Gossip sync** (`/memory/sync/1.0.0`) — Phase 5

</deferred>

---

*Phase: 03-networking-core*
*Context gathered: 2026-04-09*
