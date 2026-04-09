package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	
	"github.com/dmgn/dmgn/internal/config"
	"github.com/dmgn/dmgn/pkg/identity"
	"github.com/dmgn/dmgn/pkg/storage"
)

func StatusCmd() *cobra.Command {
	var dataDir string

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show node status",
		Long:  `Display information about the DMGN node, including identity, storage stats, and peer count.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(dataDir)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			fmt.Println("DMGN Node Status")
			fmt.Println("================")
			fmt.Println()

			if identity.Exists(cfg.IdentityDir()) {
				fmt.Println("Identity: Present")
				
				keyData, err := identity.Export(cfg.IdentityDir())
				if err == nil {
					fmt.Printf("  Key file: %d bytes\n", len(keyData))
				}
			} else {
				fmt.Println("Identity: Not initialized")
				fmt.Println("  Run 'dmgn init' to create an identity")
			}

			fmt.Println()
			fmt.Println("Configuration:")
			fmt.Printf("  Data directory: %s\n", cfg.DataDir)
			fmt.Printf("  API port: %d\n", cfg.APIPort)
			fmt.Printf("  Log level: %s\n", cfg.LogLevel)

			store, err := storage.New(storage.Options{
				DataDir: cfg.StorageDir(),
			})
			if err != nil {
				fmt.Println()
				fmt.Println("Storage: Error")
				fmt.Printf("  %v\n", err)
			} else {
				defer store.Close()
				
				stats, err := store.GetStats()
				if err != nil {
					fmt.Println()
					fmt.Println("Storage: Error reading stats")
					fmt.Printf("  %v\n", err)
				} else {
					fmt.Println()
					fmt.Println("Storage:")
					fmt.Printf("  Memories: %d\n", stats["memory_count"])
					fmt.Printf("  Edges: %d\n", stats["edge_count"])
					fmt.Printf("  Path: %s\n", cfg.StorageDir())
				}
			}

			fmt.Println()
			fmt.Println("Network:")
			fmt.Println("  Status: Not started")
			fmt.Println("  Peers: 0 connected")
			fmt.Printf("  Listen: %s\n", cfg.ListenAddr)

			return nil
		},
	}

	cmd.Flags().StringVar(&dataDir, "data-dir", "", "Data directory")

	return cmd
}
