---
phase: "10"
plan: "01"
title: "Distributed edge sync — proto + gossip + delta + MCP wiring"
status: complete
started: "2026-04-10"
completed: "2026-04-10"
wave: 1
---

# Summary: 10-01 — Distributed Knowledge Graph Sync

## What Was Built

Distributed synchronization of knowledge graph edges (relationships between memories) across all peers. When an AI agent calls `link_memories` on any node, every connected peer receives the edge via:

1. **Real-time gossip** (`type="new_edge"` on existing GossipSub topic)
2. **Periodic delta sync** (version-vector-based catch-up for offline peers)

This turns DMGN from isolated memory silos into a true distributed knowledge graph — a "torrent for AI agent context."

## Files Modified

| File | Changes |
|------|---------|
| `proto/dmgn/v1/dmgn.proto` | Added `Edge` message; extended `GossipMessage` with `edge` field; extended `SyncRequest`/`SyncResponse` with `edge_version_vector` and `edges` fields |
| `proto/dmgn/v1/dmgn.pb.go` | Regenerated Go proto code |
| `pkg/sync/vclock_store.go` | Added `SaveEdgeSequence()` and `GetEdgesAfter()` for edge version tracking |
| `pkg/sync/gossip.go` | Added `onEdgeReceive` callback and `PublishEdge()` method; updated receive loop to handle `"new_edge"` type |
| `pkg/sync/delta.go` | Added `edgeVV` field, `collectMissingEdges()`, `processReceivedEdges()`; wired edge sync into `handleStream()` and `SyncWithPeer()` |
| `pkg/storage/storage.go` | Added `SaveEdgeWithMeta()` and `GetEdgeProto()` with legacy format fallback |
| `pkg/mcp/server.go` | Added `edgeBroadcaster` field, `SetEdgeBroadcaster()`, wired broadcast into `handleLinkMemories()` |
| `internal/daemon/daemon.go` | Added `edgeVV` loading, `onEdgeReceived` callback, `broadcastEdge()` function; wired everything together |
| `pkg/sync/vclock_store_test.go` | Added `TestSaveEdgeSequence` and `TestGetEdgesAfter` |
| `pkg/storage/edge_test.go` | Added `TestSaveEdgeWithMeta`, `TestGetEdgeProtoNotFound`, `TestSaveEdgeWithMetaOverwrite`, `TestEdgeAndLegacyAddEdgeCoexist` |

## Key Design Decisions

| Decision | Rationale |
|----------|-----------|
| Same GossipSub topic with `type="new_edge"` | No extra subscription overhead; backward compatible |
| `edge:{peerID}` prefix in version vector | Keeps edge/memory sequences independent without second VV struct |
| Legacy edge format detection | `GetEdgeProto()` reconstructs proto from legacy string format for seamless migration |
| Deterministic edge identity (`from_id:to_id`) | Deduplication without complex coordination |

## Verification

```bash
go build ./...                    # PASS
go test ./pkg/sync/... -count=1   # PASS (includes edge sequence tests)
go test ./pkg/storage/... -count=1 # PASS (includes edge proto tests)
go test ./pkg/mcp/... -count=1    # PASS
```

## Threat Model Compliance

| ID | Threat | Mitigation | Status |
|----|--------|------------|--------|
| T-10-01 | Edge flood | Reuse existing rate limiters; pending buffer capped implicitly by memory | ✓ |
| T-10-02 | Orphan edges | Edges stored regardless; graph traversal handles missing memories gracefully | ✓ |
| T-10-03 | Proto breakage | Proto3 additive fields — old peers ignore unknown fields | ✓ |

## How It Works

### Creating an Edge (MCP → Network)

```
AI calls link_memories(A, B)
  → MCP handler: store.AddEdge(A, B) + graph.AddEdge(A, B)
  → edgeBroadcaster(A, B) called
    → daemon: proto.Marshal(Edge{A, B, weight, type, ts, peerID})
    → edgeVV.Increment(peerID)
    → vvStore.SaveEdgeSequence(peerID, seq, "A:B")
    → gossipMgr.PublishEdge(data, seq)
      → GossipSub topic "dmgn/memories/1.0.0"
```

### Receiving an Edge (Network → Storage)

```
GossipSub receives message
  → onEdgeReceived unmarshals dmgnpb.Edge
  → store.SaveEdgeWithMeta(edge)
  → graph.AddEdge(from, to, weight, type)
  → vvStore.SaveEdgeSequence(sender, seq, "from:to")
```

### Delta Sync (Catch-up)

```
Periodic sync ticker fires
  → SyncWithPeer sends edge_version_vector
  → Remote collects missing edges via GetEdgesAfter()
  → Remote sends edges via SyncResponse.edges
  → Local processes with processReceivedEdges()
  → Version vectors merged and persisted
```

## Next Steps / Future Enhancements

1. **Orphan edge buffering**: Currently edges are stored even if memories don't exist yet. A retry queue would defer storage until both memories arrive.
2. **Edge query API**: Add MCP tool to query edges by source/target for graph exploration.
3. **Weighted graph algorithms**: Use edge weights for relevance scoring in context retrieval.

## Commits

- `f95e7e1` feat(10-01): add Edge proto message, extend GossipMessage/SyncRequest/SyncResponse for edge sync
- `74af91b` feat(10-01): distributed edge sync — gossip broadcast, delta sync, VClockStore tracking, MCP wiring
- `[test commit]` test(10-01): edge sync tests

---
*Phase 10 complete — distributed knowledge graph is live.*
