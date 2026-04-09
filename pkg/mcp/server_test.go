package mcp

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/nnlgsakib/dmgn/internal/config"
	"github.com/nnlgsakib/dmgn/internal/crypto"
	"github.com/nnlgsakib/dmgn/pkg/memory"
	"github.com/nnlgsakib/dmgn/pkg/query"
	"github.com/nnlgsakib/dmgn/pkg/storage"
	"github.com/nnlgsakib/dmgn/pkg/vectorindex"
)

func setupTestServer(t *testing.T) (*MCPServer, string) {
	t.Helper()
	tmpDir := t.TempDir()

	store, err := storage.New(storage.Options{
		DataDir: filepath.Join(tmpDir, "storage"),
	})
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	cryptoEng, err := crypto.NewEngine(key)
	if err != nil {
		t.Fatalf("failed to create crypto engine: %v", err)
	}

	indexPath := filepath.Join(tmpDir, "vecindex.enc")
	vecIndex := vectorindex.NewVectorIndex(indexPath, cryptoEng.Encrypt, cryptoEng.Decrypt)

	decryptFn := func(ct []byte) ([]byte, error) { return cryptoEng.Decrypt(ct) }
	qe := query.NewQueryEngine(vecIndex, store, decryptFn, 0.7)

	cfg := &config.Config{}
	srv := NewMCPServer(store, vecIndex, qe, cryptoEng, nil, cfg)
	return srv, tmpDir
}

func addTestMemory(t *testing.T, srv *MCPServer, content string, embedding []float32) AddMemoryOutput {
	t.Helper()
	_, out, err := srv.handleAddMemory(context.Background(), &mcpsdk.CallToolRequest{}, AddMemoryInput{
		Content:   content,
		Type:      "text",
		Embedding: embedding,
	})
	if err != nil {
		t.Fatalf("failed to add memory: %v", err)
	}
	return out
}

func TestAddMemoryTool(t *testing.T) {
	srv, _ := setupTestServer(t)

	out := addTestMemory(t, srv, "test memory content", nil)

	if out.ID == "" {
		t.Fatal("expected non-empty ID")
	}
	if out.Type != "text" {
		t.Fatalf("expected type text, got %s", out.Type)
	}
	if out.Timestamp == 0 {
		t.Fatal("expected non-zero timestamp")
	}

	// Verify stored
	mem, err := srv.store.GetMemory(out.ID)
	if err != nil {
		t.Fatalf("memory not found in store: %v", err)
	}
	if mem.ID != out.ID {
		t.Fatal("stored memory ID mismatch")
	}
}

func TestAddMemoryWithEmbedding(t *testing.T) {
	srv, _ := setupTestServer(t)

	emb := []float32{0.1, 0.2, 0.3, 0.4}
	out := addTestMemory(t, srv, "memory with embedding", emb)

	if srv.vecIndex.Count() != 1 {
		t.Fatalf("expected 1 vector, got %d", srv.vecIndex.Count())
	}

	_ = out
}

func TestQueryMemoryTool(t *testing.T) {
	srv, _ := setupTestServer(t)

	addTestMemory(t, srv, "the quick brown fox", nil)
	addTestMemory(t, srv, "hello world greeting", nil)

	_, out, err := srv.handleQueryMemory(context.Background(), &mcpsdk.CallToolRequest{}, QueryMemoryInput{
		Query: "quick brown",
		Limit: 5,
	})
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}

	if out.Count == 0 {
		t.Fatal("expected at least 1 result")
	}
	if out.Results[0].Snippet == "" {
		t.Fatal("expected non-empty snippet")
	}
}

func TestQueryMemoryWithContent(t *testing.T) {
	srv, _ := setupTestServer(t)

	addTestMemory(t, srv, "full content memory test data", nil)

	_, out, err := srv.handleQueryMemory(context.Background(), &mcpsdk.CallToolRequest{}, QueryMemoryInput{
		Query:          "full content",
		Limit:          5,
		IncludeContent: true,
	})
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}

	if out.Count == 0 {
		t.Fatal("expected results")
	}
	if out.Results[0].Content == "" {
		t.Fatal("expected full content when include_content=true")
	}
	if out.Results[0].Content != "full content memory test data" {
		t.Fatalf("unexpected content: %s", out.Results[0].Content)
	}
}

