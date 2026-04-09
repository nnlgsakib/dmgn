package backup

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/dgraph-io/badger/v4"
)

func openTestDB(t *testing.T, dir string) *badger.DB {
	t.Helper()
	opts := badger.DefaultOptions(dir).WithLogger(nil)
	db, err := badger.Open(opts)
	if err != nil {
		t.Fatalf("failed to open test DB: %v", err)
	}
	return db
}

func addTestData(t *testing.T, db *badger.DB, count int) {
	t.Helper()
	err := db.Update(func(txn *badger.Txn) error {
		for i := 0; i < count; i++ {
			key := []byte(fmt.Sprintf("m:test-memory-%d", i))
			val := []byte(fmt.Sprintf(`{"id":"test-memory-%d","timestamp":%d}`, i, i*1000))
			if err := txn.Set(key, val); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("failed to add test data: %v", err)
	}
}

func TestExportCreatesFile(t *testing.T) {
	tmpDir := t.TempDir()
	dbDir := filepath.Join(tmpDir, "db")
	db := openTestDB(t, dbDir)
	addTestData(t, db, 5)
	defer db.Close()

	backupPath := filepath.Join(tmpDir, "test.dmgn-backup")
	err := Export(ExportConfig{
		OutputPath: backupPath,
		DB:         db,
		NodeID:     "test-node",
	})
	if err != nil {
		t.Fatalf("export failed: %v", err)
	}

	info, err := os.Stat(backupPath)
	if err != nil {
		t.Fatalf("backup file not created: %v", err)
	}
	if info.Size() == 0 {
		t.Fatal("backup file is empty")
	}
}

func TestExportContainsManifest(t *testing.T) {
	tmpDir := t.TempDir()
	dbDir := filepath.Join(tmpDir, "db")
	db := openTestDB(t, dbDir)
	defer db.Close()

	backupPath := filepath.Join(tmpDir, "test.dmgn-backup")
	err := Export(ExportConfig{
		OutputPath:  backupPath,
		DB:          db,
		NodeID:      "manifest-test",
		MemoryCount: 42,
	})
	if err != nil {
		t.Fatalf("export failed: %v", err)
	}

	// Verify by restoring and checking manifest data
	restoreDir := filepath.Join(tmpDir, "restored")
	result, err := Restore(RestoreConfig{
		InputPath: backupPath,
		DataDir:   restoreDir,
	})
	if err != nil {
		t.Fatalf("restore failed: %v", err)
	}
	if result.NodeID != "manifest-test" {
		t.Fatalf("expected node_id=manifest-test, got %s", result.NodeID)
	}
	if result.Version != "0.1.0" {
		t.Fatalf("expected version=0.1.0, got %s", result.Version)
	}
}

func TestBackupRestoreRoundTrip(t *testing.T) {
	tmpDir := t.TempDir()

	// Create source DB with data
	srcDir := filepath.Join(tmpDir, "src")
	db := openTestDB(t, srcDir)
	addTestData(t, db, 10)

	// Export backup
	backupPath := filepath.Join(tmpDir, "roundtrip.dmgn-backup")
	err := Export(ExportConfig{
		OutputPath:  backupPath,
		DB:          db,
		NodeID:      "roundtrip",
		MemoryCount: 10,
	})
	db.Close()
	if err != nil {
		t.Fatalf("export failed: %v", err)
	}

	// Restore to new location
	restoreDir := filepath.Join(tmpDir, "restored")
	result, err := Restore(RestoreConfig{
		InputPath: backupPath,
		DataDir:   restoreDir,
	})
	if err != nil {
		t.Fatalf("restore failed: %v", err)
	}
	if result.NodeID != "roundtrip" {
		t.Fatalf("expected roundtrip, got %s", result.NodeID)
	}

	// Verify restored DB has data
	restoredDB := openTestDB(t, filepath.Join(restoreDir, "storage"))
	defer restoredDB.Close()

	count := 0
	restoredDB.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = false
		it := txn.NewIterator(opts)
		defer it.Close()
		prefix := []byte("m:")
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			count++
		}
		return nil
	})

	if count != 10 {
		t.Fatalf("expected 10 memories in restored DB, got %d", count)
	}
}

func TestRestoreInvalidFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a non-gzip file
	invalidPath := filepath.Join(tmpDir, "invalid.dmgn-backup")
	os.WriteFile(invalidPath, []byte("not a backup"), 0644)

	_, err := Restore(RestoreConfig{
		InputPath: invalidPath,
		DataDir:   filepath.Join(tmpDir, "restored"),
	})
	if err == nil {
		t.Fatal("expected error for invalid backup file")
	}
}

func TestExportNoVecIndex(t *testing.T) {
	tmpDir := t.TempDir()
	dbDir := filepath.Join(tmpDir, "db")
	db := openTestDB(t, dbDir)
	defer db.Close()

	backupPath := filepath.Join(tmpDir, "no-vec.dmgn-backup")
	err := Export(ExportConfig{
		OutputPath:   backupPath,
		DB:           db,
		VecIndexPath: "/nonexistent/path",
		NodeID:       "test",
	})
	if err != nil {
		t.Fatalf("export should succeed even without vec index: %v", err)
	}
}
