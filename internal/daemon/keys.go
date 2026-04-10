package daemon

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/mr-tron/base58"

	"github.com/nnlgsakib/dmgn/pkg/identity"
	"github.com/nnlgsakib/dmgn/pkg/network"
)

// DerivedKeys holds all pre-derived key material needed by the daemon.
// This allows the parent process to prompt for the passphrase, derive keys,
// and pass them to the background daemon without terminal access.
type DerivedKeys struct {
	MasterKey      []byte             // For memory encryption (AES-GCM)
	LibP2PKey      crypto.PrivKey     // Pre-derived libp2p ed25519 key
	VectorIndexKey []byte             // For vector index encryption
	IdentityKey    ed25519.PrivateKey // Raw identity key for reconstructing Identity
}

// encodedKeys is the JSON-serializable representation of DerivedKeys.
type encodedKeys struct {
	MasterKey      string `json:"master_key"`
	LibP2PKeyBytes string `json:"libp2p_key"`
	VectorIndexKey string `json:"vector_index_key"`
	IdentityKey    string `json:"identity_key"`
}

// DeriveAll derives all key material from an identity.
// Called by the foreground `dmgn start` process after passphrase authentication.
func DeriveAll(id *identity.Identity) (*DerivedKeys, error) {
	masterKey, err := id.DeriveKey("memory-encryption", 32)
	if err != nil {
		return nil, fmt.Errorf("failed to derive master key: %w", err)
	}

	libp2pKey, err := network.DeriveLibp2pKey(id)
	if err != nil {
		return nil, fmt.Errorf("failed to derive libp2p key: %w", err)
	}

	vectorIndexKey, err := id.DeriveKey("vector-index", 32)
	if err != nil {
		return nil, fmt.Errorf("failed to derive vector index key: %w", err)
	}

	return &DerivedKeys{
		MasterKey:      masterKey,
		LibP2PKey:      libp2pKey,
		VectorIndexKey: vectorIndexKey,
		IdentityKey:    id.PrivateKey,
	}, nil
}

// Encode serializes DerivedKeys to a base64-encoded JSON string
// suitable for passing via the DMGN_DERIVED_KEYS environment variable.
func (dk *DerivedKeys) Encode() (string, error) {
	rawKey, err := crypto.MarshalPrivateKey(dk.LibP2PKey)
	if err != nil {
		return "", fmt.Errorf("failed to marshal libp2p key: %w", err)
	}

	ek := encodedKeys{
		MasterKey:      base64.StdEncoding.EncodeToString(dk.MasterKey),
		LibP2PKeyBytes: base64.StdEncoding.EncodeToString(rawKey),
		VectorIndexKey: base64.StdEncoding.EncodeToString(dk.VectorIndexKey),
		IdentityKey:    base64.StdEncoding.EncodeToString(dk.IdentityKey),
	}

	data, err := json.Marshal(ek)
	if err != nil {
		return "", fmt.Errorf("failed to marshal keys: %w", err)
	}

	return base64.StdEncoding.EncodeToString(data), nil
}

// Decode deserializes a base64-encoded JSON string back into DerivedKeys.
// Used by the daemon child process to recover keys from DMGN_DERIVED_KEYS env var.
func Decode(encoded string) (*DerivedKeys, error) {
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("failed to decode keys envelope: %w", err)
	}

	var ek encodedKeys
	if err := json.Unmarshal(data, &ek); err != nil {
		return nil, fmt.Errorf("failed to unmarshal keys: %w", err)
	}

	masterKey, err := base64.StdEncoding.DecodeString(ek.MasterKey)
	if err != nil {
		return nil, fmt.Errorf("failed to decode master key: %w", err)
	}

	libp2pKeyBytes, err := base64.StdEncoding.DecodeString(ek.LibP2PKeyBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to decode libp2p key: %w", err)
	}

	libp2pKey, err := crypto.UnmarshalPrivateKey(libp2pKeyBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal libp2p key: %w", err)
	}

	vectorIndexKey, err := base64.StdEncoding.DecodeString(ek.VectorIndexKey)
	if err != nil {
		return nil, fmt.Errorf("failed to decode vector index key: %w", err)
	}

	identityKeyBytes, err := base64.StdEncoding.DecodeString(ek.IdentityKey)
	if err != nil {
		return nil, fmt.Errorf("failed to decode identity key: %w", err)
	}

	return &DerivedKeys{
		MasterKey:      masterKey,
		LibP2PKey:      libp2pKey,
		VectorIndexKey: vectorIndexKey,
		IdentityKey:    ed25519.PrivateKey(identityKeyBytes),
	}, nil
}

// Identity reconstructs an *identity.Identity from the stored key material.
// Used by the daemon to provide Identity to subsystems that need it.
func (dk *DerivedKeys) Identity() *identity.Identity {
	publicKey := dk.IdentityKey.Public().(ed25519.PublicKey)
	return &identity.Identity{
		PublicKey:  publicKey,
		PrivateKey: dk.IdentityKey,
		ID:         base58.Encode(publicKey),
	}
}
