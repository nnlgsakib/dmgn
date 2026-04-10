# Phase 8: Networking Enhancements - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-04-10
**Phase:** 08-networking-enhancements
**Areas discussed:** Transport configuration, NAT traversal mechanisms

---

## Transport Configuration

| Option | Description | Selected |
|--------|-------------|----------|
| QUIC + TCP (both) | Listen on both TCP and QUIC. Both protocols active. | |
| QUIC only | Listen on QUIC only. Remove TCP transport entirely. | |
| Keep TCP, add QUIC | Keep current TCP but add QUIC as secondary option. | ✓ |

**User's choice:** Keep TCP, add QUIC — both transports active for maximum compatibility.
**Notes:** Default listen addresses will be `/ip4/0.0.0.0/tcp/0` AND `/ip4/0.0.0.0/udp/0/quic-v1`. QUIC v1 (RFC 9000).

---

## NAT Traversal Mechanisms

| Option | Description | Selected |
|--------|-------------|----------|
| Circuit Relay v2 | libp2p autorelay with circuit relay v2 - nodes can relay through peers with public addresses | ✓ |
| Direct Hole Punching | Direct UDP hole punching with SRE/ENR - requires coordination between peers | ✓ |
| TURN fallback | Use TURN servers as fallback - commercial third-party relay | ✓ |
| Both relay and hole punch | Enable both circuit relay and hole punching | ✓ |

**User's choice:** All of the above — Circuit Relay v2, direct hole punching, and TURN fallback as last resort.
**Notes:** Autorelay enabled by default. Node finds relay peers automatically. EnableRelayService allows node to act as relay for other peers.

---

## Agent's Discretion

- Exact QUIC tuning parameters (conn IDs, flow control)
- TURN server configuration format
- Logging verbosity for NAT traversal events
- Test strategy for NAT scenarios

---

*Phase: 08-networking-enhancements*
*Context gathered: 2026-04-10*