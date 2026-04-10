package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/nnlgsakib/dmgn/internal/config"
	"github.com/nnlgsakib/dmgn/internal/daemon"
)

// StopCmd returns the cobra command for `dmgn stop`.
func StopCmd() *cobra.Command {
	var dataDir string

	cmd := &cobra.Command{
		Use:   "stop",
		Short: "Stop the DMGN daemon",
		Long:  `Gracefully stop the running DMGN background daemon.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(dataDir)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			pid, running := daemon.CheckDaemonRunning(cfg.PIDFile())
			if !running {
				fmt.Println("No DMGN daemon is running.")
				return nil
			}

			fmt.Printf("Stopping DMGN daemon (PID: %d)...\n", pid)

			if err := daemon.StopDaemon(cfg.PIDFile(), cfg.PortFile(), 10*time.Second); err != nil {
				return fmt.Errorf("failed to stop daemon: %w", err)
			}

			fmt.Println("DMGN daemon stopped.")
			return nil
		},
	}

	cmd.Flags().StringVar(&dataDir, "data-dir", "", "Data directory")

	return cmd
}
