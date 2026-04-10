# Phase 10: Distributed Knowledge Graph Sync — Research

## Domain Analysis

### Current Sync Architecture

**Gossip path (real-time):**
```
Agent calls add_memory → broadcastMemory() → gossipMgr.Publish(memoryBytes, seq)
→ GossipSub topic → remote peers → onMemoryReceived → store.SaveMemory()
```

**Delta sync path (periodic catch-up):**
```
ticker fires → syncAllPeers() → SyncWithPeer(peer)
→ exchange VersionVectors → send missing memories → processReceivedMemories()
```

**Edge path (LOCAL ONLY — the bug):**
```
Agent calls link_memories → store.AddEdge(from, to, weight, type)
→ graph.AddEdge(from, to, weight, type)
→ [NOTHING SENT TO NETWORK]
```

### What Needs to Change

1. **Proto**: Add `Edge` message + edge field in gossip/sync messages
2. **Gossip**: Broadcast edges like memories with `"new_edge"` type
3. **Delta sync**: Include edges in version-vector-tracked sync
4. **Daemon wiring**: Create `broadcastEdge()` analogous to `broadcastMemory()`
5. **MCP handler**: Wire `handleLinkMemories` to broadcast
6. **Storage**: Track edge sequences for delta sync

## Technical Implementation

### 1. Proto Changes (`proto/dmgn/v1/dmgn.proto`)

```protobuf
// Edge represents a directed relationship between two memories.
message Edge {
  string from_id = 1;
  string to_id = 2;
  float weight = 3;
  string edge_type = 4;
  int64 timestamp = 5;
  string creator_peer_id = 6;
}

// Extend GossipMessage — already has `type` field for routing:
// type="new_memory" → existing flow
// type="new_edge" → new edge flow
// Reuse the `memory` bytes field for edge serialization (or add dedicated field)
```

**Option A**: Add `bytes edge = 6;` to GossipMessage (cleaner, separate field)
**Option B**: Reuse `bytes memory = 2;` with type-based dispatch (simpler proto change)

**Recommendation**: Option A — separate field. Avoids ambiguity.

### 2. GossipMessage Extension

```protobuf
message GossipMessage {
  string type = 1;
  bytes memory = 2;
  string sender_peer_id = 3;
  int64 timestamp = 4;
  uint64 sequence = 5;
  bytes edge = 6;  // NEW: serialized Edge message for type="new_edge"
}
```

### 3. SyncResponse Extension  

```protobuf
message SyncResponse {
  string sender_peer_id = 1;
  map<string, uint64> version_vector = 2;
  repeated bytes memories = 3;
  repeated bytes edges = 4;  // NEW: missing edges
}
```

Also need separate edge version tracking:
```protobuf
message SyncRequest {
  string sender_peer_id = 1;
  map<string, uint64> version_vector = 2;
  map<string, uint64> edge_version_vector = 3;  // NEW
}
```

### 4. Edge Version Tracking

Edges need their own sequence tracking in VClockStore:
- Key format: `edgeseq:{peer_id}:{zero-padded seq}` → `from_id:to_id`
- Separate version vector for edges (or shared with a `edge:` prefix in peer_id)

**Simpler approach**: Use a single version vector but with `edge:{peer_id}` keys:
```go
edgeSeq := vv.Increment("edge:" + localPeerID)
```

This avoids a second version vector while keeping edge/memory sequences independent.

### 5. Edge Buffering (D-10-04)

When an edge arrives before its referenced memories:
```go
type PendingEdge struct {
    Edge      *dmgnpb.Edge
    Received  time.Time
    Retries   int
}
```

Buffer up to 1000 pending edges. Retry on each sync tick. Drop after 5 minutes.

### 6. Daemon Wiring

```go
broadcastEdge := func(fromID, toID string, weight float32, edgeType string) {
    edge := &dmgnpb.Edge{
        FromId:        fromID,
        ToId:          toID,
        Weight:        weight,
        EdgeType:      edgeType,
        Timestamp:     time.Now().UnixNano(),
        CreatorPeerId: localPeerID,
    }
    data, _ := proto.Marshal(edge)
    edgeSeq := vv.Increment("edge:" + localPeerID)
    vvStore.SaveEdgeSequence(localPeerID, edgeSeq, fromID+":"+toID)
    gossipMgr.PublishEdge(ctx, data, edgeSeq)
}
```

## Risk Assessment

| Risk | Impact | Mitigation |
|------|--------|------------|
| Proto breaking change | Medium | Add new fields (backward compatible in proto3) |
| Edge storm (many edges at once) | Low | Rate limit edge broadcast same as memory |
| Orphan edges (memories not yet synced) | Medium | Buffer + retry pattern |
| Version vector bloat | Low | edge: prefix is clean separation |

## Dependencies

- Phase 5 (Query & Sync) — provides gossip + delta sync foundation
- Phase 8 (Networking Enhancements) — QUIC transport used for streams

## Validation

- Two-node test: create edge on node A, verify it appears on node B
- Restart test: edge persists across daemon restarts on both nodes
- Orphan test: edge arrives before memory, then memory arrives → edge applied
- Load test: 100 edges broadcast, all arrive on peer
