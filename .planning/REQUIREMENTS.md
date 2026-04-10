# Requirements: Distributed Memory Graph Network (DMGN)

**Defined:** 2025-04-09
**Core Value:** User owns their identity and memory data that persists across devices and time, with no central server or third-party control.

## v1 Requirements

### Identity Layer

- [ ] **IDTY-01**: System generates ed25519 keypair on first run
- [ ] **IDTY-02**: Private key is encrypted with user passphrase and stored locally
- [ ] **IDTY-03**: Public key serves as user ID (base58 encoded)
- [ ] **IDTY-04**: User can export encrypted key for backup
- [ ] **IDTY-05**: User can import key to recover identity on new device

### Memory Model

- [ ] **MEMO-01**: Memory nodes have SHA256 content-addressable IDs
- [ ] **MEMO-02**: Memory structure includes timestamp, type, embedding, encrypted_payload, links, merkle_proof
- [ ] **MEMO-03**: Memory can link to other memories via directed graph edges
- [ ] **MEMO-04**: Memory types are extensible (text, image, conversation, etc.)

### Local Storage Layer

- [ ] **STOR-01**: Use BadgerDB as LSM storage engine
- [ ] **STOR-02**: Maintain time-based index for chronological queries
- [ ] **STOR-03**: Store full recent memories (configurable retention N)
- [ ] **STOR-04**: Store graph edges for relationship traversal
- [ ] **STOR-05**: Support atomic batch writes for consistency

### Encryption Layer

- [ ] **CRPT-01**: All memory payloads encrypted with AES-GCM-256
- [ ] **CRPT-02**: Each memory has unique per-memory encryption key
- [ ] **CRPT-03**: Per-memory keys encrypted with master key derived from identity
- [ ] **CRPT-04**: Merkle proofs calculated on encrypted data
- [ ] **CRPT-05**: No plaintext data transmitted over network

### CLI Interface

- [ ] **CLI-01**: `dmgn init` - Initialize node with identity generation
- [ ] **CLI-02**: `dmgn start` - Start node daemon with networking
- [ ] **CLI-03**: `dmgn add <text>` - Add memory to local store
- [ ] **CLI-04**: `dmgn query <text>` - Search memories by similarity
- [ ] **CLI-05**: `dmgn status` - Show node status, peer count, storage stats
- [ ] **CLI-06**: `dmgn peers` - List connected peers

### REST API

- [ ] **API-01**: `POST /memory` - Add new memory via HTTP
- [ ] **API-02**: `GET /query?q=<text>` - Query memories via HTTP
- [ ] **API-03**: `GET /status` - Get node status via HTTP
- [ ] **API-04**: API uses JSON for request/response bodies
- [ ] **API-05**: API requires authentication (API key derived from identity)

### MCP Interface

- [ ] **MCP-01**: Implement stdio-based MCP protocol
- [ ] **MCP-02**: Tool `add_memory` accepts text content and optional links
- [ ] **MCP-03**: Tool `query_memory` accepts query text and returns results
- [ ] **MCP-04**: Tool `get_context` returns recent context for AI agent
- [ ] **MCP-05**: MCP tools use JSON-RPC 2.0 format

## v2 Requirements

### Distributed Storage

- **DIST-01**: Split memories into encrypted shards for distribution
- **DIST-02**: Use libp2p DHT for peer discovery and shard location
- **DIST-03**: Maintain replication factor >= 3 for each shard
- **DIST-04**: No single peer can reconstruct full data from shards
- **DIST-05**: Automatic rebalancing when peers join/leave

### Sync Layer

- **SYNC-01**: Use libp2p pubsub (gossipsub) for memory broadcast
- **SYNC-02**: Propagate new memories to connected peers
- **SYNC-03**: Eventual consistency model with conflict resolution
- **SYNC-04**: Efficient delta sync for reconnected peers

### Query Engine

- **QUER-01**: Generate embeddings using pluggable provider (OpenAI/local)
- **QUER-02**: Local vector similarity search via HNSW index
- **QUER-03**: Fetch remote shards if local results insufficient
- **QUER-04**: Rank and merge results from multiple sources
- **QUER-05**: Query latency <100ms for local, <2s with remote

### Integrity & Recovery

- **INTG-01**: Merkle tree per memory batch for integrity
- **INTG-02**: Validate data integrity on fetch
- **INTG-03**: Detect tampering or corruption
- **RCVR-01**: Reed-Solomon erasure coding for shard recovery
- **RCVR-02**: Recover data from N-of-M shards

