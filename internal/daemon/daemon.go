package daemon

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"gopkg.in/natefinch/lumberjack.v2"
	"google.golang.org/protobuf/proto"

	libnet "github.com/libp2p/go-libp2p/core/network"
	"github.com/multiformats/go-multiaddr"

	"github.com/nnlgsakib/dmgn/internal/api"
	"github.com/nnlgsakib/dmgn/internal/config"
	"github.com/nnlgsakib/dmgn/internal/crypto"
	dmgnmcp "github.com/nnlgsakib/dmgn/pkg/mcp"
	"github.com/nnlgsakib/dmgn/pkg/memory"
	"github.com/nnlgsakib/dmgn/pkg/network"
	"github.com/nnlgsakib/dmgn/pkg/query"
	"github.com/nnlgsakib/dmgn/pkg/sharding"
	"github.com/nnlgsakib/dmgn/pkg/storage"
	pkgsync "github.com/nnlgsakib/dmgn/pkg/sync"
	"github.com/nnlgsakib/dmgn/pkg/vectorindex"
	dmgnpb "github.com/nnlgsakib/dmgn/proto/dmgn/v1"
)

// Daemon manages the full DMGN node lifecycle: libp2p host, API server,
// gossip, delta sync, shard management, query engine, and MCP IPC listener.
type Daemon struct {
	cfg  *config.Config
	keys *DerivedKeys

	host        *network.Host
	store       *storage.Store
	cryptoEng   *crypto.Engine
	indexCrypto *crypto.Engine
	vecIndex    *vectorindex.VectorIndex
	queryEngine *query.QueryEngine
	remoteOrch  *query.RemoteQueryOrchestrator
	apiServer   *api.Server
	gossipMgr   *pkgsync.GossipManager
	deltaMgr    *pkgsync.DeltaSyncManager
	mcpListener net.Listener
	mcpServer   *dmgnmcp.MCPServer
	logger      *slog.Logger

	ctx       context.Context
	cancel    context.CancelFunc
	nodeCtx   context.Context
	nodeStop  context.CancelFunc
	verbose   bool
}

// New creates a new Daemon instance.
func New(cfg *config.Config, keys *DerivedKeys) *Daemon {
	return &Daemon{
		cfg:  cfg,
		keys: keys,
	}
}

// SetVerbose enables verbose logging to stderr in addition to the log file.
func (d *Daemon) SetVerbose(v bool) {
	d.verbose = v
}

