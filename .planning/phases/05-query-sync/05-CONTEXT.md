# Phase 5: Query & Sync - Context

**Gathered:** 2026-04-09
**Status:** Ready for planning

<domain>
## Phase Boundary

Enable cross-peer search and data synchronization for DMGN. This phase delivers: a vector index (pure Go HNSW) for similarity search over caller-provided embeddings, a query orchestrator that fans out to peers and merges results with source diversity, a `/dmgn/memory/query/1.0.0` protocol for cross-peer queries, libp2p GossipSub integration for real-time memory propagation, and vector-clock-based delta sync for reconnected peers. No embedding generation (callers provide embeddings), no MCP tools (Phase 6), no peer reputation scoring (Phase 6).

**Critical design principle:** DMGN is a distributed memory layer for AI agents — it stores, indexes, searches, and syncs. It is NOT a computation platform. It does not generate embeddings, run ML models, or perform heavy computation. AI agents (Claude Code, Cline, etc.) provide pre-computed embeddings when storing memories.

Requirements: QUER-01, QUER-02, QUER-03, QUER-04, QUER-05, SYNC-01, SYNC-02, SYNC-03, SYNC-04

</domain>

<decisions>
## Implementation Decisions

### Embedding Strategy
- **D-01:** DMGN accepts pre-computed embeddings from callers (AI agents) via API and MCP. DMGN does NOT generate embeddings itself — zero ML dependencies in the binary.
- **D-02:** The `Memory.Embedding` field (`[]float32`) already exists in `pkg/memory/memory.go`. Callers provide embeddings when creating memories. If no embedding is provided, the memory is stored without one and excluded from vector search (text search fallback still works).
- **D-03:** Embedding dimension is flexible — DMGN stores whatever dimension the caller provides. The HNSW index auto-detects dimension from the first indexed vector. Config field `embedding_dim` sets expected dimension for validation (default: 0 = accept any).

### Vector Index & Search
- **D-04:** Use a pure Go HNSW library (no CGo) for vector similarity search. No Faiss/Hnswlib bindings — keeps binary lightweight, cross-platform, and buildable on low-end devices.
- **D-05:** Persist the HNSW index to disk encrypted with AES-GCM (same key derivation as memory encryption, purpose `"vector-index"`). On startup, decrypt and load the index. On shutdown or periodic flush, encrypt and persist.
- **D-06:** Hybrid scoring — combine vector similarity score with text matching score (existing `scoreMatch` from `query.go`) into a blended result. Vector similarity is primary; text match is secondary boost. Final score = `α * vector_score + (1-α) * text_score` with configurable α (default 0.7).

### Cross-Peer Query Orchestration
- **D-07:** Claude's discretion on query routing strategy (fan-out vs DHT-routed vs hybrid). Recommendation: fan-out to all connected peers for simplicity given small peer counts in typical DMGN deployments.
- **D-08:** Result ranking uses score + source diversity — interleave results from different peers to avoid one peer dominating. Deduplicate by memory_id, keep highest score.
- **D-09:** Remote query results include a short plaintext snippet (first 100 chars) + metadata (memory_id, score, type, timestamp, source_peer). Full content fetched on demand via existing `/dmgn/memory/fetch/1.0.0` protocol.
- **D-10:** Query protocol: `/dmgn/memory/query/1.0.0` — request contains query embedding (`[]float32`) + limit + optional filters (type, time range). Response contains list of `(memory_id, score, type, timestamp, snippet, source_peer)`.

### Gossip Sync
- **D-11:** Use libp2p GossipSub (go-libp2p-pubsub) for real-time memory propagation. Topic: `dmgn/memories/1.0.0`. All DMGN nodes subscribe on startup.
- **D-12:** Gossip messages contain the full encrypted memory payload (including embedding). Peers store the memory directly and index the embedding. Highest bandwidth but instant replication without extra round trips.
- **D-13:** Gossip message format: JSON envelope with `{type: "new_memory", memory: <encrypted Memory JSON>, sender_peer_id, timestamp}`.

### Delta Sync (Reconnection)
- **D-14:** Vector clock / version vector for delta sync. Each peer maintains a version vector tracking the latest sequence number seen from every known peer.
- **D-15:** On reconnect, peers exchange version vectors via `/dmgn/memory/sync/1.0.0` protocol. The peer with newer data sends missing memories. Bidirectional — both sides send what the other is missing.
- **D-16:** No conflict resolution needed — content-addressable IDs are based on encrypted payload (SHA256). Since each node uses unique per-memory encryption keys, identical plaintext produces different IDs. Keep both versions.

