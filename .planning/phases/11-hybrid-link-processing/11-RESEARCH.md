# Phase 11: Hybrid Link Processing - Research

**Researched:** 2026-04-10
**Phase:** 11-hybrid-link-processing

---

## Technical Approach

### 1. Auto-Linking Trigger

**Where to hook:** `pkg/mcp/server.go` — `handleAddMemory()` (line 264)

The current flow:
1. Memory created via `memory.Create()` 
2. Saved to storage via `s.store.SaveMemory(mem)`
3. Added to vector index: `s.vecIndex.Add(mem.ID, mem.Embedding)` (if embedding exists)
4. Broadcast to network: `s.onBroadcast(mem)`

**Auto-linking insertion point:** After step 3 (vector index update), before broadcasting. The memory already has its ID and embedding assigned.

### 2. Similarity Calculation

**Existing infrastructure:** `pkg/vectorindex/index.go` provides:
- `Search(query []float32, k int) []SearchResult` — returns top-k similar memories with cosine similarity scores (0.0 to 1.0)
- `cosineSimilarity()` function (line 216) — computes cosine similarity between two vectors

**Approach:** Reuse existing vector index for similarity search. Use `Search()` with same memory's embedding to find similar memories.

### 3. Time Clustering

**Memory timestamp:** `memory.Memory` has `Timestamp` field (time.Time)

**Approach:** Query memories with timestamps within N minutes of the new memory. Storage provides `GetRecentMemories(limit)` but we need time-based filtering.

**Implementation:** Get all memories or recent memories, then filter in Go by timestamp difference:
```go
timeDiff := mem.Timestamp.Sub(existing.Timestamp).Abs()
withinWindow := timeDiff <= time.Duration(cfg.AutoLinkTimeWindowMinutes)*time.Minute
```

### 4. Edge Creation

**Existing pattern:** `handleLinkMemories()` (line 389) shows the pattern:
```go
s.store.AddEdge(fromID, toID, weight, edgeType)
graph.AddEdge(fromID, toID, weight, edgeType)  // in-memory
s.edgeBroadcaster(fromID, toID, weight, edgeType)  // network
```

**Edge struct needs:** `edge_type` field (already exists in Edge struct per Phase 10) to distinguish "auto" vs "manual"

### 5. Configuration

**Config struct:** `internal/config/config.go` — need to add:
- `EnableAutoLink bool` — feature toggle
- `AutoLinkSimilarityThreshold float64` — default 0.7
- `AutoLinkTimeWindowMinutes int` — default 60
- `MaxAutoLinksPerMemory int` — cap to prevent explosion

---

## Implementation Pattern

```go
// In handleAddMemory(), after vecIndex.Add()
if cfg.EnableAutoLink && mem.Embedding != nil && s.vecIndex != nil {
    // 1. Find similar memories by embedding
    similarResults := s.vecIndex.Search(mem.Embedding, cfg.MaxAutoLinksPerMemory)
    
    for _, result := range similarResults {
        if result.Score >= cfg.AutoLinkSimilarityThreshold && result.MemoryID != mem.ID {
            // Create auto-edge with similarity as weight
            s.store.AddEdge(mem.ID, result.MemoryID, result.Score, "auto")
            // Also broadcast to network
            if s.edgeBroadcaster != nil {
                s.edgeBroadcaster(mem.ID, result.MemoryID, result.Score, "auto")
            }
        }
    }
    
    // 2. Find time-proximate memories
    recent, _ := s.store.GetRecentMemories(100) // recent memories
    for _, recentMem := range recent {
        if recentMem.ID == mem.ID {
            continue
        }
        timeDiff := mem.Timestamp.Sub(recentMem.Timestamp)
        if timeDiff.Abs() <= time.Duration(cfg.AutoLinkTimeWindowMinutes)*time.Minute {
            // Add time-based edge
            s.store.AddEdge(mem.ID, recentMem.ID, 0.5, "auto")
            // Only broadcast if not already linked by similarity
        }
    }
}
```

---

## Considerations

| Consideration | Approach |
|---------------|----------|
| Duplicate edges | Store checks for existing edge before adding |
| Circular edges | Allow — graph handles cycles |
| Performance | Search limited to k results, configurable |
| No embedding | If `mem.Embedding` is nil, skip similarity linking, use time-only |
| Max edges | Configurable cap to prevent memory explosion |

---

## Common Pitfalls

1. **Duplicate edge creation** — Check if edge exists before adding (`store.GetEdges()` or store method)
2. **Self-linking** — Don't link memory to itself (`result.MemoryID != mem.ID`)
3. **Broadcasting duplicates** — Don't broadcast if edge already exists from similarity+time both
4. **Race condition** — If gossip arrives with same edge, idempotent handling needed

---

## Alternative Approaches

| Alternative | Trade-off |
|-------------|-----------|
| Query engine instead of vector index | More flexible but heavier weight |
| Separate goroutine for async auto-linking | Non-blocking but harder to debug |
| Periodic batch job | Better for catching missed links but delayed feedback |

**Recommendation:** Inline synchronous auto-linking for immediate feedback. User explicitly requested "auto-connects to similar memories" — immediate feedback matches user mental model.