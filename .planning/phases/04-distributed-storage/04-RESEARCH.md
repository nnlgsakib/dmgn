# Phase 4: Distributed Storage - Research

**Date:** 2026-04-09
**Phase:** 04-distributed-storage
**Focus:** Shamir's Secret Sharing, libp2p stream protocols, shard placement

## Shamir's Secret Sharing Libraries (Go)

### Options Evaluated

| Library | API | Maintenance | Notes |
|---------|-----|-------------|-------|
| `hashicorp/vault/shamir` | `Split(secret, parts, threshold)` / `Combine(parts)` | Active (Vault core) | Production-proven, but internal package — not importable directly |
| `corvus-ch/shamir` | Fork of Vault's shamir, standalone package | Low activity | Directly importable, same API as Vault |
| `codahale/sss` | `Split(n, k, secret)` / `Combine(shares)` | Stable | Pure Go over GF(2^8), clean API, MIT license |
| `SSSaaS/sssa-golang` | `Create(minimum, shares, raw)` / `Combine(shares)` | Low | Handles arbitrary-length secrets via polynomial sharing |

### Recommendation

**`codahale/sss`** — Pure Go, minimal dependencies, clean API (`Split`/`Combine`), MIT license. Operates over GF(2^8) which handles arbitrary byte slices natively. If unavailable or issues arise, implement a minimal Shamir SSS over GF(2^8) directly (< 200 lines).

**Fallback:** Self-contained implementation using `crypto/rand` for coefficient generation and GF(2^8) arithmetic. This avoids external dependency entirely.

## libp2p Stream Protocol Handlers

### Pattern

```go
// Register handler
host.SetStreamHandler("/dmgn/memory/store/1.0.0", func(s network.Stream) {
    defer s.Close()
    // Read request from stream
    // Process and respond
})

// Open stream to peer
s, err := host.NewStream(ctx, peerID, "/dmgn/memory/store/1.0.0")
// Write request, read response
s.Close()
```

### Message Framing

Length-prefixed binary: `[4-byte header length][JSON header][shard data]`

- Header: `{"memory_id": "...", "shard_index": 0, "total_shards": 5, "threshold": 3, "checksum": "sha256hex"}`
- Body: raw shard bytes
- Response: `{"status": "ok"}` or `{"status": "error", "message": "..."}`

### Stream Lifecycle
- Sender opens stream → writes header + data → reads response → closes
- Handler receives stream → reads header + data → stores → writes response → stream closes
- Timeouts: 30s for store (large payloads), 15s for fetch

## DHT Provider Records

### Shard Key Derivation
```
shard_key = SHA256(memory_id + ":" + shard_index)
```

### Provider Flow
1. **Store:** After storing shard locally, call `dht.Provide(ctx, cid.NewCidV1(cid.Raw, shardKey))` 
2. **Find:** Call `dht.FindProviders(ctx, shardCID)` to locate peers holding a shard
3. **Fetch:** Open `/dmgn/memory/fetch/1.0.0` stream to provider, request shard

### Alternative: Simple Provider Records
Since we don't need full CID/IPFS compatibility, use DHT `PutValue`/`GetValue` with custom validator:
- Key: `/dmgn/shard/<memory_id>/<shard_index>`
- Value: JSON list of peer IDs holding this shard

This is simpler and avoids CID dependency. Recommended approach.

## Shard Storage Schema (BadgerDB)

```
Key: shard:{memory_id}:{shard_index}
Value: {
    "memory_id": "abc123",
    "shard_index": 0,
    "total_shards": 5,
    "threshold": 3,
    "owner_peer_id": "QmPeer...",
    "data": <base64 shard bytes>,
    "checksum": "sha256hex",
    "received_at": 1712649600
}
```

Separate from memory storage (`mem:` prefix) to avoid conflicts.

## Rebalancing Architecture

### Event-Driven Component
- Register `network.Notifiee` on libp2p host
- On `Disconnected(network, conn)`: check if disconnected peer held any shards for our memories
- If replication count < threshold: queue re-replication to another peer

### Periodic Audit
- Timer-based (5 min default, configurable)
- For each locally-owned memory: check shard provider counts via DHT
- If any shard has < threshold providers: re-replicate

### Graceful Degradation
- If connected peers < total_shards: store shards locally, mark as "pending distribution"
- On new peer connect: check pending queue, distribute

## Architecture Summary

```
pkg/sharding/
  shamir.go      — Split/Combine using SSS
  sharding.go    — ShardManager orchestrating split/distribute/reconstruct
  
pkg/network/
  protocols.go   — Store/Fetch protocol handlers
  rebalance.go   — Rebalancing notifiee + periodic audit
  
pkg/storage/
  shards.go      — Shard-specific BadgerDB operations
```

## RESEARCH COMPLETE
