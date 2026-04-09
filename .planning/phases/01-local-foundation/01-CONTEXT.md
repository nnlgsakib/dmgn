# Phase 1: Local Foundation - Context

**Gathered:** 2026-04-09
**Status:** Ready for planning

<domain>
## Phase Boundary

Implement protobuf migration for performance optimization. This phase delivers: Protocol Buffers for all wire formats (store/fetch protocols, gossip, delta sync), BadgerDB native format with compression for disk storage, hybrid approach for memory model (protobuf for replication, JSON for local), and JSON preserved for API layer (REST, MCP required by spec).

This is Phase 1 of 6 - focuses on local foundation with identity, storage, and CLI, but includes the cross-cutting protobuf migration work.

</domain>

<decisions>
## Implementation Decisions

### Wire Format (Network Protocols)
- **D-01:** Use Protocol Buffers for all network protocol wire formats
- **D-02:** Target: store/fetch protocols (`/dmgn/memory/store/1.0.0`, `/dmgn/memory/fetch/1.0.0`)
- **D-03:** Target: gossip messages (GossipSub broadcasts)
- **D-04:** Target: delta sync messages (reconnect synchronization)
- **D-05:** Rationale: 2-3x smaller payloads, 5-10x faster parsing, schema enforcement catches bugs

### Gossip Message Format
- **D-06:** Protocol Buffers for gossip (same as wire format for consistency)
- **D-07:** GossipMessage struct migrated to protobuf with same fields: Type, Memory, SenderPeerID, Timestamp, Sequence

### Shard Persistence (Disk Format)
- **D-08:** Use BadgerDB native format with value log compression
- **D-09:** Keep JSON for disk - optimize via BadgerDB's compression and TTL support
- **D-10:** Rationale: Disk I/O lower frequency than wire, easier to debug with badger-cli

### Memory Model Serialization
- **D-11:** Hybrid approach: protobuf for replication (gossip/p2p), JSON for local storage
- **D-12:** Memory struct serialized as protobuf when transmitted over network
- **D-13:** Memory struct kept as JSON in local BadgerDB storage

### API Layer
- **D-14:** Keep JSON for REST API (required by HTTP spec)
- **D-15:** Keep JSON for MCP stdio (required by JSON-RPC 2.0 spec)
- **D-16:** Keep JSON for CLI output (human-readable)
- **D-17:** Rationale: API specs require JSON, no benefit to migrating

### Migration Implementation
- **D-18:** Create `proto/` directory for .proto files
- **D-19:** Generate Go code with `protoc` and go-protobuf
- **D-20:** Implement conversion layer: JSON ↔ Protobuf for memory model
- **D-21:** Batch migration: wire protocols first (hot path), then gossip

### Performance Targets
- **D-22:** Network payload size: target 50-70% reduction (JSON → protobuf)
- **D-23:** Parse latency: target 5-10x improvement for wire protocols

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Project Specs
- `.planning/PROJECT.md` — Core value, constraints (offline-first, performance targets)
- `.planning/REQUIREMENTS.md` — All v1 requirements
- `.planning/ROADMAP.md` §Phase 1 — Success criteria, key components
- `.planning/STATE.md` — All phases complete, performance optimization next

### Prior Phase Artifacts
- `.planning/phases/03-networking-core/03-CONTEXT.md` — Network protocols already implemented
- `.planning/phases/04-distributed-storage/04-CONTEXT.md` — Sharding strategy (Shamir SSS)
- `.planning/phases/05-query-sync/05-CONTEXT.md` — Query engine, gossip sync

### Existing Code (Migration Targets)
- `pkg/network/protocols.go` — Store/Fetch protocol handlers (5 JSON uses) — **HOT PATH**
- `pkg/sync/gossip.go` — GossipManager (2 JSON uses) — **HIGH FREQUENCY**
- `pkg/sync/delta.go` — DeltaSyncManager (2 JSON uses)
- `pkg/storage/shards.go` — Shard storage (4 JSON uses)
- `pkg/memory/memory.go` — Memory struct (5 JSON uses)
- `pkg/network/reputation.go` — Peer reputation (3 JSON uses)

### Codebase Analysis
- Total JSON marshal/unmarshal uses: 47
- Wire (network): 9 uses (high frequency, hot path)
- Disk: 9 uses (medium frequency)
- Memory model: 5 uses (core data)
- API (required): 24 uses (spec-required, no change)

</code_context>

<specifics>
## Specific Ideas

- libp2p and IPFS use Protocol Buffers internally — aligning with industry standard
- BadgerDB supports value log compression natively — use that instead of manual compression
- Conversion layer allows gradual migration: JSON → protobuf → JSON at boundaries

</specifics>

<deferred>
## Deferred Ideas

- Full protobuf everywhere — rejected in favor of hybrid (best practice)
- MessagePack or Cap'n Proto — protobuf chosen for ecosystem support
- Memory model full migration — hybrid is sufficient

</deferred>

---

*Phase: 01-local-foundation*
*Context gathered: 2026-04-09*