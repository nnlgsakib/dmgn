# DMGN Architecture

## System Overview

DMGN (Distributed Memory Graph Network) is a decentralized, encrypted memory layer for AI agents. It enables persistent, user-owned memory that works across devices without central servers.

## Component Diagram

```
                    ┌──────────────────────────────────────────┐
                    │              AI Agent                      │
                    │  (Claude Desktop / Cline / Custom)         │
                    └──────────────┬───────────────────────────┘
                                   │ stdin/stdout (JSON-RPC 2.0)
                    ┌──────────────▼───────────────────────────┐
                    │           MCP Server                      │
                    │   7 tools: add, query, context,           │
                    │   link, graph, delete, status              │
                    └──────────────┬───────────────────────────┘
                                   │
          ┌────────────────────────┼────────────────────────┐
          │                        │                        │
┌─────────▼──────────┐  ┌─────────▼──────────┐  ┌─────────▼──────────┐
│   Query Engine     │  │   Crypto Engine    │  │   Memory Graph     │
│  Vector + Text     │  │  AES-GCM-256      │  │  Directed edges    │
│  Hybrid scoring    │  │  Per-memory keys   │  │  Link traversal    │
└─────────┬──────────┘  └─────────┬──────────┘  └─────────┬──────────┘
          │                        │                        │
┌─────────▼────────────────────────▼────────────────────────▼──────────┐
│                        Local Storage                                  │
│   BadgerDB (encrypted KV)  │  Vector Index (brute-force cosine)      │
└─────────────────────────────┬────────────────────────────────────────┘
                              │
               ┌──────────────▼───────────────────────────┐
               │           libp2p Network                  │
               │  GossipSub │ Delta Sync │ Shard Protocol  │
               │  mDNS + DHT discovery                     │
               └──────────────────────────────────────────┘
```

## Package Map

### `cmd/dmgn/`
CLI entry point. Registers all cobra commands.

### `internal/`

| Package | Purpose |
|---------|---------|
| `cli/` | Cobra command implementations: init, add, query, start, mcp-serve, backup, restore |
| `config/` | Configuration loading, defaults, path helpers |
| `crypto/` | AES-GCM-256 encryption engine with per-memory key wrapping |
| `api/` | REST API server with Bearer token auth |

### `pkg/`

| Package | Purpose |
|---------|---------|
| `identity/` | ed25519 keypair generation, encrypted storage, HKDF key derivation |
| `memory/` | Memory struct, PlaintextMemory, Graph with directed edges, create/decrypt |
| `storage/` | BadgerDB wrapper: save, get, delete, time-indexed queries, graph persistence |
| `vectorindex/` | Brute-force cosine similarity search with encrypted persistence |
| `query/` | Hybrid query engine (vector + text), cross-peer query protocol |
| `sync/` | GossipSub message propagation, delta sync with version vectors |
| `network/` | libp2p host, peer discovery, shard routing, peer reputation |
| `sharding/` | Shamir secret sharing, shard distribution |
| `mcp/` | MCP server with 7 tools for AI agent integration |
| `backup/` | Encrypted backup export/restore (tar.gz) |
| `observability/` | OpenTelemetry traces/metrics, structured logging with rotation |

## Data Flow

### Memory Creation (via MCP)
1. AI agent calls `add_memory` tool via JSON-RPC
2. MCP server creates `PlaintextMemory` from input
3. Crypto engine encrypts: random per-memory key → AES-GCM payload → key wrapped with master key
4. Memory ID = SHA-256 of encrypted payload (content-addressable)
5. Stored in BadgerDB with time index
6. If embedding provided, indexed in vector index
7. If network enabled, published via GossipSub

### Memory Query (via MCP)
1. AI agent calls `query_memory` with text and/or embedding
2. Query engine performs hybrid search: `alpha * vector_score + (1-alpha) * text_score`
3. Results filtered by type/time, sorted by score
4. If `include_content=true`, memories decrypted before return
5. If network enabled, query fanned out to peers via query protocol

## Key Design Decisions

- **No embedding generation**: DMGN accepts pre-computed embeddings from callers
- **Offline-first**: All features work without network peers
- **Config-driven**: All tunables in `config.Config` with JSON serialization
- **Content-addressable**: Memory IDs derived from encrypted payload hash
- **Dual encryption**: Master key wraps per-memory keys, per-memory keys wrap payloads
- **Protobuf v2.0.0 for wire protocols**: Store/fetch, gossip, delta sync use protobuf; disk uses BadgerDB native; API uses JSON
