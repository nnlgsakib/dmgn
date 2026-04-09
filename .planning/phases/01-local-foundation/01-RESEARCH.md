# Phase 1: Local Foundation — Protobuf Migration Research

**Researched:** 2026-04-09
**Status:** Complete

## Executive Summary

This phase migrates DMGN wire protocols from JSON to Protocol Buffers for 2-3x payload reduction and 5-10x faster parsing. The codebase has ~47 JSON marshal/unmarshal calls; only the **wire** path (9 uses across 4 protocol files) and **gossip/sync** (8 uses) are migration targets. Disk storage (BadgerDB) and API layers remain JSON per CONTEXT.md decisions.

---

## 1. Current JSON Usage Audit

### Wire Protocols — HOT PATH (9 uses) → **MIGRATE**

| File | Struct | JSON Calls | Frequency |
|------|--------|------------|-----------|
| `pkg/network/protocols.go` | `StoreRequest`, `StoreResponse`, `FetchRequest`, `FetchResponse` | 5 (marshal in `writeFrame`, unmarshal in handlers + client) | Every shard store/fetch |
| `pkg/query/protocol.go` | `QueryProtocolRequest`, `QueryProtocolResponse` | 4 (encode/decode on both sides) | Every cross-peer query |

### Gossip + Delta Sync — HIGH FREQUENCY (8 uses) → **MIGRATE**

| File | Struct | JSON Calls | Frequency |
|------|--------|------------|-----------|
| `pkg/sync/gossip.go` | `GossipMessage` | 2 (`json.Marshal` in `Publish`, `json.Unmarshal` in `Start`) | Every memory broadcast |
| `pkg/sync/delta.go` | `syncRequest`, `syncResponse` | 6 (`json.NewEncoder/Decoder` × 3 exchange points, `json.Marshal/Unmarshal` in `collectMissingMemories`/`processReceivedMemories`) | Every peer reconnect |

### Disk Storage (9 uses) → **NO CHANGE** (per D-08 to D-10)

| File | Struct | JSON Calls |
|------|--------|------------|
| `pkg/storage/shards.go` | `sharding.Shard` | 4 (marshal/unmarshal in Save/Get/GetForMemory) |
| `pkg/storage/storage.go` | `memory.Memory` via `ToJSON()`/`FromJSON()` | 2 (Save/Get) |
| `pkg/network/reputation.go` | `PeerReputation` | 3 (save/load/loadAll) |

### API Layer (24 uses) → **NO CHANGE** (per D-14 to D-17)

REST API handlers, MCP server, CLI output — all spec-required JSON.

### Memory Model (5 uses) → **HYBRID** (per D-11 to D-13)

| File | Function | JSON Call | Migration |
|------|----------|-----------|-----------|
| `pkg/memory/memory.go` | `Create()` | `json.Marshal(plaintext)` | No change (internal encryption input) |
| `pkg/memory/memory.go` | `Decrypt()` | `json.Unmarshal(plaintextJSON)` | No change (internal decryption output) |
| `pkg/memory/memory.go` | `ToJSON()` | `json.Marshal(m)` | Keep for disk; add `ToProto()` for wire |
| `pkg/memory/memory.go` | `FromJSON()` | `json.Unmarshal(data)` | Keep for disk; add `FromProto()` for wire |
| `pkg/storage/storage.go` | `loadGraph()` | `memory.FromJSON(val)` | No change (disk path) |

---

## 2. Protobuf Schema Design

### Proto File: `proto/dmgn/v1/dmgn.proto`

```protobuf
syntax = "proto3";
package dmgn.v1;
option go_package = "github.com/nnlgsakib/dmgn/proto/dmgnpb";

// Core memory struct (wire representation)
message Memory {
  string id = 1;
  int64 timestamp = 2;
  string type = 3;
  repeated float embedding = 4 [packed = true];
  bytes encrypted_payload = 5;
  repeated string links = 6;
  string merkle_proof = 7;
  map<string, string> metadata = 8;
}

// Store/Fetch protocol
message StoreRequest {
  string memory_id = 1;
  int32 shard_index = 2;
  int32 total_shards = 3;
  int32 threshold = 4;
  string checksum = 5;
  int32 data_len = 6;
}

message StoreResponse {
  string status = 1;
  string message = 2;
}

message FetchRequest {
  string memory_id = 1;
  int32 shard_index = 2;
}

message FetchResponse {
  string status = 1;
  string memory_id = 2;
  int32 shard_index = 3;
  int32 total_shards = 4;
  int32 threshold = 5;
  string checksum = 6;
  int32 data_len = 7;
  string message = 8;
}

// Gossip messages
message GossipMessage {
  string type = 1;
  bytes memory = 2;       // Serialized Memory proto
  string sender_peer_id = 3;
  int64 timestamp = 4;
  uint64 sequence = 5;
}

// Delta sync messages
message SyncRequest {
  string sender_peer_id = 1;
  map<string, uint64> version_vector = 2;
}

message SyncResponse {
  string sender_peer_id = 1;
  map<string, uint64> version_vector = 2;
  repeated bytes memories = 3;  // Each is a serialized Memory proto
}

// Query protocol
message QueryFilters {
  string type = 1;
  int64 after = 2;
  int64 before = 3;
}

message QueryRequest {
  string query_id = 1;
  repeated float embedding = 2 [packed = true];
  string text_query = 3;
  int32 limit = 4;
  QueryFilters filters = 5;
}

message QueryResult {
  string memory_id = 1;
  double score = 2;
  string type = 3;
  int64 timestamp = 4;
  string snippet = 5;
  string source_peer = 6;
}

message QueryResponse {
  string query_id = 1;
  repeated QueryResult results = 2;
  int32 total_searched = 3;
}
```

