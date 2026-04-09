package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
	"golang.org/x/term"
	
	"github.com/dmgn/dmgn/internal/config"
	"github.com/dmgn/dmgn/pkg/identity"
)

func InitCmd() *cobra.Command {
	var dataDir string

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize a new DMGN node",
		Long:  `Initialize creates a new identity and storage for your DMGN node.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dataDir == "" {
				dataDir = config.DefaultDataDir()
			}

			if identity.Exists(dataDir) {
				return fmt.Errorf("identity already exists at %s", dataDir)
			}

			fmt.Println("Creating new DMGN identity...")
			fmt.Println()

			passphrase, err := promptPassphrase()
			if err != nil {
				return err
			}

			id, err := identity.Generate(passphrase, dataDir)
			if err != nil {
				return fmt.Errorf("failed to create identity: %w", err)
			}

			cfg := config.DefaultConfig()
			cfg.DataDir = dataDir
			if err := cfg.Save(); err != nil {
				return fmt.Errorf("failed to save config: %w", err)
			}

			fmt.Println()
			fmt.Println("✓ Identity created successfully!")
			fmt.Printf("  ID: %s\n", id.ID)
			fmt.Printf("  Data directory: %s\n", dataDir)
			fmt.Println()
			fmt.Println("IMPORTANT: Backup your identity with 'dmgn export' and store it safely.")
			fmt.Println("Your passphrase is required to recover this identity.")

			return nil
		},
	}

	cmd.Flags().StringVar(&dataDir, "data-dir", "", "Data directory (default: platform-specific)")

	return cmd
}

func promptPassphrase() (string, error) {
	reader := bufio.NewReader(os.Stdin)

	for {
		fmt.Print("Enter passphrase: ")
		pass1, err := term.ReadPassword(int(syscall.Stdin))
		if err != nil {
			return "", fmt.Errorf("failed to read passphrase: %w", err)
		}
		fmt.Println()

		if len(pass1) < 8 {
			fmt.Println("Passphrase must be at least 8 characters.")
			continue
		}

		fmt.Print("Confirm passphrase: ")
		pass2, err := term.ReadPassword(int(syscall.Stdin))
		if err != nil {
			return "", fmt.Errorf("failed to read confirmation: %w", err)
		}
		fmt.Println()

		if string(pass1) != string(pass2) {
			fmt.Println("Passphrases do not match. Please try again.")
			continue
		}

		return string(pass1), nil
	}
}

func promptPassphraseOnce() (string, error) {
	fmt.Print("Enter passphrase: ")
	pass, err := term.ReadPassword(int(syscall.Stdin))
	if err != nil {
		return "", fmt.Errorf("failed to read passphrase: %w", err)
	}
	fmt.Println()
	return string(pass), nil
}

func confirm(prompt string) bool {
	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("%s [y/N]: ", prompt)
	response, _ := reader.ReadString('\n')
	response = strings.TrimSpace(strings.ToLower(response))
	return response == "y" || response == "yes"
}