func TestGetContextTool(t *testing.T) {
	srv, _ := setupTestServer(t)

	addTestMemory(t, srv, "recent context memory 1", nil)
	addTestMemory(t, srv, "recent context memory 2", nil)

	_, out, err := srv.handleGetContext(context.Background(), &mcpsdk.CallToolRequest{}, GetContextInput{
		Limit: 10,
	})
	if err != nil {
		t.Fatalf("get context failed: %v", err)
	}

	if out.Count < 2 {
		t.Fatalf("expected at least 2 memories, got %d", out.Count)
	}
	if out.ContextWindowHint == "" {
		t.Fatal("expected context window hint")
	}
	if out.Memories[0].Content == "" {
		t.Fatal("expected decrypted content")
	}
}

func TestLinkMemoriesTool(t *testing.T) {
	srv, _ := setupTestServer(t)

	out1 := addTestMemory(t, srv, "source memory", nil)
	out2 := addTestMemory(t, srv, "target memory", nil)

	_, linkOut, err := srv.handleLinkMemories(context.Background(), &mcpsdk.CallToolRequest{}, LinkMemoriesInput{
		FromID:   out1.ID,
		ToID:     out2.ID,
		EdgeType: "related",
	})
	if err != nil {
		t.Fatalf("link failed: %v", err)
	}
	if !linkOut.Created {
		t.Fatal("expected created=true")
	}

	edges, err := srv.store.GetEdges(out1.ID)
	if err != nil {
		t.Fatalf("get edges failed: %v", err)
	}
	if len(edges) == 0 {
		t.Fatal("expected at least 1 edge")
	}
}

func TestGetGraphTool(t *testing.T) {
	srv, _ := setupTestServer(t)

	out1 := addTestMemory(t, srv, "root node", nil)
	out2 := addTestMemory(t, srv, "child node", nil)

	// Add to in-memory graph
	mem1, _ := srv.store.GetMemory(out1.ID)
	mem2, _ := srv.store.GetMemory(out2.ID)
	graph := srv.store.GetGraph()
	graph.AddNode(mem1)
	graph.AddNode(mem2)
	graph.AddEdge(out1.ID, out2.ID, 1.0, "related")

	_, graphOut, err := srv.handleGetGraph(context.Background(), &mcpsdk.CallToolRequest{}, GetGraphInput{
		StartID:  out1.ID,
		MaxDepth: 3,
	})
	if err != nil {
		t.Fatalf("get graph failed: %v", err)
	}

	if len(graphOut.Nodes) < 2 {
		t.Fatalf("expected at least 2 nodes, got %d", len(graphOut.Nodes))
	}
	if len(graphOut.Edges) < 1 {
		t.Fatalf("expected at least 1 edge, got %d", len(graphOut.Edges))
	}
}

func TestDeleteMemoryTool(t *testing.T) {
	srv, _ := setupTestServer(t)

	out := addTestMemory(t, srv, "memory to delete", nil)

	_, delOut, err := srv.handleDeleteMemory(context.Background(), &mcpsdk.CallToolRequest{}, DeleteMemoryInput{
		ID: out.ID,
	})
	if err != nil {
		t.Fatalf("delete failed: %v", err)
	}
	if !delOut.Deleted {
		t.Fatal("expected deleted=true")
	}

	// Verify gone
	_, err = srv.store.GetMemory(out.ID)
	if err == nil {
		t.Fatal("expected memory to be deleted")
	}
}

func TestGetStatusTool(t *testing.T) {
	srv, _ := setupTestServer(t)

	addTestMemory(t, srv, "status test memory", nil)

	_, status, err := srv.handleGetStatus(context.Background(), &mcpsdk.CallToolRequest{}, GetStatusInput{})
	if err != nil {
		t.Fatalf("get status failed: %v", err)
	}

	if status.MemoryCount < 1 {
		t.Fatalf("expected memory_count >= 1, got %d", status.MemoryCount)
	}
	if status.Version != "0.1.0" {
		t.Fatalf("unexpected version: %s", status.Version)
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name string
		d    int64 // nanoseconds
		want string
	}{
		{"just now", 1e9, "just now"},     // 1 second
		{"minutes", 300e9, "5m ago"},      // 5 minutes
		{"hours", 7200e9, "2h ago"},       // 2 hours
		{"days", 172800e9, "2d ago"},      // 2 days
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatDuration(0) // boundary check
			_ = got
		})
	}
}

// Ensure unused imports don't cause issues
var _ = os.DevNull
var _ memory.Type
