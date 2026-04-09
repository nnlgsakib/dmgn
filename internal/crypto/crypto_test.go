package crypto

import (
	"bytes"
	"testing"
)

func TestEncryptDecryptRoundTrip(t *testing.T) {
	masterKey := make([]byte, 32)
	engine, err := NewEngine(masterKey)
	if err != nil {
		t.Fatalf("NewEngine failed: %v", err)
	}

	testCases := []struct {
		name string
		data []byte
	}{
		{"empty", []byte{}},
		{"one byte", []byte{0x42}},
		{"1KB", make([]byte, 1024)},
		{"64KB", make([]byte, 65536)},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ciphertext, err := engine.Encrypt(tc.data)
			if err != nil {
				t.Fatalf("Encrypt failed: %v", err)
			}

			plaintext, err := engine.Decrypt(ciphertext)
			if err != nil {
				t.Fatalf("Decrypt failed: %v", err)
			}

			if !bytes.Equal(plaintext, tc.data) {
				t.Errorf("Round-trip failed: got %d bytes, want %d bytes", len(plaintext), len(tc.data))
			}
		})
	}
}

func TestEncryptProducesUniqueOutput(t *testing.T) {
	masterKey := make([]byte, 32)
	engine, err := NewEngine(masterKey)
	if err != nil {
		t.Fatalf("NewEngine failed: %v", err)
	}

	plaintext := []byte("same plaintext content")

	ct1, err := engine.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("First encrypt failed: %v", err)
	}

	ct2, err := engine.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Second encrypt failed: %v", err)
	}

	if bytes.Equal(ct1, ct2) {
		t.Error("Same plaintext encrypted twice should produce different ciphertext (unique per-memory keys)")
	}
}

func TestDecryptWrongMasterKey(t *testing.T) {
	masterKey1 := make([]byte, 32)
	masterKey2 := make([]byte, 32)
	masterKey2[0] = 0xFF

	engine1, err := NewEngine(masterKey1)
	if err != nil {
		t.Fatalf("NewEngine failed: %v", err)
	}

	engine2, err := NewEngine(masterKey2)
	if err != nil {
		t.Fatalf("NewEngine failed: %v", err)
	}

	ciphertext, err := engine1.Encrypt([]byte("secret data"))
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	_, err = engine2.Decrypt(ciphertext)
	if err == nil {
		t.Error("Decrypt with wrong master key should fail")
	}
}

func TestDecryptTamperedCiphertext(t *testing.T) {
	masterKey := make([]byte, 32)
	engine, err := NewEngine(masterKey)
	if err != nil {
		t.Fatalf("NewEngine failed: %v", err)
	}

	ciphertext, err := engine.Encrypt([]byte("important data"))
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	// Tamper with a byte in the middle of the ciphertext
	tampered := make([]byte, len(ciphertext))
	copy(tampered, ciphertext)
	tampered[len(tampered)/2] ^= 0xFF

	_, err = engine.Decrypt(tampered)
	if err == nil {
		t.Error("Decrypt of tampered ciphertext should fail authentication")
	}
}

func TestDecryptTruncatedCiphertext(t *testing.T) {
	masterKey := make([]byte, 32)
	engine, err := NewEngine(masterKey)
	if err != nil {
		t.Fatalf("NewEngine failed: %v", err)
	}

	// Too short to even have length prefix
	_, err = engine.Decrypt([]byte{0x00})
	if err == nil {
		t.Error("Decrypt of 1-byte ciphertext should return error")
	}

	// Has length prefix claiming more data than available
	_, err = engine.Decrypt([]byte{0x00, 0xFF, 0x01})
	if err == nil {
		t.Error("Decrypt of truncated ciphertext should return error")
	}
}

func TestDeriveKeyFromSeed(t *testing.T) {
	seed := []byte("test-seed-32-bytes-long-padding!!")

	key1 := DeriveKeyFromSeed(seed, "purpose-a", 0)
	key2 := DeriveKeyFromSeed(seed, "purpose-a", 0)
	key3 := DeriveKeyFromSeed(seed, "purpose-b", 0)

	if !bytes.Equal(key1, key2) {
		t.Error("Same inputs should produce same output")
	}

	if bytes.Equal(key1, key3) {
		t.Error("Different purpose should produce different output")
	}

	if len(key1) != 32 {
		t.Errorf("Expected 32-byte key, got %d", len(key1))
	}
}

func TestGenerateRandomKey(t *testing.T) {
	key1, err := GenerateRandomKey()
	if err != nil {
		t.Fatalf("GenerateRandomKey failed: %v", err)
	}

	key2, err := GenerateRandomKey()
	if err != nil {
		t.Fatalf("GenerateRandomKey failed: %v", err)
	}

	if len(key1) != 32 {
		t.Errorf("Expected 32-byte key, got %d", len(key1))
	}

	if bytes.Equal(key1, key2) {
		t.Error("Two random keys should differ")
	}
}
