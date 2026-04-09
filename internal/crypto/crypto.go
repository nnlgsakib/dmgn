package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"io"
)

type Engine struct {
	masterKey []byte
}

func NewEngine(masterKey []byte) (*Engine, error) {
	if len(masterKey) != 32 {
		return nil, fmt.Errorf("master key must be 32 bytes, got %d", len(masterKey))
	}

	return &Engine{
		masterKey: masterKey,
	}, nil
}

func (e *Engine) Encrypt(plaintext []byte) ([]byte, error) {
	perMemoryKey := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, perMemoryKey); err != nil {
		return nil, fmt.Errorf("failed to generate per-memory key: %w", err)
	}

	encryptedKey, err := e.encryptWithMaster(perMemoryKey)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt per-memory key: %w", err)
	}

	encryptedPayload, err := e.encryptWithKey(plaintext, perMemoryKey)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt payload: %w", err)
	}

	result := make([]byte, 0, len(encryptedKey)+len(encryptedPayload))
	result = append(result, encryptedKey...)
	result = append(result, encryptedPayload...)

	return result, nil
}

func (e *Engine) Decrypt(ciphertext []byte) ([]byte, error) {
	if len(ciphertext) < 60 {
		return nil, fmt.Errorf("ciphertext too short")
	}

	encryptedKeyLen := 28
	encryptedKey := ciphertext[:encryptedKeyLen]
	encryptedPayload := ciphertext[encryptedKeyLen:]

	perMemoryKey, err := e.decryptWithMaster(encryptedKey)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt per-memory key: %w", err)
	}

	plaintext, err := e.decryptWithKey(encryptedPayload, perMemoryKey)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt payload: %w", err)
	}

	return plaintext, nil
}

func (e *Engine) encryptWithMaster(plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(e.masterKey)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return ciphertext, nil
}

func (e *Engine) decryptWithMaster(ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(e.masterKey)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	return gcm.Open(nil, nonce, ciphertext, nil)
}

func (e *Engine) encryptWithKey(plaintext []byte, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return ciphertext, nil
}

func (e *Engine) decryptWithKey(ciphertext []byte, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	return gcm.Open(nil, nonce, ciphertext, nil)
}

func DeriveKeyFromSeed(seed []byte, purpose string, index uint32) []byte {
	h := sha256.New()
	h.Write(seed)
	h.Write([]byte(purpose))
	
	b := make([]byte, 4)
	b[0] = byte(index >> 24)
	b[1] = byte(index >> 16)
	b[2] = byte(index >> 8)
	b[3] = byte(index)
	h.Write(b)
	
	return h.Sum(nil)
}

func GenerateRandomKey() ([]byte, error) {
	key := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, err
	}
	return key, nil
}

func Hash(data []byte) []byte {
	h := sha256.Sum256(data)
	return h[:]
}
