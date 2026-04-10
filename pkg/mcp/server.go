package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/nnlgsakib/dmgn/internal/config"
	"github.com/nnlgsakib/dmgn/internal/crypto"
	"github.com/nnlgsakib/dmgn/pkg/identity"
	"github.com/nnlgsakib/dmgn/pkg/memory"
	"github.com/nnlgsakib/dmgn/pkg/query"
	"github.com/nnlgsakib/dmgn/pkg/skill"
	"github.com/nnlgsakib/dmgn/pkg/storage"
	"github.com/nnlgsakib/dmgn/pkg/vectorindex"
)

// MCPServer wraps a Model Context Protocol server for AI agent integration.
type MCPServer struct {
	store           *storage.Store
	vecIndex        *vectorindex.VectorIndex
	queryEngine     *query.QueryEngine
	cryptoEng       *crypto.Engine
	identity        *identity.Identity
	config          *config.Config
	logger          *slog.Logger
	onBroadcast     func(mem *memory.Memory)
	edgeBroadcaster func(fromID, toID string, weight float32, edgeType string)
}

// NewMCPServer creates a new MCP server with all required dependencies.
func NewMCPServer(
	store *storage.Store,
	vecIndex *vectorindex.VectorIndex,
	queryEngine *query.QueryEngine,
	cryptoEng *crypto.Engine,
	id *identity.Identity,
	cfg *config.Config,
) *MCPServer {
	return &MCPServer{
		store:       store,
		vecIndex:    vecIndex,
		queryEngine: queryEngine,
		cryptoEng:   cryptoEng,
		identity:    id,
		config:      cfg,
		logger:      slog.New(slog.NewJSONHandler(os.Stderr, nil)),
	}
}

// SetLogger overrides the default logger.
func (s *MCPServer) SetLogger(l *slog.Logger) {
	s.logger = l
}

// SetBroadcaster sets the callback invoked after a memory is saved,
// to broadcast it to the gossip network and track its sequence.
func (s *MCPServer) SetBroadcaster(fn func(mem *memory.Memory)) {
	s.onBroadcast = fn
}

// SetEdgeBroadcaster sets the callback invoked after an edge is created,
// to broadcast it to the gossip network for distributed graph sync.
func (s *MCPServer) SetEdgeBroadcaster(fn func(fromID, toID string, weight float32, edgeType string)) {
	s.edgeBroadcaster = fn
}

// newServer creates a configured MCP server instance with all tools registered.
func (s *MCPServer) newServer() *mcp.Server {
	server := mcp.NewServer(
		&mcp.Implementation{Name: "dmgn", Version: "0.1.0"},
		nil,
	)

	// Register all 7 tools
	mcp.AddTool(server, &mcp.Tool{
		Name:        "add_memory",
		Description: "Store a new memory with content, type, links, embedding, and metadata",
	}, s.handleAddMemory)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "query_memory",
		Description: "Search memories by text query and/or embedding vector. Returns snippets by default; set include_content=true for full content.",
	}, s.handleQueryMemory)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_context",
		Description: "Get recent memories formatted as context for AI agents",
	}, s.handleGetContext)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "link_memories",
		Description: "Create a directed edge between two memories",
	}, s.handleLinkMemories)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_graph",
		Description: "Traverse the memory graph from a starting memory ID",
	}, s.handleGetGraph)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "delete_memory",
		Description: "Delete a memory by ID",
	}, s.handleDeleteMemory)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_status",
		Description: "Get node status including memory count, vector index size, and config summary",
	}, s.handleGetStatus)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "load_skill",
		Description: "Load DMGN skill content for agent context. Call this when user mentions DMGN or wants to initialize DMGN capabilities.",
	}, s.handleLoadSkill)

	return server
}

// Run starts the MCP server on stdio and blocks until the context is canceled.
func (s *MCPServer) Run(ctx context.Context) error {
	server := s.newServer()
	s.logger.Info("starting MCP server on stdio", "tools", 8)
	return server.Run(ctx, &mcp.StdioTransport{})
}

