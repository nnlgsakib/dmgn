# Research: Phase 2 — Encryption & API

**Researched:** 2025-04-09
**Areas:** Go AES-GCM patterns, key derivation, REST API auth, BadgerDB encryption

## 1. Key Derivation — HKDF Required

### Finding
The current `Identity.DeriveKey()` uses `sha256(seed + purpose)` with iterative hashing. This is **not cryptographically sound** for key derivation:
- Simple SHA256 concatenation is vulnerable to related-key attacks
- No domain separation between different key purposes
- Trail of Bits (Jan 2025) explicitly recommends HKDF for subkey derivation

### Recommendation
Replace with HKDF-SHA256 from `golang.org/x/crypto/hkdf` (already an indirect dependency via x/crypto):

```go
import "golang.org/x/crypto/hkdf"

func (i *Identity) DeriveKey(purpose string, keyLen int) ([]byte, error) {
    ikm := i.PrivateKey.Seed() // 32-byte ed25519 seed = high-entropy IKM
    info := []byte(purpose)     // unique per subkey
    reader := hkdf.New(sha256.New, ikm, nil, info) // nil salt = zero salt per RFC 5869
    key := make([]byte, keyLen)
    _, err := io.ReadFull(reader, key)
    return key, err
}
```

Key points from Trail of Bits best practices:
- Use `info` parameter for domain separation (e.g., "memory-encryption", "api-key")
- Salt can be nil/constant when IKM is already high-entropy (ed25519 seed qualifies)
- Never use attacker-controlled values as salt
- HKDF output is indistinguishable from random for each unique info value

### Impact
- Changes `Identity.DeriveKey` signature (adds error return, keyLen param)
- All callers in `add.go`, `query.go`, `serve.go` need updating
- Must ensure backward compatibility with Phase 1 data or provide migration

### Decision Required
**Breaking change**: Phase 1 data encrypted with old DeriveKey will NOT decrypt with new HKDF-based key. Options:
1. **Accept break** — Phase 1 is dev/prototype, no production data to migrate
2. **Dual-path** — Try HKDF first, fall back to legacy SHA256 derivation
3. **Migration tool** — Re-encrypt existing data on first run

**Recommendation:** Option 1 (accept break). Phase 1 is development-only.

## 2. Envelope Encryption — Framing Bug

### Finding
`internal/crypto/crypto.go` has `encryptedKeyLen := 28` hardcoded in `Decrypt()`. This is wrong:
- AES-GCM with 12-byte nonce encrypting 32-byte key = 12 (nonce) + 32 + 16 (tag) = **60 bytes**
- The value 28 would only work for a 0-byte payload, which makes no sense

The `Encrypt()` method concatenates `encryptedKey + encryptedPayload` without any length delimiter, so `Decrypt()` needs to know the exact key ciphertext length.

### Recommendation
Use length-prefixed framing:
- `Encrypt()`: prepend 2-byte big-endian length of encrypted key
- `Decrypt()`: read 2 bytes, extract key ciphertext by length
- Format: `[2-byte keyLen][encryptedKey][encryptedPayload]`

This is self-describing and survives nonce size changes (e.g., if we switch GCM variants).

### Alternative Considered
Fixed-size scheme: since we always encrypt a 32-byte key with AES-GCM (12-byte nonce), the encrypted key is always 60 bytes. Could hardcode 60 instead of 28. However, length-prefixed is more robust and only costs 2 bytes.

## 3. REST API Authentication

### Finding
For a local-first system where:
- The user controls both client and server
- API is primarily for localhost AI agent access
- Identity already provides cryptographic material for key derivation

A **Bearer token** derived from identity is the simplest secure approach:
- Derive API key: `HKDF(seed, info="api-key")` → 32 bytes → hex-encode → 64-char token
- Server stores SHA256(apiKey) for comparison using `hmac.Equal` (constant-time)
- Client sends: `Authorization: Bearer <hex-token>`

### Why Not HMAC Request Signing
HMAC request signing (per-request signatures over method+path+body+timestamp) is more secure against replay attacks but adds significant complexity:
- Client must implement canonical string building
- Clock synchronization required
- Overkill for localhost-only API in Phase 2

### Why Not JWT
- JWT requires token expiration/refresh logic
- Adds dependency (jwt-go library)
- No benefit over static derived key for single-user system

### Recommendation
Simple Bearer token with:
- Constant-time comparison (`crypto/subtle.ConstantTimeCompare` or `hmac.Equal`)
- Rate limiting on auth failures (defer to Phase 6)
- Token displayed once on `dmgn serve` startup

## 4. BadgerDB Encryption-at-Rest

### Finding
BadgerDB v2+ supports native encryption at rest via `Options.EncryptionKey`:
```go
opts := badger.DefaultOptions(path)
opts.EncryptionKey = []byte("32-byte-encryption-key-here....")
```

This encrypts the WAL and SST files using AES-CTR with random IVs.

### Recommendation
**Do NOT use BadgerDB encryption in Phase 2.** Reasons:
- We already encrypt payloads before storage (defense in depth)
- BadgerDB encryption adds ~15% write overhead
- It complicates key management (need key at DB open time)
- It would encrypt metadata (time indexes, edge keys) which are not sensitive

**Consider for Phase 4** (distributed storage) where raw files may transit the network.

## 5. Go stdlib HTTP vs Framework

### Finding
For 3 endpoints (POST /memory, GET /query, GET /status), Go stdlib `net/http` is sufficient:
- `http.ServeMux` handles routing since Go 1.22 (method-aware patterns)
- No need for Gin/Echo/Chi framework overhead
- Middleware chaining via `func(http.Handler) http.Handler` pattern
- `httptest.NewServer` for clean testing

### Recommendation
Use Go stdlib. Add a framework only if endpoint count exceeds ~10 (Phase 6 MCP might warrant it).

## 6. Merkle Proofs on Encrypted Data (CRPT-04)

### Finding
CRPT-04 requires "Merkle proofs calculated on encrypted data." Current implementation:
- `memory.Memory.ID` = SHA256 of encrypted payload ✓ (content-addressable)
- No batch Merkle tree yet

For Phase 2, the per-memory hash (ID = hash of encrypted payload) satisfies the single-memory integrity requirement. Full Merkle trees over batches are Phase 4 (distributed storage) where integrity proofs are needed for remote verification.

### Recommendation
Phase 2 scope: ensure `Memory.ID = SHA256(encrypted_payload)` is always verified on load. Defer batch Merkle trees to Phase 4.

---

## Summary of Plan Impacts

| Finding | Impact | Plan |
|---------|--------|------|
| HKDF for key derivation | Rewrite `Identity.DeriveKey`, update all callers | 02-01 (new task) |
| Envelope framing bug | Fix Encrypt/Decrypt with length prefix | 02-01 (existing task) |
| Bearer token auth | Simpler than originally planned | 02-02 (simplifies) |
| No BadgerDB encryption | Remove from scope | 02-01 (no change) |
| Go stdlib HTTP | Confirms original plan | 02-02 (confirms) |
| Merkle = per-memory hash | Already done, verify on load | 02-01 (minor addition) |
