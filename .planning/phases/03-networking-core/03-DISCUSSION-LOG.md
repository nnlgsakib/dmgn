# Phase 3: Networking Core - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-04-09
**Phase:** 03-networking-core
**Areas discussed:** Daemon architecture, libp2p peer identity, Bootstrap & discovery, Connection management

---

## Daemon Architecture

| Option | Description | Selected |
|--------|-------------|----------|
| Merge into start | `dmgn start` launches both libp2p and REST API. `dmgn serve` removed. | |
| Keep separate | `dmgn start` = networking only, `dmgn serve` = API only. | |
| Start includes serve | `dmgn start` launches libp2p + API by default, `--no-api` to disable. | ✓ |

**User's choice:** Start includes serve — single command for common case, `--no-api` for headless nodes.
**Notes:** `dmgn serve` remains for API-only mode (no networking).

| Option | Description | Selected |
|--------|-------------|----------|
| Foreground only | Ctrl+C to stop. Simple, no PID management. | |
| Background daemon | Fork to background, PID file, `dmgn stop`. | |
| Foreground default + --daemon flag | Foreground by default, `--daemon` deferred. | ✓ |

**User's choice:** Foreground default, `--daemon` deferred to future need.

---

## libp2p Peer Identity

| Option | Description | Selected |
|--------|-------------|----------|
| Reuse ed25519 identity | Same private key for libp2p. Links network to crypto identity. | |
| Separate network key | Independent ed25519 key for libp2p. Network pseudonymity. | |
| HKDF-derived network key | Derive libp2p key via HKDF (purpose='libp2p-host'). Domain separation. | ✓ |

**User's choice:** HKDF-derived network key — leverages Phase 2 HKDF infrastructure with proper domain separation.

---

## Bootstrap & Discovery

| Option | Description | Selected |
|--------|-------------|----------|
| IPFS public bootstrap | Use IPFS default bootstrap. Works immediately. Shared DHT. | |
| Custom DMGN bootstrap list | Private DHT namespace. Requires running bootstrap infra. | ✓ |
| Configurable with IPFS defaults | IPFS defaults out of box, user override via config. | |

**User's choice:** Custom DMGN bootstrap list — private DHT namespace.

| Option | Description | Selected |
|--------|-------------|----------|
| Fixed service name | Hardcoded `_dmgn._tcp`. Zero config. | |
| Configurable rendezvous | Custom mDNS string per config. | |
| Fixed default + optional override | `_dmgn._tcp` default, config override. | ✓ |

**User's choice:** Fixed default `_dmgn._tcp` with config override.

---

## Connection Management

| Option | Description | Selected |
|--------|-------------|----------|
| Conservative limits | 8-12 max peers. Low resources. | |
| Generous limits | 50+ peers. High redundancy. | |
| Adaptive with sensible defaults | ~20 max, low=15/high=25 watermarks, ConnManager. | ✓ |

**User's choice:** Adaptive defaults with libp2p ConnManager (low=15, high=25), configurable.

---

## Claude's Discretion

- Network package internal structure
- libp2p option configuration details
- Test strategy for networking
- Log output format

## Deferred Ideas

- `--daemon` flag for background operation
- Peer reputation scoring (NETW-05) — Phase 6
- Protocol handlers — Phase 4
- Gossip sync — Phase 5
