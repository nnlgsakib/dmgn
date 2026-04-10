package cli

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/nnlgsakib/dmgn/internal/config"
	"github.com/nnlgsakib/dmgn/pkg/identity"
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

			if identity.Exists(cfg.DataDir) && !force {
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
					body := keyStr[start+len("-----BEGIN DMGN IDENTITY-----") : end]
					body = strings.TrimSpace(body)
					decoded, err := base64.StdEncoding.DecodeString(body)
					if err != nil {
						return fmt.Errorf("failed to decode armored identity: %w", err)
					}
					keyData = decoded
				}
			}

			// Validate key file has expected fields
			var keyCheck struct {
				Version    int    `json:"version"`
				PublicKey  string `json:"public_key"`
				Salt       []byte `json:"salt"`
				Nonce      []byte `json:"nonce"`
				Ciphertext []byte `json:"ciphertext"`
			}
			if err := json.Unmarshal(keyData, &keyCheck); err != nil {
				return fmt.Errorf("invalid key file format: %w", err)
			}
			if keyCheck.PublicKey == "" || keyCheck.Salt == nil || keyCheck.Nonce == nil || keyCheck.Ciphertext == nil {
				return fmt.Errorf("invalid key file: missing required fields")
			}

			if err := identity.Import(keyData, cfg.DataDir); err != nil {
				return fmt.Errorf("failed to import identity: %w", err)
			}

			if err := cfg.Save(); err != nil {
				return fmt.Errorf("failed to save config: %w", err)
			}

			fmt.Println("\u2713 Identity imported successfully!")
			fmt.Printf("  Node ID: %s\n", keyCheck.PublicKey)
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
