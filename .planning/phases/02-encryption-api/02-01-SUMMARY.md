# Plan 02-01 Summary: Harden Crypto Engine + HKDF Key Derivation + Configurable Retention

**Status:** Complete  
**Commit:** feat(02-01)

## Changes Made

### Task 0: HKDF-SHA256 Key Derivation
- Replaced `SHA256(seed||purpose)` with HKDF-SHA256 (RFC 5869) in `pkg/identity/identity.go`
- New signature: `DeriveKey(purpose string, keyLen int) ([]byte, error)`
- Added `golang.org/x/crypto/hkdf` and `io` imports
- Updated callers in `internal/cli/add.go` and `internal/cli/query.go` to handle `([]byte, error)` return
- Updated `pkg/identity/identity_test.go` with determinism, separation, and key-length tests

### Task 1: Length-Prefixed Crypto Framing
- Replaced hardcoded `encryptedKeyLen := 28` with 2-byte BigEndian length prefix
- `Encrypt`: writes `uint16(len(encryptedKey))` before key+payload
- `Decrypt`: reads length prefix to split key from payload
- Added `encoding/binary` import

### Task 2: Crypto Engine Tests
- Created `internal/crypto/crypto_test.go` with 7 test functions
- Round-trip (empty, 1B, 1KB, 64KB), unique output, wrong key, tampered, truncated

### Task 3: Configurable Retention
- Added `MaxRetention int` to `storage.Options` and `Store`
- Created `pkg/storage/retention.go` with `EnforceRetention()` using time-index iteration
- `SaveMemory` calls `EnforceRetention` when `maxRetention > 0`

### Task 4: Retention Tests
- Created `pkg/storage/retention_test.go` with 4 tests
- Unlimited, limit enforcement, keeps-newest, trigger-on-save

## Bug Fixes
- Fixed `encoding/base58` (non-existent stdlib) → `github.com/mr-tron/base58`
- Fixed BadgerDB API compatibility (`MaxTableSize`, `ValueLogMaxEntries` type, `Backup` signature)
- Fixed `os.ReadAll` → `io.ReadAll` in import.go
- Removed unused `reader` variable in init.go

## Test Results
- `go test ./internal/crypto/...` — 7/7 PASS
- `go test ./pkg/storage/...` — 4/4 PASS
- `go test ./pkg/identity/...` — 7/7 PASS
- `go build ./...` — exits 0
