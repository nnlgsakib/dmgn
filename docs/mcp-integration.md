# MCP Integration Guide

DMGN works as an MCP (Model Context Protocol) server, allowing AI agents to store and retrieve persistent memories via stdio.

## Prerequisites

1. Build DMGN: `go build -o dmgn ./cmd/dmgn`
2. Initialize identity: `dmgn init`
3. Ensure `dmgn` is on your PATH

## Claude Desktop Setup

Add to your Claude Desktop configuration (`claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "dmgn": {
      "command": "dmgn",
      "args": ["mcp-serve"],
      "env": {}
    }
  }
}
```

**With custom data directory:**
```json
{
  "mcpServers": {
    "dmgn": {
      "command": "dmgn",
      "args": ["mcp-serve", "--data-dir", "/path/to/your/dmgn-data"],
      "env": {}
    }
  }
}
```

## Cline Setup (VS Code)

Add to your Cline MCP settings:

```json
{
  "mcpServers": {
    "dmgn": {
      "command": "dmgn",
      "args": ["mcp-serve", "--data-dir", "~/.dmgn"],
      "disabled": false
    }
  }
}
```

## Available MCP Tools

### `add_memory`
Store a new memory with content, type, links, embedding, and metadata.

**Input:**
| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `content` | string | yes | The text content to store |
| `type` | string | no | Memory type: `text`, `conversation`, `observation`, `document` |
| `links` | string[] | no | IDs of related memories to link to |
| `embedding` | float32[] | no | Pre-computed embedding vector |
| `metadata` | object | no | Key-value metadata pairs |

**Output:** `{ id, timestamp, type }`

### `query_memory`
Search memories by text query and/or embedding vector.

**Input:**
| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `query` | string | no | Text search query |
| `embedding` | float32[] | no | Embedding vector for semantic search |
| `limit` | int | no | Max results (default 10) |
| `include_content` | bool | no | Return full decrypted content instead of snippets |
| `filter_type` | string | no | Filter by memory type |
| `filter_after` | int64 | no | Unix nano timestamp lower bound |
| `filter_before` | int64 | no | Unix nano timestamp upper bound |

**Output:** `{ results: [{ memory_id, score, type, timestamp, content/snippet }], count }`

### `get_context`
Get recent memories formatted as context for AI agents.

**Input:**
| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `limit` | int | no | Number of recent memories (default 10) |

**Output:** `{ memories: [{ id, content, type, timestamp, time_ago, metadata }], count, context_window_hint }`

### `link_memories`
Create a directed edge between two memories.

**Input:**
| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `from_id` | string | yes | Source memory ID |
| `to_id` | string | yes | Target memory ID |
| `weight` | float32 | no | Edge weight (default 1.0) |
| `edge_type` | string | no | Relationship type (default "related") |

**Output:** `{ from_id, to_id, edge_type, created }`

### `get_graph`
Traverse the memory graph from a starting memory ID.

**Input:**
| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `start_id` | string | yes | Starting memory ID |
| `max_depth` | int | no | Maximum traversal depth (default 3) |

**Output:** `{ root_id, nodes: [{ id, type, timestamp, link_count }], edges: [{ from, to, type, weight }], depth }`

### `delete_memory`
Delete a memory by ID.

**Input:**
| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `id` | string | yes | Memory ID to delete |

**Output:** `{ id, deleted }`

### `get_status`
Get node status including memory count, vector index size, and config summary.

**Input:** (none)

**Output:** `{ node_id, version, memory_count, edge_count, vector_index_size, storage_path }`

## Network Mode

By default, `dmgn mcp-serve` runs local-only for fast startup and offline operation.

To enable P2P features (cross-peer queries, gossip sync):

```bash
dmgn mcp-serve --network
```

This starts the libp2p host, GossipSub, and delta sync alongside the MCP server.

## Troubleshooting

### MCP server not responding
- Ensure `dmgn` binary is on PATH
- Check stderr output for errors (stdout is reserved for JSON-RPC)
- Verify identity exists: `dmgn status`

### "No identity found" error
- Run `dmgn init` to create a new identity
- Enter your passphrase when `mcp-serve` starts

### Query returns no results
- Verify memories exist: use `get_status` tool to check `memory_count`
- For semantic search, ensure embeddings are provided in `add_memory` and `query_memory`
- Text search requires matching words in content
