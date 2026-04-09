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

func QueryCmd() *cobra.Command {
	var (
		limit   int
		dataDir string
		recent  bool
	)

	cmd := &cobra.Command{
		Use:   "query <text>",
		Short: "Search for memories",
		Long:  `Query searches for memories by text content or returns recent memories.`,
		RunE: func(cmd *cobra.Command, args []string) error {
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

			decryptFn := func(ciphertext []byte) ([]byte, error) {
				return cryptoEngine.Decrypt(ciphertext)
			}

			store, err := storage.New(storage.Options{
				DataDir: cfg.StorageDir(),
			})
			if err != nil {
				return fmt.Errorf("failed to open storage: %w", err)
			}
			defer store.Close()

			var memories []*memory.Memory

			if recent || len(args) == 0 {
				memories, err = store.GetRecentMemories(limit)
				if err != nil {
					return fmt.Errorf("failed to get recent memories: %w", err)
				}
			} else {
				queryText := strings.Join(args, " ")
				memories, err = searchMemories(store, queryText, limit, decryptFn)
				if err != nil {
					return fmt.Errorf("failed to search memories: %w", err)
				}
			}

			if len(memories) == 0 {
				fmt.Println("No memories found.")
				return nil
			}

			fmt.Printf("Found %d memories:\n\n", len(memories))

			for i, mem := range memories {
				plaintext, err := mem.Decrypt(decryptFn)
				if err != nil {
					fmt.Printf("%d. [Error decrypting: %v]\n", i+1, err)
					continue
				}

				preview := plaintext.Content
				if len(preview) > 100 {
					preview = preview[:100] + "..."
				}

				fmt.Printf("%d. %s\n", i+1, preview)
				fmt.Printf("   ID: %s... | Type: %s | Links: %d\n", 
					mem.ID[:16], mem.Type, len(mem.Links))
				fmt.Println()
			}

			return nil
		},
	}

	cmd.Flags().IntVar(&limit, "limit", 10, "Maximum number of results")
	cmd.Flags().StringVar(&dataDir, "data-dir", "", "Data directory")
	cmd.Flags().BoolVar(&recent, "recent", false, "Show most recent memories")

	return cmd
}

func searchMemories(store *storage.Store, query string, limit int, decryptFn func([]byte) ([]byte, error)) ([]*memory.Memory, error) {
	allMemories, err := store.GetRecentMemories(1000)
	if err != nil {
		return nil, err
	}

	queryLower := strings.ToLower(query)
	var results []*memory.Memory

	for _, mem := range allMemories {
		if len(results) >= limit {
			break
		}

		plaintext, err := mem.Decrypt(decryptFn)
		if err != nil {
			continue
		}

		contentLower := strings.ToLower(plaintext.Content)
		if strings.Contains(contentLower, queryLower) {
			results = append(results, mem)
		}
	}

	return results, nil
}
