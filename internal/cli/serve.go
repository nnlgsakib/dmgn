package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/dmgn/dmgn/internal/api"
	"github.com/dmgn/dmgn/internal/config"
	"github.com/dmgn/dmgn/internal/crypto"
	"github.com/dmgn/dmgn/pkg/identity"
	"github.com/dmgn/dmgn/pkg/storage"
)

func ServeCmd() *cobra.Command {
	var dataDir string

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the REST API server",
		Long:  `Start the DMGN REST API server with Bearer token authentication.`,
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

			masterKey, err := id.DeriveKey("memory-encryption", 32)
			if err != nil {
				return fmt.Errorf("failed to derive master key: %w", err)
			}

			cryptoEngine, err := crypto.NewEngine(masterKey)
			if err != nil {
				return fmt.Errorf("failed to initialize crypto: %w", err)
			}

			store, err := storage.New(storage.Options{
				DataDir:      cfg.StorageDir(),
				MaxRetention: cfg.MaxRecentMemories,
			})
			if err != nil {
				return fmt.Errorf("failed to open storage: %w", err)
			}
			defer store.Close()

			server, err := api.NewServer(cfg, store, cryptoEngine, id)
			if err != nil {
				return fmt.Errorf("failed to create API server: %w", err)
			}

			apiKeyStr := server.APIKey()
			fmt.Println()
			fmt.Printf("API Key: %s\n", apiKeyStr)
			fmt.Printf("(Use as: Authorization: Bearer %s)\n", apiKeyStr)
			fmt.Println()

			// Graceful shutdown on SIGINT/SIGTERM
			quit := make(chan os.Signal, 1)
			signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

			go func() {
				if err := server.Start(); err != nil && err.Error() != "http: Server closed" {
					fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
					os.Exit(1)
				}
			}()

			<-quit
			fmt.Println("\nShutting down server...")

			ctx, cancel := context.WithTimeout(context.Background(), 5000000000) // 5 seconds
			defer cancel()

			if err := server.Stop(ctx); err != nil {
				return fmt.Errorf("server shutdown failed: %w", err)
			}

			fmt.Println("Server stopped.")
			return nil
		},
	}

	cmd.Flags().StringVar(&dataDir, "data-dir", "", "Data directory")

	return cmd
}
