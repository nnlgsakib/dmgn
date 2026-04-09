# REST API Reference

The DMGN REST API is available when the daemon is running (`dmgn start`).

**Base URL:** `http://localhost:8080` (configurable via `api_port`)

**Authentication:** Bearer token (generated on `dmgn init`, shown on `dmgn start`)

```
Authorization: Bearer <api-key>
```

## Endpoints

### POST /memory

Add a new memory.

**Request:**
```json
{
  "content": "Meeting notes: discussed project timeline",
  "type": "text",
  "links": ["abc123..."],
  "embedding": [0.1, 0.2, 0.3],
  "metadata": {"source": "meeting"}
}
```

**Response (201):**
```json
{
  "id": "a9aa027f0946...",
  "timestamp": 1712678400000000000,
  "type": "text",
  "links": ["abc123..."]
}
```

**curl example:**
```bash
curl -X POST http://localhost:8080/memory \
  -H "Authorization: Bearer <api-key>" \
  -H "Content-Type: application/json" \
  -d '{"content": "Hello DMGN", "type": "text"}'
```

### GET /query

Search memories by text and/or embedding.

**Query Parameters:**
| Param | Type | Description |
|-------|------|-------------|
| `q` | string | Text search query |
| `limit` | int | Max results (default 10) |
| `type` | string | Filter by memory type |
| `embedding` | string | JSON-encoded float32 array |

**Response (200):**
```json
{
  "results": [
    {
      "memory_id": "a9aa027f...",
      "score": 0.85,
      "type": "text",
      "timestamp": 1712678400000000000,
      "snippet": "Meeting notes: discussed project..."
    }
  ],
  "count": 1
}
```

**curl example:**
```bash
curl "http://localhost:8080/query?q=meeting+notes&limit=5" \
  -H "Authorization: Bearer <api-key>"
```

### GET /status

Get node status and statistics.

**Response (200):**
```json
{
  "node_id": "7Xk9Fp2...",
  "peer_count": 3,
  "memory_count": 142,
  "edge_count": 28,
  "vector_index_size": 95,
  "uptime": "2h34m",
  "version": "0.1.0"
}
```

**curl example:**
```bash
curl http://localhost:8080/status \
  -H "Authorization: Bearer <api-key>"
```

### GET /peers

List connected peers.

**Response (200):**
```json
{
  "peers": [
    {
      "id": "QmPeer1...",
      "address": "/ip4/192.168.1.5/tcp/4001",
      "connected_since": "2024-01-15T10:30:00Z"
    }
  ],
  "count": 1
}
```

**curl example:**
```bash
curl http://localhost:8080/peers \
  -H "Authorization: Bearer <api-key>"
```

## Error Responses

All errors return JSON:

```json
{
  "error": "description of the error"
}
```

| Code | Meaning |
|------|---------|
| 400 | Invalid request body or parameters |
| 401 | Missing or invalid Authorization header |
| 404 | Memory or resource not found |
| 500 | Internal server error |
