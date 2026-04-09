package backup

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/dgraph-io/badger/v4"
)

// Manifest describes a backup's contents.
type Manifest struct {
	Version      string `json:"version"`
	NodeID       string `json:"node_id"`
	Timestamp    string `json:"timestamp"`
	DMGNVersion  string `json:"dmgn_version"`
	MemoryCount  int64  `json:"memory_count,omitempty"`
}

// ExportConfig holds parameters for creating a backup.
type ExportConfig struct {
	OutputPath     string
	DB             *badger.DB
	VecIndexPath   string
	NodeID         string
	MemoryCount    int64
}

// RestoreConfig holds parameters for restoring a backup.
type RestoreConfig struct {
	InputPath string
	DataDir   string
}

// RestoreResult holds info about a completed restore.
type RestoreResult struct {
	NodeID    string
	Timestamp time.Time
	Version   string
}

// Export creates a .dmgn-backup tar.gz file containing BadgerDB backup,
// vector index file, and a manifest.
func Export(cfg ExportConfig) error {
	outFile, err := os.Create(cfg.OutputPath)
	if err != nil {
		return fmt.Errorf("failed to create backup file: %w", err)
	}
	defer outFile.Close()

	gw := gzip.NewWriter(outFile)
	defer gw.Close()

	tw := tar.NewWriter(gw)
	defer tw.Close()

	// 1. Write manifest
	manifest := Manifest{
		Version:     "1",
		NodeID:      cfg.NodeID,
		Timestamp:   time.Now().UTC().Format(time.RFC3339),
		DMGNVersion: "0.1.0",
		MemoryCount: cfg.MemoryCount,
	}
	manifestData, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal manifest: %w", err)
	}
	if err := writeToTar(tw, "manifest.json", manifestData); err != nil {
		return fmt.Errorf("failed to write manifest: %w", err)
	}

	// 2. Write BadgerDB backup stream
	if cfg.DB != nil {
		pr, pw := io.Pipe()
		errCh := make(chan error, 1)
		go func() {
			_, err := cfg.DB.Backup(pw, 0)
			pw.CloseWithError(err)
			errCh <- err
		}()

		data, err := io.ReadAll(pr)
		if err != nil {
			return fmt.Errorf("failed to read badger backup: %w", err)
		}
		if backupErr := <-errCh; backupErr != nil {
			return fmt.Errorf("badger backup failed: %w", backupErr)
		}
		if err := writeToTar(tw, "badger.backup", data); err != nil {
			return fmt.Errorf("failed to write badger backup: %w", err)
		}
	}

	// 3. Write vector index file (if exists)
	if cfg.VecIndexPath != "" {
		vecData, err := os.ReadFile(cfg.VecIndexPath)
		if err == nil && len(vecData) > 0 {
			if err := writeToTar(tw, "vector-index.enc", vecData); err != nil {
				return fmt.Errorf("failed to write vector index: %w", err)
			}
		}
	}

	return nil
}

// Restore extracts a .dmgn-backup tar.gz and restores BadgerDB and vector index.
func Restore(cfg RestoreConfig) (*RestoreResult, error) {
	inFile, err := os.Open(cfg.InputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open backup file: %w", err)
	}
	defer inFile.Close()

	gr, err := gzip.NewReader(inFile)
	if err != nil {
		return nil, fmt.Errorf("invalid backup file (not gzip): %w", err)
	}
	defer gr.Close()

	tr := tar.NewReader(gr)

	var manifest Manifest
	var badgerData []byte
	var vecIndexData []byte

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("error reading backup: %w", err)
		}

		data, err := io.ReadAll(tr)
		if err != nil {
			return nil, fmt.Errorf("error reading %s: %w", header.Name, err)
		}

		switch header.Name {
		case "manifest.json":
			if err := json.Unmarshal(data, &manifest); err != nil {
				return nil, fmt.Errorf("invalid manifest: %w", err)
			}
		case "badger.backup":
			badgerData = data
		case "vector-index.enc":
			vecIndexData = data
		}
	}

	if manifest.Version == "" {
		return nil, fmt.Errorf("backup missing manifest")
	}

	// Restore BadgerDB
	if len(badgerData) > 0 {
		storageDir := filepath.Join(cfg.DataDir, "storage")
		if err := os.MkdirAll(storageDir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create storage dir: %w", err)
		}

		opts := badger.DefaultOptions(storageDir).WithLogger(nil)
		db, err := badger.Open(opts)
		if err != nil {
			return nil, fmt.Errorf("failed to open new DB: %w", err)
		}

		pr, pw := io.Pipe()
		go func() {
			pw.Write(badgerData)
			pw.Close()
		}()

		if err := db.Load(pr, 256); err != nil {
			db.Close()
			return nil, fmt.Errorf("failed to load badger backup: %w", err)
		}
		db.Close()
	}

	// Restore vector index
	if len(vecIndexData) > 0 {
		vecPath := filepath.Join(cfg.DataDir, "vector-index.enc")
		if err := os.WriteFile(vecPath, vecIndexData, 0644); err != nil {
			return nil, fmt.Errorf("failed to write vector index: %w", err)
		}
	}

	ts, _ := time.Parse(time.RFC3339, manifest.Timestamp)
	return &RestoreResult{
		NodeID:    manifest.NodeID,
		Timestamp: ts,
		Version:   manifest.DMGNVersion,
	}, nil
}

func writeToTar(tw *tar.Writer, name string, data []byte) error {
	header := &tar.Header{
		Name:    name,
		Size:    int64(len(data)),
		Mode:    0644,
		ModTime: time.Now(),
	}
	if err := tw.WriteHeader(header); err != nil {
		return err
	}
	_, err := tw.Write(data)
	return err
}
