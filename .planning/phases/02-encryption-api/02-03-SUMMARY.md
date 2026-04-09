# Plan 02-03 Summary: CLI Query Enhancement + Integration Tests

**Status:** Complete  
**Commit:** feat(02-03)

## Changes Made

### Task 1: CLI Query with Similarity Scoring
- Rewrote `internal/cli/query.go` with scored search pipeline
- Scoring tiers: exact match (1.0), substring (0.8), word match (0.5), partial (0.3)
- Results sorted by score descending, displayed with `[score]` brackets
- Added `--format json` flag for machine-readable output
- Added timestamp display in text output
- Maintains backward compatibility for `--recent` flag

### Task 2: Export/Import Hardening
- `internal/cli/export.go`: armored export now base64-encodes key data
- `internal/cli/import.go`: armored import base64-decodes before processing
- Added key file validation (checks version, public_key, salt, nonce, ciphertext fields)
- Prints Node ID on successful import

### Task 3: Full Pipeline Integration Tests
- Created `tests/integration_test.go` with 8 test functions:
  - `TestFullPipelineEncryptStoreQueryDecrypt` — end-to-end via API
  - `TestNoPlaintextLeakage` — encrypted payload contains no plaintext
  - `TestHKDFDeterminism` — same identity+purpose → same key
  - `TestCryptoFramingRoundTrip` — 7 payload sizes (0B to 64KB)
  - `TestRetentionIntegration` — store 10 with retention=5, verify 5 remain
  - `TestAPIAuthDerived` — API key matches HKDF derivation
  - `TestExportImportRoundTrip` — export → import → decrypt with imported identity
  - `TestMultipleMemoriesQueryScoring` — query returns matching results

## Test Results
- `go test ./tests/...` — 8/8 PASS (15 subtests total)
- `go test ./...` — all packages PASS
- `go build ./...` — exits 0