// Start initializes and starts all daemon subsystems.
// This mirrors the wiring from the old internal/cli/start.go RunE.
func (d *Daemon) Start(ctx context.Context) error {
	d.ctx, d.cancel = context.WithCancel(ctx)
	d.setupLogger()

	d.logger.Info("daemon starting", "data_dir", d.cfg.DataDir)

	// 1. Create crypto engine from master key
	var err error
	d.cryptoEng, err = crypto.NewEngine(d.keys.MasterKey)
	if err != nil {
		return fmt.Errorf("failed to initialize crypto: %w", err)
	}

	// 2. Open storage
	d.store, err = storage.New(storage.Options{
		DataDir:      d.cfg.StorageDir(),
		MaxRetention: d.cfg.MaxRecentMemories,
	})
	if err != nil {
		return fmt.Errorf("failed to open storage: %w", err)
	}

	// 3. Create connection gater
	gater := network.NewReputationGater(
		nil,
		d.cfg.BlockedPeers,
		d.cfg.AllowedPeers,
		d.cfg.ReputationThreshold,
	)

	// 4. Create and start libp2p host
	hostCfg := network.HostConfig{
		ListenAddrs:        d.cfg.GetListenAddrs(),
		BootstrapPeers:     d.cfg.BootstrapPeers,
		MDNSService:        d.cfg.MDNSService,
		MaxPeersLow:        d.cfg.MaxPeersLow,
		MaxPeersHigh:       d.cfg.MaxPeersHigh,
		PrivateKey:         d.keys.LibP2PKey,
		EnableHolePunching: d.cfg.EnableHolePunching,
		EnableRelayService: d.cfg.EnableRelayService,
		RelayServers:       d.cfg.RelayServers,
		ConnectionGater:    gater,
	}

	d.host, err = network.NewHost(hostCfg)
	if err != nil {
		d.store.Close()
		return fmt.Errorf("failed to create network host: %w", err)
	}

	if err := d.host.Start(); err != nil {
		d.store.Close()
		return fmt.Errorf("failed to start network host: %w", err)
	}

	// Wire rate limiters for protocol handlers
	storeLimiter := network.NewPeerRateLimiter(10, 20)
	fetchLimiter := network.NewPeerRateLimiter(20, 40)
	d.host.SetRateLimiters(storeLimiter, fetchLimiter)

	peerID := d.host.ID().String()
	d.logger.Info("network host started",
		"peer_id", peerID,
		"addrs", d.host.Addrs(),
	)

	// Persist this node's full multiaddresses to config
	d.persistMultiaddrs(peerID)

	// Register peer connect/disconnect event logger
	d.host.RegisterConnectionNotifier(&peerEventNotifier{logger: d.logger})

	// 4. Reconstruct identity for subsystems that need it
	id := d.keys.Identity()

	// 5. Create API server
	d.apiServer, err = api.NewServer(d.cfg, d.store, d.cryptoEng, id)
	if err != nil {
		d.host.Stop()
		d.store.Close()
		return fmt.Errorf("failed to create API server: %w", err)
	}
	d.apiServer.SetNetworkHost(d.host)

	// 6. Register shard protocol handlers
	d.host.RegisterStoreHandler(d.store)
	d.host.RegisterFetchHandler(d.store)

	shardCfg := sharding.ShardConfig{
		Threshold:   d.cfg.ShardThreshold,
		TotalShards: d.cfg.ShardCount,
	}
	var router *network.ShardRouter
	if d.host.DHT() != nil {
		router = network.NewShardRouter(d.host.DHT())
	}
	dist := sharding.NewDistributor(nil, d.store, router, shardCfg)

	if dist != nil {
		auditor := network.NewShardAuditor(dist, 5*time.Minute)
		auditor.Start(d.ctx)
	}

	d.logger.Info("shard config",
		"threshold", shardCfg.Threshold,
		"total", shardCfg.TotalShards,
	)

	// 7. Create vector index
	d.indexCrypto, err = crypto.NewEngine(d.keys.VectorIndexKey)
	if err != nil {
		d.host.Stop()
		d.store.Close()
		return fmt.Errorf("failed to init vector index crypto: %w", err)
	}

	d.vecIndex = vectorindex.NewVectorIndex(
		d.cfg.VectorIndexPath(),
		d.indexCrypto.Encrypt,
		d.indexCrypto.Decrypt,
	)
	if err := d.vecIndex.Load(); err != nil {
		d.logger.Warn("failed to load vector index", "err", err)
	}

	// 8. Create query engine
	decryptFn := func(ciphertext []byte) ([]byte, error) {
		return d.cryptoEng.Decrypt(ciphertext)
	}
	d.queryEngine = query.NewQueryEngine(d.vecIndex, d.store, decryptFn, d.cfg.HybridScoreAlpha)

	d.remoteOrch = query.NewRemoteQueryOrchestrator(
		d.host.LibP2PHost(), d.queryEngine, d.host.ID().String(), d.cfg.QueryTimeoutDuration(),
	)
	query.RegisterQueryHandler(d.host.LibP2PHost(), d.queryEngine, d.host.ID().String())

	// 9. Version vector and sync
	vvStore := pkgsync.NewVClockStore(d.store.DB())
	vv, err := vvStore.Load(d.host.ID().String())
	if err != nil {
		d.logger.Warn("failed to load version vector", "err", err)
		vv = pkgsync.NewVersionVector()
	}

	onMemoryReceived := func(mem *memory.Memory) {
		if err := d.store.SaveMemory(mem); err != nil {
			d.logger.Error("failed to save received memory", "id", mem.ID, "err", err)
			return
		}
		if len(mem.Embedding) > 0 {
			d.vecIndex.Add(mem.ID, mem.Embedding)
		}
		d.logger.Info("memory received and saved from peer", "id", mem.ID, "type", mem.Type)
	}

	// 10. Start gossip manager
	d.nodeCtx, d.nodeStop = context.WithCancel(d.ctx)

	d.gossipMgr, err = pkgsync.NewGossipManager(d.nodeCtx, d.host.LibP2PHost(), d.cfg.GossipTopic,
		func(msg *dmgnpb.GossipMessage) {
			pb := &dmgnpb.Memory{}
			if err := proto.Unmarshal(msg.Memory, pb); err != nil {
				return
			}
			mem := memory.MemoryFromProto(pb)
			onMemoryReceived(mem)
			if msg.Sequence > vv.Get(msg.SenderPeerId) {
				vv.Set(msg.SenderPeerId, msg.Sequence)
				vvStore.SaveSequence(msg.SenderPeerId, msg.Sequence, mem.ID)
				vvStore.Save(d.host.ID().String(), vv)
			}
		})
	if err != nil {
		d.logger.Warn("gossip init failed", "err", err)
	} else {
		d.gossipMgr.Start(d.nodeCtx)
		d.logger.Info("gossip started", "topic", d.cfg.GossipTopic)
	}

	// 11. Start delta sync manager
	d.deltaMgr = pkgsync.NewDeltaSyncManager(
		d.host.LibP2PHost(), vv, vvStore, d.store,
		d.host.ID().String(), d.cfg.SyncIntervalDuration(), onMemoryReceived,
	)
	d.deltaMgr.RegisterHandler()
	d.deltaMgr.Start(d.nodeCtx)
	d.logger.Info("delta sync started", "interval", d.cfg.SyncInterval)

	// 12. Create broadcast function for memory propagation
	localPeerID := d.host.ID().String()
	broadcastMemory := func(mem *memory.Memory) {
		if d.gossipMgr == nil {
			return
		}
		// Increment version vector and track sequence
		seq := vv.Increment(localPeerID)
		vvStore.SaveSequence(localPeerID, seq, mem.ID)
		vvStore.Save(localPeerID, vv)

		// Serialize and publish to gossip
		pb := mem.ToProto()
		data, err := proto.Marshal(pb)
		if err != nil {
			d.logger.Error("failed to marshal memory for gossip", "id", mem.ID, "err", err)
			return
		}
		if err := d.gossipMgr.Publish(d.nodeCtx, data, seq); err != nil {
			d.logger.Error("failed to publish memory to gossip", "id", mem.ID, "err", err)
		} else {
			d.logger.Info("memory broadcast to network", "id", mem.ID, "seq", seq)
		}
	}

	// 13. Wire query engine and gossip into API server
	d.apiServer.SetQueryEngine(d.queryEngine, d.remoteOrch)
	if d.gossipMgr != nil {
		d.apiServer.SetGossipManager(d.gossipMgr)
	}
	d.apiServer.SetVectorIndex(d.vecIndex)
	d.apiServer.SetBroadcaster(broadcastMemory)

	d.logger.Info("query engine configured",
		"alpha", d.cfg.HybridScoreAlpha,
		"timeout", d.cfg.QueryTimeout,
	)

	// 14. Start API server
	go func() {
		if err := d.apiServer.Start(); err != nil && err.Error() != "http: Server closed" {
			d.logger.Error("API server error", "err", err)
		}
	}()

	// 15. Create MCP server
	d.mcpServer = dmgnmcp.NewMCPServer(d.store, d.vecIndex, d.queryEngine, d.cryptoEng, id, d.cfg)
	d.mcpServer.SetLogger(d.logger)
	d.mcpServer.SetBroadcaster(broadcastMemory)

	// 16. Start MCP IPC listener
	listenAddr := fmt.Sprintf("127.0.0.1:%d", d.cfg.MCPIPCPort)
	d.mcpListener, err = net.Listen("tcp", listenAddr)
	if err != nil {
		d.logger.Error("failed to start MCP IPC listener", "err", err)
		// Non-fatal: daemon can run without MCP IPC
	} else {
		port := d.mcpListener.Addr().(*net.TCPAddr).Port
		if err := d.writePortFile(port); err != nil {
			d.logger.Error("failed to write port file", "err", err)
		}
		d.logger.Info("MCP IPC listener started", "port", port)
		go d.acceptMCPConnections()
	}

	d.logger.Info("daemon started successfully")
	return nil
}

