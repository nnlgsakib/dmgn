# Phase 5: Query & Sync - Discussion Log

**Date:** 2026-04-09
**Mode:** Interactive discuss

## Areas Discussed

### 1. Embedding Provider Strategy
| # | Question | Options | Selected |
|---|----------|---------|----------|
| 1 | How should DMGN generate vector embeddings? | Pluggable+OpenAI / Local-only / Pluggable no default / You decide | You decide |
| 2 | Embedding dimension target? | 384 / 768 / 1536 / You decide | You decide |
| 3 | (Revisited) Low-end device concern — how to handle? | Small local 384-dim / Optional external+local fallback / TF-IDF/BM25 | Free-text: AI agents provide embeddings |
| 4 | Embeddings provided by caller? | Yes caller-provided / External API only / Both | Yes, embeddings provided by caller |

**Key insight from user:** DMGN is a distributed memory layer for AI — NOT a computation platform. AI agents generate embeddings, DMGN just stores and indexes them. Zero ML dependencies.

### 2. Vector Index & Search Approach
| # | Question | Options | Selected |
|---|----------|---------|----------|
| 1 | HNSW library approach? | Pure Go / CGo binding / Custom | CGo binding → later revised to Pure Go |
| 2 | Index + encrypted data handling? | Decrypt-on-startup / Persist encrypted / Embeddings-only | Persist encrypted index |
| 3 | Keep brute-force text search? | Keep both / Replace / Hybrid | Hybrid scoring |
| 4 | (Revisited) Reconsider CGo given low-end device constraint? | Switch to pure Go / Keep CGo / You decide | Switch to pure Go HNSW |

### 3. Cross-Peer Query Orchestration
| # | Question | Options | Selected |
|---|----------|---------|----------|
| 1 | Query routing strategy? | Fan-out / DHT-routed / Gossip relay / You decide | You decide |
| 2 | Result ranking/merging? | Score-based / Score+recency / Score+diversity | Score + source diversity |
| 3 | Remote result content? | Metadata only / Full encrypted / Snippet+full on request | Snippet + full on request |

### 4. Gossip Sync & Delta Mechanism
| # | Question | Options | Selected |
|---|----------|---------|----------|
| 1 | Memory propagation method? | GossipSub / Direct push / You decide | libp2p GossipSub topic |
| 2 | Gossip message content? | Announcement only / Announcement+embedding / Full encrypted | Full encrypted memory |
| 3 | Delta sync mechanism? | Timestamp-based / Bloom filter / Vector clock | Vector clock / version vector |
| 4 | Conflict resolution? | Last-writer-wins / Keep both / You decide | Keep both (no dedup) |

## User Clarifications

- **"DMGN is a distributed memory layer for AI for persistent data — it should NOT be a computation platform"**
- **Reference:** Similar to [claude-mem](https://github.com/thedotmack/claude-mem) but distributed
- **Target users:** AI agents (Claude Code, Cline, etc.) that need cross-device memory
- **Low-end device concern:** Must be usable on CPU-only, low-end devices — no heavy ML dependencies

---
*Discussion log: 2026-04-09*
