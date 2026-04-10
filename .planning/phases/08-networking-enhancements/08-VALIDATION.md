---
phase: 8
slug: networking-enhancements
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-04-10
---

# Phase 8 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test |
| **Config file** | none — existing test infrastructure |
| **Quick run command** | `go test ./pkg/network/... ./internal/config/...` |
| **Full suite command** | `go test ./...` |
| **Estimated runtime** | ~30 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./pkg/network/... ./internal/config/...`
- **After every plan wave:** Run `go test ./...`
- **Before `/gsd-verify-work`:** Full suite must be green
- **Max feedback latency:** 30 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 08-01-01 | 01 | 1 | NETW-02 | — | N/A | unit | `go test ./internal/config/... -run TestGetListenAddrs` | ❌ W0 | ⬜ pending |
| 08-01-02 | 01 | 1 | NETW-02 | — | N/A | unit | `go test ./pkg/network/... -run TestQUICListenAddr` | ❌ W0 | ⬜ pending |
| 08-02-01 | 02 | 1 | NETW-04 | — | N/A | unit | `go test ./pkg/network/... -run TestRelayService` | ❌ W0 | ⬜ pending |
| 08-02-02 | 02 | 1 | NETW-04 | — | N/A | unit | `go test ./pkg/network/... -run TestHolePunching` | ❌ W0 | ⬜ pending |
| 08-03-01 | 03 | 2 | NETW-02 | — | N/A | integration | `go test ./internal/daemon/... -run TestDaemonQUIC` | ❌ W0 | ⬜ pending |
| 08-04-01 | 04 | 2 | NETW-06 | T-08-01 | Blocked peers rejected at gater | unit | `go test ./pkg/network/... -run TestReputationGater` | ❌ W0 | ⬜ pending |
| 08-04-02 | 04 | 2 | NETW-08 | — | Rate limited peers get error response | unit | `go test ./pkg/network/... -run TestPeerRateLimiter` | ❌ W0 | ⬜ pending |
| 08-04-03 | 04 | 2 | NETW-09 | — | N/A | unit | `go test ./internal/config/... -run TestDefaultConfig_Security` | ❌ W0 | ⬜ pending |
| 08-05-01 | 05 | 3 | NETW-06 | T-08-01 | Gater blocks connection at libp2p level | integration | `go test ./pkg/network/... -run TestHostWithGater` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `pkg/network/host_test.go` — extend with QUIC and NAT traversal tests
- [ ] `internal/config/config_test.go` — add GetListenAddrs migration tests

*Existing infrastructure covers most phase requirements. Wave 0 extends existing test files.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| NAT traversal via hole punching | NETW-04 | Requires actual NAT environment | Test with two nodes on different networks |
| QUIC connectivity across firewall | NETW-02 | Requires real network conditions | Verify with `dmgn status` showing QUIC addresses |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 30s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
