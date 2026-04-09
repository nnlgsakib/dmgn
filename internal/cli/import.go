package cli

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	
	"github.com/dmgn/dmgn/internal/config"
	"github.com/dmgn/dmgn/pkg/identity"
)

func ImportCmd() *cobra.Command {
	var (
		dataDir string
		inFile  string
		force   bool
	)

	cmd := &cobra.Command{
		Use:   "import",
		Short: "Import identity from backup",
		Long:  `Import a previously exported identity key.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(dataDir)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			if identity.Exists(cfg.IdentityDir()) && !force {
				return fmt.Errorf("identity already exists. Use --force to overwrite")
			}

			var keyData []byte
			var errRead error

			if inFile == "" || inFile == "-" {
				fmt.Println("Reading identity from stdin...")
				keyData, errRead = io.ReadAll(os.Stdin)
			} else {
				keyData, errRead = os.ReadFile(inFile)
			}

			if errRead != nil {
				return fmt.Errorf("failed to read identity: %w", errRead)
			}

			keyStr := string(keyData)
			if strings.Contains(keyStr, "-----BEGIN DMGN IDENTITY-----") {
				start := strings.Index(keyStr, "-----BEGIN DMGN IDENTITY-----")
				end := strings.Index(keyStr, "-----END DMGN IDENTITY-----")
				if start != -1 && end != -1 {
					keyStr = keyStr[start+len("-----BEGIN DMGN IDENTITY-----") : end]
					keyData = []byte(strings.TrimSpace(keyStr))
				}
			}

			if err := identity.Import(keyData, cfg.IdentityDir()); err != nil {
				return fmt.Errorf("failed to import identity: %w", err)
			}

			if err := cfg.Save(); err != nil {
				return fmt.Errorf("failed to save config: %w", err)
			}

			fmt.Println("✓ Identity imported successfully!")
			fmt.Printf("  Data directory: %s\n", cfg.DataDir)
			fmt.Println()
			fmt.Println("You can now use this node with your existing identity.")

			return nil
		},
	}

	cmd.Flags().StringVar(&dataDir, "data-dir", "", "Data directory")
	cmd.Flags().StringVarP(&inFile, "input", "i", "", "Input file (default: stdin)")
	cmd.Flags().BoolVar(&force, "force", false, "Overwrite existing identity")

	return cmd
}
