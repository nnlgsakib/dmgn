# CLI Reference

All commands support the global `--data-dir` flag to override the default data directory.

## Commands

### `dmgn init`

Create a new identity and initialize the node.

```bash
dmgn init [--data-dir <path>]
```

Prompts for a passphrase (min 8 characters) and creates:
- ed25519 keypair
- Encrypted identity file
- Default configuration

### `dmgn add`

Add a memory to local storage.

```bash
dmgn add <content> [flags]
```

**Flags:**
| Flag | Type | Description |
|------|------|-------------|
| `--type` | string | Memory type: text, conversation, observation, document (default: text) |
| `--link` | string[] | Link to existing memory IDs |
| `--metadata` | string[] | Key=value metadata pairs |
| `--embedding` | string | JSON-encoded float32 embedding vector |

**Example:**
```bash
dmgn add "Important meeting notes" --type observation --metadata "source=meeting"
```

### `dmgn query`

Search memories by content.

```bash
dmgn query <text> [flags]
```

**Flags:**
| Flag | Type | Description |
|------|------|-------------|
| `--recent` | bool | List recent memories instead of searching |
| `--limit` | int | Max results (default 10) |
| `--type` | string | Filter by memory type |
| `--embedding` | string | JSON-encoded embedding for semantic search |

**Example:**
```bash
dmgn query "meeting notes" --limit 5 --type observation
```

### `dmgn status`

Show node status and statistics.

```bash
dmgn status [--data-dir <path>]
```

**Flags:**
| Flag | Type | Description |
|------|------|-------------|
| `--verbose` | bool | Show verbose logging output |

**Example:**
```bash
dmgn status --verbose
```

### `dmgn start`

Start the DMGN daemon with networking and API server.

```bash
dmgn start [flags]
```

**Flags:**
| Flag | Type | Description |
|------|------|-------------|
| `--data-dir` | string | Data directory |
| `--no-api` | bool | Disable REST API server |
| `--verbose` | bool | Enable verbose logging for debug |

### `dmgn stop`

Gracefully stop the running DMGN daemon.

```bash
dmgn stop [--data-dir <path>]
```

### `dmgn mcp`

stdio↔TCP proxy for AI agent MCP integration. Used by Claude Desktop and other AI tools.

```bash
dmgn mcp [--data-dir <path>]
```

**Note:** The daemon must be running (`dmgn start`) before using `dmgn mcp`.

### `dmgn mcp-serve`

Start DMGN as an MCP server on stdio for AI agent integration.

```bash
dmgn mcp-serve [flags]
```

**Flags:**
| Flag | Type | Description |
|------|------|-------------|
| `--data-dir` | string | Data directory |
| `--network` | bool | Enable P2P networking |
| `--log-level` | string | Log level: debug, info, warn, error (default: info) |

**Example (Claude Desktop):**
```json
{
  "mcpServers": {
    "dmgn": { "command": "dmgn", "args": ["mcp-serve"] }
  }
}
```

### `dmgn backup`

Export an encrypted backup of the DMGN node.

```bash
dmgn backup [output-file] [--data-dir <path>]
```

Default output: `dmgn-backup-<timestamp>.dmgn-backup`

The backup contains encrypted BadgerDB data, vector index, and manifest. All data remains encrypted — safe to store anywhere.

**Example:**
```bash
dmgn backup my-backup.dmgn-backup
```

### `dmgn restore`

Restore DMGN node from an encrypted backup.

```bash
dmgn restore <backup-file> [flags]
```

**Flags:**
| Flag | Type | Description |
|------|------|-------------|
| `--data-dir` | string | Data directory to restore into |
| `--force` | bool | Overwrite existing data |

**Example:**
```bash
dmgn restore my-backup.dmgn-backup --data-dir ~/.dmgn-new
```

### `dmgn export`

Export encrypted identity key for backup.

```bash
dmgn export -o <output-file>
```

### `dmgn import`

Import identity key from backup.

```bash
dmgn import -i <input-file>
```

### `dmgn peers`

Show connected peers (requires running daemon).

```bash
dmgn peers [--data-dir <path>]
```