// RunOnConnection starts the MCP server over an arbitrary io.ReadWriteCloser.
// Used by the daemon to serve MCP over TCP IPC connections.
func (s *MCPServer) RunOnConnection(ctx context.Context, conn io.ReadWriteCloser) error {
	server := s.newServer()
	transport := &mcp.IOTransport{
		Reader: conn,
		Writer: conn,
	}
	return server.Run(ctx, transport)
}

// --- Tool Input/Output Structs ---

type AddMemoryInput struct {
	Content   string            `json:"content"`
	Type      string            `json:"type,omitempty"`
	Links     []string          `json:"links,omitempty"`
	Embedding []float32         `json:"embedding,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

type AddMemoryOutput struct {
	ID        string `json:"id"`
	Timestamp int64  `json:"timestamp"`
	Type      string `json:"type"`
}

type QueryMemoryInput struct {
	Query          string    `json:"query,omitempty"`
	Embedding      []float32 `json:"embedding,omitempty"`
	Limit          int       `json:"limit,omitempty"`
	IncludeContent bool      `json:"include_content,omitempty"`
	FilterType     string    `json:"filter_type,omitempty"`
	FilterAfter    int64     `json:"filter_after,omitempty"`
	FilterBefore   int64     `json:"filter_before,omitempty"`
}

type QueryMemoryOutput struct {
	Results []QueryResultItem `json:"results"`
	Count   int               `json:"count"`
}

type QueryResultItem struct {
	MemoryID  string  `json:"memory_id"`
	Score     float64 `json:"score"`
	Type      string  `json:"type"`
	Timestamp int64   `json:"timestamp"`
	Content   string  `json:"content,omitempty"`
	Snippet   string  `json:"snippet,omitempty"`
}

type GetContextInput struct {
	Limit int `json:"limit,omitempty"`
}

type GetContextOutput struct {
	Memories          []ContextMemory `json:"memories"`
	Count             int             `json:"count"`
	ContextWindowHint string          `json:"context_window_hint"`
}

type ContextMemory struct {
	ID        string            `json:"id"`
	Content   string            `json:"content"`
	Type      string            `json:"type"`
	Timestamp int64             `json:"timestamp"`
	TimeAgo   string            `json:"time_ago"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

type LinkMemoriesInput struct {
	FromID   string  `json:"from_id"`
	ToID     string  `json:"to_id"`
	Weight   float32 `json:"weight,omitempty"`
	EdgeType string  `json:"edge_type,omitempty"`
}

type LinkMemoriesOutput struct {
	FromID   string `json:"from_id"`
	ToID     string `json:"to_id"`
	EdgeType string `json:"edge_type"`
	Created  bool   `json:"created"`
}

type GetGraphInput struct {
	StartID  string `json:"start_id"`
	MaxDepth int    `json:"max_depth,omitempty"`
}

type GetGraphOutput struct {
	RootID string      `json:"root_id"`
	Nodes  []GraphNode `json:"nodes"`
	Edges  []GraphEdge `json:"edges"`
	Depth  int         `json:"depth"`
}

type GraphNode struct {
	ID        string `json:"id"`
	Type      string `json:"type"`
	Timestamp int64  `json:"timestamp"`
	LinkCount int    `json:"link_count"`
}

type GraphEdge struct {
	From   string  `json:"from"`
	To     string  `json:"to"`
	Type   string  `json:"type"`
	Weight float32 `json:"weight"`
}

type DeleteMemoryInput struct {
	ID string `json:"id"`
}

type DeleteMemoryOutput struct {
	ID      string `json:"id"`
	Deleted bool   `json:"deleted"`
}

type GetStatusInput struct{}

type GetStatusOutput struct {
	NodeID          string `json:"node_id"`
	Version         string `json:"version"`
	MemoryCount     int64  `json:"memory_count"`
	EdgeCount       int64  `json:"edge_count"`
	VectorIndexSize int    `json:"vector_index_size"`
	StoragePath     string `json:"storage_path"`
}

// --- Tool Handlers ---

// autoLinkNewMemory automatically creates edges from a new memory to similar
// and time-proximate memories based on config thresholds.
func (s *MCPServer) autoLinkNewMemory(ctx context.Context, mem *memory.Memory) {
	cfg := s.config
	if !cfg.EnableAutoLink {
		return
	}
	if s.vecIndex == nil || s.store == nil {
		return
	}
	if len(mem.Embedding) == 0 {
		return
	}

	similarResults := s.vecIndex.Search(mem.Embedding, cfg.MaxAutoLinksPerMemory)

	linked := make(map[string]bool)

	for _, result := range similarResults {
		if result.MemoryID == mem.ID {
			continue
		}
		if result.Score < cfg.AutoLinkSimilarityThreshold {
			continue
		}
		if linked[result.MemoryID] {
			continue
		}

		weight := float32(result.Score)
		if err := s.store.AddEdge(mem.ID, result.MemoryID, weight, "auto"); err == nil {
			linked[result.MemoryID] = true
			graph := s.store.GetGraph()
			_ = graph.AddEdge(mem.ID, result.MemoryID, weight, "auto")

			if s.edgeBroadcaster != nil {
				s.edgeBroadcaster(mem.ID, result.MemoryID, weight, "auto")
			}
		}
	}

	recent, err := s.store.GetRecentMemories(100)
	if err != nil {
		return
	}
	timeWindowNanos := int64(cfg.AutoLinkTimeWindowMinutes) * 60 * 1e9

	for _, recentMem := range recent {
		if recentMem.ID == mem.ID {
			continue
		}
		if linked[recentMem.ID] {
			continue
		}

		timeDiff := mem.Timestamp - recentMem.Timestamp
		if timeDiff < 0 {
			timeDiff = -timeDiff
		}
		if timeDiff <= timeWindowNanos {
			weight := float32(0.5)
			if err := s.store.AddEdge(mem.ID, recentMem.ID, weight, "auto"); err == nil {
				linked[recentMem.ID] = true
				graph := s.store.GetGraph()
				_ = graph.AddEdge(mem.ID, recentMem.ID, weight, "auto")

				if s.edgeBroadcaster != nil {
					s.edgeBroadcaster(mem.ID, recentMem.ID, weight, "auto")
				}
			}
		}
	}
}

func (s *MCPServer) handleAddMemory(ctx context.Context, req *mcp.CallToolRequest, input AddMemoryInput) (*mcp.CallToolResult, AddMemoryOutput, error) {
	memType := memory.TypeText
	if input.Type != "" {
		memType = memory.Type(input.Type)
	}

	plain := &memory.PlaintextMemory{
		Content:  input.Content,
		Type:     memType,
		Metadata: input.Metadata,
	}

	mem, err := memory.Create(plain, input.Links, s.cryptoEng.Encrypt)
	if err != nil {
		return nil, AddMemoryOutput{}, fmt.Errorf("failed to create memory: %w", err)
	}

	if len(input.Embedding) > 0 {
		mem.Embedding = input.Embedding
	}

	if err := s.store.SaveMemory(mem); err != nil {
		return nil, AddMemoryOutput{}, fmt.Errorf("failed to save memory: %w", err)
	}

	if len(mem.Embedding) > 0 && s.vecIndex != nil {
		s.vecIndex.Add(mem.ID, mem.Embedding)
	}

	// Auto-link to similar/time-proximate memories
	s.autoLinkNewMemory(ctx, mem)

	// Broadcast to gossip network
	if s.onBroadcast != nil {
		s.onBroadcast(mem)
	}

	s.logger.Info("memory added via MCP", "id", mem.ID, "type", mem.Type)
	return nil, AddMemoryOutput{
		ID:        mem.ID,
		Timestamp: mem.Timestamp,
		Type:      string(mem.Type),
	}, nil
}

func (s *MCPServer) handleQueryMemory(ctx context.Context, req *mcp.CallToolRequest, input QueryMemoryInput) (*mcp.CallToolResult, QueryMemoryOutput, error) {
	limit := input.Limit
	if limit <= 0 {
		limit = 10
	}

	qr := s.queryEngine.BuildRequest(input.Query, input.Embedding, limit)
	qr.Filters = query.QueryFilters{
		Type:   input.FilterType,
		After:  input.FilterAfter,
		Before: input.FilterBefore,
	}

	results, err := s.queryEngine.SearchLocal(qr)
	if err != nil {
		return nil, QueryMemoryOutput{}, fmt.Errorf("query failed: %w", err)
	}

	items := make([]QueryResultItem, 0, len(results))
	for _, r := range results {
		item := QueryResultItem{
			MemoryID:  r.MemoryID,
			Score:     r.Score,
			Type:      r.Type,
			Timestamp: r.Timestamp,
			Snippet:   r.Snippet,
		}

		if input.IncludeContent {
			mem, err := s.store.GetMemory(r.MemoryID)
			if err == nil {
				plain, err := mem.Decrypt(s.cryptoEng.Decrypt)
				if err == nil {
					item.Content = plain.Content
				}
			}
		}

		items = append(items, item)
	}

	return nil, QueryMemoryOutput{
		Results: items,
		Count:   len(items),
	}, nil
}

func (s *MCPServer) handleGetContext(ctx context.Context, req *mcp.CallToolRequest, input GetContextInput) (*mcp.CallToolResult, GetContextOutput, error) {
	limit := input.Limit
	if limit <= 0 {
		limit = 10
	}

	recent, err := s.store.GetRecentMemories(limit)
	if err != nil {
		return nil, GetContextOutput{}, fmt.Errorf("failed to get recent memories: %w", err)
	}

	memories := make([]ContextMemory, 0, len(recent))
	now := time.Now().UnixNano()
	for _, mem := range recent {
		plain, err := mem.Decrypt(s.cryptoEng.Decrypt)
		if err != nil {
			continue
		}
		ago := time.Duration(now - mem.Timestamp)
		memories = append(memories, ContextMemory{
			ID:        mem.ID,
			Content:   plain.Content,
			Type:      string(mem.Type),
			Timestamp: mem.Timestamp,
			TimeAgo:   formatDuration(ago),
			Metadata:  mem.Metadata,
		})
	}

	return nil, GetContextOutput{
		Memories:          memories,
		Count:             len(memories),
		ContextWindowHint: "Recent memories from DMGN",
	}, nil
}

func (s *MCPServer) handleLinkMemories(ctx context.Context, req *mcp.CallToolRequest, input LinkMemoriesInput) (*mcp.CallToolResult, LinkMemoriesOutput, error) {
	if input.Weight == 0 {
		input.Weight = 1.0
	}
	if input.EdgeType == "" {
		input.EdgeType = "related"
	}

	if err := s.store.AddEdge(input.FromID, input.ToID, input.Weight, input.EdgeType); err != nil {
		return nil, LinkMemoriesOutput{}, fmt.Errorf("failed to link memories: %w", err)
	}

	// Also add to in-memory graph
	graph := s.store.GetGraph()
	_ = graph.AddEdge(input.FromID, input.ToID, input.Weight, input.EdgeType)

	// Broadcast edge to network
	if s.edgeBroadcaster != nil {
		s.edgeBroadcaster(input.FromID, input.ToID, input.Weight, input.EdgeType)
	}

	return nil, LinkMemoriesOutput{
		FromID:   input.FromID,
		ToID:     input.ToID,
		EdgeType: input.EdgeType,
		Created:  true,
	}, nil
}

func (s *MCPServer) handleGetGraph(ctx context.Context, req *mcp.CallToolRequest, input GetGraphInput) (*mcp.CallToolResult, GetGraphOutput, error) {
	maxDepth := input.MaxDepth
	if maxDepth <= 0 {
		maxDepth = 3
	}

	graph := s.store.GetGraph()
	nodes := graph.TraverseFrom(input.StartID, maxDepth)

	graphNodes := make([]GraphNode, 0, len(nodes))
	graphEdges := make([]GraphEdge, 0)

	for _, node := range nodes {
		graphNodes = append(graphNodes, GraphNode{
			ID:        node.Memory.ID,
			Type:      string(node.Memory.Type),
			Timestamp: node.Memory.Timestamp,
			LinkCount: len(node.Children) + len(node.Parents),
		})
		for _, edge := range node.Children {
			graphEdges = append(graphEdges, GraphEdge{
				From:   edge.From,
				To:     edge.To,
				Type:   edge.Type,
				Weight: edge.Weight,
			})
		}
	}

	return nil, GetGraphOutput{
		RootID: input.StartID,
		Nodes:  graphNodes,
		Edges:  graphEdges,
		Depth:  maxDepth,
	}, nil
}

func (s *MCPServer) handleDeleteMemory(ctx context.Context, req *mcp.CallToolRequest, input DeleteMemoryInput) (*mcp.CallToolResult, DeleteMemoryOutput, error) {
	if s.vecIndex != nil {
		s.vecIndex.Remove(input.ID)
	}

	if err := s.store.DeleteMemory(input.ID); err != nil {
		return nil, DeleteMemoryOutput{}, fmt.Errorf("failed to delete memory: %w", err)
	}

	s.logger.Info("memory deleted via MCP", "id", input.ID)
	return nil, DeleteMemoryOutput{
		ID:      input.ID,
		Deleted: true,
	}, nil
}

func (s *MCPServer) handleGetStatus(ctx context.Context, req *mcp.CallToolRequest, input GetStatusInput) (*mcp.CallToolResult, GetStatusOutput, error) {
	stats, err := s.store.GetStats()
	if err != nil {
		return nil, GetStatusOutput{}, fmt.Errorf("failed to get stats: %w", err)
	}

	vecSize := 0
	if s.vecIndex != nil {
		vecSize = s.vecIndex.Count()
	}

	nodeID := ""
	if s.identity != nil {
		nodeID = s.identity.ID
	}

	return nil, GetStatusOutput{
		NodeID:          nodeID,
		Version:         "0.1.0",
		MemoryCount:     stats["memory_count"],
		EdgeCount:       stats["edge_count"],
		VectorIndexSize: vecSize,
		StoragePath:     s.store.DataDir(),
	}, nil
}

// --- load_skill handler ---

type LoadSkillInput struct{}

type LoadSkillOutput struct {
	Prompt string `json:"prompt"`
	Source string `json:"source"`
}

func (s *MCPServer) handleLoadSkill(ctx context.Context, req *mcp.CallToolRequest, input LoadSkillInput) (*mcp.CallToolResult, LoadSkillOutput, error) {
	content, err := skill.Load()
	if err != nil {
		return nil, LoadSkillOutput{}, fmt.Errorf("failed to load skill: %w", err)
	}

	// Determine source for transparency
	source := "embedded"
	for _, p := range skill.SkillSearchPaths {
		if _, err := os.Stat(p); err == nil {
			source = p
			break
		}
	}

	return nil, LoadSkillOutput{
		Prompt: string(content),
		Source: source,
	}, nil
}

// --- Helpers ---

func formatDuration(d time.Duration) string {
	hours := int(d.Hours())
	if hours >= 24 {
		days := hours / 24
		return fmt.Sprintf("%dd ago", days)
	}
	if hours >= 1 {
		return fmt.Sprintf("%dh ago", hours)
	}
	mins := int(d.Minutes())
	if mins >= 1 {
		return fmt.Sprintf("%dm ago", mins)
	}
	return "just now"
}

// MarshalResult marshals a tool output to JSON for tool result text content.
func MarshalResult(v interface{}) string {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Sprintf(`{"error": "%s"}`, err.Error())
	}
	return string(data)
}
