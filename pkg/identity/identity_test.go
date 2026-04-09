package identity

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGenerate(t *testing.T) {
	tmpDir := t.TempDir()
	passphrase := "test-passphrase-123"

	id, err := Generate(passphrase, tmpDir)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	if id.ID == "" {
		t.Error("ID should not be empty")
	}

	if id.PublicKey == nil {
		t.Error("PublicKey should not be nil")
	}

	if id.PrivateKey == nil {
		t.Error("PrivateKey should not be nil")
	}

	if !Exists(tmpDir) {
		t.Error("Key file should exist after generation")
	}
}

func TestLoad(t *testing.T) {
	tmpDir := t.TempDir()
	passphrase := "test-passphrase-123"

	generated, err := Generate(passphrase, tmpDir)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	loaded, err := Load(passphrase, tmpDir)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if loaded.ID != generated.ID {
		t.Errorf("Loaded ID %s != generated ID %s", loaded.ID, generated.ID)
	}
}

func TestLoadWrongPassphrase(t *testing.T) {
	tmpDir := t.TempDir()
	passphrase := "test-passphrase-123"

	_, err := Generate(passphrase, tmpDir)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	_, err = Load("wrong-passphrase", tmpDir)
	if err == nil {
		t.Error("Load should fail with wrong passphrase")
	}
}

func TestExportImport(t *testing.T) {
	tmpDir := t.TempDir()
	importDir := filepath.Join(t.TempDir(), "import")
	passphrase := "test-passphrase-123"

	generated, err := Generate(passphrase, tmpDir)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	keyData, err := Export(tmpDir)
	if err != nil {
		t.Fatalf("Export failed: %v", err)
	}

	if len(keyData) == 0 {
		t.Error("Exported key data should not be empty")
	}

	if err := os.MkdirAll(importDir, 0755); err != nil {
		t.Fatalf("Failed to create import dir: %v", err)
	}

	if err := Import(keyData, importDir); err != nil {
		t.Fatalf("Import failed: %v", err)
	}

	if !Exists(importDir) {
		t.Error("Key file should exist after import")
	}

	loaded, err := Load(passphrase, importDir)
	if err != nil {
		t.Fatalf("Load after import failed: %v", err)
	}

	if loaded.ID != generated.ID {
		t.Errorf("Imported ID %s != original ID %s", loaded.ID, generated.ID)
	}
}

func TestSignVerify(t *testing.T) {
	tmpDir := t.TempDir()
	passphrase := "test-passphrase-123"

	id, err := Generate(passphrase, tmpDir)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	data := []byte("test message")
	signature := id.Sign(data)

	if len(signature) == 0 {
		t.Error("Signature should not be empty")
	}

	if !id.Verify(data, signature) {
		t.Error("Signature verification should succeed")
	}

	invalidData := []byte("different message")
	if id.Verify(invalidData, signature) {
		t.Error("Signature verification should fail for different data")
	}
}

func TestDeriveKey(t *testing.T) {
	tmpDir := t.TempDir()
	passphrase := "test-passphrase-123"

	id, err := Generate(passphrase, tmpDir)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	key1 := id.DeriveKey("test-purpose", 0)
	key2 := id.DeriveKey("test-purpose", 0)
	key3 := id.DeriveKey("test-purpose", 1)

	if len(key1) != 32 {
		t.Errorf("Derived key length should be 32, got %d", len(key1))
	}

	for i := range key1 {
		if key1[i] != key2[i] {
			t.Error("Same purpose and index should produce same key")
			break
		}
	}

	different := false
	for i := range key1 {
		if key1[i] != key3[i] {
			different = true
			break
		}
	}

	if !different {
		t.Error("Different index should produce different key")
	}
}

func TestExists(t *testing.T) {
	tmpDir := t.TempDir()

	if Exists(tmpDir) {
		t.Error("Exists should return false for non-existent identity")
	}

	_, err := Generate("test-passphrase-123", tmpDir)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	if !Exists(tmpDir) {
		t.Error("Exists should return true after identity generation")
	}
}
