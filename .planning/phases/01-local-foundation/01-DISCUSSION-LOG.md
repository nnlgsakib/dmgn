# Phase 1: Local Foundation - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-04-09
**Phase:** 01-local-foundation
**Mode:** discuss
**Areas discussed:** JSON → Protobuf Migration (wire format, gossip, disk, memory model, hybrid strategy)

---

## JSON Usage Assessment

### Wire (Network) - HIGH FREQUENCY
| Location | Uses | Priority |
|----------|------|----------|
| protocols.go (store/fetch) | 5 | HOT PATH |
| gossip.go (broadcast) | 2 | HIGH FREQUENCY |
| delta.go (sync) | 2 | MEDIUM |

### Disk - MEDIUM FREQUENCY
| Location | Uses | Priority |
|----------|------|----------|
| shards.go (persistence) | 4 | MEDIUM |
| reputation.go (periodic) | 3 | LOW |
| config.go (startup) | 2 | LOW |
| identity.go (startup) | 3 | LOW |

### Memory Model
| Location | Uses |
|----------|------|
| memory.go | 5 |

### API (Required - No Change)
| Location | Uses |
|----------|------|
| REST API, MCP, CLI | 24 |

**Total: 47 JSON marshal/unmarshal uses**

---

## Wire Format (Network Protocols)

| Option | Description | Selected |
|--------|-------------|----------|
| Protocol Buffers | 2-3x smaller, 5-10x faster, schema enforcement | ✓ |
| MessagePack | Binary JSON-like, compact, no schema | |
| Cap'n Proto | Zero-copy, fastest, complex setup | |
| Keep JSON | More debuggable, 2-3x larger, 5-10x slower | |

**User's choice:** Protocol Buffers (Recommended)
**Rationale:** Industry standard for P2P systems (libp2p, IPFS use them)

---

## Gossip Message Format

| Option | Description | Selected |
|--------|-------------|----------|
| Protocol Buffers (same as wire) | Consistent, efficient | ✓ |
| Keep JSON for gossip | Easier debugging | |
| Hybrid (protobuf + JSON) | protobuf payload, JSON wrapper | |

**User's choice:** Protocol Buffers (same as wire)

---

## Shard Persistence (Disk Format)

| Option | Description | Selected |
|--------|-------------|----------|
| Protocol Buffers | 2-3x smaller, faster read/write | |
| Keep JSON for disk | Easier to inspect with badger-cli | |
| BadgerDB native | Value log compression, TTL support | ✓ |

**User's choice:** BadgerDB native format (optimize JSON storage)
**Rationale:** Lower frequency than wire, easier debugging

---

## Memory Model Serialization

| Option | Description | Selected |
|--------|-------------|----------|
| Protocol Buffers | Consistency for replication | |
| Keep JSON | Already encrypted, less critical | |
| Encrypted payload only | No extra serialization | |
| Hybrid | protobuf replication, JSON local | ✓ |

**User's choice:** Hybrid (protobuf for replication, keep JSON for local)

---

## Hybrid Strategy Summary

| Area | Decision |
|------|----------|
| Wire (network) | Protocol Buffers |
| Gossip | Protocol Buffers (same as wire) |
| Shard disk | BadgerDB native format |
| Memory model | Hybrid (protobuf replication, JSON local) |
| API layer | JSON (required by spec) |

**User's choice:** Best Practice Hybrid (Recommended)

---

## Implementation Notes

- Create `proto/` directory for .proto files
- Generate Go code with `protoc` and go-protobuf
- Implement JSON ↔ Protobuf conversion layer
- Batch migration: wire protocols first (hot path), then gossip

---

*Phase: 01-local-foundation*
*Discussion completed: 2026-04-09*