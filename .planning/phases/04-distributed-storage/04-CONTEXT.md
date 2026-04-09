# Phase 4: Distributed Storage - Context

**Gathered:** 2026-04-09
**Status:** Ready for planning

<domain>
## Phase Boundary

Distribute encrypted memory shards across peers with redundancy. This phase delivers: a sharding package that splits encrypted payloads using Shamir's Secret Sharing, libp2p protocol handlers for `/dmgn/memory/store/1.0.0` and `/dmgn/memory/fetch/1.0.0`, DHT-based shard placement and provider tracking, and automatic rebalancing when peers join or leave. No query protocols (Phase 5) or sync/gossip (Phase 5).

Requirements: DIST-01, DIST-02, DIST-03, DIST-04, DIST-05, NETW-03

</domain>

<decisions>
## Implementation Decisions

### Sharding Strategy
- **D-01:** Use Shamir's Secret Sharing (SSS) to split encrypted payloads into shards. This satisfies DIST-04 — no single peer (or any k-1 peers) can reconstruct the original data.
- **D-02:** Default threshold k=3, total shards n=5. Configurable via `shard_threshold` and `shard_count` config fields. k=3 satisfies DIST-03 (replication factor ≥3).
- **D-03:** Sharding operates on the already-encrypted `Memory.EncryptedPayload` — double-layer protection (encryption + secret sharing).

### Shard Placement & Routing
- **D-04:** DHT-based shard routing. Each shard gets a deterministic key: `SHA256(memory_id + shard_index)`. The DHT `Provide()`/`FindProviders()` API tracks which peers hold each shard.
- **D-05:** Actual shard data is transferred via the `/dmgn/memory/store/1.0.0` protocol stream, not stored in the DHT itself. DHT only stores provider records (peer locations).
- **D-06:** Shard placement prefers peers with lowest latency and highest uptime from the connected peer set. Falls back to random selection if insufficient data.

### Protocol Handler Design
- **D-07:** Store protocol (`/dmgn/memory/store/1.0.0`): Sender writes a length-prefixed JSON header (memory_id, shard_index, total_shards, threshold, checksum) followed by raw shard bytes. Receiver validates checksum, stores shard, responds with ACK/NACK.
- **D-08:** Fetch protocol (`/dmgn/memory/fetch/1.0.0`): Requester sends JSON request (memory_id, shard_index). Responder sends length-prefixed JSON header + shard bytes, or error if not found.
- **D-09:** Protocol namespace: `/dmgn/memory/store/1.0.0` and `/dmgn/memory/fetch/1.0.0` — consistent with Phase 3's `/dmgn/kad/1.0.0` namespace pattern.

### Rebalancing Strategy
- **D-10:** Event-driven rebalancing on peer disconnect (via libp2p network notifiee `Disconnected()` callback). When a shard's provider count drops below threshold, re-replicate to another available peer.
- **D-11:** Periodic audit every 5 minutes checks all locally-owned memories for adequate shard distribution. Catches missed disconnect events.
- **D-12:** Graceful degradation — if fewer than n peers are available, store all shards locally and queue distribution for when peers connect. Aligns with offline-first principle.

### Local Shard Storage
- **D-13:** Remote shards received from other peers are stored in BadgerDB under a `shard:` key prefix, separate from local memories.
- **D-14:** Each stored shard record includes: memory_id, shard_index, shard_data, owner_peer_id, received_timestamp.

### Claude's Discretion
- Internal package structure for sharding code
- Specific SSS library choice (hashicorp/vault shamir, or pure Go implementation)
- Protocol message framing details (varint vs fixed-length prefix)
- Test strategy (mock streams vs real libp2p hosts)
- Shard storage key format in BadgerDB

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Project Specs
- `.planning/PROJECT.md` — Core value, constraints (offline-first, replication ≥3, no plaintext over network)
- `.planning/REQUIREMENTS.md` — DIST-01 through DIST-05, NETW-03
- `.planning/ROADMAP.md` §Phase 4 — Success criteria, key components

### Prior Phase Artifacts
- `.planning/phases/03-networking-core/03-CONTEXT.md` — Network decisions: DHT namespace, protocol naming convention
- `.planning/phases/03-networking-core/03-01-SUMMARY.md` — Network host API, DHT setup, peer management
- `.planning/phases/03-networking-core/03-02-SUMMARY.md` — CLI integration, API server with network host

### Existing Code
- `pkg/memory/memory.go` — `Memory` struct with `EncryptedPayload`, `VerifyIntegrity()`, content-addressable IDs
- `pkg/network/host.go` — `Host.LibP2PHost()` for registering stream handlers, `Host.Start()`/`Stop()` lifecycle
- `pkg/network/peers.go` — `ConnectedPeers()`, `PeerCount()` for placement decisions
- `pkg/network/discovery.go` — DHT instance accessible for `Provide()`/`FindProviders()`
- `pkg/storage/` — BadgerDB store for local shard persistence
- `internal/config/config.go` — Config struct pattern for new `shard_threshold`/`shard_count` fields
- `internal/crypto/` — AES-GCM engine for pre-sharding encryption

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- **`network.Host.LibP2PHost()`** (`pkg/network/host.go`): Returns raw libp2p host for `SetStreamHandler()` registration
- **`network.Host.dht`** (`pkg/network/host.go`): DHT instance for `Provide()`/`FindProviders()` — currently unexported, needs accessor
- **`memory.Memory.EncryptedPayload`** (`pkg/memory/memory.go`): The byte slice to be sharded
- **`memory.Memory.VerifyIntegrity()`**: Validates shard checksums on reassembly
- **`storage.Store`** (`pkg/storage/`): BadgerDB store — can store shards under `shard:` prefix
- **`config.Config`** pattern: Add `ShardThreshold`, `ShardCount` fields with defaults

### Established Patterns
- **Config-driven tunables**: All new settings go in `config.Config` with JSON serialization and defaults
- **Package separation**: Network code in `pkg/network/`, memory in `pkg/memory/`, new sharding in `pkg/sharding/`
- **Test pattern**: Real libp2p hosts in tests (Phase 3 precedent), `t.TempDir()` for storage

### Integration Points
- `pkg/network/host.go` — Register `/dmgn/memory/store/1.0.0` and `/dmgn/memory/fetch/1.0.0` stream handlers
- `pkg/network/host.go` — Expose DHT accessor for provider records
- `internal/config/config.go` — Add shard config fields
- `internal/cli/start.go` — Wire shard manager into node startup
- `internal/api/handlers.go` — Optionally expose shard stats in `/status`

</code_context>

<specifics>
## Specific Ideas

No specific requirements — open to standard approaches for secret sharing and libp2p stream protocols.

</specifics>

<deferred>
## Deferred Ideas

- **Reed-Solomon erasure coding** (RCVR-01, RCVR-02) — More advanced recovery, Phase 5 or later
- **Peer reputation scoring** for shard placement (NETW-05) — Phase 6
- **Gossip sync protocol** (`/memory/sync/1.0.0`) — Phase 5
- **Query protocol** (`/memory/query/1.0.0`) — Phase 5
- **Shard compression** before distribution — defer unless payload sizes warrant it

</deferred>

---

*Phase: 04-distributed-storage*
*Context gathered: 2026-04-09*
