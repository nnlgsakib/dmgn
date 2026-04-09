# Phase 5: Query & Sync — Research

**Researched:** 2026-04-09
**Phase:** 05-query-sync
**Requirements:** QUER-01, QUER-02, QUER-03, QUER-04, QUER-05, SYNC-01, SYNC-02, SYNC-03, SYNC-04

## 1. Pure Go HNSW Libraries

### github.com/coder/hnsw (Recommended)
- **Pure Go**, zero CGo dependencies
- Generic type parameter for keys: `hnsw.NewGraph[int]()`
- Built-in persistence: `Graph.Export(io.Writer)` / `Graph.Import(io.Reader)` — binary encoding at near disk speed
- `SavedGraph` convenience type for file-based persistence
- Benchmark: 100 vectors × 256 dims → Export 1232 MB/s, Import 796 MB/s on M3
- API: `g.Add(MakeNode(key, []float32{...}))`, `g.Search(vec, k)` → returns `[]Node`
- Tunable: `Graph.M` (max neighbors), `Graph.Ml` (level generation)
- Memory: `n * log(n) * size(key) * M` graph overhead + `n * d * 4` vector data
- License: Apache 2.0

### github.com/Bithack/go-hnsw
- Also pure Go, older library
- Less active maintenance, no generics
- Manual ID management (IDs must start from 1)

### github.com/TFMV/hnsw
- Fork of coder/hnsw with additional features
- Added optimizations but less community adoption

**Decision:** Use `github.com/coder/hnsw` — best maintained, generic API, built-in persistence, Apache 2.0 license.

### Integration with Encrypted Persistence (D-05)
The `Graph.Export(io.Writer)` / `Graph.Import(io.Reader)` API works perfectly with encrypted persistence:
1. Export graph to `bytes.Buffer`
2. Encrypt buffer with AES-GCM (derive key with HKDF purpose `"vector-index"`)
3. Write encrypted bytes to `{data_dir}/vector-index.enc`
4. On startup: read file → decrypt → `Graph.Import(reader)`

## 2. libp2p GossipSub (go-libp2p-pubsub)

### Library: `github.com/libp2p/go-libp2p-pubsub`
- Canonical pubsub for libp2p ecosystem
- Three routers: FloodSub, RandomSub, **GossipSub** (recommended)
- API flow:
  ```go
  ps, _ := pubsub.NewGossipSub(ctx, host)
  topic, _ := ps.Join("dmgn/memories/1.0.0")
  sub, _ := topic.Subscribe()
  // Publish
  topic.Publish(ctx, msgBytes)
  // Receive
  msg, _ := sub.Next(ctx)
  ```
- GossipSub v1.1 features: peer exchange, adaptive gossip factor, flood publish
- Already in the libp2p dependency tree (DMGN uses `go-libp2p v0.48.0`)

### Topic Design
- Single topic: `dmgn/memories/1.0.0` for all memory broadcasts
- Message format: JSON envelope with full encrypted memory
- Future: could add per-namespace topics if multi-tenant needed

### Message Validation
- Use `topic.RegisterTopicValidator()` for message validation
- Validate: JSON parseable, memory_id not empty, sender_peer_id matches message sender
- Reject malformed messages before propagation

### Integration with Host
- Create GossipSub in `Host.Start()` after DHT setup
- Store `*pubsub.PubSub` and `*pubsub.Topic` on Host struct
- Message loop runs in goroutine, feeds into local storage + index

## 3. Vector Clock / Version Vector for Delta Sync

### Design
Each peer maintains a version vector: `map[peerID]uint64` where the value is the latest sequence number seen from that peer.

### Storage
- Store version vector in BadgerDB under key `vv:{local_peer_id}`
- Each memory gets a sequence number: `seq:{peer_id}` → auto-incrementing counter
- On memory creation: increment local counter, tag memory with `(peer_id, seq)`