### Design Notes

1. **`repeated float embedding` with `[packed = true]`**: Wire-efficient for dense float arrays. Go protobuf already packs repeated scalars by default in proto3, but explicit annotation for clarity.
2. **`bytes memory` in GossipMessage**: Holds a serialized `Memory` proto. Matches existing pattern where `GossipMessage.Memory` is `[]byte` (the full encrypted memory JSON). Decode only when needed.
3. **`repeated bytes memories` in SyncResponse**: Replaces `[]json.RawMessage`. Each element is a serialized `Memory` proto — same lazy-deserialization pattern.
4. **`double score` vs `float`**: QueryResult uses Go `float64`, which maps to protobuf `double`.
5. **Field numbering**: Sequential, no gaps, matching the Go struct field order for readability.

---

## 3. Toolchain & Dependencies

### Required Tools

| Tool | Version | Purpose |
|------|---------|---------|
| `protoc` | 3.21+ | Protocol Buffer compiler |
| `protoc-gen-go` | latest (from `google.golang.org/protobuf/cmd/protoc-gen-go`) | Go code generation |

### Go Dependencies

- **Promote to direct**: `google.golang.org/protobuf` (already v1.36.11 as indirect via libp2p)
- **No new external deps needed**: `proto.Marshal()` / `proto.Unmarshal()` from `google.golang.org/protobuf/proto`

### Code Generation Command

```bash
protoc --go_out=. --go_opt=paths=source_relative proto/dmgn/v1/dmgn.proto
```

Output: `proto/dmgn/v1/dmgn.pb.go` — generated structs with marshal/unmarshal methods.

### Makefile Target

```makefile
proto:
	protoc --go_out=. --go_opt=paths=source_relative proto/dmgn/v1/dmgn.proto
```

---

## 4. Conversion Layer Design

### Memory Model: `pkg/memory/memory.go`

Add two new methods to `Memory`:

```go
func (m *Memory) ToProto() *dmgnpb.Memory { ... }
func MemoryFromProto(pb *dmgnpb.Memory) *Memory { ... }
```

These convert between the Go `Memory` struct and the generated protobuf struct. The existing `ToJSON()` / `FromJSON()` remain untouched for disk storage.

### Wire Frame Functions: `pkg/network/protocols.go`

Replace `writeFrame` / `readFrame` internals:

**Current:** `json.Marshal(header)` → length-prefixed bytes → write
**New:** `proto.Marshal(header)` → length-prefixed bytes → write

The length-prefixed framing pattern (`4-byte big-endian length + payload`) is already binary-friendly and stays as-is. Only the payload encoding changes from JSON to protobuf.

New functions:
```go
func writeProtoFrame(w io.Writer, msg proto.Message, data []byte) error
func readProtoFrame(r io.Reader, msg proto.Message, dataLen int) ([]byte, error)
```

### Gossip: `pkg/sync/gossip.go`

- `Publish()`: Replace `json.Marshal(msg)` with `proto.Marshal(&dmgnpb.GossipMessage{...})`
- `Start()`: Replace `json.Unmarshal(msg.Data, &gossipMsg)` with `proto.Unmarshal(msg.Data, &pbMsg)`

### Delta Sync: `pkg/sync/delta.go`

- Replace `json.NewEncoder/Decoder` streaming with length-prefixed protobuf frames
- `collectMissingMemories()`: Replace `json.Marshal(mem)` with `proto.Marshal(mem.ToProto())`
- `processReceivedMemories()`: Replace `json.Unmarshal` with `proto.Unmarshal` + `MemoryFromProto`

### Query Protocol: `pkg/query/protocol.go`

- Replace `json.NewEncoder/Decoder` with length-prefixed protobuf frames
- Convert `QueryRequest`/`QueryResponse` Go structs to/from protobuf messages

---

## 5. Protocol Version Strategy

### Option A: Version Bump (Recommended)

Bump protocol IDs from `/1.0.0` to `/2.0.0`:
- `/dmgn/memory/store/2.0.0`
- `/dmgn/memory/fetch/2.0.0`
- `/dmgn/memory/sync/2.0.0`
- `/dmgn/memory/query/2.0.0`

**Pros:** Clean break. Old peers reject new streams (unknown protocol), preventing corruption.
**Cons:** No backward compatibility with v1.0.0 peers.

### Option B: Dual Registration

Register both `/1.0.0` (JSON) and `/2.0.0` (protobuf) handlers. Prefer v2 when connecting.

