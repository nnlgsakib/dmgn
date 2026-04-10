# Phase 10: Distributed Knowledge Graph Sync — Context

## Phase Boundary

**In scope:**
- Sync explicit edges (from `link_memories`) across all peers via gossip + delta sync
- Add `Edge` protobuf message and edge-aware gossip message type
- Edge-aware delta sync (version-tracked edge propagation)
- Broadcast edges on creation (from MCP `link_memories` handler)
- Rebuild in-memory Graph from synced edge data on peer startup/receive

**Out of scope:**
- Changing how Memory.Links work (already synced as part of Memory proto)
- Encrypted edge payloads (edges are metadata — from/to/weight/type — not content)
- Custom graph query languages
- Partial/chunked memory content distribution (memories already sync fully)

## Problem Statement

Current state: gossip and delta sync only propagate Memory objects. When an AI agent calls `link_memories` to create a relationship between two memories, that edge exists only on the local node. Peers receive the memories but not the knowledge graph structure connecting them. This means:

1. `get_graph` on a remote peer returns an incomplete/empty graph
2. Context continuity breaks — an agent on peer B can't traverse relationships created on peer A
3. The "knowledge container" is fragmented — each node has memories but not the semantic web between them

## Key Decisions

- **D-10-01**: Edge gossip uses the same GossipSub topic (`dmgn/memories/1.0.0`) with a new message type `"new_edge"` rather than a separate topic — simpler, fewer subscriptions
- **D-10-02**: Edges get their own sequence in the version vector (separate from memory sequences) to avoid coupling edge sync with memory sync
- **D-10-03**: Delta sync protocol bumped to `/dmgn/sync/3.0.0` — adds `repeated bytes edges` field to SyncResponse
- **D-10-04**: On receiving an edge, validate both from_id and to_id exist locally before storing — if memories haven't arrived yet, buffer the edge for retry
- **D-10-05**: Edge identity is `from_id + ":" + to_id` — deterministic, deduplicatable

## Canonical References

- `proto/dmgn/v1/dmgn.proto` — wire format definitions
- `pkg/sync/gossip.go` — GossipManager, Publish(), message handling
- `pkg/sync/delta.go` — DeltaSyncManager, SyncWithPeer(), collectMissingMemories()
- `pkg/sync/vclock.go` — VersionVector
- `pkg/sync/vclock_store.go` — VClockStore, SaveSequence(), GetMemoriesAfter()
- `pkg/storage/storage.go` — AddEdge(), GetEdges()
- `pkg/mcp/server.go` — handleLinkMemories() (line ~382)
- `internal/daemon/daemon.go` — broadcastMemory(), gossip/delta wiring (line ~220-290)
- `pkg/memory/memory.go` — Edge struct, Graph struct
