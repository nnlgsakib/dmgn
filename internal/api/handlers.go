package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/nnlgsakib/dmgn/pkg/memory"
)

type AddMemoryRequest struct {
	Content   string            `json:"content"`
	Type      string            `json:"type,omitempty"`
	Links     []string          `json:"links,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"`
	Embedding []float32         `json:"embedding,omitempty"`
}

type AddMemoryResponse struct {
	ID        string `json:"id"`
	Timestamp int64  `json:"timestamp"`
	Type      string `json:"type"`
}

type QueryResultAPI struct {
	ID         string            `json:"id"`
	Content    string            `json:"content"`
	Type       string            `json:"type"`
	Score      float64           `json:"score"`
	Timestamp  int64             `json:"timestamp"`
	Links      []string          `json:"links"`
	Metadata   map[string]string `json:"metadata,omitempty"`
	SourcePeer string            `json:"source_peer,omitempty"`
}

type QueryResponse struct {
	Results []QueryResultAPI `json:"results"`
	Count   int              `json:"count"`
}

type StatusResponse struct {
	NodeID  string       `json:"node_id"`
	Version string       `json:"version"`
	Storage StorageStats `json:"storage"`
	Network NetworkStats `json:"network"`
	Shards  ShardStats   `json:"shards"`
}

type ShardStats struct {
	LocalShards int64 `json:"local_shards"`
}

type StorageStats struct {
	MemoryCount int64  `json:"memory_count"`
	EdgeCount   int64  `json:"edge_count"`
	Path        string `json:"path"`
}

type NetworkStats struct {
	Status      string   `json:"status"`
	Peers       int      `json:"peers"`
	PeerID      string   `json:"peer_id,omitempty"`
	ListenAddrs []string `json:"listen_addrs,omitempty"`
}

func (s *Server) HandleAddMemory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	var req AddMemoryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
		return
	}

	if req.Content == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "content is required"})
		return
	}

	memType := memory.Type(req.Type)
	if memType == "" {
		memType = memory.TypeText
	}

	metadata := req.Metadata
	if metadata == nil {
		metadata = map[string]string{}
	}
	metadata["source"] = "api"
	metadata["author"] = s.identity.ID

	plaintext := &memory.PlaintextMemory{
		Content:  req.Content,
		Type:     memType,
		Metadata: metadata,
	}

	encryptFn := func(data []byte) ([]byte, error) {
		return s.cryptoEng.Encrypt(data)
	}

	mem, err := memory.Create(plaintext, req.Links, encryptFn)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("failed to create memory: %v", err)})
		return
	}

	// Set embedding if provided by caller
	if len(req.Embedding) > 0 {
		mem.Embedding = req.Embedding
	}

	// Index embedding in vector index if available
	if s.vecIndex != nil && len(mem.Embedding) > 0 {
		s.vecIndex.Add(mem.ID, mem.Embedding)
	}

	if err := s.store.SaveMemory(mem); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("failed to save memory: %v", err)})
		return
	}

	writeJSON(w, http.StatusCreated, AddMemoryResponse{
		ID:        mem.ID,
		Timestamp: mem.Timestamp,
		Type:      string(mem.Type),
	})
}