### Sync Protocol (`/dmgn/memory/sync/1.0.0`)
1. **Initiator** sends own version vector
2. **Responder** compares vectors, identifies missing ranges
3. **Responder** sends missing memories (newest first, up to batch limit)
4. **Responder** sends own version vector
5. **Initiator** compares, sends what responder is missing
6. Both update their version vectors

### Reconnection Trigger
- On `Connected()` notifiee callback → schedule sync after 2s debounce
- Periodic sync every `sync_interval` (default 60s) as fallback

## 4. Hybrid Scoring

### Design
```
final_score = α * vector_similarity + (1-α) * text_score
```
- `vector_similarity`: cosine similarity from HNSW search (0.0 to 1.0)
- `text_score`: existing `scoreMatch()` from query.go (0.0 to 1.0)
- `α`: configurable, default 0.7

### Fallback Behavior
- If memory has no embedding → use text score only (score = text_score)
- If query has no embedding → use text search only
- If both have embeddings → use hybrid score

## 5. Query Protocol (`/dmgn/memory/query/1.0.0`)

### Request Format
```json
{
  "query_id": "uuid",
  "embedding": [0.1, 0.2, ...],
  "text_query": "optional text fallback",
  "limit": 10,
  "filters": {
    "type": "text",
    "after": 1700000000,
    "before": 1700099999
  }
}
```

### Response Format
```json
{
  "query_id": "uuid",
  "results": [
    {
      "memory_id": "sha256...",
      "score": 0.85,
      "type": "text",
      "timestamp": 1700000123,
      "snippet": "First 100 chars of decrypted content...",
      "source_peer": "12D3Koo..."
    }
  ],
  "total_searched": 1500
}
```

### Fan-out Strategy
- Send query to all connected peers in parallel
- Per-peer timeout: 2s (QUER-05 requires <2s for remote)
- Aggregate results, deduplicate by memory_id (keep highest score)
- Apply source diversity: round-robin interleave from different peers
- Return top-K after dedup + interleave

## 6. New Config Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `embedding_dim` | int | 0 | Expected embedding dimension (0 = accept any) |
| `hybrid_score_alpha` | float64 | 0.7 | Weight for vector vs text scoring |
| `query_timeout` | string | "2s" | Per-peer query timeout |
| `sync_interval` | string | "60s" | Periodic delta sync interval |
| `gossip_topic` | string | "dmgn/memories/1.0.0" | GossipSub topic name |

## 7. Package Structure

```
pkg/
  vectorindex/           # NEW — HNSW wrapper with encrypted persistence
    index.go             # VectorIndex type: Add, Search, Save, Load
    index_test.go
  query/                 # NEW — query orchestrator
    engine.go            # QueryEngine: local + remote query, hybrid scoring
    engine_test.go
    protocol.go          # Query protocol handler (/dmgn/memory/query/1.0.0)
  sync/                  # NEW — gossip + delta sync
    gossip.go            # GossipManager: pubsub setup, message handling
    gossip_test.go
    delta.go             # DeltaSync: version vector, sync protocol
    delta_test.go
    vclock.go            # VersionVector type + BadgerDB persistence
    vclock_test.go
```

## 8. Wave Analysis (Dependency Order)

**Wave 1 (independent foundations):**
- `pkg/vectorindex/` — HNSW wrapper (no network dependency)
- `pkg/sync/vclock.go` — Version vector (no network dependency)
- Config extensions — new fields in `config.Config`

**Wave 2 (requires Wave 1):**
- `pkg/query/engine.go` — local query engine using vectorindex
- `pkg/sync/gossip.go` — GossipSub integration (requires Host)
- `pkg/sync/delta.go` — Delta sync protocol (requires vclock + Host)

**Wave 3 (requires Wave 2):**
- `pkg/query/protocol.go` — Cross-peer query protocol (requires engine + Host)
- CLI/API updates — wire query engine into `dmgn query` and `GET /query`
- `internal/cli/start.go` — Wire gossip + sync into node startup

---
*Research completed: 2026-04-09*
