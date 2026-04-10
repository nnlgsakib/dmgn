# Phase 11: Hybrid Link Processing - Context

**Gathered:** 2026-04-10
**Status:** Ready for planning

<domain>
## Phase Boundary

Add automatic link/edge generation when memories are added. System automatically creates knowledge graph edges (via embedding similarity + time clustering) while preserving existing manual `link_memories` tool for AI agent explicit linking.

- **In scope:** Auto-linking on add_memory, hybrid linking algorithm (embedding + time), configurable thresholds, edge weight from similarity, auto-linked edges to network
- **Out of scope:** Changing existing link_memories tool interface, encrypted edges (already metadata-only), graph query languages

</domain>

<decisions>
## Implementation Decisions

### Auto-linking trigger
- **D-01:** Auto-linking happens immediately on `add_memory` — when a memory is created, system scans existing memories and creates edges to related ones
- **D-02:** Auto-linking also runs on incoming gossip memories from peers (edge from peer A's new memory → local related memories)

### Linking algorithm
- **D-03:** Hybrid algorithm: First check embedding similarity, then also link time-proximate memories
- **D-04:** Similarity threshold: default 0.7, configurable via config field `AutoLinkSimilarityThreshold`
- **D-05:** Time clustering: memories created within 60 minutes of each other (configurable: `AutoLinkTimeWindowMinutes`)

### Edge weight calculation
- **D-06:** Edge weight = similarity score (0.0-1.0). A 0.9 similarity = 0.9 weight preserves the confidence signal
- **D-07:** Manual links (via link_memories) can specify explicit weight or default to 1.0

### Network publishing
- **D-08:** Auto-generated edges broadcast to network via gossip (same as manual links — Phase 10)
- **D-09:** Edge type field distinguishes "auto" vs "manual" edges — `edge_type: "auto"` or `"manual"`

### Existing manual linking preserved
- **D-10:** `link_memories` MCP tool unchanged — AI agents can still manually create links
- **D-11:** Manual links take precedence over auto-links (if conflict, keep manual)

### Agent's Discretion
- Exact embedding similarity calculation (cosine vs dot product)
- Max auto-links per memory (prevent explosion)
- Logging verbosity for auto-linking events

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Memory and storage
- `pkg/memory/memory.go` — Memory struct, Edge struct, Graph struct
- `pkg/storage/storage.go` — AddEdge(), GetEdges(), existing edge storage
- `pkg/mcp/server.go` — handleAddMemory() (line ~264), existing add_memory handler

### MCP tools
- `pkg/mcp/server.go` — handleLinkMemories() (line ~389), existing manual link tool
- `.planning/phases/10-graph-sync/10-CONTEXT.md` — Edge sync decisions (D-10-01 through D-10-05)

### Embedding and similarity
- `pkg/query/index.go` — Existing vector index, similarity calculation
- `internal/config/config.go` — Config struct needs new fields for thresholds

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `handleAddMemory()` in MCP server — hook point for auto-linking
- `handleLinkMemories()` — existing manual linking to preserve
- Vector index for similarity calculations (embedding-based ranking already exists)
- Gossip broadcast for edges from Phase 10

### Established Patterns
- Embedding similarity already used in query ranking (hybrid scoring)
- Edge type field available in Edge struct for "auto" vs "manual" distinction
- Delta sync supports edge propagation (Phase 10)

### Integration Points
- Add auto-linking call in handleAddMemory after memory is saved
- Add config fields: AutoLinkSimilarityThreshold, AutoLinkTimeWindowMinutes, EnableAutoLink
- Broadcast auto-edges via same gossip path as manual edges

</code_context>

<specifics>
## Specific Ideas

- "I want it to work like my brain — when I add something related, it auto-connects to similar memories"
- "But I still want the AI to be able to manually link things when it knows better"
- Auto-links should publish to network so peers benefit too
- Edge weight should reflect confidence — similar memories = stronger connection

</specifics>

<deferred>
## Deferred Ideas

- Periodic background auto-linking (run every N minutes to catch relationships missed during offline) — defer to future phase
- Auto-unlinking (remove stale edges when memories deleted) — defer to future phase
- Graph traversal suggestions (AI asks "did you mean to link this?") — defer to future phase

---

*Phase: 11-hybrid-link-processing*
*Context gathered: 2026-04-10*