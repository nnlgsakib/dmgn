---
phase: 1
slug: local-foundation
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-04-09
---

# Phase 1 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test |
| **Config file** | none — standard Go test toolchain |
| **Quick run command** | `go test ./pkg/memory/... ./pkg/network/... ./pkg/sync/... ./pkg/query/...` |
| **Full suite command** | `go test ./...` |
| **Estimated runtime** | ~15 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./pkg/memory/... ./pkg/network/... ./pkg/sync/... ./pkg/query/...`
- **After every plan wave:** Run `go test ./...`
- **Before `/gsd-verify-work`:** Full suite must be green
- **Max feedback latency:** 15 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 1-01-01 | 01 | 1 | D-18, D-19 | — | N/A | unit | `go test ./proto/...` | ❌ W0 | ⬜ pending |
| 1-01-02 | 01 | 1 | D-11, D-12, D-13 | — | N/A | unit | `go test ./pkg/memory/...` | ✅ | ⬜ pending |
| 1-02-01 | 02 | 2 | D-01, D-02 | — | N/A | unit | `go test ./pkg/network/...` | ✅ | ⬜ pending |
| 1-02-02 | 02 | 2 | D-22, D-23 | — | N/A | benchmark | `go test -bench=. ./pkg/network/...` | ❌ W0 | ⬜ pending |
| 1-03-01 | 03 | 3 | D-03, D-06, D-07 | — | N/A | unit | `go test ./pkg/sync/...` | ✅ | ⬜ pending |
| 1-03-02 | 03 | 3 | D-04 | — | N/A | unit | `go test ./pkg/sync/...` | ✅ | ⬜ pending |
| 1-03-03 | 03 | 3 | D-01 | — | N/A | unit | `go test ./pkg/query/...` | ✅ | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `proto/dmgn/v1/dmgn.proto` — protobuf schema definition
- [ ] `proto/dmgn/v1/dmgn.pb.go` — generated Go bindings
- [ ] Benchmark test stubs for JSON vs proto comparison

*Existing test infrastructure (go test) covers all phase requirements.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Proto compilation on Windows | D-19 | Depends on protoc binary in PATH | Run `protoc --version` and `make proto` |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 15s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
