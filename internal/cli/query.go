package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/nnlgsakib/dmgn/internal/config"
	"github.com/nnlgsakib/dmgn/internal/crypto"
	"github.com/nnlgsakib/dmgn/pkg/identity"
	"github.com/nnlgsakib/dmgn/pkg/memory"
	"github.com/nnlgsakib/dmgn/pkg/storage"
)

type scoredResult struct {
	memory *memory.Memory
	plain  *memory.PlaintextMemory
	score  float64
}

type jsonResult struct {
	ID        string            `json:"id"`
	Content   string            `json:"content"`
	Type      string            `json:"type"`
	Score     float64           `json:"score"`
	Timestamp int64             `json:"timestamp"`
	Links     []string          `json:"links"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

type jsonOutput struct {
	Results []jsonResult `json:"results"`
}

func QueryCmd() *cobra.Command {
	var (
		limit      int
		dataDir    string
		recent     bool
		formatFlag string
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

			if !identity.Exists(cfg.DataDir) {
				return fmt.Errorf("no identity found. Run 'dmgn init' first")
			}

			passphrase, err := promptPassphraseOnce()
			if err != nil {
				return err
			}

			id, err := identity.Load(passphrase, cfg.DataDir)
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

			if recent || len(args) == 0 {
				memories, err := store.GetRecentMemories(limit)
				if err != nil {
					return fmt.Errorf("failed to get recent memories: %w", err)
				}

				if len(memories) == 0 {
					fmt.Println("No memories found.")
					return nil
				}

				// Convert to scored results with score 0 for recent listing
				results := make([]scoredResult, 0, len(memories))
				for _, mem := range memories {
					plain, err := mem.Decrypt(decryptFn)
					if err != nil {
						continue
					}
					results = append(results, scoredResult{memory: mem, plain: plain, score: 0})
				}

				return outputResults(results, formatFlag)
			}

			queryText := strings.Join(args, " ")
			results, err := searchMemoriesScored(store, queryText, limit, decryptFn)
			if err != nil {
				return fmt.Errorf("failed to search memories: %w", err)
			}

			if len(results) == 0 {
				fmt.Println("No memories found.")
				return nil
			}

			return outputResults(results, formatFlag)
		},
	}

	cmd.Flags().IntVar(&limit, "limit", 10, "Maximum number of results")
	cmd.Flags().StringVar(&dataDir, "data-dir", "", "Data directory")
	cmd.Flags().BoolVar(&recent, "recent", false, "Show most recent memories")
	cmd.Flags().StringVar(&formatFlag, "format", "text", "Output format (text or json)")

	return cmd
}

func searchMemoriesScored(store *storage.Store, query string, limit int, decryptFn func([]byte) ([]byte, error)) ([]scoredResult, error) {
	allMemories, err := store.GetRecentMemories(1000)
	if err != nil {
		return nil, err
	}

	queryLower := strings.ToLower(query)
	queryWords := strings.Fields(queryLower)
	var results []scoredResult

	for _, mem := range allMemories {
		plain, err := mem.Decrypt(decryptFn)
		if err != nil {
			continue
		}

		score := scoreMatch(plain.Content, queryLower, queryWords)
		if score > 0 {
			results = append(results, scoredResult{memory: mem, plain: plain, score: score})
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].score > results[j].score
	})

	if len(results) > limit {
		results = results[:limit]
	}

	return results, nil
}

func scoreMatch(content string, queryLower string, queryWords []string) float64 {
	contentLower := strings.ToLower(content)

	// Exact match
	if contentLower == queryLower {
		return 1.0
	}

	// Case-insensitive contains
	if strings.Contains(contentLower, queryLower) {
		return 0.8
	}

	// Word-level matching
	if len(queryWords) > 0 {
		matchCount := 0
		contentWords := strings.Fields(contentLower)
		contentSet := make(map[string]bool, len(contentWords))
		for _, w := range contentWords {
			contentSet[w] = true
		}

		for _, qw := range queryWords {
			if contentSet[qw] {
				matchCount++
			}
		}

		ratio := float64(matchCount) / float64(len(queryWords))
		if ratio > 0.5 {
			return 0.5
		}

		// Partial word match
		for _, qw := range queryWords {
			for _, cw := range contentWords {
				if strings.Contains(cw, qw) || strings.Contains(qw, cw) {
					return 0.3
				}
			}
		}
	}

	return 0
}

func outputResults(results []scoredResult, format string) error {
	if format == "json" {
		out := jsonOutput{Results: make([]jsonResult, 0, len(results))}
		for _, r := range results {
			out.Results = append(out.Results, jsonResult{
				ID:        r.memory.ID,
				Content:   r.plain.Content,
				Type:      string(r.memory.Type),
				Score:     r.score,
				Timestamp: r.memory.Timestamp,
				Links:     r.memory.Links,
				Metadata:  r.memory.Metadata,
			})
		}
		data, err := json.Marshal(out)
		if err != nil {
			return fmt.Errorf("failed to marshal JSON output: %w", err)
		}
		fmt.Println(string(data))
		return nil
	}

	fmt.Printf("Found %d memories:\n\n", len(results))

	for i, r := range results {
		preview := r.plain.Content
		if len(preview) > 100 {
			preview = preview[:100] + "..."
		}

		if r.score > 0 {
			fmt.Printf("%d. [%.2f] %s\n", i+1, r.score, preview)
		} else {
			fmt.Printf("%d. %s\n", i+1, preview)
		}

		ts := time.Unix(0, r.memory.Timestamp)
		fmt.Printf("   ID: %s... | Type: %s | Links: %d | %s\n",
			r.memory.ID[:16], r.memory.Type, len(r.memory.Links), ts.Format("2006-01-02 15:04:05"))
		fmt.Println()
	}

	return nil
}
