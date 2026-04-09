# Plan 02-02 Summary: REST API Server with Authentication

**Status:** Complete  
**Commit:** feat(02-02)

## Changes Made

### Task 1: Auth Middleware
- Created `internal/api/auth.go` with Bearer token authentication
- API key derived via HKDF with purpose `"api-key"`
- Constant-time comparison using `crypto/hmac.Equal` on SHA256 hashes
- Returns JSON error responses for missing/invalid auth

### Task 2: API Handlers
- Created `internal/api/handlers.go` with 3 endpoints:
  - `POST /memory` — add encrypted memory with JSON request/response
  - `GET /query` — search memories by text, optional `?q=` and `?limit=`
  - `GET /status` — node ID, version, storage stats, network status
- All responses are `application/json`

### Task 3: API Server
- Created `internal/api/server.go` with Go stdlib `net/http`
- Go 1.22+ method-aware routing patterns (`POST /memory`, `GET /query`, etc.)
- Configurable timeouts (read: 15s, write: 15s, idle: 60s)
- Request logging middleware
- Upgraded `go.mod` from `go 1.21` to `go 1.22`

### Task 4: CLI Command
- Created `internal/cli/serve.go` with `dmgn serve` command
- Prints API key on startup for user convenience
- Graceful shutdown on SIGINT/SIGTERM
- Registered in `cmd/dmgn/main.go`

### Task 5: API Tests
- Created `internal/api/server_test.go` with 8 test functions
- Add memory, empty content validation, query with results, recent query,
  status endpoint, auth required, wrong key rejected, correct key accepted

## Bug Fixes
- Fixed `GetMemoriesByTime` broken reverse iteration with inverted timestamps
  - Was using `Reverse=true` which skipped entries due to seek logic
  - Changed to forward iteration (smallest inverted timestamp = newest first)

## Test Results
- `go test ./internal/api/...` — 8/8 PASS
- `go build ./...` — exits 0