func (s *Server) HandleQuery(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	queryText := r.URL.Query().Get("q")
	limitStr := r.URL.Query().Get("limit")
	embeddingStr := r.URL.Query().Get("embedding")
	limit := 10
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	// Parse optional embedding parameter
	var embedding []float32
	if embeddingStr != "" {
		if err := json.Unmarshal([]byte(embeddingStr), &embedding); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid embedding JSON"})
			return
		}
	}

	// Use query engine if available
	if s.queryEngine != nil && (queryText != "" || len(embedding) > 0) {
		qReq := s.queryEngine.BuildRequest(queryText, embedding, limit)
		results, err := s.queryEngine.SearchLocal(qReq)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("query failed: %v", err)})
			return
		}

		apiResults := make([]QueryResultAPI, 0, len(results))
		for _, r := range results {
			apiResults = append(apiResults, QueryResultAPI{
				ID:         r.MemoryID,
				Content:    r.Snippet,
				Type:       r.Type,
				Score:      r.Score,
				Timestamp:  r.Timestamp,
				SourcePeer: r.SourcePeer,
			})
		}

		writeJSON(w, http.StatusOK, QueryResponse{
			Results: apiResults,
			Count:   len(apiResults),
		})
		return
	}

	// Fallback: original search logic
	decryptFn := func(ciphertext []byte) ([]byte, error) {
		return s.cryptoEng.Decrypt(ciphertext)
	}

	var memories []*memory.Memory
	var err error

	if queryText == "" {
		memories, err = s.store.GetRecentMemories(limit)
	} else {
		memories, err = s.searchMemories(queryText, limit, decryptFn)
	}

	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("query failed: %v", err)})
		return
	}

	apiResults := make([]QueryResultAPI, 0, len(memories))
	for _, mem := range memories {
		plain, err := mem.Decrypt(decryptFn)
		if err != nil {
			continue
		}

		apiResults = append(apiResults, QueryResultAPI{
			ID:        mem.ID,
			Content:   plain.Content,
			Type:      string(mem.Type),
			Timestamp: mem.Timestamp,
		})
	}

	writeJSON(w, http.StatusOK, QueryResponse{
		Results: apiResults,
		Count:   len(apiResults),
	})
}

func (s *Server) HandleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	stats, err := s.store.GetStats()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("failed to get stats: %v", err)})
		return
	}

	netStats := NetworkStats{
		Status: "offline",
		Peers:  0,
	}
	if s.networkHost != nil {
		netStats.Status = "running"
		netStats.Peers = s.networkHost.PeerCount()
		netStats.PeerID = s.networkHost.ID().String()
		addrs := s.networkHost.Addrs()
		netStats.ListenAddrs = make([]string, 0, len(addrs))
		for _, a := range addrs {
			netStats.ListenAddrs = append(netStats.ListenAddrs, a.String())
		}
	}

	shardStats, _ := s.store.GetShardStats()
	var localShards int64
	if shardStats != nil {
		localShards = shardStats["shard_count"]
	}

	writeJSON(w, http.StatusOK, StatusResponse{
		NodeID:  s.identity.ID,
		Version: s.config.Version,
		Storage: StorageStats{
			MemoryCount: stats["memory_count"],
			EdgeCount:   stats["edge_count"],
			Path:        s.store.Path(),
		},
		Network: netStats,
		Shards: ShardStats{
			LocalShards: localShards,
		},
	})
}

func (s *Server) HandlePeers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	if s.networkHost == nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"peers": []interface{}{},
			"count": 0,
		})
		return
	}

	peers := s.networkHost.ConnectedPeers()
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"peers": peers,
		"count": len(peers),
	})
}

func (s *Server) searchMemories(query string, limit int, decryptFn func([]byte) ([]byte, error)) ([]*memory.Memory, error) {
	allMemories, err := s.store.GetRecentMemories(1000)
	if err != nil {
		return nil, err
	}

	queryLower := strings.ToLower(query)
	var results []*memory.Memory

	for _, mem := range allMemories {
		if len(results) >= limit {
			break
		}

		plain, err := mem.Decrypt(decryptFn)
		if err != nil {
			continue
		}

		contentLower := strings.ToLower(plain.Content)
		if strings.Contains(contentLower, queryLower) {
			results = append(results, mem)
		}
	}

	return results, nil
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// jsonContentType middleware sets Content-Type header
func jsonContentType(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		next.ServeHTTP(w, r)
	})
}

// requestLogger middleware logs requests
func requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		fmt.Printf("[API] %s %s %s\n", r.Method, r.URL.Path, time.Since(start))
	})
}
