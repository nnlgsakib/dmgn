# Phase 6: MCP & Polish - Context

**Gathered:** 2026-04-09
**Status:** Ready for planning

<domain>
## Phase Boundary

Full MCP protocol support and production readiness for DMGN. This phase delivers: a stdio-based MCP server using JSON-RPC 2.0 (`dmgn mcp-serve` command), 7 MCP tools for AI agent integration, OpenTelemetry observability (traces + metrics + logs), encrypted backup export/restore, peer reputation scoring, and comprehensive documentation suite. No new storage formats, no new network protocols, no embedding generation.

Requirements: MCP-01, MCP-02, MCP-03, MCP-04, MCP-05, INTG-01, INTG-02, SAFE-01, NETW-05

</domain>

<decisions>
## Implementation Decisions

### MCP Transport & Lifecycle
- **D-01:** MCP server runs as `dmgn mcp-serve` — a dedicated CLI command in the same binary. Claude Desktop / Cline launches it directly via stdio. No daemon required for basic operation.
- **D-02:** Local-only by default — opens BadgerDB, vector index, and crypto directly. `dmgn mcp-serve --network` flag optionally starts libp2p + gossip + delta sync for cross-peer features. This keeps startup fast and offline-first.
- **D-03:** MCP protocol over stdin/stdout using JSON-RPC 2.0. All logs go to stderr + rotating log file in data dir (never stdout — reserved for protocol).

### MCP Tool Design
- **D-04:** Seven MCP tools:
  - `add_memory` — content, type, links, embedding, metadata
  - `query_memory` — text query, embedding, limit, filters, include_content flag
  - `get_context` — returns recent N memories as context for AI agent
  - `link_memories` — create directed edge between two memory IDs
  - `get_graph` — traverse links from a memory ID, configurable depth
  - `delete_memory` — remove a memory by ID
  - `get_status` — node status, peer count, storage stats, vector index size
- **D-05:** `query_memory` returns snippets (100 chars) by default. Optional `include_content: true` parameter returns full decrypted content. Agent chooses per query.
- **D-06:** All tools accept and return JSON. Embeddings are `[]float32` arrays. Tool descriptions follow MCP schema with inputSchema for each tool.

### Logging & Observability
- **D-07:** Full OpenTelemetry SDK for traces + metrics + logs. Exportable to Jaeger/Prometheus/OTLP collectors.
- **D-08:** Key metrics: memory_count, query_latency_ms, peer_count, sync_events, gossip_messages, vector_index_size, shard_count.
- **D-09:** Logs: dual output — stderr for real-time + rotating log file in `{data_dir}/logs/`. Use Go's slog with OTel bridge for structured logging. Levels: debug/info/warn/error configurable via config.
- **D-10:** In MCP mode (`dmgn mcp-serve`), stdout is exclusively JSON-RPC. Logs go to stderr + file. In daemon mode (`dmgn start`), logs go to stdout + file.

### Backup & Safety
- **D-11:** `dmgn backup` exports full encrypted backup (BadgerDB snapshot + vector index + config) to a single `.dmgn-backup` file. `dmgn restore` imports from backup file.
- **D-12:** Backup format: tar.gz containing encrypted BadgerDB backup stream + encrypted vector index + config.json. All data remains encrypted — backup is safe to store anywhere.

### Peer Reputation Scoring
- **D-13:** Track per-peer metrics: uptime ratio, response latency, successful syncs, failed requests. Store in BadgerDB with `rep:{peer_id}` key prefix.
- **D-14:** Reputation score influences shard placement preference and query fan-out priority. Low-reputation peers deprioritized but not blacklisted (unless manually).

### Documentation
- **D-15:** Full docs suite in `docs/` folder:
  - `architecture.md` — System overview, component diagram, data flow
  - `mcp-integration.md` — Guide for Claude Desktop, Cline, custom agents with config examples
  - `api-reference.md` — REST API endpoints with request/response examples
  - `cli-reference.md` — All CLI commands with flags and examples
  - `config-reference.md` — All config fields with defaults and descriptions
  - `troubleshooting.md` — Common issues and solutions
- **D-16:** Update README.md with MCP section, quick start, and links to docs.
- **D-17:** Example configs: `examples/claude_desktop_config.json`, `examples/cline_config.json`.

