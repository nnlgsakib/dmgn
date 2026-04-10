---
gsd_state_version: 1.0
milestone: v1.0
milestone_name: milestone
status: Milestone complete
last_updated: "2026-04-10T15:37:27.098Z"
progress:
  total_phases: 11
  completed_phases: 8
  total_plans: 30
  completed_plans: 17
  percent: 57
---

# State: DMGN

## Project Reference

See: `.planning/PROJECT.md` (updated 2025-04-09)

**Core value:** User owns their identity and memory data that persists across devices and time, with no central server or third-party control.

**Current focus:** Phase 11 — hybrid-link-processing

## Phase Progress

| Phase | Status | Notes |
|-------|--------|-------|
| 1: Local Foundation | **Complete** | Identity, storage, CLI init/add |
| 2: Encryption & API | **Complete** | HKDF, crypto framing, REST API, retention, integration tests |
| 3: Networking Core | **Complete** | libp2p host, DHT, mDNS, peers CLI, live status |
| 4: Distributed Storage | **Complete** | Shamir sharding, DHT-based distribution, store/fetch protocols |
| 5: Query & Sync | **Complete** | Vector index, hybrid scoring, GossipSub, delta sync, cross-peer query |
| 6: MCP & Polish | **Complete** | MCP server (7 tools), OTel, backup/restore, peer reputation, docs |
| 7: Daemon Architecture | **Complete** | Background daemon, integrated MCP, start/stop commands |
| 8: Networking Enhancements | **In Progress** | QUIC transport, NAT traversal (Circuit Relay v2, hole punching, TURN) |
| 9: Skill Loader | **Complete** | Conversational skill-trigger system, load_skill MCP tool |
| 10: Graph Sync | **Complete** | Distributed edge sync via gossip + delta sync |

## Active Work

Phase 10 complete — distributed knowledge graph edges now propagate across all peers.

Active: Phase 8 (Networking Enhancements — QUIC, NAT traversal).

### Performance: Protobuf Migration (Phase 01-local-foundation)

- Plan 01: Proto schema + codegen + conversion layer ✓
- Plan 02: Store/fetch protocol migration ✓
- Plan 03: Gossip + delta sync + query migration ✓
- **Verification: PASS** — 14/14 test packages, ~500ns/op proto frames

Phase 6 Completed Plans:

- 06-01 (Wave 1): ✓ MCP server core + 7 tools + mcp-serve CLI (10 tests)
- 06-02 (Wave 2): ✓ OpenTelemetry traces/metrics + structured logging with rotation (6 tests)
- 06-03 (Wave 3): ✓ Backup/restore + peer reputation scoring (12 tests)
- 06-04 (Wave 4): ✓ Documentation suite (architecture, MCP, API, CLI, config, troubleshooting)

## Decisions Made

1. **BadgerDB chosen over Pebble**: Better Go ecosystem integration and proven in IPFS
2. **Argon2id for key derivation**: Industry-standard memory-hard KDF
3. **Per-memory encryption keys**: Limits exposure if single key compromised
4. **HKDF-SHA256 for subkey derivation**: Replaces raw SHA256 per Trail of Bits best practices (RFC 5869)
5. **Bearer token over HMAC signing**: Simpler auth appropriate for local-first single-user API
6. **Go stdlib over HTTP framework**: net/http sufficient for 3 endpoints, no Gin/Echo needed
7. **Phase 1 data break accepted**: HKDF changes key derivation output, dev-only data
8. **HKDF-derived libp2p key**: Purpose `"libp2p-host"` for domain separation from master identity
9. **Custom DMGN DHT namespace**: Protocol prefix `/dmgn/kad/1.0.0`, not shared with IPFS
10. **`dmgn start` includes API**: Single command for full operation, `--no-api` for headless nodes
11. **Pure Go vector index over coder/hnsw**: coder/hnsw incompatible with Windows (renameio build tag). Brute-force cosine similarity adequate for expected dataset sizes.
12. **Caller-provided embeddings**: DMGN is a storage/index/sync layer, not a computation platform. AI agents provide pre-computed embeddings.
13. **GossipSub with full encrypted memory**: Entire encrypted memory struct in gossip messages. Simple and no separate fetch needed.
14. **Version vector delta sync**: On reconnect, peers exchange version vectors and send missing memories bidirectionally.
15. **Official MCP Go SDK**: `modelcontextprotocol/go-sdk` for long-term stability over community alternatives.
16. **Local-only MCP by default**: MCP server works offline-first, `--network` flag opts into P2P features.
17. **Weighted reputation scoring**: `0.3*uptime + 0.3*latency + 0.2*sync + 0.2*availability` with exponential decay toward neutral.
18. **Protobuf migration (hybrid)**: Wire (store/fetch, gossip, delta) = protobuf, disk = BadgerDB native, memory = hybrid (protobuf replication + JSON local), API = JSON (required)
19. **QUIC transport**: Add QUIC v1 alongside TCP for improved latency and NAT traversal support
20. **NAT traversal**: Enable Circuit Relay v2, direct hole punching, and TURN fallback for nodes behind NAT

## Blockers

None.

## Quick Tasks Completed

| # | Description | Date | Commit | Directory |
|---|-------------|------|--------|-----------|
| 260410-vom | Update README.md and docs to reflect current DMGN project state | 2026-04-10 | 5effcf6 | [260410-vom-update-the-readme-md-and-docs-based-on-c](./quick/260410-vom-update-the-readme-md-and-docs-based-on-c/) |
| 260409-g3l | Update the README like a pro - it should be a proper README with stuffs like the contributor info, add CONTRIBUTING.md, add the license | 2026-04-09 | dd0d722 | [260409-g3l-update-the-readme-like-a-pro-it-should-b](./quick/260409-g3l-update-the-readme-like-a-pro-it-should-b/) |

## Recent Changes

- 2026-04-10: Phase 8 context captured — QUIC transport, NAT traversal (Circuit Relay v2, hole punching, TURN)
- 2026-04-09: Phase 01 protobuf migration verified — all 4 protocols at v2.0.0, JSON eliminated from wire
- 2026-04-09: Phase 1 context captured — Protobuf migration decisions (wire format, gossip, disk, memory model)
- 2026-04-09: Phase 6 complete — 4 plans executed, 28 new tests, 13 test packages all passing
- 2026-04-09: Phase 6 context gathered — MCP, OTel, docs, backup, peer reputation
- 2026-04-09: Phase 5 complete — 3 plans executed, all tests passing (8 vectorindex + 16 sync + 6 query)
- 2026-04-09: Phase 4 complete — 3 plans executed
- 2026-04-09: Phase 3 complete — 2 plans executed, 36 tests passing (5 network + 3 integration)
- 2026-04-09: Phase 2 complete — all 3 plans executed, 23+ tests passing
- 2025-04-09: Phase 2 planned with research — HKDF, crypto fix, REST API, query scoring
- 2025-04-09: Phase 1 complete - Identity, Storage, Memory model, CLI, Crypto, Config
- 2025-04-09: Project initialized with PROJECT.md, REQUIREMENTS.md, ROADMAP.md

---
*State updated: 2026-04-09 after Phase 6 execution complete*
