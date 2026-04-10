# Phase 11: Hybrid Link Processing - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-04-10
**Phase:** 11-hybrid-link-processing
**Areas discussed:** Auto-linking trigger, Linking algorithm, Edge weight calculation

---

## Auto-linking trigger

| Option | Description | Selected |
|--------|-------------|----------|
| On add_memory | Immediately when memory is added — new memories auto-linked to related existing ones | ✓ |
| After gossip sync | When peer sends gossip message with new memories from other peers | |
| On daemon startup | On daemon startup — scan and link all existing local memories | |
| Periodic background job | Run as background job every N minutes to find new links | |

**User's choice:** On add_memory (Recommended)
**Notes:** Immediate feedback when adding memories

---

## Linking algorithm

| Option | Description | Selected |
|--------|-------------|----------|
| Embedding similarity | Calculate cosine similarity between embeddings. Link if similarity > threshold (configurable, default 0.7) | |
| Time clustering | Memories created within N minutes of each other (configurable, default 60 min). No embeddings needed. | |
| Hybrid (similarity + time) | First check similarity, additionally link time-proximate memories. Combines both approaches. | ✓ |

**User's choice:** Hybrid (similarity + time)
**Notes:** Best of both worlds — content-based AND time-based connections

---

## Edge weight calculation

| Option | Description | Selected |
|--------|-------------|----------|
| Similarity score as weight | Use the similarity score as the edge weight. 0.9 similarity = 0.9 weight, etc. | ✓ |
| Fixed weight 1.0 | All auto-links get weight 1.0 (binary). Simple but loses similarity info. | |
| Similarity × recency | Use similarity * time_proximity_factor. Recent similar memories get higher weight. | |

**User's choice:** Similarity score as weight (Recommended)
**Notes:** Preserves confidence signal in the graph

---

## Manual linking

| Option | Description | Selected |
|--------|-------------|----------|
| Preserve link_memories tool | Keep existing link_memories MCP tool for AI explicit linking | ✓ |
| Modify interface | Change link_memories tool (not requested) | |

**User's choice:** Preserve link_memories tool unchanged
**Notes:** "exiting manual one shhould be still thhee" — manual linking preserved

---

## Agent's Discretion

- Exact embedding similarity calculation (cosine vs dot product)
- Max auto-links per memory (prevent explosion)
- Logging verbosity for auto-linking events