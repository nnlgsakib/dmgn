package cli

import (
	"fmt"
	"io"
	"net"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/nnlgsakib/dmgn/internal/config"
	"github.com/nnlgsakib/dmgn/internal/daemon"
)

// MCPCmd returns the cobra command for `dmgn mcp`.
// This is a thin stdio↔TCP proxy that AI tools spawn to communicate with
// the running DMGN daemon's MCP server.
func MCPCmd() *cobra.Command {
	var dataDir string

	cmd := &cobra.Command{
		Use:   "mcp",
		Short: "Connect to DMGN daemon via MCP (stdio)",
		Long: `Bridges stdin/stdout to the running DMGN daemon's MCP server.

AI tools (Claude Desktop, Cline, Windsurf, etc.) should be configured to run:
  dmgn mcp

All MCP JSON-RPC messages pass through transparently. The daemon must be
running (start with: dmgn start).`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(dataDir)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: failed to load config: %v\n", err)
				os.Exit(1)
			}

			// Check if daemon is running
			_, running := daemon.CheckDaemonRunning(cfg.PIDFile())
			if !running {
				fmt.Fprintf(os.Stderr, "DMGN daemon is not running. Start it with: dmgn start\n")
				os.Exit(1)
			}

			// Read port file
			portData, err := os.ReadFile(cfg.PortFile())
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: cannot read MCP port file: %v\n", err)
				fmt.Fprintf(os.Stderr, "The daemon may not have started its MCP listener.\n")
				os.Exit(1)
			}
			port := strings.TrimSpace(string(portData))

			// Connect to daemon's MCP IPC
			conn, err := net.Dial("tcp", "127.0.0.1:"+port)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: cannot connect to daemon MCP (port %s): %v\n", port, err)
				os.Exit(1)
			}
			defer conn.Close()

			// Bidirectional bridge: stdin→conn, conn→stdout
			errChan := make(chan error, 2)

			go func() {
				_, err := io.Copy(conn, os.Stdin)
				errChan <- err
			}()

			go func() {
				_, err := io.Copy(os.Stdout, conn)
				errChan <- err
			}()

			// Wait for either direction to finish
			<-errChan
			return nil
		},
	}

	cmd.Flags().StringVar(&dataDir, "data-dir", "", "Data directory")

	return cmd
}
