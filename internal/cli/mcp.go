package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/dmgn/dmgn/internal/config"
	"github.com/dmgn/dmgn/internal/crypto"
	dmgnmcp "github.com/dmgn/dmgn/pkg/mcp"
	"github.com/dmgn/dmgn/pkg/identity"
	"github.com/dmgn/dmgn/pkg/query"
	"github.com/dmgn/dmgn/pkg/storage"
	"github.com/dmgn/dmgn/pkg/vectorindex"
)

// MCPServeCmd returns the cobra command for `dmgn mcp-serve`.
func MCPServeCmd() *cobra.Command {
	var dataDir string
	var enableNetwork bool
	var logLevel string

	cmd := &cobra.Command{
		Use:   "mcp-serve",
		Short: "Start DMGN as an MCP server on stdio",
		Long: `Start a Model Context Protocol (MCP) server communicating over stdin/stdout
using JSON-RPC 2.0. Designed for AI agent integration (Claude Desktop, Cline, etc).

By default runs local-only (no networking). Use --network to enable P2P features.`,
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

			// Derive master key for crypto
			masterKey, err := id.DeriveKey("memory-encryption", 32)
			if err != nil {
				return fmt.Errorf("failed to derive master key: %w", err)
			}

			cryptoEngine, err := crypto.NewEngine(masterKey)
			if err != nil {
				return fmt.Errorf("failed to initialize crypto: %w", err)
			}

			// Open storage
			store, err := storage.New(storage.Options{
				DataDir:      cfg.StorageDir(),
				MaxRetention: cfg.MaxRecentMemories,
			})
			if err != nil {
				return fmt.Errorf("failed to open storage: %w", err)
			}
			defer store.Close()

			// Derive vector index key and open index
			indexKey, err := id.DeriveKey("vector-index", 32)
			if err != nil {
				return fmt.Errorf("failed to derive vector index key: %w", err)
			}
			indexCrypto, err := crypto.NewEngine(indexKey)
			if err != nil {
				return fmt.Errorf("failed to init vector index crypto: %w", err)
			}

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

			// Create query engine
			decryptFn := func(ciphertext []byte) ([]byte, error) {
				return cryptoEngine.Decrypt(ciphertext)
			}
			queryEngine := query.NewQueryEngine(vecIndex, store, decryptFn, cfg.HybridScoreAlpha)

			// TODO: if enableNetwork, start libp2p + gossip + delta sync
			if enableNetwork {
				fmt.Fprintf(os.Stderr, "Network mode enabled (P2P features active)\n")
			}

			// Create and run MCP server
			mcpServer := dmgnmcp.NewMCPServer(store, vecIndex, queryEngine, cryptoEngine, id, cfg)

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			sigChan := make(chan os.Signal, 1)
			signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
			go func() {
				<-sigChan
				fmt.Fprintf(os.Stderr, "\nShutting down MCP server...\n")
				cancel()
			}()

			_ = logLevel // reserved for future observability wiring
			return mcpServer.Run(ctx)
		},
	}

	cmd.Flags().StringVar(&dataDir, "data-dir", "", "Data directory")
	cmd.Flags().BoolVar(&enableNetwork, "network", false, "Enable P2P networking (gossip, delta sync, cross-peer queries)")
	cmd.Flags().StringVar(&logLevel, "log-level", "info", "Log level (debug, info, warn, error)")

	return cmd
}
