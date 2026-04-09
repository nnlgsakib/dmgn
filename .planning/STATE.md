---
gsd_state_version: 1.0
milestone: v1.0
milestone_name: milestone
status: Ready to execute
last_updated: "2026-04-09T10:15:19.581Z"
progress:
  total_phases: 6
  completed_phases: 4
  total_plans: 15
  completed_plans: 11
  percent: 73
---

# State: DMGN

## Project Reference

See: `.planning/PROJECT.md` (updated 2025-04-09)

**Core value:** User owns their identity and memory data that persists across devices and time, with no central server or third-party control.

**Current focus:** Phase 06 — MCP & Polish (context gathered, ready for planning)

## Phase Progress

| Phase | Status | Notes |
|-------|--------|-------|
| 1: Local Foundation | **Complete** | Identity, storage, CLI init/add |
| 2: Encryption & API | **Complete** | HKDF, crypto framing, REST API, retention, integration tests |
| 3: Networking Core | **Complete** | libp2p host, DHT, mDNS, peers CLI, live status |
| 4: Distributed Storage | **Complete** | Shamir sharding, DHT-based distribution, store/fetch protocols |
| 5: Query & Sync | **Complete** | Vector index, hybrid scoring, GossipSub, delta sync, cross-peer query |
| 6: MCP & Polish | Not Started | MCP protocol, docs |

## Active Work

Phase 5 complete. Ready for Phase 6 (MCP & Polish).

Phase 5 Completed Plans:

- 05-01 (Wave 1): ✓ Pure Go vector index with encrypted persistence, version vector, config extensions
- 05-02 (Wave 2): ✓ Local query engine with hybrid scoring, GossipSub integration, delta sync protocol
- 05-03 (Wave 3): ✓ Cross-peer query protocol, CLI/API embedding support, full startup wiring

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

## Blockers

None.

## Quick Tasks Completed

| # | Description | Date | Commit | Directory |
|---|-------------|------|--------|-----------|
| 260409-g3l | Update the README like a pro - it should be a proper README with stuffs like the contributor info, add CONTRIBUTING.md, add the license | 2026-04-09 | dd0d722 | [260409-g3l-update-the-readme-like-a-pro-it-should-b](./quick/260409-g3l-update-the-readme-like-a-pro-it-should-b/) |

## Recent Changes

- 2026-04-09: Phase 6 context gathered — MCP, OTel, docs, backup, peer reputation
- 2026-04-09: Phase 5 complete — 3 plans executed, all tests passing (8 vectorindex + 16 sync + 6 query)
- 2026-04-09: Phase 4 complete — 3 plans executed
- 2026-04-09: Phase 3 complete — 2 plans executed, 36 tests passing (5 network + 3 integration)
- 2026-04-09: Phase 2 complete — all 3 plans executed, 23+ tests passing
- 2025-04-09: Phase 2 planned with research — HKDF, crypto fix, REST API, query scoring
- 2025-04-09: Phase 1 complete - Identity, Storage, Memory model, CLI, Crypto, Config
- 2025-04-09: Project initialized with PROJECT.md, REQUIREMENTS.md, ROADMAP.md

---
*State updated: 2026-04-09 after Phase 6 context gathering*
