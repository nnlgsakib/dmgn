# Configuration Reference

Configuration file location: `{data-dir}/config.json`

## General

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `data_dir` | string | OS-specific | Root data directory |
| `version` | string | `"0.1.0"` | DMGN version |
| `log_level` | string | `"info"` | Log level: debug, info, warn, error |

## Network

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `listen_addr` | string | `"/ip4/0.0.0.0/tcp/0"` | libp2p listen address (0 = random port) |
| `bootstrap_peers` | string[] | `[]` | Bootstrap peer multiaddrs for DHT |
| `mdns_service` | string | `"_dmgn._tcp"` | mDNS service name for local discovery |
| `max_peers_low` | int | `15` | Connection manager low watermark |
| `max_peers_high` | int | `25` | Connection manager high watermark |

## API

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `api_port` | int | `8080` | REST API listen port |

## Storage

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `max_recent_memories` | int | `1000` | Max memories returned by recent queries |
| `shard_threshold` | int | `3` | Minimum shards for Shamir splitting |
| `shard_count` | int | `5` | Total shards per memory |

## Query

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `embedding_dim` | int | `0` | Expected embedding dimension (0 = auto-detect) |
| `hybrid_score_alpha` | float64 | `0.7` | Hybrid score weight: alpha×vector + (1-alpha)×text |
| `query_timeout` | string | `"2s"` | Timeout for cross-peer queries (Go duration) |

## Sync

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `sync_interval` | string | `"60s"` | Delta sync interval (Go duration) |
| `gossip_topic` | string | `"dmgn/memories/1.0.0"` | GossipSub topic for memory propagation |

## Observability

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `otlp_endpoint` | string | `""` | OTLP gRPC endpoint for traces/metrics (empty = disabled) |

## Path Helpers

These are derived from `data_dir`:

| Path | Location | Purpose |
|------|----------|---------|
| Identity | `{data_dir}/identity/` | Encrypted ed25519 keypair |
| Storage | `{data_dir}/storage/` | BadgerDB data |
| Vector Index | `{data_dir}/vector-index.enc` | Encrypted vector index |
| Logs | `{data_dir}/logs/` | Rotating log files |
| Backups | `{data_dir}/backups/` | Default backup location |

## Default Data Directories

| OS | Path |
|----|------|
| Linux | `~/.config/dmgn/` |
| macOS | `~/Library/Application Support/dmgn/` |
| Windows | `%APPDATA%/dmgn/` |

## Example Configuration

```json
{
  "data_dir": "/home/user/.config/dmgn",
  "listen_addr": "/ip4/0.0.0.0/tcp/4001",
  "api_port": 8080,
  "max_recent_memories": 1000,
  "log_level": "info",
  "bootstrap_peers": [
    "/ip4/104.131.131.82/tcp/4001/p2p/QmaCpDMGvV2BGHeYER..."
  ],
  "hybrid_score_alpha": 0.7,
  "query_timeout": "2s",
  "sync_interval": "60s",
  "gossip_topic": "dmgn/memories/1.0.0",
  "otlp_endpoint": "localhost:4317"
}
```
