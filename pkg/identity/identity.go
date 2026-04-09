package identity

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base58"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/chacha20poly1305"
)

const (
	KeyFileName    = "identity.key"
	Argon2Time     = 3
	Argon2Memory   = 64 * 1024
	Argon2Threads  = 4
	Argon2KeyLen   = 32
	KeyVersion     = 1
)

type EncryptedKey struct {
	Version     int    `json:"version"`
	PublicKey   string `json:"public_key"`
	Salt        []byte `json:"salt"`
	Nonce       []byte `json:"nonce"`
	Ciphertext  []byte `json:"ciphertext"`
}

type Identity struct {
	PublicKey  ed25519.PublicKey
	PrivateKey ed25519.PrivateKey
	ID         string
}

func Generate(passphrase string, dataDir string) (*Identity, error) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate keypair: %w", err)
	}

	id := base58.Encode(publicKey)
	
	if err := saveEncryptedKey(privateKey, publicKey, passphrase, dataDir); err != nil {
		return nil, fmt.Errorf("failed to save encrypted key: %w", err)
	}

	return &Identity{
		PublicKey:  publicKey,
		PrivateKey: privateKey,
		ID:         id,
	}, nil
}

func Load(passphrase string, dataDir string) (*Identity, error) {
	keyPath := filepath.Join(dataDir, KeyFileName)
	
	data, err := os.ReadFile(keyPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("identity not found, run 'dmgn init' first")
		}
		return nil, fmt.Errorf("failed to read key file: %w", err)
	}

	var encKey EncryptedKey
	if err := json.Unmarshal(data, &encKey); err != nil {
		return nil, fmt.Errorf("failed to parse key file: %w", err)
	}

	if encKey.Version != KeyVersion {
		return nil, fmt.Errorf("unsupported key version: %d", encKey.Version)
	}

	masterKey := argon2.IDKey([]byte(passphrase), encKey.Salt, Argon2Time, Argon2Memory, Argon2Threads, Argon2KeyLen)

	aead, err := chacha20poly1305.New(masterKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	privateKeyBytes, err := aead.Open(nil, encKey.Nonce, encKey.Ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("invalid passphrase: %w", err)
	}

	privateKey := ed25519.PrivateKey(privateKeyBytes)
	publicKey := privateKey.Public().(ed25519.PublicKey)

	expectedID := base58.Encode(publicKey)
	if encKey.PublicKey != expectedID {
		return nil, fmt.Errorf("key verification failed")
	}

	return &Identity{
		PublicKey:  publicKey,
		PrivateKey: privateKey,
		ID:         expectedID,
	}, nil
}

func Export(dataDir string) ([]byte, error) {
	keyPath := filepath.Join(dataDir, KeyFileName)
	return os.ReadFile(keyPath)
}

func Import(data []byte, dataDir string) error {
	keyPath := filepath.Join(dataDir, KeyFileName)
	
	if err := os.MkdirAll(dataDir, 0700); err != nil {
		return fmt.Errorf("failed to create data directory: %w", err)
	}

	var encKey EncryptedKey
	if err := json.Unmarshal(data, &encKey); err != nil {
		return fmt.Errorf("invalid key file format: %w", err)
	}

	if encKey.Version != KeyVersion {
		return fmt.Errorf("unsupported key version: %d", encKey.Version)
	}

	return os.WriteFile(keyPath, data, 0600)
}

func Exists(dataDir string) bool {
	keyPath := filepath.Join(dataDir, KeyFileName)
	_, err := os.Stat(keyPath)
	return err == nil
}

func saveEncryptedKey(privateKey ed25519.PrivateKey, publicKey ed25519.PublicKey, passphrase string, dataDir string) error {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return fmt.Errorf("failed to generate salt: %w", err)
	}

	masterKey := argon2.IDKey([]byte(passphrase), salt, Argon2Time, Argon2Memory, Argon2Threads, Argon2KeyLen)

	aead, err := chacha20poly1305.New(masterKey)
	if err != nil {
		return fmt.Errorf("failed to create cipher: %w", err)
	}

	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return fmt.Errorf("failed to generate nonce: %w", err)
	}

	ciphertext := aead.Seal(nil, nonce, []byte(privateKey), nil)

	encKey := EncryptedKey{
		Version:    KeyVersion,
		PublicKey:  base58.Encode(publicKey),
		Salt:       salt,
		Nonce:      nonce,
		Ciphertext: ciphertext,
	}

	data, err := json.Marshal(encKey)
	if err != nil {
		return fmt.Errorf("failed to marshal key: %w", err)
	}

	if err := os.MkdirAll(dataDir, 0700); err != nil {
		return fmt.Errorf("failed to create data directory: %w", err)
	}

	keyPath := filepath.Join(dataDir, KeyFileName)
	return os.WriteFile(keyPath, data, 0600)
}

func (i *Identity) Sign(data []byte) []byte {
	return ed25519.Sign(i.PrivateKey, data)
}

func (i *Identity) Verify(data []byte, signature []byte) bool {
	return ed25519.Verify(i.PublicKey, data, signature)
}

func (i *Identity) DeriveKey(purpose string, index uint32) []byte {
	seed := sha256.Sum256(append(i.PrivateKey.Seed(), []byte(purpose)...))
	for i := uint32(0); i < index; i++ {
		seed = sha256.Sum256(seed[:])
	}
	return seed[:]
}
