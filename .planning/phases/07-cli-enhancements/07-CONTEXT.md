# Phase 7: Daemon Architecture & CLI Restructure - Context

**Gathered:** 2026-04-09
**Status:** Ready for planning
**Source:** User request for daemon-centric architecture

<domain>
## Phase Boundary

This phase fundamentally restructures DMGN around a **persistent background daemon**:
- Daemon always runs in background, auto-connects to peers via bootnodes
- MCP server is integrated into daemon — user does NOT manually run `mcp-serve`
- AI agents communicate with DMGN via stdio MCP protocol (standard MCP config)
- Daemon lifecycle controlled by `dmgn start` / `dmgn stop`
- Remove standalone `mcp-serve` and `serve` commands (absorbed by daemon)

</domain>

<decisions>
## Implementation Decisions

### Daemon Process (Priority 1)
- **D-01:** `dmgn start` forks a background daemon process (detached from terminal)
- **D-02:** Daemon writes PID file to `$DATA_DIR/daemon.pid` for lifecycle management
- **D-03:** Daemon auto-connects to peers using `bootstrap_peers` from config on startup
- **D-04:** Daemon runs libp2p host + gossip + delta sync + API server + MCP server — all in one process
- **D-05:** Daemon persists until `dmgn stop` is called (no Ctrl+C, truly background)

### MCP Integration (Priority 1)
- **D-06:** MCP server runs as part of daemon — not a separate process
- **D-07:** AI tools (Claude Desktop, Cline, Windsurf) configure MCP with `dmgn mcp` as the stdio command
- **D-08:** `dmgn mcp` is a thin stdio proxy — it connects to the running daemon and bridges MCP JSON-RPC over stdio
- **D-09:** If daemon is not running, `dmgn mcp` returns a clear error telling user to run `dmgn start` first
- **D-10:** The MCP tools (add_memory, query_memory, get_context, etc.) remain identical — only the transport changes

### Stop Command (Priority 1)
- **D-11:** `dmgn stop` reads PID file, sends SIGTERM (or equivalent on Windows), waits for graceful shutdown
- **D-12:** If daemon doesn't stop within timeout, force kill
- **D-13:** Clean up PID file on stop

### CLI Restructure
- **D-14:** Remove `mcp-serve` command (replaced by `dmgn mcp` stdio proxy)
- **D-15:** Remove `serve` command (REST API is always part of daemon)
- **D-16:** `dmgn start` no longer blocks terminal — it prints status and returns
- **D-17:** `dmgn start --foreground` flag for debugging (keeps current foreground behavior)

### Claude's Discretion
- IPC mechanism between `dmgn mcp` proxy and daemon (Unix socket, named pipe, or TCP localhost)
- Daemon logging strategy (file-based since no terminal)
- Windows-specific daemon implementation details (no fork() on Windows)
- Whether to use a process supervisor pattern or simple PID management

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### CLI Structure
- `cmd/dmgn/main.go` — CLI entry point, all command registrations
- `internal/cli/start.go` — Current foreground daemon with full libp2p+API wiring
- `internal/cli/mcp.go` — Current standalone MCP server (to be replaced)
- `internal/cli/serve.go` — Current standalone REST API server (to be removed)

### MCP Server
- `pkg/mcp/server.go` — MCP server implementation (7 tools, stdio transport)

### Networking
- `pkg/network/host.go` — libp2p host creation and management
- `pkg/network/discovery.go` — DHT and mDNS discovery

### Config
- `internal/config/config.go` — Config struct with BootstrapPeers, ListenAddr, etc.

</canonical_refs>

<specifics>
## Specific Architecture

### Current State (BEFORE)
```
User runs: dmgn start        → Foreground daemon (blocks terminal)
User runs: dmgn mcp-serve    → Separate foreground MCP process (blocks terminal)
User runs: dmgn serve         → Separate foreground API process (blocks terminal)
AI config: "command": "dmgn mcp-serve"  → Spawns new MCP process each time
```

### Desired State (AFTER)
```
User runs: dmgn start        → Background daemon (returns immediately)
                               → Daemon runs: libp2p + API + gossip + sync
                               → Daemon connects to bootnodes automatically
AI config: "command": "dmgn mcp"  → Thin stdio proxy to daemon
User runs: dmgn stop          → Graceful daemon shutdown
```

### MCP Config Example (Claude Desktop)
```json
{
  "mcpServers": {
    "dmgn": {
      "command": "dmgn",
      "args": ["mcp"]
    }
  }
}
```

</specifics>

<deferred>
## Deferred Ideas

- Systemd/launchd service integration (future)
- Auto-start on boot (future)
- Multi-instance daemon support (future)

</deferred>

---

*Phase: 07-cli-enhancements*
*Context gathered: 2026-04-09 via daemon architecture restructure*