### Claude's Discretion
- MCP JSON-RPC 2.0 implementation approach (custom or library)
- OpenTelemetry SDK configuration and default exporter setup
- Log rotation strategy (size-based, time-based, max files)
- Backup file internal structure details
- Peer reputation decay/refresh algorithm
- Documentation depth and formatting style

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Project Specs
- `.planning/PROJECT.md` — Core value, constraints (offline-first, no plaintext over network, stdio MCP)
- `.planning/REQUIREMENTS.md` — MCP-01 through MCP-05, INTG-01, INTG-02, SAFE-01, NETW-05
- `.planning/ROADMAP.md` §Phase 6 — Success criteria, key components

### MCP Protocol
- MCP specification: stdio transport, JSON-RPC 2.0, tool schema with inputSchema
- Reference: https://modelcontextprotocol.io/docs

### Prior Phase Artifacts
- `.planning/phases/05-query-sync/05-CONTEXT.md` — Caller-provided embeddings, vector index design, query engine, gossip + delta sync
- `.planning/phases/04-distributed-storage/04-CONTEXT.md` — Shard protocols, storage backend patterns
- `.planning/phases/03-networking-core/03-CONTEXT.md` — libp2p host lifecycle, config patterns

### Existing Code
- `internal/api/server.go` — Server struct with all Phase 5 components wired. Pattern for MCP server.
- `internal/api/handlers.go` — REST handler pattern: HandleAddMemory, HandleQuery — MCP tools mirror these.
- `internal/cli/start.go` — Full node startup wiring. Pattern for `mcp-serve` command.
- `pkg/query/engine.go` — QueryEngine with BuildRequest/SearchLocal — reuse for query_memory tool.
- `pkg/vectorindex/index.go` — VectorIndex with Load/Save/Add/Search — reuse in MCP server.
- `pkg/storage/storage.go` — Store with DB() accessor, SaveMemory, GetRecentMemories, Backup.
- `pkg/memory/memory.go` — Memory struct, Graph with TraverseFrom for get_graph tool.
- `internal/config/config.go` — Config pattern for new OTel/MCP fields.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- **`query.QueryEngine`** (`pkg/query/engine.go`): Direct reuse for `query_memory` MCP tool — BuildRequest + SearchLocal
- **`vectorindex.VectorIndex`** (`pkg/vectorindex/index.go`): Load/Save/Add/Search for MCP server
- **`storage.Store`** (`pkg/storage/storage.go`): SaveMemory, GetRecentMemories, GetMemory, Backup for all MCP tools
- **`memory.Graph.TraverseFrom()`** (`pkg/memory/memory.go`): Direct reuse for `get_graph` tool
- **`crypto.Engine`** (`internal/crypto/crypto.go`): Encrypt/Decrypt for MCP server
- **Existing CLI pattern** (`internal/cli/*.go`): Cobra command structure for `mcp-serve` command

### Established Patterns
- **Config-driven tunables**: All new settings in `config.Config` with JSON tags and defaults
- **Identity-based key derivation**: `id.DeriveKey(purpose, size)` for MCP-specific keys if needed
- **Graceful shutdown**: `context.WithCancel` + signal handling in `start.go` — reuse for `mcp-serve`
- **Setter injection**: `server.SetQueryEngine()` pattern for wiring dependencies

### Integration Points
- `cmd/dmgn/main.go` — Add `mcp-serve` command registration
- `internal/config/config.go` — Add OTel endpoint, log file path, MCP-specific config fields
- `internal/cli/` — New `mcp.go` file for `dmgn mcp-serve` command
- `pkg/` — New `mcp/` package for MCP server, JSON-RPC handler, tool implementations
- `docs/` — New documentation directory

</code_context>

<specifics>
## Specific Ideas

- MCP is the primary AI agent interface — REST API is secondary. MCP tools should be the most polished part of DMGN.
- Claude Desktop config example should be copy-paste ready: `{"mcpServers": {"dmgn": {"command": "dmgn", "args": ["mcp-serve"]}}}`
- `get_context` should be optimized for AI agent context windows — return recent memories formatted for LLM consumption (structured, concise, with timestamps)
- Backup must preserve all data needed for full restore on a new device: identity + storage + vector index + config

</specifics>

<deferred>
## Deferred Ideas

- **gRPC API** — REST sufficient for v1, add gRPC later if performance demands
- **Automatic memory summarization** — AI-dependent, out of scope for infrastructure layer
- **Query caching** — Local cache for frequent queries, optimize post-v1
- **Scheduled automatic backups** (SAFE-02) — Manual backup first, scheduled later
- **Compression for long-term storage** (SAFE-03) — Future optimization

</deferred>

---

*Phase: 06-mcp-polish*
*Context gathered: 2026-04-09*
