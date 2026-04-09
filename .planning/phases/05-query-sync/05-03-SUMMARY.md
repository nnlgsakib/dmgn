# Summary 05-03: Cross-Peer Query + CLI/API + Wiring

**Status:** ✅ Complete
**Committed:** 41075ef

## What was built

- **`pkg/query/protocol.go`** — Cross-peer query protocol `/dmgn/memory/query/1.0.0` with RegisterQueryHandler and QueryPeer
- **`pkg/query/remote.go`** — RemoteQueryOrchestrator: fan-out to connected peers, dedup by memory_id, source diversity interleaving
- **`internal/cli/add.go`** — Added `--embedding` flag for caller-provided embeddings
- **`internal/api/handlers.go`** — AddMemoryRequest now accepts `embedding` field; HandleQuery accepts `embedding` query parameter; results include `score` and `source_peer`
- **`internal/api/server.go`** — Added queryEngine, remoteOrch, gossipMgr, vecIndex fields with setters
- **`internal/cli/start.go`** — Full Phase 5 wiring: vector index (encrypted), query engine, remote orchestrator, query protocol handler, version vector, gossip manager, delta sync manager
- **`pkg/storage/storage.go`** — Added DB() accessor for cross-package BadgerDB use

## Tests: All existing tests continue to pass

No regressions. Full `go test ./...` passes across all 10 test packages.
