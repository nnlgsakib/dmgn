# Plan 03-02 Summary: CLI Commands — start, peers, status

**Status:** Complete
**Wave:** 2

## What Was Built

### `dmgn start` Rewrite (`internal/cli/start.go`)
- Loads identity with passphrase prompt
- Derives libp2p host key via `DeriveLibp2pKey(id)`
- Creates and starts libp2p host with config-driven settings
- Prints peer ID and listen addresses on startup
- Optionally starts REST API server (unless `--no-api` flag)
- Attaches network host to API server for live stats
- Graceful shutdown: API → network host → exit
- New flags: `--no-api`, `--data-dir`

### `dmgn peers` Command (`internal/cli/peers.go`)
- Queries local API `GET /peers` endpoint with auth
- Displays connected peers with IDs, addresses, and latency
- Registered in `cmd/dmgn/main.go`

### `dmgn status` Update (`internal/cli/status.go`)
- TCP port check to detect running node (no passphrase needed)
- Shows "Running" with hint to use `dmgn peers` for details
- Shows "Not running" when node is offline
- No UX regression — status remains non-interactive

### API Server Updates
- **`internal/api/server.go`**: Added `networkHost` field, `SetNetworkHost()` method, `/peers` route
- **`internal/api/handlers.go`**: 
  - `HandleStatus` returns live network stats (status, peer_id, peers, listen_addrs) when host attached
  - `HandlePeers` returns connected peer list as JSON
  - `NetworkStats` struct extended with `PeerID` and `ListenAddrs` fields

### Integration Tests (`tests/integration_test.go`)
- `TestStartWithNetworking` — host start, peer ID, DHT active
- `TestAPIStatusWithNetwork` — /status returns live network info
- `TestTwoPeersDiscoverViaDirect` — two hosts connect and see each other

## Requirements Addressed
- **CLI-02:** `dmgn start` launches full node (networking + API)
- **CLI-05:** `dmgn peers` lists connected peers
- **CLI-06:** `dmgn status` shows live network status
