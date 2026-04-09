---
gsd_state_version: 1.0
milestone: v1.0
milestone_name: milestone
status: in-progress
last_updated: "2026-04-09T03:53:00.000Z"
progress:
  total_phases: 6
  completed_phases: 2
  total_plans: 3
  completed_plans: 3
  percent: 33
---

# State: DMGN

## Project Reference

See: `.planning/PROJECT.md` (updated 2025-04-09)

**Core value:** User owns their identity and memory data that persists across devices and time, with no central server or third-party control.

**Current focus:** Phase 03 — networking-core

## Phase Progress

| Phase | Status | Notes |
|-------|--------|-------|
| 1: Local Foundation | **Complete** | Identity, storage, CLI init/add |
| 2: Encryption & API | **Complete** | HKDF, crypto framing, REST API, retention, integration tests |
| 3: Networking Core | Not Started | libp2p, DHT, mDNS |
| 4: Distributed Storage | Not Started | Sharding, replication |
| 5: Query & Sync | Not Started | Vector search, gossip |
| 6: MCP & Polish | Not Started | MCP protocol, docs |

## Active Work

Phase 2 complete. Ready for Phase 3 (Networking Core).

Completed Plans:

- 02-01 (Wave 1): ✓ HKDF key derivation, crypto framing fix, configurable retention
- 02-02 (Wave 1): ✓ REST API server with Bearer auth, `dmgn serve` command
- 02-03 (Wave 2): ✓ CLI query scoring, export/import hardening, integration tests

## Decisions Made

1. **BadgerDB chosen over Pebble**: Better Go ecosystem integration and proven in IPFS
2. **Argon2id for key derivation**: Industry-standard memory-hard KDF
3. **Per-memory encryption keys**: Limits exposure if single key compromised
4. **HKDF-SHA256 for subkey derivation**: Replaces raw SHA256 per Trail of Bits best practices (RFC 5869)
5. **Bearer token over HMAC signing**: Simpler auth appropriate for local-first single-user API
6. **Go stdlib over HTTP framework**: net/http sufficient for 3 endpoints, no Gin/Echo needed
7. **Phase 1 data break accepted**: HKDF changes key derivation output, dev-only data

## Blockers

None.

## Recent Changes

- 2026-04-09: Phase 2 complete — all 3 plans executed, 23+ tests passing
- 2025-04-09: Phase 2 planned with research — HKDF, crypto fix, REST API, query scoring
- 2025-04-09: Phase 1 complete - Identity, Storage, Memory model, CLI, Crypto, Config
- 2025-04-09: Project initialized with PROJECT.md, REQUIREMENTS.md, ROADMAP.md

---
*State updated: 2026-04-09 after Phase 2 execution*