// Stop gracefully shuts down all daemon subsystems in reverse order.
func (d *Daemon) Stop() error {
	d.logger.Info("daemon shutting down")

	// Close MCP listener
	if d.mcpListener != nil {
		d.mcpListener.Close()
	}

	// Stop API server
	if d.apiServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := d.apiServer.Stop(ctx); err != nil {
			d.logger.Error("API server shutdown error", "err", err)
		}
	}

	// Stop gossip and delta sync
	if d.nodeStop != nil {
		d.nodeStop()
	}
	if d.gossipMgr != nil {
		d.gossipMgr.Stop()
	}
	if d.deltaMgr != nil {
		d.deltaMgr.Stop()
	}

	// Save vector index
	if d.vecIndex != nil && d.vecIndex.Dirty() {
		if err := d.vecIndex.Save(); err != nil {
			d.logger.Error("failed to save vector index", "err", err)
		}
	}

	// Close storage
	if d.store != nil {
		d.store.Close()
	}

	// Stop network host
	if d.host != nil {
		if err := d.host.Stop(); err != nil {
			d.logger.Error("network host shutdown error", "err", err)
		}
	}

	// Remove port file
	os.Remove(d.cfg.PortFile())

	d.cancel()
	d.logger.Info("daemon stopped")
	return nil
}

func (d *Daemon) acceptMCPConnections() {
	for {
		conn, err := d.mcpListener.Accept()
		if err != nil {
			if d.ctx.Err() != nil {
				return // shutting down
			}
			d.logger.Error("MCP accept error", "err", err)
			continue
		}
		go d.handleMCPConnection(conn)
	}
}

