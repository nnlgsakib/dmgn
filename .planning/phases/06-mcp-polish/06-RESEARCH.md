# Phase 6: MCP & Polish - Research

**Researched:** 2026-04-09
**Phase requirement IDs:** MCP-01, MCP-02, MCP-03, MCP-04, MCP-05, INTG-01, INTG-02, SAFE-01, NETW-05

## 1. MCP Protocol Implementation

### Official Go SDK
- **`github.com/modelcontextprotocol/go-sdk`** — Official Go SDK maintained by Google + Anthropic
- Packages: `mcp` (server/client), `jsonrpc` (transport), `auth` (OAuth)
- Supports MCP spec versions: 2024-11-05 through 2025-11-25
- Stdio transport: `mcp.StdioTransport{}` — reads stdin, writes stdout
- Tool registration: `mcp.AddTool(server, &mcp.Tool{Name, Description}, handler)`
- Handler signature: `func(ctx, *mcp.CallToolRequest, InputStruct) (*mcp.CallToolResult, OutputStruct, error)`
- Input structs use `jsonschema` tags for automatic schema generation
- Server lifecycle: `server.Run(ctx, transport)` blocks until client disconnects

### Alternative: mark3labs/mcp-go
- Community SDK, also widely used
- Similar API but less official support
- **Recommendation:** Use official `modelcontextprotocol/go-sdk` for long-term stability

### Key Implementation Details
- JSON-RPC 2.0 over stdio (newline-delimited JSON)
- Server must NOT write anything to stdout except JSON-RPC messages
- All logging must go to stderr or file
- Tools declare `inputSchema` as JSON Schema — the SDK generates this from Go struct tags
- `initialize` handshake: client sends capabilities, server responds with supported tools/resources

## 2. OpenTelemetry Go SDK

### Core Packages
- `go.opentelemetry.io/otel` — API
- `go.opentelemetry.io/otel/sdk/trace` — Trace SDK
- `go.opentelemetry.io/otel/sdk/metric` — Metric SDK  
- `go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc` — OTLP trace exporter
- `go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc` — OTLP metric exporter
- `go.opentelemetry.io/otel/exporters/stdout/stdouttrace` — Stdout trace exporter (dev)
- `go.opentelemetry.io/contrib/bridges/otelslog` — slog bridge for OTel logs

### slog Bridge
- `otelslog.NewHandler()` bridges Go's `log/slog` to OTel log pipeline
- Injects trace/span IDs automatically into log records
- Can be composed with other handlers via `slog.Handler` interface

### Metrics Pattern
```go
meter := otel.Meter("dmgn")
memoryCount, _ := meter.Int64Counter("dmgn.memory.count")
queryLatency, _ := meter.Float64Histogram("dmgn.query.latency_ms")
peerGauge, _ := meter.Int64UpDownCounter("dmgn.peer.count")
```

### Setup Pattern
- Create TracerProvider with exporter + resource
- Create MeterProvider with exporter + resource
- Set global providers via `otel.SetTracerProvider()` and `otel.SetMeterProvider()`
- Shutdown providers on exit with `provider.Shutdown(ctx)`

## 3. Backup & Restore

### BadgerDB Backup
- `db.Backup(w io.Writer, since uint64)` — streams incremental backup
- `db.Load(r io.Reader, maxPendingWrites int)` — restores from backup stream
- Backup is a KV stream, not a full snapshot — fast and space-efficient

### Approach
- `dmgn backup` → tar.gz containing:
  - `badger.backup` — BadgerDB backup stream (already encrypted data)
  - `vector-index.enc` — Encrypted vector index binary
  - `config.json` — Node config (non-sensitive)
  - `manifest.json` — Backup metadata (timestamp, version, node ID)
- `dmgn restore <file>` → Extract tar.gz, load BadgerDB, load vector index
- Identity NOT included in backup (separate export/import via `dmgn export-key`)

## 4. Peer Reputation Scoring

### Metrics to Track
- **Uptime ratio:** Time connected / total time known
- **Response latency:** Rolling average of query/sync response times
- **Sync success rate:** Successful syncs / total sync attempts
- **Data availability:** Successful shard fetches / total fetch requests

### Score Algorithm
- Weighted sum: `0.3*uptime + 0.3*latency_score + 0.2*sync_rate + 0.2*availability`
- Latency score: `max(0, 1 - avg_latency/5000ms)` (0 if >5s average)
- Scores decay toward neutral (0.5) over time without interaction
- Store in BadgerDB: `rep:{peer_id}` → JSON with metrics and computed score

### Usage
- Query fan-out: sort peers by reputation, query highest-rep first
- Shard placement: prefer higher-rep peers for new shard replicas
- No automatic blacklisting — manual only via config

## 5. Log Rotation

### lumberjack
- `gopkg.in/natefinished/lumberjack.v2` — standard Go log rotation
- Implements `io.WriteCloser` — drop-in replacement for file writer
- Config: MaxSize (MB), MaxBackups, MaxAge (days), Compress
- Works with slog via `slog.NewJSONHandler(lumberjackWriter, opts)`

## 6. Documentation Structure

### docs/ Layout
```
docs/
├── architecture.md      — System overview, component diagram
├── mcp-integration.md   — Claude Desktop, Cline setup guides
├── api-reference.md     — REST API with curl examples
├── cli-reference.md     — All commands with usage
├── config-reference.md  — All config fields
└── troubleshooting.md   — Common issues
examples/
├── claude_desktop_config.json
└── cline_config.json
```

## Validation Architecture

### Testable Assertions
1. MCP server starts on stdio and responds to `initialize` → verify with test client
2. All 7 tools registered and callable → test each tool's happy path
3. OTel metrics exported → check metric provider has expected instruments
4. Backup creates valid tar.gz → backup + restore round-trip test
5. Peer reputation persists → save + load + decay test
6. Log rotation works → write enough to trigger rotation, check file count

---

*Research completed: 2026-04-09*
