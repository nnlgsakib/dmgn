package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/dmgn/dmgn/internal/api"
	"github.com/dmgn/dmgn/internal/config"
	"github.com/dmgn/dmgn/internal/crypto"
	"github.com/dmgn/dmgn/pkg/identity"
	"github.com/dmgn/dmgn/pkg/network"
	"github.com/dmgn/dmgn/pkg/storage"
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
