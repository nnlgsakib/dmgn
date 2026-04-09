package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/dmgn/dmgn/internal/config"
	"github.com/dmgn/dmgn/pkg/backup"
	"github.com/dmgn/dmgn/pkg/storage"
)

// BackupCmd returns the cobra command for `dmgn backup`.
func BackupCmd() *cobra.Command {
	var dataDir string

	cmd := &cobra.Command{
		Use:   "backup [output-file]",
		Short: "Export an encrypted backup of the DMGN node",
		Long:  `Creates a tar.gz backup containing encrypted BadgerDB data, vector index, and manifest.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(dataDir)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			store, err := storage.New(storage.Options{
				DataDir: cfg.StorageDir(),
			})
			if err != nil {
				return fmt.Errorf("failed to open storage: %w", err)
			}
			defer store.Close()

			stats, err := store.GetStats()
			if err != nil {
				return fmt.Errorf("failed to get stats: %w", err)
			}

			outputPath := ""
			if len(args) > 0 {
				outputPath = args[0]
			} else {
				ts := time.Now().Format("20060102-150405")
				outputPath = fmt.Sprintf("dmgn-backup-%s.dmgn-backup", ts)
			}

			err = backup.Export(backup.ExportConfig{
				OutputPath:   outputPath,
				DB:           store.DB(),
				VecIndexPath: cfg.VectorIndexPath(),
				NodeID:       "local",
				MemoryCount:  stats["memory_count"],
			})
			if err != nil {
				return fmt.Errorf("backup failed: %w", err)
			}

			info, _ := os.Stat(outputPath)
			size := "unknown"
			if info != nil {
				size = formatSize(info.Size())
			}
			fmt.Printf("✓ Backup saved: %s (%s)\n", outputPath, size)
			return nil
		},
	}

	cmd.Flags().StringVar(&dataDir, "data-dir", "", "Data directory")
	return cmd
}

// RestoreCmd returns the cobra command for `dmgn restore`.
func RestoreCmd() *cobra.Command {
	var dataDir string
	var force bool

	cmd := &cobra.Command{
		Use:   "restore <backup-file>",
		Short: "Restore DMGN node from an encrypted backup",
		Long:  `Restores BadgerDB data and vector index from a .dmgn-backup file.`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dataDir == "" {
				dataDir = config.DefaultDataDir()
			}

			storageDir := filepath.Join(dataDir, "storage")
			if _, err := os.Stat(storageDir); err == nil && !force {
				return fmt.Errorf("data directory already has data at %s. Use --force to overwrite", storageDir)
			}

			result, err := backup.Restore(backup.RestoreConfig{
				InputPath: args[0],
				DataDir:   dataDir,
			})
			if err != nil {
				return fmt.Errorf("restore failed: %w", err)
			}

			fmt.Printf("✓ Restored: node=%s, from %s (DMGN %s)\n",
				result.NodeID, result.Timestamp.Format(time.RFC3339), result.Version)
			return nil
		},
	}

	cmd.Flags().StringVar(&dataDir, "data-dir", "", "Data directory")
	cmd.Flags().BoolVar(&force, "force", false, "Overwrite existing data")
	return cmd
}

func formatSize(bytes int64) string {
	if bytes < 1024 {
		return fmt.Sprintf("%d B", bytes)
	}
	if bytes < 1024*1024 {
		return fmt.Sprintf("%.1f KB", float64(bytes)/1024)
	}
	return fmt.Sprintf("%.1f MB", float64(bytes)/(1024*1024))
}
