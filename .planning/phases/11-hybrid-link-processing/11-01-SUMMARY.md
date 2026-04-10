---
phase: 11-hybrid-link-processing
plan: "01"
status: complete
completed: "2026-04-10"
---

## Plan 11-01: Hybrid Link Processing

### Objective
Implement hybrid link processing — automatic memory linking on add_memory while preserving the existing manual link_memories tool.

### Tasks Completed

**Task 1: Add auto-linking config fields** ✓
- Added `EnableAutoLink` (default true) — feature toggle
- Added `AutoLinkSimilarityThreshold` (default 0.7) — similarity threshold
- Added `AutoLinkTimeWindowMinutes` (default 60) — time clustering window
- Added `MaxAutoLinksPerMemory` (default 10) — cap to prevent edge explosion

**Task 2: Implement auto-linking in MCP server** ✓
- Created `autoLinkNewMemory()` function
- Hybrid algorithm: embedding similarity + time clustering
- Edge weight = similarity score (0.0-1.0)
- edge_type = "auto" distinguishes from manual
- Broadcasts to network via gossip

**Task 3: Integrate auto-linking in handleAddMemory hook point** ✓
- Called `autoLinkNewMemory()` after vecIndex.Add, before onBroadcast
- Runs inline synchronously for immediate feedback

### What Was Built

- **Config fields** in `internal/config/config.go`: 4 new auto-linking config options
- **Auto-linking function** in `pkg/mcp/server.go`: `autoLinkNewMemory()` method
- **Hook integration**: Auto-linking triggers on every `add_memory` call

### Key Design Decisions

1. **Inline synchronous**: Immediate feedback matches user mental model ("auto-connects to similar memories right away")
2. **Hybrid algorithm**: Both similarity + time clustering for richer graph
3. **Edge weight = confidence**: 0.9 similarity = 0.9 weight preserves confidence signal
4. **Dual linking**: Similarity-based edges get exact score, time-based edges get 0.5 weight

### Verification

- [x] `go build ./...` compiles without errors
- [x] Config fields added correctly (4 new fields with defaults)
- [x] Auto-linking function implemented (`autoLinkNewMemory`)
- [x] handleAddMemory calls auto-linking (line 368)
- [x] Edge type "auto" distinguishes from manual

### Success Criteria

- [x] Adding a memory triggers auto-linking to similar/time-proximate memories
- [x] Similarity threshold configurable (default 0.7)
- [x] Edge weight reflects confidence (similarity score)
- [x] Edges broadcast to peers via gossip
- [x] manual link_memories tool unchanged
- [x] Auto-links have edge_type = "auto"

---

*Plan 11-01 complete: 2026-04-10*