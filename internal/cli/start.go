package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/nnlgsakib/dmgn/internal/config"
	"github.com/nnlgsakib/dmgn/internal/daemon"
	"github.com/nnlgsakib/dmgn/pkg/identity"
)

const derivedKeysEnvVar = "DMGN_DERIVED_KEYS"

func StartCmd() *cobra.Command {
	var dataDir string
	var foreground bool
	var daemonMode bool
	var passFlag string

	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start the DMGN daemon",
		Long: `Start the DMGN background daemon with libp2p networking, REST API, and MCP server.

The daemon connects to peers via bootnodes and runs until stopped with 'dmgn stop'.
AI tools communicate with the daemon through 'dmgn mcp'.

Use --foreground to run in the current terminal (useful for debugging).`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(dataDir)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			// Path A: Daemon mode (child process, started by parent)
			if daemonMode {
				return runDaemonMode(cfg)
			}

			// Path B: Normal start (parent launcher)
			return runLauncher(cfg, foreground, passFlag)
		},
	}

	cmd.Flags().StringVar(&dataDir, "data-dir", "", "Data directory")
	cmd.Flags().StringVar(&passFlag, "pass", "", "Passphrase (skip interactive prompt)")
	cmd.Flags().BoolVar(&foreground, "foreground", false, "Run daemon in foreground (debug mode)")
	cmd.Flags().BoolVar(&daemonMode, "daemon-mode", false, "Internal: run as daemon child process")
	cmd.Flags().MarkHidden("daemon-mode")

	return cmd
}

// runDaemonMode is the child daemon process entry point.
// It receives pre-derived keys via environment variable.
func runDaemonMode(cfg *config.Config) error {
	encoded := os.Getenv(derivedKeysEnvVar)
	if encoded == "" {
		return fmt.Errorf("internal error: %s environment variable not set", derivedKeysEnvVar)
	}

	keys, err := daemon.Decode(encoded)
	if err != nil {
		return fmt.Errorf("failed to decode derived keys: %w", err)
	}

	d := daemon.New(cfg, keys)

	// Write PID file
	if err := daemon.WritePID(cfg.PIDFile()); err != nil {
		return fmt.Errorf("failed to write PID file: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := d.Start(ctx); err != nil {
		daemon.RemovePID(cfg.PIDFile())
		return fmt.Errorf("daemon start failed: %w", err)
	}

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	d.Stop()
	daemon.RemovePID(cfg.PIDFile())
	return nil
}

// runLauncher is the foreground parent process that prompts for passphrase,
// derives keys, and either runs the daemon directly or spawns a background child.
func runLauncher(cfg *config.Config, foreground bool, passFlag string) error {
	// Check if daemon already running
	if pid, running := daemon.CheckDaemonRunning(cfg.PIDFile()); running {
		return fmt.Errorf("DMGN daemon already running (PID: %d)", pid)
	}

	// Clean up stale PID file if present
	daemon.RemovePID(cfg.PIDFile())

	if !identity.Exists(cfg.DataDir) {
		return fmt.Errorf("no identity found. Run 'dmgn init' first")
	}

	var passphrase string
	var err error
	if passFlag != "" {
		passphrase = passFlag
	} else {
		passphrase, err = promptPassphraseOnce()
		if err != nil {
			return err
		}
	}

	id, err := identity.Load(passphrase, cfg.DataDir)
	if err != nil {
		return fmt.Errorf("failed to load identity: %w", err)
	}

	// Derive all key material
	keys, err := daemon.DeriveAll(id)
	if err != nil {
		return fmt.Errorf("failed to derive keys: %w", err)
	}

	if foreground {
		return runForeground(cfg, keys)
	}

	return spawnBackground(cfg, keys)
}

// runForeground runs the daemon in the current terminal (debug mode).
func runForeground(cfg *config.Config, keys *daemon.DerivedKeys) error {
	d := daemon.New(cfg, keys)

	if err := daemon.WritePID(cfg.PIDFile()); err != nil {
		return fmt.Errorf("failed to write PID file: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := d.Start(ctx); err != nil {
		daemon.RemovePID(cfg.PIDFile())
		return fmt.Errorf("daemon start failed: %w", err)
	}

	// Reload config to get persisted multiaddresses
	if updated, err := config.Load(cfg.DataDir); err == nil && len(updated.NodeMultiaddrs) > 0 {
		fmt.Println("Node multiaddresses:")
		for _, addr := range updated.NodeMultiaddrs {
			fmt.Printf("  %s\n", addr)
		}
		fmt.Println()
	}

	fmt.Println("DMGN daemon running in foreground. Press Ctrl+C to stop.")

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	fmt.Println("\nShutting down...")
	d.Stop()
	daemon.RemovePID(cfg.PIDFile())
	fmt.Println("Daemon stopped.")
	return nil
}

// spawnBackground spawns the daemon as a detached background process.
func spawnBackground(cfg *config.Config, keys *daemon.DerivedKeys) error {
	encoded, err := keys.Encode()
	if err != nil {
		return fmt.Errorf("failed to encode keys: %w", err)
	}

	// Build args for child process
	args := []string{"start", "--daemon-mode"}
	if cfg.DataDir != "" {
		args = append(args, "--data-dir", cfg.DataDir)
	}

	// Build env: inherit current env + add derived keys
	env := os.Environ()
	// Remove any existing DMGN_DERIVED_KEYS to be safe
	filtered := make([]string, 0, len(env))
	for _, e := range env {
		if !strings.HasPrefix(e, derivedKeysEnvVar+"=") {
			filtered = append(filtered, e)
		}
	}
	filtered = append(filtered, derivedKeysEnvVar+"="+encoded)

	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to resolve executable path: %w", err)
	}

	pid, err := daemon.SpawnDaemon(execPath, args, filtered)
	if err != nil {
		return fmt.Errorf("failed to spawn daemon: %w", err)
	}

	// Wait for daemon to become healthy
	fmt.Printf("Starting DMGN daemon (PID: %d)...\n", pid)
	if err := daemon.WaitForHealthy(cfg.PIDFile(), 10*time.Second); err != nil {
		return fmt.Errorf("daemon failed to start: %w", err)
	}

	// Reload config to get persisted multiaddresses and port
	updated, _ := config.Load(cfg.DataDir)
	portData, _ := os.ReadFile(cfg.PortFile())
	port := strings.TrimSpace(string(portData))

	fmt.Printf("DMGN daemon started successfully!\n")
	fmt.Printf("  PID:          %d\n", pid)
	if port != "" {
		fmt.Printf("  MCP IPC port: %s\n", port)
	}
	if updated != nil && len(updated.NodeMultiaddrs) > 0 {
		fmt.Println("  Multiaddresses:")
		for _, addr := range updated.NodeMultiaddrs {
			fmt.Printf("    %s\n", addr)
		}
	}
	fmt.Println()
	fmt.Println("  To stop:      dmgn stop")
	fmt.Println("  AI tools:     dmgn mcp")
	fmt.Println()
	fmt.Println("Share your multiaddress with other nodes as a bootstrap peer.")
	return nil
}