func (d *Daemon) handleMCPConnection(conn net.Conn) {
	defer conn.Close()
	d.logger.Info("MCP IPC connection accepted", "remote", conn.RemoteAddr())

	connCtx, connCancel := context.WithCancel(d.ctx)
	defer connCancel()

	if err := d.mcpServer.RunOnConnection(connCtx, conn); err != nil {
		d.logger.Error("MCP IPC session error", "remote", conn.RemoteAddr(), "err", err)
	}

	d.logger.Info("MCP IPC connection closed", "remote", conn.RemoteAddr())
}

func (d *Daemon) setupLogger() {
	logDir := d.cfg.LogDir()
	os.MkdirAll(logDir, 0755)

	logPath := filepath.Join(logDir, "daemon.log")
	writer := &lumberjack.Logger{
		Filename:   logPath,
		MaxSize:    50, // MB
		MaxBackups: 3,
		MaxAge:     30, // days
		Compress:   true,
	}

	if d.verbose {
		// Tee logs to both file and stderr
		multiWriter := io.MultiWriter(writer, os.Stderr)
		d.logger = slog.New(slog.NewTextHandler(multiWriter, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		}))
	} else {
		d.logger = slog.New(slog.NewJSONHandler(writer, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		}))
	}
}

func (d *Daemon) writePortFile(port int) error {
	return os.WriteFile(d.cfg.PortFile(), []byte(strconv.Itoa(port)), 0600)
}

// persistMultiaddrs builds full multiaddresses (/ip4/.../tcp/.../p2p/<peerID>)
// and writes them back to config.json so they can be shared as bootnodes.
// It also persists the actual bound listen address so the same port is reused
// on restart (important when the default port is 0 / auto-assign).
func (d *Daemon) persistMultiaddrs(peerID string) {
	addrs := d.host.Addrs()
	fullAddrs := make([]string, 0, len(addrs))
	for _, addr := range addrs {
		fullAddrs = append(fullAddrs, fmt.Sprintf("%s/p2p/%s", addr.String(), peerID))
	}

	// Extract bound addresses and update ListenAddrs
	// so the same ports are reused on next restart.
	// Deduplicate: multiple reported IPs share the same port.
	seen := make(map[string]bool)
	listenAddrs := make([]string, 0, 2)
	for _, addr := range addrs {
		parts := strings.Split(addr.String(), "/")
		for i, p := range parts {
			var la string
			if p == "tcp" && i+1 < len(parts) {
				la = fmt.Sprintf("/ip4/0.0.0.0/tcp/%s", parts[i+1])
			}
			if p == "udp" && i+1 < len(parts) {
				la = fmt.Sprintf("/ip4/0.0.0.0/udp/%s/quic-v1", parts[i+1])
			}
			if la != "" && !seen[la] {
				seen[la] = true
				listenAddrs = append(listenAddrs, la)
			}
		}
	}
	if len(listenAddrs) > 0 {
		d.cfg.ListenAddrs = listenAddrs
	}
	// Keep legacy ListenAddr for backward compat
	for _, addr := range addrs {
		parts := strings.Split(addr.String(), "/")
		for i, p := range parts {
			if p == "tcp" && i+1 < len(parts) {
				d.cfg.ListenAddr = fmt.Sprintf("/ip4/0.0.0.0/tcp/%s", parts[i+1])
				break
			}
		}
		if d.cfg.ListenAddr != "/ip4/0.0.0.0/tcp/0" {
			break
		}
	}

	d.cfg.NodeMultiaddrs = fullAddrs
	if err := d.cfg.Save(); err != nil {
		d.logger.Error("failed to persist multiaddresses to config", "err", err)
	} else {
		d.logger.Info("node multiaddresses persisted to config",
			"addrs", fullAddrs,
			"listen_addrs", d.cfg.ListenAddrs,
		)
	}
}

// peerEventNotifier logs peer connect/disconnect events.
type peerEventNotifier struct {
	logger *slog.Logger
}

func (n *peerEventNotifier) Connected(_ libnet.Network, conn libnet.Conn) {
	n.logger.Info("peer connected",
		"peer", conn.RemotePeer().String(),
		"addr", conn.RemoteMultiaddr().String(),
	)
	fmt.Printf("Peer connected: %s (%s)\n", conn.RemotePeer(), conn.RemoteMultiaddr())
}

func (n *peerEventNotifier) Disconnected(_ libnet.Network, conn libnet.Conn) {
	n.logger.Info("peer disconnected",
		"peer", conn.RemotePeer().String(),
	)
	fmt.Printf("Peer disconnected: %s\n", conn.RemotePeer())
}

func (n *peerEventNotifier) Listen(libnet.Network, multiaddr.Multiaddr)      {}
func (n *peerEventNotifier) ListenClose(libnet.Network, multiaddr.Multiaddr) {}