### Claude's Discretion
- HNSW library selection (e.g. `github.com/coder/hnsw` or similar pure Go lib)
- Internal package structure for query engine, sync manager
- GossipSub configuration (heartbeat interval, message TTL, flood publish threshold)
- Version vector storage format in BadgerDB
- Query timeout and concurrency limits for cross-peer fan-out
- Hybrid score weighting α default and config key name
- Test strategy (mock pubsub, in-memory index tests)

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Project Specs
- `.planning/PROJECT.md` — Core value, constraints (offline-first, no plaintext over network, local queries <100ms, cross-peer sync <5s)
- `.planning/REQUIREMENTS.md` — QUER-01 through QUER-05, SYNC-01 through SYNC-04
- `.planning/ROADMAP.md` §Phase 5 — Success criteria, key components, dependency on Phase 2 and 4

### Prior Phase Artifacts
- `.planning/phases/03-networking-core/03-CONTEXT.md` — DHT namespace, protocol naming convention (`/dmgn/...`), config-driven pattern
- `.planning/phases/04-distributed-storage/04-CONTEXT.md` — Shamir sharding, store/fetch protocols, length-prefixed JSON framing, rebalancing strategy

### Existing Code
- `pkg/memory/memory.go` — `Memory` struct with `Embedding []float32` field (unpopulated), `Decrypt()`, content-addressable IDs
- `pkg/network/host.go` — `Host.LibP2PHost()` for pubsub setup, `Host.DHT()` for provider operations, `Host.Start()`/`Stop()` lifecycle
- `pkg/network/protocols.go` — Store/Fetch protocol pattern with `writeFrame()`/`readFrame()`, length-prefixed JSON + data framing
- `pkg/sharding/distributor.go` — `Distributor.ReconstructFromPeers()` for cross-peer data fetching pattern
- `internal/cli/query.go` — Existing brute-force `scoreMatch()` text search to be integrated into hybrid scoring
- `internal/api/handlers.go` — `HandleQuery()` and `HandleAddMemory()` — need embedding field in API request/response
- `internal/config/config.go` — Config struct pattern for new fields

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- **`Memory.Embedding`** (`pkg/memory/memory.go`): `[]float32` field already in struct — ready for caller-provided embeddings
- **`scoreMatch()`** (`internal/cli/query.go`): Existing text scoring function for hybrid scoring integration
- **`writeFrame()`/`readFrame()`** (`pkg/network/protocols.go`): Protocol framing pattern for new query/sync protocols
- **`Host.LibP2PHost()`** (`pkg/network/host.go`): Raw libp2p host for GossipSub setup and stream handler registration
- **`Host.DHT()`** (`pkg/network/host.go`): DHT accessor for provider records (if needed for query routing)
- **`Distributor.ReconstructFromPeers()`** (`pkg/sharding/distributor.go`): Pattern for fan-out + collect from multiple peers

### Established Patterns
- **Config-driven tunables**: All new settings go in `config.Config` with JSON serialization and defaults
- **Protocol naming**: `/dmgn/{domain}/{version}` — e.g. `/dmgn/memory/query/1.0.0`
- **Length-prefixed JSON framing**: 4-byte big-endian length + JSON header + optional data payload
- **Graceful degradation**: If peers unavailable, degrade to local-only (offline-first)

### Integration Points
- `pkg/network/host.go` — Register GossipSub, query protocol, sync protocol stream handlers
- `internal/config/config.go` — Add `EmbeddingDim`, `HybridScoreAlpha`, `QueryTimeout`, `SyncInterval` fields
- `internal/cli/query.go` — Replace brute-force with hybrid vector + text search
- `internal/api/handlers.go` — Accept `embedding` field in `POST /memory`, return `score` in query results
- `internal/cli/start.go` — Wire GossipSub and sync manager into node startup

</code_context>

<specifics>
## Specific Ideas

- Reference project: [claude-mem](https://github.com/thedotmack/claude-mem) — DMGN serves a similar role (persistent memory for AI) but distributed and encrypted
- Target users: AI agents (Claude Code, Cline, etc.) that need cross-device, cross-session memory
- DMGN is a storage/index/sync layer, NOT a computation platform — this principle must guide all implementation choices

</specifics>

<deferred>
## Deferred Ideas

- **Reed-Solomon erasure coding** (RCVR-01, RCVR-02) — Advanced shard recovery, future phase
- **Peer reputation scoring** (NETW-05) — Phase 6
- **Local embedding model** — If users want to generate embeddings without an external API, add as optional plugin later
- **Query caching** — Cache frequent query results locally, optimize in Phase 6
- **Embedding re-indexing** — If embedding dimension changes, provide a migration path. Defer to when it's needed.

</deferred>

---

*Phase: 05-query-sync*
*Context gathered: 2026-04-09*
