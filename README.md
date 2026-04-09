# Distributed Memory Graph Network (DMGN)

[![Go Version](https://img.shields.io/badge/Go-1.21%2B-blue)](https://go.dev/)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellowgreen)](LICENSE)
[![Status](https://img.shields.io/badge/Status-Alpha-orange)](README.md#project-status)

A decentralized, encrypted, lifetime memory layer for AI agents. DMGN enables AI agents to store, retrieve, and synchronize memories across devices without relying on central servers.

## Core Value

User owns their identity and memory data that persists across devices and time, with no central server or third-party control.

## Core Principles

- **No central server**: Fully peer-to-peer using libp2p
- **User owns identity**: Self-sovereign ed25519 keys
- **Lifetime persistence**: Data persists across devices and time
- **End-to-end encryption**: AES-GCM-256 for all memory data
- **Resilient to failure**: Automatic replication and recovery
- **Offline-first**: Works without connectivity, syncs when available

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                         DMGN Node                           │
├─────────────────────────────────────────────────────────────┤
│  CLI  │  REST API  │  MCP (stdio)                           │
├───────┴────────────┴──────────────────────────────────────────┤
│  Query Engine │ Encryption │ Memory Graph                   │
├───────────────┴────────────┴────────────────────────────────┤
│  Local Storage (BadgerDB) │  libp2p Network                │
├───────────────────────────┴──────────────────────────────────┤
│  Identity (ed25519) │  Sync │  Shard Distribution            │
└─────────────────────────────────────────────────────────────┘
```

## Table of Contents

- [Quick Start](#quick-start)
- [Installation](#installation)
- [CLI Commands](#cli-commands)
- [Architecture](#architecture)
- [Memory Model](#memory-model)
- [Security](#security)
- [Configuration](#configuration)
- [Development](#development)
- [Roadmap](#roadmap)
- [Contributing](#contributing)
- [License](#license)

## Project Status

| Phase | Status | Description |
|-------|--------|-------------|
| 1 | ✅ Complete | Local Foundation — Identity, storage, CLI, local memory |
| 2 | ✅ Complete | Encryption & API — Full E2E encryption, REST API |
| 3 | ✅ Complete | Networking Core — libp2p peer discovery, mDNS, DHT |
| 4 | ✅ Complete | Distributed Storage — Shamir sharding, replication |
| 5 | ✅ Complete | Query & Sync — Vector search, gossip sync, delta sync |
| 6 | ✅ Complete | MCP & Polish — MCP protocol, observability, backup, docs |

## Installation

### Requirements

- Go 1.21+
- BadgerDB (pulled via go modules)

### Build from source

```bash
git clone https://github.com/nnlgsakib/dmgn
cd dmgn
go build -o dmgn ./cmd/dmgn
```

## Quick Start

### 1. Initialize your identity

```bash
./dmgn init
# Enter a strong passphrase (min 8 characters)
# Confirm the passphrase
```

This creates:
- ed25519 keypair (public key = your ID)
- Encrypted private key stored in data directory
- Configuration file

### 2. Add a memory

```bash
./dmgn add "This is my first memory stored in DMGN"
```

Output:
```
✓ Memory added: a3f7b2d8e1c9...
```

### 3. Query memories

```bash
./dmgn query "first memory"
```

Output:
```
Found 1 memories:

1. This is my first memory stored in DMGN
   ID: a3f7b2d8e1c9... | Type: text | Links: 0
```

### 4. View recent memories

```bash
./dmgn query --recent --limit 5
```

### 5. Check status

```bash
./dmgn status
```

## CLI Commands

| Command | Description |
|---------|-------------|
| `dmgn init` | Create new identity and initialize node |
| `dmgn add <text>` | Add a memory to local storage |
| `dmgn query <text>` | Search memories by content |
| `dmgn query --recent` | List recent memories |
| `dmgn status` | Show node status and stats |
| `dmgn start` | Start the daemon with networking and API |
| `dmgn mcp-serve` | Start MCP server on stdio (for AI agents) |
| `dmgn backup` | Export encrypted backup of node data |
| `dmgn restore` | Restore node from encrypted backup |
| `dmgn export` | Export encrypted identity for backup |
| `dmgn import` | Import identity from backup |
| `dmgn peers` | Show connected peers |

See [docs/cli-reference.md](docs/cli-reference.md) for complete usage.

## Memory Model

```go
{
  id: "sha256_hash_of_encrypted_payload",
  timestamp: int64_nanoseconds,
  type: "text|conversation|observation|document",
  embedding: [float32_vector],
  encrypted_payload: bytes,
  links: ["memory_id_1", "memory_id_2"],
  merkle_proof: "integrity_hash"
}
```

### Memory Types

- `text` - General text content
- `conversation` - Dialog or chat messages
- `observation` - Noted observations or facts
- `document` - Document chunks or summaries

### Graph Relationships

Memories can link to other memories, forming a directed graph:

```bash
./dmgn add "User asked about Go channels" --link <previous-memory-id>
```

## Security

### Encryption

- **Key derivation**: Argon2id (time=3, memory=64MB, threads=4)
- **Identity encryption**: XChaCha20-Poly1305
- **Memory encryption**: AES-GCM-256 with per-memory keys
- **Master key**: Derived from ed25519 seed + purpose

### Key Hierarchy

```
ed25519 Private Key (encrypted with passphrase)
    ↓
Master Key (HKDF-derived from seed)
    ↓
Per-Memory Keys (random, encrypted with master key)
    ↓
Encrypted Payload (AES-GCM-256)
```

### Identity Backup

```bash
# Export (encrypted - safe to store)
./dmgn export -o backup.key

# Import on new device
./dmgn import -i backup.key
```

## Configuration

Data directory locations:
- **Linux**: `~/.config/dmgn/`
- **macOS**: `~/Library/Application Support/dmgn/`
- **Windows**: `%APPDATA%/dmgn/`

Override with `--data-dir` flag.

## Development

### Project Structure

```
cmd/dmgn/              # CLI entry point
pkg/identity/          # ed25519 key management
pkg/memory/            # Memory model and graph
pkg/storage/           # BadgerDB interface
pkg/vectorindex/       # Vector similarity search
pkg/query/             # Hybrid query engine + cross-peer protocol
pkg/sync/              # GossipSub + delta sync + version vectors
pkg/network/           # libp2p host, peer discovery, reputation
pkg/sharding/          # Shamir secret sharing
pkg/mcp/               # MCP server for AI agent integration
pkg/backup/            # Encrypted backup/restore
pkg/observability/     # OpenTelemetry, structured logging
internal/cli/          # Cobra commands
internal/crypto/       # AES-GCM encryption
internal/config/       # Configuration management
internal/api/          # REST API server
docs/                  # Architecture, MCP, API, CLI, config docs
examples/              # Claude Desktop, Cline config examples
```

### Running Tests

```bash
go test ./...
```

### Phase 1 Success Criteria

- [x] User can run `dmgn init` and create a new identity with ed25519 keypair
- [x] User can run `dmgn add "text"` and store memory locally with content-addressable ID
- [x] Memory graph can be traversed via link relationships
- [x] Data persists across CLI restarts
- [x] Time-based queries return memories in chronological order

## AI Agent Integration (MCP)

DMGN works as an MCP server — Claude Desktop, Cline, and other AI agents can use it as persistent memory.

```bash
# Build and initialize
go build -o dmgn ./cmd/dmgn
./dmgn init

# Add to Claude Desktop config
# See examples/claude_desktop_config.json
```

**7 MCP tools:** `add_memory`, `query_memory`, `get_context`, `link_memories`, `get_graph`, `delete_memory`, `get_status`

See [docs/mcp-integration.md](docs/mcp-integration.md) for full setup guide.

## Documentation

| Document | Description |
|----------|-------------|
| [Architecture](docs/architecture.md) | System overview, component diagram, data flow |
| [MCP Integration](docs/mcp-integration.md) | Claude Desktop, Cline setup guides |
| [API Reference](docs/api-reference.md) | REST API with curl examples |
| [CLI Reference](docs/cli-reference.md) | All commands with usage |
| [Config Reference](docs/config-reference.md) | All config fields with defaults |
| [Troubleshooting](docs/troubleshooting.md) | Common issues and fixes |

## Roadmap

### Phase 1: Local Foundation ✅
- Identity generation and storage
- Local memory storage with BadgerDB
- CLI commands (init, add, query, status)
- Memory graph with links

### Phase 2: Encryption & API ✅
- Full AES-GCM-256 encryption for all data
- REST API with Bearer token authentication
- Memory backup and import/export
- Key hierarchy with per-memory keys

### Phase 3: Networking Core ✅
- libp2p host with TCP and QUIC transports
- DHT and mDNS peer discovery
- Connection management with watermarks
- Basic protocol handlers

### Phase 4: Distributed Storage ✅
- Shamir secret sharing for memory shards
- Encrypted shard distribution to peers
- Replication factor management
- `/memory/store/1.0.0` and `/memory/fetch/1.0.0` protocols

### Phase 5: Query & Sync ✅
- Brute-force cosine vector similarity search
- Hybrid scoring (vector + text)
- Cross-peer query aggregation via libp2p streams
- GossipSub memory propagation + delta sync with version vectors

### Phase 6: MCP & Polish ✅
- MCP stdio server with 7 tools (official Go SDK)
- OpenTelemetry traces/metrics + structured logging with rotation
- Encrypted backup/restore (tar.gz with manifest)
- Peer reputation scoring (weighted: uptime, latency, sync, availability)
- Comprehensive documentation suite

## Security

See [SECURITY.md](SECURITY.md) for vulnerability reporting and security policies.

### Reporting Security Issues

If you discover a security vulnerability, please report it responsibly:

1. Do NOT open a public issue
2. Email security reports privately
3. Include reproduction steps and potential impact
4. Wait for acknowledgment before public disclosure

## Community

### Discussion

Join the conversation:

- [GitHub Discussions](https://github.com/nnlgsakib/dmgn/discussions) - Q&A and ideas
- [Discord](https://discord.gg/dmgn) - Real-time chat

### Staying Updated

- Watch the repository for release notifications
- Check [CHANGELOG.md](CHANGELOG.md) for version history

## License

This project is licensed under the MIT License - see [LICENSE](LICENSE) for details.

## Contributing

Contributions are welcome! Please read [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines on:

- Setting up your development environment
- Submitting pull requests
- Code style and commit conventions
- Recognizing contributors

### Contributors

Thanks to all contributors who have helped build DMGN:

<!-- CONTRIBUTORS:START -->
| Contributor | Contribution |
|------------|--------------|
| [@dmgn](https://github.com/dmgn) | Original author |
<!-- CONTRIBUTORS:END -->

_(This section is updated for each release. Thank you for your contributions!)_

## Acknowledgments

Special thanks to the following projects and communities:

- **[libp2p](https://libp2p.io/)** - Peer-to-peer networking foundation
- **[BadgerDB](https://github.com/dgraph-io/badger)** - Fast LSM storage
- **[Argon2](https://github.com/P-H-C/phc-winner-argon2)** - Memory-hard key derivation
- **[Go Crypto](https://pkg.go.dev/golang.org/x/crypto)** - Cryptographic primitives
- **[IPFS](https://ipfs.io/)** - Distributed storage concepts
- **[MCP](https://modelcontextprotocol.io/)** - Model Context Protocol for AI agent integration
- **[OpenTelemetry](https://opentelemetry.io/)** - Observability framework
