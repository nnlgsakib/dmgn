# Distributed Memory Graph Network (DMGN)

A decentralized, encrypted, lifetime memory layer for AI agents. DMGN enables AI agents to store, retrieve, and synchronize memories across devices without relying on central servers.

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

## Project Status

| Phase | Status | Description |
|-------|--------|-------------|
| 1 | ✅ Local Foundation | Identity, storage, CLI, local memory |
| 2 | ⏳ Encryption & API | Full E2E encryption, REST API |
| 3 | ⏳ Networking Core | libp2p peer discovery |
| 4 | ⏳ Distributed Storage | Sharding, replication factor 3+ |
| 5 | ⏳ Query & Sync | Vector search, gossip sync |
| 6 | ⏳ MCP & Polish | MCP protocol, docs, production ready |

## Installation

### Requirements

- Go 1.21+
- BadgerDB (pulled via go modules)

### Build from source

```bash
git clone https://github.com/dmgn/dmgn
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
| `dmgn start` | Start the daemon (Phase 3+) |
| `dmgn export` | Export encrypted identity for backup |
| `dmgn import` | Import identity from backup |

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
cmd/dmgn/          # CLI entry point
pkg/identity/      # ed25519 key management
pkg/memory/        # Memory model and graph
pkg/storage/       # BadgerDB interface
internal/cli/      # Cobra commands
internal/crypto/   # AES-GCM encryption
internal/config/   # Configuration management
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

## Roadmap

### Phase 1: Local Foundation ✅
- Identity generation and storage
- Local memory storage with BadgerDB
- CLI commands (init, add, query, status)
- Memory graph with links

### Phase 2: Encryption & API
- Full AES-GCM-256 encryption for all data
- REST API with authentication
- Memory backup and import/export
- Local query with basic search

### Phase 3: Networking Core
- libp2p host initialization
- DHT and mDNS peer discovery
- TCP and QUIC transports
- Basic protocol handlers

### Phase 4: Distributed Storage
- Memory sharding algorithm
- Encrypted shard distribution
- Replication factor management
- `/memory/store/1.0.0` and `/memory/fetch/1.0.0` protocols

### Phase 5: Query & Sync
- Embedding generation (OpenAI/local)
- Vector similarity search (HNSW)
- Cross-peer query aggregation
- Gossip-based memory sync

### Phase 6: MCP & Polish
- MCP stdio protocol implementation
- `add_memory`, `query_memory`, `get_context` tools
- Comprehensive documentation
- Production readiness

## License

MIT License - See [LICENSE](LICENSE) file.

## Contributing

Contributions welcome! Please read [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

## Acknowledgments

- libp2p for peer-to-peer networking
- BadgerDB for fast LSM storage
- Argon2 and AES-GCM for security
