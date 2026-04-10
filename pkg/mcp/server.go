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
	"github.com/nnlgsakib/dmgn/pkg/graph"
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
	kgBroadcaster   func(nodeID, nodeType, label string, meta map[string]string)
	kgGraph         *graph.Graph
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
	mcpServer := &MCPServer{
		store:       store,
		vecIndex:    vecIndex,
		queryEngine: queryEngine,
		cryptoEng:   cryptoEng,
		identity:    id,
		config:      cfg,
		logger:      slog.New(slog.NewJSONHandler(os.Stderr, nil)),
	}

	if cfg.EnableKnowledgeGraph && store != nil {
		mcpServer.kgGraph = graph.NewGraph(store.DB())
	}

	return mcpServer
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

// SetKGBroadcaster sets the callback invoked after a knowledge graph node is created,
// to broadcast it to the gossip network.
func (s *MCPServer) SetKGBroadcaster(fn func(nodeID, nodeType, label string, meta map[string]string)) {
	s.kgBroadcaster = fn
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

	mcp.AddTool(server, &mcp.Tool{
		Name:        "add_node",
		Description: "Add a node to the knowledge graph. Node can be any entity: person, concept, memory, file, function, etc.",
	}, s.handleAddNode)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "add_edge",
		Description: "Add an edge between two knowledge graph nodes with typed relationship (CREATES, USES, BUILT_BY, RELATED_TO, etc).",
	}, s.handleAddEdgeKG)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "query_graph",
		Description: "Query the knowledge graph for incoming/outgoing edges from a node.",
	}, s.handleQueryGraph)

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

// autoAddToKG extracts entities from memory content and adds them to knowledge graph
func (s *MCPServer) autoAddToKG(ctx context.Context, mem *memory.Memory) {
	if s.kgGraph == nil || mem == nil {
		return
	}

	plain, err := mem.Decrypt(s.cryptoEng.Decrypt)
	if err != nil {
		return
	}

	nodes, err := graph.ExtractEntities(plain.Content, graph.SourceTypeMemory)
	if err != nil || len(nodes) == 0 {
		return
	}

	for _, node := range nodes {
		node.ID = fmt.Sprintf("mem:%s:%s", mem.ID[:8], node.Label)
		if err := s.kgGraph.AddNode(&node); err != nil {
			s.logger.Debug("autoAddToKG: failed to add node", "err", err)
			continue
		}

		if s.kgBroadcaster != nil {
			metaStr := make(map[string]string)
			for k, v := range node.Meta {
				metaStr[k] = fmt.Sprintf("%v", v)
			}
			s.kgBroadcaster(node.ID, node.Type, node.Label, metaStr)
		}
	}

	edge := &graph.Edge{
		From:   mem.ID,
		To:     nodes[0].ID,
		Type:   "CONTAINS",
		Weight: 1.0,
	}
	if err := s.kgGraph.AddEdge(edge); err == nil && s.kgBroadcaster != nil {
		s.kgBroadcaster(edge.ID, "memory", "contains", map[string]string{"from": mem.ID})
	}

	s.logger.Info("autoAddToKG: entities extracted", "count", len(nodes), "memory", mem.ID)
}

// autoLinkNewMemory automatically creates edges from a new memory to similar
// and time-proximate memories based on config thresholds.
func (s *MCPServer) autoLinkNewMemory(ctx context.Context, mem *memory.Memory) {
	cfg := s.config
	if !cfg.EnableAutoLink {
		s.logger.Debug("auto-linking disabled in config")
		return
	}
	if s.vecIndex == nil || s.store == nil {
		s.logger.Debug("auto-linking skipped: vecIndex or store nil")
		return
	}
	if len(mem.Embedding) == 0 {
		s.logger.Debug("auto-linking skipped: no embedding in memory", "memID", mem.ID)
		return
	}

	s.logger.Info("auto-linking starting", "memID", mem.ID, "embeddingDim", len(mem.Embedding))

	similarResults := s.vecIndex.Search(mem.Embedding, cfg.MaxAutoLinksPerMemory)
	s.logger.Info("auto-linking search complete", "results", len(similarResults))

	linked := make(map[string]bool)
	similarCount := 0

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
			similarCount++

			if s.edgeBroadcaster != nil {
				s.logger.Info("auto-linking broadcasting edge", "from", mem.ID, "to", result.MemoryID, "weight", weight, "type", "auto")
				s.edgeBroadcaster(mem.ID, result.MemoryID, weight, "auto")
			}
		} else {
			s.logger.Error("auto-linking failed to add edge", "err", err)
		}
	}

	recent, err := s.store.GetRecentMemories(100)
	if err != nil {
		s.logger.Error("auto-linking failed to get recent memories", "err", err)
		return
	}
	timeWindowNanos := int64(cfg.AutoLinkTimeWindowMinutes) * 60 * 1e9
	timeCount := 0

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
				timeCount++

				if s.edgeBroadcaster != nil {
					s.logger.Info("auto-linking broadcasting time edge", "from", mem.ID, "to", recentMem.ID, "weight", weight, "type", "auto")
					s.edgeBroadcaster(mem.ID, recentMem.ID, weight, "auto")
				}
			}
		}
	}

	s.logger.Info("auto-linking complete", "memID", mem.ID, "similarEdges", similarCount, "timeEdges", timeCount)
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

	// Auto-extract entities and add to knowledge graph
	if s.config.EnableKnowledgeGraph && s.kgGraph != nil {
		s.autoAddToKG(ctx, mem)
	}

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

