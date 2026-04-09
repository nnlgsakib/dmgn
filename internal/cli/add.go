package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	
	"github.com/dmgn/dmgn/internal/config"
	"github.com/dmgn/dmgn/internal/crypto"
	"github.com/dmgn/dmgn/pkg/identity"
	"github.com/dmgn/dmgn/pkg/memory"
	"github.com/dmgn/dmgn/pkg/storage"
)

func AddCmd() *cobra.Command {
	var (
		memoryType string
		links      []string
		dataDir    string
	)

	cmd := &cobra.Command{
		Use:   "add <text>",
		Short: "Add a memory to the network",
		Long:  `Add stores a new memory locally with content-addressable ID and optional links.`,
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			content := strings.Join(args, " ")

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

			encryptFn := func(plaintext []byte) ([]byte, error) {
				return cryptoEngine.Encrypt(plaintext)
			}

			memType := memory.Type(memoryType)
			if memType == "" {
				memType = memory.TypeText
			}

			plaintext := &memory.PlaintextMemory{
				Content:  content,
				Type:     memType,
				Metadata: map[string]string{
					"source": "cli",
					"author": id.ID,
				},
			}

			mem, err := memory.Create(plaintext, links, encryptFn)
			if err != nil {
				return fmt.Errorf("failed to create memory: %w", err)
			}

			store, err := storage.New(storage.Options{
				DataDir: cfg.StorageDir(),
			})
			if err != nil {
				return fmt.Errorf("failed to open storage: %w", err)
			}
			defer store.Close()

			if err := store.SaveMemory(mem); err != nil {
				return fmt.Errorf("failed to save memory: %w", err)
			}

			fmt.Printf("✓ Memory added: %s\n", mem.ID[:16])
			if len(links) > 0 {
				fmt.Printf("  Linked to: %d memories\n", len(links))
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&memoryType, "type", "text", "Memory type (text, conversation, observation)")
	cmd.Flags().StringSliceVar(&links, "link", nil, "Link to memory IDs")
	cmd.Flags().StringVar(&dataDir, "data-dir", "", "Data directory")

	return cmd
}
