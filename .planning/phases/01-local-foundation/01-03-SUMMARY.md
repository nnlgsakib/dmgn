---
phase: 01-local-foundation
plan: 03
status: complete
started: "2026-04-09T13:50:00Z"
completed: "2026-04-09T14:00:00Z"
---

# Summary: Gossip, delta sync, and query protocol migration to protobuf

## What was built
- **Gossip**: Removed `GossipMessage` JSON struct, callback now takes `*dmgnpb.GossipMessage`, Publish/Start use `proto.Marshal`/`proto.Unmarshal`
- **Delta sync**: Replaced JSON encoder/decoder with `writeSyncMsg`/`readSyncMsg` (length-prefixed proto), memories serialized via `Memory.ToProto()`/`MemoryFromProto()`, protocol bumped to `/2.0.0`
- **Query protocol**: Removed `QueryProtocolRequest`/`QueryProtocolResponse` JSON types, handler and client use `dmgnpb.QueryRequest`/`dmgnpb.QueryResponse` with `writeQueryMsg`/`readQueryMsg`, protocol bumped to `/2.0.0`
- **Callers updated**: `internal/cli/start.go` gossip callback uses `proto.Unmarshal` + `MemoryFromProto`, `pkg/query/remote.go` constructs `*dmgnpb.QueryRequest` and converts response results
- **Tests updated**: `gossip_test.go`, `delta_test.go` rewritten for proto types, added `TestSyncMsgFrameRoundtrip`

## All protocol IDs now at v2
- `/dmgn/memory/store/2.0.0`
- `/dmgn/memory/fetch/2.0.0`
- `/dmgn/memory/sync/2.0.0`
- `/dmgn/memory/query/2.0.0`

## Commits
- `ef6b170` feat(01-03): migrate gossip and delta sync protocols to protobuf v2
- `a4168be` feat(01-03): migrate query protocol to protobuf v2

## Deviations
- None

## Self-Check: PASSED
- Full test suite: 14/14 packages pass, 0 failures
