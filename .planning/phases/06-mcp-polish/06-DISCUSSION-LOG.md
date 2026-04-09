# Phase 6: MCP & Polish - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-04-09
**Phase:** 06-mcp-polish
**Areas discussed:** MCP Transport & Lifecycle, MCP Tool Design, Logging & Observability, Documentation & Polish

---

## MCP Transport & Lifecycle

| Option | Description | Selected |
|--------|-------------|----------|
| Same binary, separate command | dmgn mcp-serve runs stdio MCP in foreground. Claude Desktop launches it directly. | ✓ |
| Embedded in daemon | dmgn start launches both REST API and MCP stdio listener. | |
| Standalone MCP-only binary | Separate dmgn-mcp binary. Simpler but duplicates setup logic. | |

**User's choice:** Same binary, separate command
**Notes:** Keeps binary distribution simple. Claude Desktop / Cline can launch `dmgn mcp-serve` directly.

| Option | Description | Selected |
|--------|-------------|----------|
| Local-only by default | MCP server opens BadgerDB directly, no networking. | |
| Full stack with --network flag | Local-only default, --network starts libp2p + gossip + sync. | ✓ |
| Always full stack | Always starts networking. | |

**User's choice:** Full stack with --network flag
**Notes:** Offline-first default, opt-in networking for cross-peer features.

---

## MCP Tool Design

| Option | Description | Selected |
|--------|-------------|----------|
| Just the three required | add_memory, query_memory, get_context. | |
| Add link/graph tools | Also link_memories and get_graph. | |
| Add management tools too | Core + link_memories, get_graph, delete_memory, get_status. | ✓ |

**User's choice:** Add management tools too (7 total tools)
**Notes:** Full CRUD + observability for agents.

| Option | Description | Selected |
|--------|-------------|----------|
| Full decrypted content | Return full plaintext + metadata + score always. | |
| Snippet + fetch on demand | Return snippet only, separate call for full content. | |
| Configurable depth | Default snippets, include_content=true for full content. | ✓ |

**User's choice:** Configurable depth
**Notes:** Agent chooses per query. Balances response size with convenience.

---

## Logging & Observability

| Option | Description | Selected |
|--------|-------------|----------|
| Structured logging only | Go's slog, zero dependencies. | |
| Logging + basic metrics | slog + internal counters, GET /metrics as JSON. | |
| Full OpenTelemetry | OTel SDK for traces + metrics + logs. Exportable to Jaeger/Prometheus. | ✓ |

**User's choice:** Full OpenTelemetry
**Notes:** Industry standard, exportable to multiple backends.

| Option | Description | Selected |
|--------|-------------|----------|
| Stderr only | All logs to stderr. Simplest. | |
| Stderr + log file | Stderr for real-time + rotating log file in data dir. | ✓ |
| Log file only | No stderr in MCP mode. Everything to log file. | |

**User's choice:** Stderr + log file
**Notes:** Dual output for real-time monitoring and post-mortem debugging.

---

## Documentation & Polish

| Option | Description | Selected |
|--------|-------------|----------|
| README + MCP config examples | Minimal: README update + config examples. | |
| Full docs suite | docs/ folder with architecture, integration, API, CLI, config, troubleshooting. | ✓ |
| README + integration guide only | README + single INTEGRATION.md. | |

**User's choice:** Full docs suite
**Notes:** Comprehensive documentation for users and developers.

| Option | Description | Selected |
|--------|-------------|----------|
| Both included | Backup export + peer reputation scoring. | ✓ |
| Backup only, defer reputation | Backup is essential, reputation is optimization. | |
| Defer both | Focus purely on MCP + logging + docs. | |

**User's choice:** Both included
**Notes:** Full production readiness in Phase 6.

---

## Claude's Discretion

- MCP JSON-RPC 2.0 implementation approach
- OpenTelemetry SDK configuration
- Log rotation strategy
- Backup file internal structure
- Peer reputation decay algorithm
- Documentation depth and formatting

## Deferred Ideas

- gRPC API — REST sufficient for v1
- Automatic memory summarization — AI-dependent feature
- Query caching — post-v1 optimization
- Scheduled automatic backups (SAFE-02)
- Compression for long-term storage (SAFE-03)