// --- knowledge graph handlers ---

type AddNodeInput struct {
	ID    string         `json:"id"`
	Type  string         `json:"type"`
	Label string         `json:"label"`
	Meta  map[string]any `json:"meta,omitempty"`
}

type AddNodeOutput struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Label   string `json:"label"`
	Created bool   `json:"created"`
}

type AddEdgeKGInput struct {
	From   string  `json:"from"`
	To     string  `json:"to"`
	Type   string  `json:"type"`
	Weight float32 `json:"weight"`
}

type AddEdgeKGOutput struct {
	From    string `json:"from"`
	To      string `json:"to"`
	Type    string `json:"type"`
	Created bool   `json:"created"`
}

type QueryGraphInput struct {
	NodeID    string `json:"node_id"`
	Direction string `json:"direction"`
	EdgeType  string `json:"edge_type"`
	MaxDepth  int    `json:"max_depth"`
}

type QueryGraphOutput struct {
	Node     string     `json:"node"`
	Incoming []EdgeInfo `json:"incoming"`
	Outgoing []EdgeInfo `json:"outgoing"`
}

type EdgeInfo struct {
	From   string  `json:"from"`
	To     string  `json:"to"`
	Type   string  `json:"type"`
	Weight float32 `json:"weight"`
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

func (s *MCPServer) handleAddNode(ctx context.Context, req *mcp.CallToolRequest, input AddNodeInput) (*mcp.CallToolResult, AddNodeOutput, error) {
	if s.kgGraph == nil {
		return nil, AddNodeOutput{}, fmt.Errorf("knowledge graph not initialized")
	}

	metaAny := make(map[string]any)
	if input.Meta != nil {
		for k, v := range input.Meta {
			metaAny[k] = v
		}
	}

	node := &graph.Node{
		ID:    input.ID,
		Type:  input.Type,
		Label: input.Label,
		Meta:  metaAny,
	}

	if err := s.kgGraph.AddNode(node); err != nil {
		return nil, AddNodeOutput{}, fmt.Errorf("failed to add node: %w", err)
	}

	if s.kgBroadcaster != nil {
		metaStr := make(map[string]string)
		for k, v := range metaAny {
			metaStr[k] = fmt.Sprintf("%v", v)
		}
		s.kgBroadcaster(node.ID, node.Type, node.Label, metaStr)
	}

	return nil, AddNodeOutput{
		ID:      node.ID,
		Type:    node.Type,
		Label:   node.Label,
		Created: true,
	}, nil
}

func (s *MCPServer) handleAddEdgeKG(ctx context.Context, req *mcp.CallToolRequest, input AddEdgeKGInput) (*mcp.CallToolResult, AddEdgeKGOutput, error) {
	if s.kgGraph == nil {
		return nil, AddEdgeKGOutput{}, fmt.Errorf("knowledge graph not initialized")
	}

	if input.Weight == 0 {
		input.Weight = 1.0
	}

	edge := &graph.Edge{
		From:   input.From,
		To:     input.To,
		Type:   input.Type,
		Weight: input.Weight,
	}

	if err := s.kgGraph.AddEdge(edge); err != nil {
		return nil, AddEdgeKGOutput{}, fmt.Errorf("failed to add edge: %w", err)
	}

	return nil, AddEdgeKGOutput{
		From:    edge.From,
		To:      edge.To,
		Type:    edge.Type,
		Created: true,
	}, nil
}

func (s *MCPServer) handleQueryGraph(ctx context.Context, req *mcp.CallToolRequest, input QueryGraphInput) (*mcp.CallToolResult, QueryGraphOutput, error) {
	if s.kgGraph == nil {
		return nil, QueryGraphOutput{}, fmt.Errorf("knowledge graph not initialized")
	}

	var incoming, outgoing []graph.Edge

	if input.Direction == "incoming" || input.Direction == "" {
		incoming, _ = s.kgGraph.GetIncoming(input.NodeID, input.EdgeType)
	}
	if input.Direction == "outgoing" || input.Direction == "" {
		outgoing, _ = s.kgGraph.GetOutgoing(input.NodeID, input.EdgeType)
	}

	inInfo := make([]EdgeInfo, len(incoming))
	for i, e := range incoming {
		inInfo[i] = EdgeInfo{From: e.From, To: e.To, Type: e.Type, Weight: e.Weight}
	}

	outInfo := make([]EdgeInfo, len(outgoing))
	for i, e := range outgoing {
		outInfo[i] = EdgeInfo{From: e.From, To: e.To, Type: e.Type, Weight: e.Weight}
	}

	return nil, QueryGraphOutput{
		Node:     input.NodeID,
		Incoming: inInfo,
		Outgoing: outInfo,
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
