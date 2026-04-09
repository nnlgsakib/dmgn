package cli

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	
	"github.com/dmgn/dmgn/internal/config"
	"github.com/dmgn/dmgn/pkg/identity"
)

func StartCmd() *cobra.Command {
	var dataDir string

	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start the DMGN daemon",
		Long:  `Start the DMGN node daemon with networking enabled.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(dataDir)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			if !identity.Exists(cfg.IdentityDir()) {
				return fmt.Errorf("no identity found. Run 'dmgn init' first")
			}

			fmt.Println("Starting DMGN node...")
			fmt.Println()
			fmt.Println("NOTE: Full networking is not yet implemented in Phase 1.")
			fmt.Println("This command will start a local-only node for testing.")
			fmt.Println()
			fmt.Printf("Data directory: %s\n", cfg.DataDir)
			fmt.Printf("Storage: %s\n", cfg.StorageDir())
			fmt.Println()
			fmt.Println("Press Ctrl+C to stop.")
			fmt.Println()

			sigChan := make(chan os.Signal, 1)
			signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

			<-sigChan

			fmt.Println()
			fmt.Println("Shutting down...")

			return nil
		},
	}

	cmd.Flags().StringVar(&dataDir, "data-dir", "", "Data directory")

	return cmd
}
