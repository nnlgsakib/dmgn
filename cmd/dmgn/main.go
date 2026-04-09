package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/dmgn/dmgn/internal/cli"
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "dmgn",
		Short: "Distributed Memory Graph Network",
		Long: `DMGN is a decentralized, encrypted, lifetime memory layer for AI agents.

It provides user-owned, persistent memory that works across devices
without relying on central servers. All data is end-to-end encrypted
and resilient to node failure.`,
		Version: "0.1.0",
	}

	rootCmd.AddCommand(cli.InitCmd())
	rootCmd.AddCommand(cli.AddCmd())
	rootCmd.AddCommand(cli.QueryCmd())
	rootCmd.AddCommand(cli.StatusCmd())
	rootCmd.AddCommand(cli.StartCmd())
	rootCmd.AddCommand(cli.ExportCmd())
	rootCmd.AddCommand(cli.ImportCmd())

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
