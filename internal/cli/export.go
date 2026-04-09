package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	
	"github.com/dmgn/dmgn/internal/config"
	"github.com/dmgn/dmgn/pkg/identity"
)

func ExportCmd() *cobra.Command {
	var (
		dataDir  string
		outFile  string
		armored  bool
	)

	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export identity for backup",
		Long:  `Export the encrypted identity key for backup and recovery.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(dataDir)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			if !identity.Exists(cfg.IdentityDir()) {
				return fmt.Errorf("no identity found. Run 'dmgn init' first")
			}

			keyData, err := identity.Export(cfg.IdentityDir())
			if err != nil {
				return fmt.Errorf("failed to export identity: %w", err)
			}

			output := keyData
			if armored {
				output = []byte(fmt.Sprintf("-----BEGIN DMGN IDENTITY-----\n%s\n-----END DMGN IDENTITY-----\n", keyData))
			}

			if outFile == "" {
				fmt.Print(string(output))
			} else {
				if err := os.WriteFile(outFile, output, 0600); err != nil {
					return fmt.Errorf("failed to write export file: %w", err)
				}
				fmt.Printf("Identity exported to: %s\n", outFile)
			}

			fmt.Println()
			fmt.Println("IMPORTANT:")
			fmt.Println("- Store this export securely")
			fmt.Println("- Your passphrase is required to use this backup")
			fmt.Println("- Never share your passphrase with anyone")

			return nil
		},
	}

	cmd.Flags().StringVar(&dataDir, "data-dir", "", "Data directory")
	cmd.Flags().StringVarP(&outFile, "output", "o", "", "Output file (default: stdout)")
	cmd.Flags().BoolVar(&armored, "armored", false, "Add armor headers")

	return cmd
}
