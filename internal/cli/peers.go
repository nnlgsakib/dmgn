package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/spf13/cobra"

	"github.com/dmgn/dmgn/internal/api"
	"github.com/dmgn/dmgn/internal/config"
	"github.com/dmgn/dmgn/pkg/identity"
)

func PeersCmd() *cobra.Command {
	var dataDir string
	var apiAddr string

	cmd := &cobra.Command{
		Use:   "peers",
		Short: "List connected peers",
		Long:  `List peers currently connected to this DMGN node via the local API.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(dataDir)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			if apiAddr == "" {
				apiAddr = fmt.Sprintf("http://localhost:%d", cfg.APIPort)
			}

			passphrase, err := promptPassphraseOnce()
			if err != nil {
				return err
			}

			id, err := identity.Load(passphrase, cfg.IdentityDir())
			if err != nil {
				return fmt.Errorf("failed to load identity: %w", err)
			}

			apiKeyBytes, err := id.DeriveKey(api.APIKeyPurpose, 32)
			if err != nil {
				return fmt.Errorf("failed to derive API key: %w", err)
			}
			apiKey := api.DeriveAPIKey(apiKeyBytes)

			req, err := http.NewRequest("GET", apiAddr+"/peers", nil)
			if err != nil {
				return fmt.Errorf("failed to create request: %w", err)
			}
			req.Header.Set("Authorization", "Bearer "+apiKey)

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				return fmt.Errorf("failed to connect to API at %s: %w\nIs the node running? Start with: dmgn start", apiAddr, err)
			}
			defer resp.Body.Close()

			body, err := io.ReadAll(resp.Body)
			if err != nil {
				return fmt.Errorf("failed to read response: %w", err)
			}

			if resp.StatusCode != http.StatusOK {
				return fmt.Errorf("API error (%d): %s", resp.StatusCode, string(body))
			}

			var result struct {
				Peers []struct {
					ID      string   `json:"id"`
					Addrs   []string `json:"addrs"`
					Latency string   `json:"latency,omitempty"`
				} `json:"peers"`
				Count int `json:"count"`
			}
			if err := json.Unmarshal(body, &result); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}

			fmt.Printf("Connected Peers (%d)\n", result.Count)
			fmt.Println("==================")
			fmt.Println()

			if result.Count == 0 {
				fmt.Println("No peers connected.")
				return nil
			}

			for _, p := range result.Peers {
				fmt.Printf("  Peer: %s\n", p.ID)
				for _, a := range p.Addrs {
					fmt.Printf("    Addr: %s\n", a)
				}
				if p.Latency != "" {
					fmt.Printf("    Latency: %s\n", p.Latency)
				}
				fmt.Println()
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&dataDir, "data-dir", "", "Data directory")
	cmd.Flags().StringVar(&apiAddr, "api", "", "API server address (default: http://localhost:{api_port})")

	return cmd
}
