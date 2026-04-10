# Phase 7: Daemon Architecture & CLI Restructure - Research

**Researched:** 2026-04-09
**Status:** Complete

## 1. Go Background Daemon Patterns

### Challenge
Go doesn't support `fork()` natively. The Go runtime requires all OS threads to be managed, making traditional Unix daemon patterns impossible.

### Recommended Pattern: Self-Re-exec
The proven cross-platform approach:

1. `dmgn start` (foreground) prompts for passphrase, validates identity, derives keys
2. Re-executes itself with internal `--daemon-mode` flag as a detached child process
3. Parent passes derived key material to child via environment variable or file descriptor
4. Parent waits briefly for child health check, then exits
5. Child (daemon) runs the full node: libp2p, API, MCP handler, gossip, delta sync

**Implementation:** Use `os/exec.Command` with the same binary path and `--daemon-mode` flag:
```go
cmd := exec.Command(os.Args[0], "--daemon-mode", "--data-dir", dataDir)
cmd.SysProcAttr = &syscall.SysProcAttr{/* platform-specific detach */}
cmd.Start()
// write PID file
// wait for health check
// exit parent
```

### Platform-Specific Detach
- **Linux/Mac:** `syscall.SysProcAttr{Setsid: true}` — creates new session
- **Windows:** `syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x00000008}` (DETACHED_PROCESS) — no console window

### Passphrase Handling for Background Daemon
Current `promptPassphraseOnce()` reads from terminal — can't work in background.

**Solution:** Parent process (foreground `dmgn start`):
1. Prompts passphrase interactively
2. Derives ALL key material (master key, libp2p key, vector index key)
3. Passes derived keys to child via `DMGN_DERIVED_KEYS` environment variable (base64-encoded)
4. Child daemon uses pre-derived keys, never needs passphrase

Alternative: `DMGN_PASSPHRASE` env var for headless/CI scenarios.

## 2. PID File Management

**Location:** `$DATA_DIR/daemon.pid`

**Lifecycle:**
- On `dmgn start`: Check if PID file exists → if yes, check if process alive → if alive, error "daemon already running" → if stale, remove and continue
- Write PID after successful daemon startup
- On `dmgn stop`: Read PID, send signal, wait for exit, clean up PID file
- On daemon crash: PID file remains (stale) — next start detects and cleans up

**Stale PID detection:** `os.FindProcess(pid)` + platform-specific liveness check:
- Linux/Mac: `syscall.Kill(pid, 0)` — error means not running
- Windows: `OpenProcess` with `PROCESS_QUERY_LIMITED_INFORMATION`

## 3. IPC for MCP Proxy

### Architecture
```
AI Agent ←stdio→ [dmgn mcp] ←IPC→ [daemon process]
                   (proxy)         (actual MCP handler)
```

### IPC Transport Options

| Option | Cross-platform | Latency | Complexity |
|--------|---------------|---------|------------|
| TCP localhost | ✓ | ~50μs | Low |
| Unix socket / Named pipe | Partial | ~10μs | Medium |
| Shared memory | ✓ | ~1μs | High |

**Recommendation: TCP localhost** — simplest, fully cross-platform, latency is negligible for JSON-RPC.

**Implementation:**
1. Daemon listens on `127.0.0.1:{mcp_port}` for MCP IPC (separate from API port)
2. Port stored in `$DATA_DIR/daemon.port` file alongside PID
3. `dmgn mcp` reads port file, connects via TCP, bridges stdio↔TCP
4. JSON-RPC messages pass through unmodified — the proxy is transparent

### MCP Port Configuration
- Config: `mcp_ipc_port` field (default: `0` = auto-assign)
- Auto-assigned port written to `$DATA_DIR/daemon.port`
- `dmgn mcp` reads this file to discover the port

## 4. `dmgn mcp` Stdio Proxy

Thin process spawned by AI tools:

