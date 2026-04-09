package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/nnlgsakib/dmgn/internal/api"
	"github.com/nnlgsakib/dmgn/internal/config"
	"github.com/nnlgsakib/dmgn/internal/crypto"
	"github.com/nnlgsakib/dmgn/pkg/identity"
	"github.com/nnlgsakib/dmgn/pkg/memory"
	"github.com/nnlgsakib/dmgn/pkg/network"
	"github.com/nnlgsakib/dmgn/pkg/query"
	"github.com/nnlgsakib/dmgn/pkg/sharding"
	"github.com/nnlgsakib/dmgn/pkg/storage"
	pkgsync "github.com/nnlgsakib/dmgn/pkg/sync"
	"github.com/nnlgsakib/dmgn/pkg/vectorindex"
	dmgnpb "github.com/nnlgsakib/dmgn/proto/dmgn/v1"
	"google.golang.org/protobuf/proto"
)

func StartCmd() *cobra.Command {
	var dataDir string
	var noAPI bool

	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start the DMGN daemon",
		Long:  `Start the DMGN node with libp2p networking and optional REST API server.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(dataDir)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			if !identity.Exists(cfg.IdentityDir()) {
				return fmt.Errorf("no identity found. Run 'dmgn init' first")
			}

			passphrase, err := promptPassphraseOnce()
			if err != nil {
				return err
			}

			id, err := identity.Load(passphrase, cfg.IdentityDir())
			if err != nil {
				return fmt.Errorf("failed to load identity: %w", err)
			}

			// Derive libp2p host key from identity
			privKey, err := network.DeriveLibp2pKey(id)
			if err != nil {
				return fmt.Errorf("failed to derive network key: %w", err)
			}

			// Create and start libp2p host
			hostCfg := network.HostConfig{
				ListenAddrs:    []string{cfg.ListenAddr},
				BootstrapPeers: cfg.BootstrapPeers,
				MDNSService:    cfg.MDNSService,
				MaxPeersLow:    cfg.MaxPeersLow,
				MaxPeersHigh:   cfg.MaxPeersHigh,
				PrivateKey:     privKey,
			}

			h, err := network.NewHost(hostCfg)
			if err != nil {
				return fmt.Errorf("failed to create network host: %w", err)
			}

			if err := h.Start(); err != nil {
				return fmt.Errorf("failed to start network host: %w", err)
			}

			fmt.Println("DMGN node started")
			fmt.Printf("  Peer ID:  %s\n", h.ID())
			for _, addr := range h.Addrs() {
				fmt.Printf("  Listen:   %s/p2p/%s\n", addr, h.ID())
			}
			fmt.Println()

			// Optionally start API server
			var server *api.Server
			if !noAPI {
				masterKey, err := id.DeriveKey("memory-encryption", 32)
				if err != nil {
					h.Stop()
					return fmt.Errorf("failed to derive master key: %w", err)
				}

				cryptoEngine, err := crypto.NewEngine(masterKey)
				if err != nil {
					h.Stop()
					return fmt.Errorf("failed to initialize crypto: %w", err)
				}

				store, err := storage.New(storage.Options{
					DataDir:      cfg.StorageDir(),
					MaxRetention: cfg.MaxRecentMemories,
				})
				if err != nil {
					h.Stop()
					return fmt.Errorf("failed to open storage: %w", err)
				}
				defer store.Close()

				server, err = api.NewServer(cfg, store, cryptoEngine, id)
				if err != nil {
					h.Stop()
					return fmt.Errorf("failed to create API server: %w", err)
				}
				server.SetNetworkHost(h)

				// Register shard protocol handlers
				h.RegisterStoreHandler(store)
				h.RegisterFetchHandler(store)

				// Set up shard distributor and rebalancing
				shardCfg := sharding.ShardConfig{
					Threshold:   cfg.ShardThreshold,
					TotalShards: cfg.ShardCount,
				}
				var router *network.ShardRouter
				if h.DHT() != nil {
					router = network.NewShardRouter(h.DHT())
				}
				_ = sharding.NewDistributor(nil, store, router, shardCfg)

				// Start rebalance auditor
				auditor := network.NewShardAuditor(nil, 5*time.Minute)
				auditor.Start(context.Background())

				fmt.Printf("Shard config: threshold=%d, total=%d\n", shardCfg.Threshold, shardCfg.TotalShards)

				// --- Phase 5: Query & Sync wiring ---

				// 1. Derive vector index encryption key
				indexKey, err := id.DeriveKey("vector-index", 32)
				if err != nil {
					h.Stop()
					return fmt.Errorf("failed to derive vector index key: %w", err)
				}
				indexCrypto, err := crypto.NewEngine(indexKey)
				if err != nil {
					h.Stop()
					return fmt.Errorf("failed to init vector index crypto: %w", err)
				}

				// 2. Create and load vector index
				vecIndex := vectorindex.NewVectorIndex(
					cfg.VectorIndexPath(),
					indexCrypto.Encrypt,
					indexCrypto.Decrypt,
				)
				if err := vecIndex.Load(); err != nil {
					fmt.Fprintf(os.Stderr, "Warning: failed to load vector index: %v\n", err)
				}
				defer func() {
					if vecIndex.Dirty() {
						if err := vecIndex.Save(); err != nil {
							fmt.Fprintf(os.Stderr, "Warning: failed to save vector index: %v\n", err)
						}
					}
				}()

				// 3. Create query engine
				decryptFn := func(ciphertext []byte) ([]byte, error) {
					return cryptoEngine.Decrypt(ciphertext)
				}
				queryEngine := query.NewQueryEngine(vecIndex, store, decryptFn, cfg.HybridScoreAlpha)

				// 4. Create remote query orchestrator
				remoteOrch := query.NewRemoteQueryOrchestrator(
					h.LibP2PHost(), queryEngine, h.ID().String(), cfg.QueryTimeoutDuration(),
				)

				// 5. Register query protocol handler
				query.RegisterQueryHandler(h.LibP2PHost(), queryEngine, h.ID().String())

				// 6. Set up version vector
				vvStore := pkgsync.NewVClockStore(store.DB())
				vv, err := vvStore.Load(h.ID().String())
				if err != nil {
					fmt.Fprintf(os.Stderr, "Warning: failed to load version vector: %v\n", err)
					vv = pkgsync.NewVersionVector()
				}

				// 7. Memory receive callback (shared by gossip + delta sync)
				onMemoryReceived := func(mem *memory.Memory) {
					store.SaveMemory(mem)
					if len(mem.Embedding) > 0 {
						vecIndex.Add(mem.ID, mem.Embedding)
					}
				}

				// 8. Start gossip manager
				nodeCtx, nodeCancel := context.WithCancel(context.Background())
				defer nodeCancel()

				gossipMgr, err := pkgsync.NewGossipManager(nodeCtx, h.LibP2PHost(), cfg.GossipTopic,
					func(msg *dmgnpb.GossipMessage) {
						pb := &dmgnpb.Memory{}
						if err := proto.Unmarshal(msg.Memory, pb); err != nil {
							return
						}
						mem := memory.MemoryFromProto(pb)
						onMemoryReceived(mem)
						// Update version vector from gossip
						if msg.Sequence > vv.Get(msg.SenderPeerId) {
							vv.Set(msg.SenderPeerId, msg.Sequence)
							vvStore.SaveSequence(msg.SenderPeerId, msg.Sequence, mem.ID)
							vvStore.Save(h.ID().String(), vv)
						}
					})
				if err != nil {
					fmt.Fprintf(os.Stderr, "Warning: gossip init failed: %v\n", err)
				} else {
					gossipMgr.Start(nodeCtx)
					defer gossipMgr.Stop()
					fmt.Printf("GossipSub: topic=%s\n", cfg.GossipTopic)
				}

				// 9. Start delta sync manager
				deltaMgr := pkgsync.NewDeltaSyncManager(
					h.LibP2PHost(), vv, vvStore, store,
					h.ID().String(), cfg.SyncIntervalDuration(), onMemoryReceived,
				)
				deltaMgr.RegisterHandler()
				deltaMgr.Start(nodeCtx)
				defer deltaMgr.Stop()
				fmt.Printf("Delta sync: interval=%s\n", cfg.SyncInterval)

				// 10. Wire query engine and gossip into API server
				server.SetQueryEngine(queryEngine, remoteOrch)
				if gossipMgr != nil {
					server.SetGossipManager(gossipMgr)
				}
				server.SetVectorIndex(vecIndex)

				fmt.Printf("Query engine: alpha=%.2f, timeout=%s\n", cfg.HybridScoreAlpha, cfg.QueryTimeout)

				apiKeyStr := server.APIKey()
				fmt.Printf("API Key: %s\n", apiKeyStr)
				fmt.Printf("(Use as: Authorization: Bearer %s)\n", apiKeyStr)
				fmt.Println()

				go func() {
					if err := server.Start(); err != nil && err.Error() != "http: Server closed" {
						fmt.Fprintf(os.Stderr, "API server error: %v\n", err)
					}
				}()
			} else {
				fmt.Println("API server disabled (--no-api)")
				fmt.Println()
			}

			fmt.Println("Press Ctrl+C to stop.")

			sigChan := make(chan os.Signal, 1)
			signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
			<-sigChan

			fmt.Println("\nShutting down...")

			if server != nil {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				if err := server.Stop(ctx); err != nil {
					fmt.Fprintf(os.Stderr, "API server shutdown error: %v\n", err)
				}
			}

			if err := h.Stop(); err != nil {
				return fmt.Errorf("network host shutdown error: %w", err)
			}

			fmt.Println("Node stopped.")
			return nil
		},
	}

	cmd.Flags().StringVar(&dataDir, "data-dir", "", "Data directory")
	cmd.Flags().BoolVar(&noAPI, "no-api", false, "Disable REST API server")

	return cmd
}
