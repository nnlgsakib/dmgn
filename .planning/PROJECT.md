# Distributed Memory Graph Network (DMGN)

## What This Is

A decentralized, encrypted, lifetime memory layer for AI agents. DMGN enables AI agents to store, retrieve, and synchronize memories across devices without relying on central servers. Data is end-to-end encrypted, user-owned, and resilient to node failure.

## Core Value

User owns their identity and memory data that persists across devices and time, with no central server or third-party control.

## Requirements

### Validated

(None yet — ship to validate)

### Active

- [ ] Identity Layer with ed25519 key generation and secure local storage
- [ ] Memory Model with content-addressable nodes and graph structure
- [ ] Local Storage Layer using LSM database (BadgerDB or Pebble)
- [ ] Encryption layer using AES-GCM for all memory data
- [ ] CLI interface using Cobra with init, start, add, query, status commands
- [ ] REST API for external integration
- [ ] MCP stdio-based protocol support

### Out of Scope

- Mobile app clients (Phase 6+) — Focus on CLI/API first
- Social recovery for key loss — Complex, defer to later phase
- Reed-Solomon shard recovery — Advanced feature, Phase 5
- gRPC API — Start with REST, add gRPC later
- WebRTC transport — Use TCP+QUIC only for MVP
- Public blockchain integration — Pure libp2p approach for decentralization

## Context

Building a distributed memory system requires balancing security, performance, and decentralization. This system targets AI agents that need persistent, cross-device memory without privacy risks of centralized storage.

Key technical challenges:
- Peer discovery and NAT traversal in libp2p
- Efficient vector similarity search on encrypted data
- Merkle tree integrity verification
- Handling node churn with adequate redundancy

## Constraints

- **Tech stack**: Go, libp2p, BadgerDB/Pebble, Cobra, stdio MCP
- **Security**: End-to-end encryption mandatory, no plaintext over network
- **Offline-first**: Must work without connectivity, sync when available
- **Performance**: Local queries <100ms, cross-peer sync <5s
- **Redundancy**: Replication factor >= 3 for distributed shards

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| Use BadgerDB over Pebble | Better Go ecosystem integration, proven in IPFS | — Pending |
| libp2p over custom protocol | Battle-tested DHT, pubsub, NAT traversal | — Pending |
| SHA256 content addressing | Industry standard, collision resistant | — Pending |
| Separate per-memory keys + master key | Limits exposure if single key compromised | — Pending |

## Evolution

This document evolves at phase transitions and milestone boundaries.

**After each phase transition** (via `/gsd-transition`):
1. Requirements invalidated? → Move to Out of Scope with reason
2. Requirements validated? → Move to Validated with phase reference
3. New requirements emerged? → Add to Active
4. Decisions to log? → Add to Key Decisions
5. "What This Is" still accurate? → Update if drifted

**After each milestone** (via `/gsd-complete-milestone`):
1. Full review of all sections
2. Core Value check — still the right priority?
3. Audit Out of Scope — reasons still valid?
4. Update Context with current state

---
*Last updated: April 9, 2026 after initialization*
