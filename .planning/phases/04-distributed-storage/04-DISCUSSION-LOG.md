# Phase 4: Distributed Storage - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-04-09
**Phase:** 04-distributed-storage
**Mode:** --auto (all decisions auto-selected)
**Areas discussed:** Sharding strategy, Shard placement & routing, Protocol handler design, Rebalancing strategy

---

## Sharding Strategy

| Option | Description | Selected |
|--------|-------------|----------|
| Shamir's Secret Sharing | k-of-n threshold scheme, no single shard reveals data | ✓ |
| Simple byte splitting | Split payload into equal chunks, simpler but leaks structure | |
| AES-CTR stream splitting | Split ciphertext stream, each chunk is random but reconstructable from any subset | |

**User's choice:** [auto] Shamir's Secret Sharing (recommended default)
**Notes:** DIST-04 requires no single peer can reconstruct data. SSS is the only option that provides this cryptographic guarantee. k=3, n=5 default satisfies DIST-03 replication factor.

---

## Shard Placement & Routing

| Option | Description | Selected |
|--------|-------------|----------|
| DHT provider records | Use Provide()/FindProviders() for shard location tracking | ✓ |
| Custom shard registry | Maintain centralized shard→peer mapping | |
| Gossip-based discovery | Broadcast shard availability via pubsub | |

**User's choice:** [auto] DHT-based routing with provider records (recommended default)
**Notes:** Native libp2p DHT pattern. Shard key = SHA256(memory_id + shard_index). Consistent with Phase 3 DHT infrastructure.

---

## Protocol Handler Design

| Option | Description | Selected |
|--------|-------------|----------|
| Length-prefixed JSON header + binary | JSON metadata header followed by raw shard bytes | ✓ |
| Full protobuf messages | Structured binary protocol with schema | |
| JSON-only with base64 shards | Pure JSON but larger payload size | |

**User's choice:** [auto] Length-prefixed JSON header + binary (recommended default)
**Notes:** Avoids protobuf dependency while being efficient for binary transfer. Protocol names follow DMGN namespace: `/dmgn/memory/store/1.0.0`, `/dmgn/memory/fetch/1.0.0`.

---

## Rebalancing Strategy

| Option | Description | Selected |
|--------|-------------|----------|
| Event-driven + periodic audit | Rebalance on disconnect events, audit every 5 min | ✓ |
| Purely event-driven | Only rebalance when disconnect detected | |
| Periodic-only | Scan and rebalance on fixed interval | |

**User's choice:** [auto] Event-driven + periodic audit (recommended default)
**Notes:** Event-driven catches immediate failures, periodic audit is safety net. Graceful degradation stores shards locally when insufficient peers (offline-first).

---

## Claude's Discretion

- Internal package structure for sharding code
- Specific SSS library choice
- Protocol message framing details
- Test strategy
- Shard storage key format in BadgerDB

## Deferred Ideas

- Reed-Solomon erasure coding (RCVR-01, RCVR-02) — advanced recovery, later phase
- Peer reputation scoring for shard placement (NETW-05) — Phase 6
- Shard compression — defer unless payload sizes warrant it
