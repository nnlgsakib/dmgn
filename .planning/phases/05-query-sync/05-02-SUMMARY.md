# Summary 05-02: Query Engine + GossipSub + Delta Sync

**Status:** ✅ Complete
**Committed:** 5c9aef4

## What was built

- **`pkg/query/engine.go`** — Local query engine with hybrid vector+text scoring (configurable α), filter support (type, time range), snippet generation, BuildRequest helper
- **`pkg/sync/gossip.go`** — GossipSub manager wrapping go-libp2p-pubsub: publish/subscribe to topic, message validation, callback-based receive
- **`pkg/sync/delta.go`** — Delta sync protocol `/dmgn/memory/sync/1.0.0` with version vector exchange, bidirectional memory transfer, periodic sync ticker

## Tests: 11 passing

- 6 query engine tests (text-only, vector-only, hybrid, filters, empty index, snippets)
- 5 sync tests (gossip envelope, invalid JSON, sync request/response marshal, missing logic)