```go
func mcpProxy() error {
    port := readPortFile()  // $DATA_DIR/daemon.port
    conn, err := net.Dial("tcp", "127.0.0.1:"+port)
    // Bidirectional copy: stdin→conn, conn→stdout
    go io.Copy(conn, os.Stdin)
    io.Copy(os.Stdout, conn)
}
```

**Error handling:** If daemon not running → print to stderr: "DMGN daemon not running. Start with: dmgn start"

## 5. `dmgn stop` Implementation

```go
func stopDaemon(dataDir string) error {
    pid := readPIDFile(dataDir)
    process, _ := os.FindProcess(pid)
    process.Signal(syscall.SIGTERM)  // or os.Kill on Windows
    // Wait up to 10s for graceful shutdown
    // If still running, force kill
    // Remove PID and port files
}
```

## 6. Daemon Internal MCP Listener

The daemon runs a **TCP MCP listener** alongside everything else:

```go
// Inside daemon main loop
mcpListener, _ := net.Listen("tcp", "127.0.0.1:0")
writePortFile(mcpListener.Addr().(*net.TCPAddr).Port)

// Accept connections and handle MCP JSON-RPC
go func() {
    for {
        conn, _ := mcpListener.Accept()
        go handleMCPConnection(conn, mcpServer)
    }
}()
```

Each connection gets a fresh MCP session. The existing `pkg/mcp/server.go` tool handlers remain unchanged — only the transport layer switches from stdio to TCP.

**Key change to `pkg/mcp/server.go`:** Add `RunOnConnection(ctx, conn)` method that runs MCP over an arbitrary `io.ReadWriteCloser` instead of hard-coded stdio.

## 7. CLI Command Changes

### Commands to KEEP (unchanged)
- `dmgn init` — identity creation (no daemon needed)
- `dmgn add` — direct local storage (no daemon needed? or proxy to daemon?)
- `dmgn export` / `dmgn import` — identity management
- `dmgn backup` / `dmgn restore` — data management

### Commands to MODIFY
- `dmgn start` — becomes background daemon launcher (current logic moves to `--daemon-mode`)
- `dmgn status` — queries running daemon for status
- `dmgn peers` — queries running daemon for peer list
- `dmgn query` — could proxy to daemon or work locally

### Commands to ADD
- `dmgn stop` — daemon lifecycle
- `dmgn mcp` — stdio proxy for AI tools

### Commands to REMOVE
- `dmgn mcp-serve` — replaced by `dmgn mcp` proxy
- `dmgn serve` — REST API is always part of daemon

## 8. Existing Code Reuse

The current `internal/cli/start.go` RunE function (293 lines) contains ALL the daemon wiring:
- libp2p host, DHT, mDNS
- Storage, crypto, vector index
- Gossip, delta sync
- API server, query engine

**Strategy:** Extract this into a `internal/daemon/daemon.go` package that:
1. Accepts pre-derived key material (no passphrase prompting)
2. Runs all subsystems
3. Exposes MCP IPC listener
4. Handles graceful shutdown on SIGTERM

The CLI `start.go` becomes a thin wrapper that prompts passphrase, derives keys, spawns daemon, writes PID.

## 9. Config Changes

Add to `Config` struct:
```go
MCPIPCPort int `json:"mcp_ipc_port"` // 0 = auto-assign
```

Add helper methods:
```go
func (c *Config) PIDFile() string    { return filepath.Join(c.DataDir, "daemon.pid") }
func (c *Config) PortFile() string   { return filepath.Join(c.DataDir, "daemon.port") }
```

---

## RESEARCH COMPLETE

**Key architectural decisions:**
1. Self-re-exec pattern with `--daemon-mode` for cross-platform daemon
2. TCP localhost IPC for MCP proxy (simplest cross-platform)
3. PID file + port file for daemon discovery
4. Pre-derived key material passed via env to avoid passphrase in background
5. Extract daemon logic from `start.go` into `internal/daemon/` package
6. `dmgn mcp` is a transparent stdio↔TCP proxy

*Phase: 07-cli-enhancements*
*Researched: 2026-04-09*