### Availability Management

- **AVLB-01**: Cache high-priority memories locally always
- **AVLB-02**: Peer health scoring (uptime, latency, storage)
- **AVLB-03**: Auto-replicate to new peers when existing drop
- **AVLB-04**: Graceful degradation when peers unavailable

### Local Safety

- **SAFE-01**: Full local encrypted backup export
- **SAFE-02**: Scheduled automatic snapshots
- **SAFE-03**: Compression for long-term storage
- **SAFE-04**: Import/restore from backup files

### Networking

- **NETW-01**: Peer discovery via DHT (global) and mDNS (local)
- **NETW-02**: Support TCP and QUIC transports
- **NETW-03**: Protocol handlers: /memory/store/1.0.0, /memory/fetch/1.0.0, /memory/query/1.0.0, /memory/sync/1.0.0
- **NETW-04**: NAT traversal via libp2p autorelay
- **NETW-05**: Peer reputation scoring and blacklisting

### Daemon Architecture & CLI Restructure

- [ ] **DAEMON-01**: `dmgn start` launches a background daemon process (detached from terminal)
- [ ] **DAEMON-02**: Daemon auto-connects to peers via configured bootnodes on startup
- [ ] **DAEMON-03**: Daemon integrates MCP server — no separate `mcp-serve` command needed
- [ ] **DAEMON-04**: AI agents connect to DMGN via stdio MCP protocol (standard MCP config in Claude Desktop, Cline, etc.)
- [ ] **DAEMON-05**: `dmgn stop` gracefully stops the running background daemon
- [ ] **DAEMON-06**: Daemon persists in background until explicitly stopped (PID file, process lifecycle)
- [ ] **DAEMON-07**: All daemon-dependent CLI commands (status, peers, query) communicate with the running daemon

## Out of Scope

| Feature | Reason |
|---------|--------|
| Social recovery | Complex multi-party cryptography, defer to advanced phase |
| gRPC API | REST sufficient for v1, add gRPC later if performance demands |
| Mobile app clients | Focus on CLI/API first, mobile requires additional stack |
| WebRTC transport | TCP+QUIC sufficient, WebRTC adds complexity |
| Public blockchain | Libp2p provides adequate decentralization |
| Real-time collaborative editing | CRDT complexity, defer to later versions |
| Automatic memory summarization | AI-dependent feature, defer to AI integration phase |
| Federated identity (OIDC) | Contradicts self-sovereign identity principle |

## Traceability

| Requirement | Phase | Status |
|-------------|-------|--------|
| IDTY-01 | Phase 1 | Pending |
| IDTY-02 | Phase 1 | Pending |
| IDTY-03 | Phase 1 | Pending |
| IDTY-04 | Phase 2 | Pending |
| IDTY-05 | Phase 2 | Pending |
| MEMO-01 | Phase 1 | Pending |
| MEMO-02 | Phase 1 | Pending |
| MEMO-03 | Phase 1 | Pending |
| MEMO-04 | Phase 1 | Pending |
| STOR-01 | Phase 1 | Pending |
| STOR-02 | Phase 1 | Pending |
| STOR-03 | Phase 2 | Pending |
| STOR-04 | Phase 1 | Pending |
| STOR-05 | Phase 1 | Pending |
| CRPT-01 | Phase 2 | Pending |
| CRPT-02 | Phase 2 | Pending |
| CRPT-03 | Phase 2 | Pending |
| CRPT-04 | Phase 2 | Pending |
| CRPT-05 | Phase 2 | Pending |
| CLI-01 | Phase 1 | Pending |
| CLI-02 | Phase 3 | Pending |
| CLI-03 | Phase 1 | Pending |
| CLI-04 | Phase 2 | Pending |
| CLI-05 | Phase 3 | Pending |
| CLI-06 | Phase 3 | Pending |
| API-01 | Phase 2 | Pending |
| API-02 | Phase 2 | Pending |
| API-03 | Phase 2 | Pending |
| API-04 | Phase 2 | Pending |
| API-05 | Phase 2 | Pending |
| MCP-01 | Phase 6 | Pending |
| MCP-02 | Phase 6 | Pending |
| MCP-03 | Phase 6 | Pending |
| MCP-04 | Phase 6 | Pending |
| MCP-05 | Phase 6 | Pending |

**Coverage:**
- v1 requirements: 33 total
- Mapped to phases: 33
- Unmapped: 0 ✓

---
*Requirements defined: 2025-04-09*
*Last updated: 2025-04-09 after initial definition*
