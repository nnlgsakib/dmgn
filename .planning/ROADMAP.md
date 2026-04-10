# Roadmap: Distributed Memory Graph Network

**Project:** DMGN  
**Created:** 2025-04-09  
**Granularity:** Standard (8 phases)

## Summary

| # | Phase | Goal | Key Deliverables | Requirements |
|---|-------|------|------------------|--------------|
| 1 | [Local Foundation](#phase-1-local-foundation) | Core local storage with identity | CLI, BadgerDB, memory graph | 11 |
| 2 | [Encryption & API](#phase-2-encryption--api) | End-to-end encryption and interfaces | AES-GCM, REST API, MCP start | 12 |
| 3 | [Networking Core](#phase-3-networking-core) ✓ | libp2p peer discovery and connection | DHT, mDNS, peer management | 5 |
| 4 | [Distributed Storage](#phase-4-distributed-storage) | Shard and replicate across peers | Sharding, replication factor 3+ | 5 |
| 5 | [Query & Sync](#phase-5-query--sync) | Cross-peer search and consistency | Vector search, gossip sync | 5 |
| 6 | [MCP & Polish](#phase-6-mcp--polish) | Full MCP support and production readiness | MCP tools, metrics, docs | 5 |
| 7 | [Daemon Architecture](#phase-7-daemon-architecture--cli-restructure) | Persistent background daemon with integrated MCP and auto peer networking | Daemon, MCP auto-serve, stop cmd | 7 |
| 8 | [Networking Enhancements](#phase-8-networking-enhancements) | QUIC transport, NAT traversal, networking security | QUIC v1, Relay v2, hole punching, connection gater, resource mgr | 4 |

---

## Phase 1: Local Foundation

**Goal:** Build core local storage with identity and CLI interface

**Requirements:** IDTY-01, IDTY-02, IDTY-03, MEMO-01, MEMO-02, MEMO-03, MEMO-04, STOR-01, STOR-02, STOR-04, STOR-05, CLI-01, CLI-03

**Success Criteria:**
1. User can run `dmgn init` and create a new identity with ed25519 keypair
2. User can run `dmgn add "text"` and store memory locally with content-addressable ID
3. Memory graph can be traversed via link relationships
4. Data persists across CLI restarts
5. Time-based queries return memories in chronological order

**Key Components:**
- Identity package (key generation, storage)
- Memory model package (node structure, hashing)
- Storage package (BadgerDB integration)
- CLI commands: init, add

---

## Phase 2: Encryption & API

**Goal:** Add encryption layer and external interfaces

**Requirements:** IDTY-04, IDTY-05, STOR-03, CRPT-01, CRPT-02, CRPT-03, CRPT-04, CRPT-05, CLI-04, API-01, API-02, API-03, API-04, API-05

**Success Criteria:**
1. Memory payloads are AES-GCM-256 encrypted before disk storage
2. User can export/import encrypted identity for backup/recovery
3. REST API accepts authenticated requests to add/query memories
4. Query returns ranked results by text similarity
5. No plaintext data visible in storage files or network traces

**Key Components:**
- Crypto package (AES-GCM, key derivation)
- REST API server ( Gin or stdlib http)
- Query engine (basic text search)
- CLI commands: query (local)

---

## Phase 3: Networking Core

**Goal:** Establish libp2p networking and peer management

**Requirements:** CLI-02, CLI-05, CLI-06, NETW-01, NETW-02, NETW-04

**Success Criteria:**
1. `dmgn start` launches daemon with libp2p host
2. Node discovers peers via mDNS on local network
3. Node discovers peers via DHT on global network
4. `dmgn status` shows peer count and network stats
5. `dmgn peers` lists connected peers with IDs

**Key Components:**
- Network package (libp2p host, DHT, mDNS)
- Daemon mode (background process)
- Peer management (connection, health checks)

---

## Phase 4: Distributed Storage

**Goal:** Distribute encrypted shards across peers

**Requirements:** DIST-01, DIST-02, DIST-03, DIST-04, DIST-05, NETW-03

**Success Criteria:**
1. Memories are split into encrypted shards before distribution
2. Each shard stored on >=3 peers for redundancy
3. No single peer can reconstruct original data from shards alone
4. System rebalances when peers join or leave
5. `/memory/store/1.0.0` and `/memory/fetch/1.0.0` protocols functional

**Key Components:**
- Sharding package (split/combine logic)
- Shard placement algorithm
- Protocol handlers for store/fetch
- Redundancy management

---

## Phase 5: Query & Sync

**Goal:** Enable cross-peer search and data synchronization

**Requirements:** QUER-01, QUER-02, QUER-03, QUER-04, QUER-05, SYNC-01, SYNC-02, SYNC-03, SYNC-04

**Success Criteria:**
1. Query engine generates embeddings via configurable provider
2. Vector similarity search uses local HNSW index
3. Cross-peer queries aggregate and rank results
4. New memories propagate via gossip to connected peers
5. Reconnected peers receive missed updates efficiently

**Key Components:**
- Embedding provider interface (OpenAI, local)
- Vector index (HNSW implementation)
- Query orchestrator (local + remote)
- Gossip protocol (pubsub integration)
- Delta sync mechanism

---

## Phase 6: MCP & Polish

**Goal:** Full MCP protocol support and production readiness

**Requirements:** MCP-01, MCP-02, MCP-03, MCP-04, MCP-05, INTG-01, INTG-02, SAFE-01, NETW-05

**Success Criteria:**
1. MCP server runs on stdio with JSON-RPC 2.0
2. `add_memory`, `query_memory`, `get_context` tools functional
3. AI agents can integrate DMGN as memory backend
4. System has comprehensive logging and metrics
5. Documentation complete for users and developers

**Key Components:**
- MCP server package (stdio, JSON-RPC)
- Tool implementations
- Logging and telemetry
- Documentation and examples

---

## Phase 7: Daemon Architecture & CLI Restructure

**Goal:** Restructure around a persistent background daemon with integrated MCP server and automatic peer networking

**Requirements:** DAEMON-01, DAEMON-02, DAEMON-03, DAEMON-04, DAEMON-05, DAEMON-06, DAEMON-07

**Success Criteria:**
1. `dmgn start` launches a background daemon that connects to peers via bootnodes
2. Daemon auto-serves MCP on stdio — no separate `mcp-serve` command needed
3. AI agents connect to DMGN via stdio MCP protocol (standard MCP config)
4. `dmgn stop` gracefully stops the background daemon
5. Daemon persists in background until explicitly stopped via `dmgn stop`
6. All existing memory/query/network features work through the daemon
7. CLI commands that need daemon (query, peers, status) communicate with running daemon

**Key Components:**
- Background daemon process management (PID file, fork/detach)
- Integrated MCP server in daemon (replaces standalone mcp-serve)
- `dmgn stop` command for daemon lifecycle
- CLI restructuring for daemon-centric model
- Bootnode auto-connect on daemon start

---

## Phase 8: Networking Enhancements

**Goal:** Add QUIC transport, NAT traversal, and networking layer security for production-grade P2P connectivity

**Requirements:** NETW-02, NETW-04, NETW-06, NETW-07, NETW-08, NETW-09

**Success Criteria:**
1. Node listens on both TCP and QUIC v1 transports
2. QUIC transport functional for peer connections
3. Circuit Relay v2 enables nodes behind NAT to be reachable
4. Direct hole punching reduces relay dependency
5. Configuration supports listen address arrays and NAT options
6. Connection gater blocks low-reputation and explicitly blocked peers
7. Resource Manager enforces per-peer connection and stream limits
8. Protocol handlers rate-limited per peer
9. Peer blocklist/allowlist configurable via config

**Key Components:**
- QUIC transport configuration (quic-v1 multiaddr)
- Circuit Relay v2 service (relay for other peers)
- Hole punching (direct NAT traversal)
- TURN fallback configuration
- Updated config struct (ListenAddrs array, NAT booleans)
- Connection gater integrated with ReputationManager
- libp2p Resource Manager (connection/stream limits)
- Per-peer protocol rate limiter
- Config-driven peer blocklist/allowlist

---

## Dependency Graph

```
Phase 1: Local Foundation
    ↓
Phase 2: Encryption & API (depends on Phase 1)
    ↓
Phase 3: Networking Core (can parallel with Phase 2 parts)
    ↓
Phase 4: Distributed Storage (depends on Phase 2, 3)
    ↓
Phase 5: Query & Sync (depends on Phase 2, 4)
    ↓
Phase 6: MCP & Polish (depends on all previous)
    ↓
Phase 7: Daemon Architecture & CLI Restructure (depends on all previous)
    ↓
Phase 8: Networking Enhancements (depends on Phase 3, 7)
    ↓
Phase 9: Skill Loader MCP Tool (depends on Phase 6, 7)
```

---

## Risk Areas

| Risk | Phase | Mitigation |
|------|-------|------------|
| libp2p complexity | 3 | Use proven patterns from ipfs/kubo |
| Vector search perf | 5 | Benchmark HNSW early, fallback to brute force |
| Encryption key mgmt | 2 | Extensive testing, clear UX for backup/recovery |
| NAT traversal | 3 | Use libp2p autorelay, document port forwarding |
| Shard placement | 4 | Simple consistent hashing initially |

---

## Progress Tracking

| Phase | Status | Started | Completed |
|-------|--------|---------|-----------|
| 1 | **Complete** | 2025-04-09 | 2025-04-09 |
| 2 | **Complete** | 2026-04-09 | 2026-04-09 |
| 3 | **Complete** | 2026-04-09 | 2026-04-09 |
| 4 | **Complete** | 2026-04-09 | 2026-04-09 |
| 5 | **Complete** | 2026-04-09 | 2026-04-09 |
| 6 | **Complete** | 2026-04-09 | 2026-04-09 |
| 7 | **Planned** | — | — |
| 8 | **Planned** | — | — |
| 9 | **Planned** | — | — |

---

## Phase 9: Skill Loader MCP Tool

**Goal:** Add conversational skill-trigger system for DMGN — when user mentions "dmgn" in conversation, AI agent triggers skill loading and provides full DMGN tools reference

**Requirements:** None currently defined

**Success Criteria:**
1. AI agent detects trigger phrases like "init dmgn", "dmgn context", "load dmgn" in conversation
2. On trigger, skill content loaded from `./skill/SKILL.md` file or embedded fallback
3. Full skill content (all 7 tools reference, behavioral protocol) injected to agent context
4. Build-time embedding works via go:embed for offline/disk-less scenarios
5. Both direct match and fuzzy match trigger modes functional

**Key Components:**
- Skill loader in MCP server (trigger detection, file loading)
- Embedded skill fallback (go:embed)
- Skill content format (tools reference, behavioral protocol)
- Trigger patterns (direct + fuzzy match)

**Plans:** 1 plan

**Plan list:**
- [x] 09-01-PLAN.md — Skill package + MCP load_skill tool + SKILL.md update

---

*Last updated: 2026-04-10 — Added Phase 9 for skill-loader feature*