**Pros:** Gradual rollout. Mixed clusters work.
**Cons:** Doubles handler code, harder to test, complexity for a pre-1.0 project.

### Recommendation

**Option A** — this is a pre-release project with no deployed production peers. Clean version bump is simpler and sufficient. Add a constant set:

```go
const (
    StoreProtocolV1 = protocol.ID("/dmgn/memory/store/1.0.0")  // deprecated
    StoreProtocolV2 = protocol.ID("/dmgn/memory/store/2.0.0")
    StoreProtocol   = StoreProtocolV2  // current
)
```

---

## 6. Migration Order

Per D-21: Wire protocols first (hot path), then gossip.

### Wave 1: Proto Schema + Code Generation + Conversion Layer
1. Create `proto/dmgn/v1/dmgn.proto`
2. Generate Go code
3. Add `ToProto()` / `FromProto()` to `Memory` struct
4. Add `writeProtoFrame()` / `readProtoFrame()` utility functions
5. Tests for conversion roundtrip

### Wave 2: Store/Fetch Protocol Migration
1. Migrate `protocols.go` structs to use generated protobuf types
2. Replace `writeFrame`/`readFrame` JSON calls with proto equivalents
3. Update `RegisterStoreHandler`, `RegisterFetchHandler`, `SendShard`, `FetchShard`
4. Bump protocol IDs to v2
5. Update `pkg/network/protocols_test.go`

### Wave 3: Gossip + Delta Sync + Query Protocol Migration
1. Migrate `gossip.go` to use protobuf `GossipMessage`
2. Migrate `delta.go` to use protobuf `SyncRequest`/`SyncResponse` with length-prefixed framing
3. Migrate `query/protocol.go` to use protobuf `QueryRequest`/`QueryResponse`
4. Update tests for all three

---

## 7. Testing Strategy

### Unit Tests

| Test | Purpose |
|------|---------|
| `proto/dmgnpb_test.go` | Roundtrip: Go struct → Proto → bytes → Proto → Go struct |
| `memory.TestToProtoFromProto` | Memory conversion preserves all fields |
| `protocols.TestProtoFrame` | writeProtoFrame/readProtoFrame roundtrip |
| `protocols.TestStoreProtocolV2` | Full store protocol with protobuf |
| `protocols.TestFetchProtocolV2` | Full fetch protocol with protobuf |
| `gossip.TestProtobufPublish` | GossipMessage marshal/unmarshal |
| `delta.TestProtobufSync` | Full delta sync exchange with protobuf |
| `query.TestProtobufQuery` | Query protocol protobuf roundtrip |

### Size Comparison Benchmarks

```go
func BenchmarkStoreRequestJSON(b *testing.B) { ... }
func BenchmarkStoreRequestProto(b *testing.B) { ... }
```

Target: Confirm 2-3x size reduction and 5-10x parse speed improvement.

### Integration Test

Existing `tests/integration_test.go` should still pass after migration — verify that end-to-end store/fetch/query flows work with protobuf wire format.

---

## 8. Risk Assessment

| Risk | Impact | Likelihood | Mitigation |
|------|--------|------------|------------|
| Proto compilation fails on Windows | High | Low | Use pre-built `protoc` binary for Windows; verify path setup |
| Embedding precision loss (float32 → proto float) | Low | None | Proto `float` = IEEE 754 float32, exact match |
| Delta sync streaming breaks with proto | Medium | Medium | Length-prefix each proto message; test bidirectional exchange |
| Existing tests break | Medium | High (expected) | Update tests alongside migration; run `go test ./...` after each wave |
| `json.RawMessage` pattern in delta.go | Medium | Medium | Replace with `[]byte` holding serialized proto; same lazy pattern |

---

## 9. Files Modified (Summary)

| File | Change Type | Scope |
|------|-------------|-------|
| `proto/dmgn/v1/dmgn.proto` | **NEW** | Proto schema definition |
| `proto/dmgn/v1/dmgn.pb.go` | **GENERATED** | Protobuf Go bindings |
| `pkg/memory/memory.go` | **MODIFY** | Add `ToProto()` / `FromProto()` |
| `pkg/network/protocols.go` | **MODIFY** | Replace JSON with proto in write/readFrame + handlers |
| `pkg/sync/gossip.go` | **MODIFY** | Replace JSON with proto marshal/unmarshal |
| `pkg/sync/delta.go` | **MODIFY** | Replace JSON streaming with proto frames |
| `pkg/query/protocol.go` | **MODIFY** | Replace JSON with proto encode/decode |
| `go.mod` | **MODIFY** | Promote `google.golang.org/protobuf` to direct dependency |
| `Makefile` | **NEW/MODIFY** | Add `proto` target |

---

## Validation Architecture

### Dimension Coverage

| Dimension | Validation Approach |
|-----------|-------------------|
| Correctness | Roundtrip tests: Go struct → proto → bytes → proto → Go struct for all message types |
| Performance | Benchmarks comparing JSON vs proto marshal size and parse latency |
| Compatibility | Protocol version bump ensures no silent corruption with old peers |
| Completeness | All 17 wire JSON calls migrated; all 24 API JSON calls untouched |

## RESEARCH COMPLETE
