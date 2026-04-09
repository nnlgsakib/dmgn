package sync

import (
	"fmt"

	badger "github.com/dgraph-io/badger/v4"
)

// VClockStore persists version vectors and sequence mappings in BadgerDB.
type VClockStore struct {
	db *badger.DB
}

// NewVClockStore creates a version clock store backed by BadgerDB.
func NewVClockStore(db *badger.DB) *VClockStore {
	return &VClockStore{db: db}
}

// Save persists a version vector for the local peer.
func (s *VClockStore) Save(localPeerID string, vv *VersionVector) error {
	data, err := vv.Marshal()
	if err != nil {
		return fmt.Errorf("marshal version vector: %w", err)
	}

	key := []byte(fmt.Sprintf("vv:%s", localPeerID))
	return s.db.Update(func(txn *badger.Txn) error {
		return txn.Set(key, data)
	})
}

// Load retrieves the persisted version vector. Returns a new empty vector if not found.
func (s *VClockStore) Load(localPeerID string) (*VersionVector, error) {
	key := []byte(fmt.Sprintf("vv:%s", localPeerID))
	var data []byte

	err := s.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(key)
		if err != nil {
			return err
		}
		data, err = item.ValueCopy(nil)
		return err
	})

	if err == badger.ErrKeyNotFound {
		return NewVersionVector(), nil
	}
	if err != nil {
		return nil, fmt.Errorf("load version vector: %w", err)
	}

	return UnmarshalVersionVector(data)
}

// SaveSequence persists a memory's sequence tag.
// Key format: seq:{peer_id}:{zero-padded seq} -> memory_id
func (s *VClockStore) SaveSequence(peerID string, seq uint64, memoryID string) error {
	key := []byte(fmt.Sprintf("seq:%s:%020d", peerID, seq))
	return s.db.Update(func(txn *badger.Txn) error {
		return txn.Set(key, []byte(memoryID))
	})
}

// GetMemoriesAfter returns memory IDs with sequence > afterSeq for a given peer.
func (s *VClockStore) GetMemoriesAfter(peerID string, afterSeq uint64) ([]string, error) {
	prefix := []byte(fmt.Sprintf("seq:%s:", peerID))
	startKey := []byte(fmt.Sprintf("seq:%s:%020d", peerID, afterSeq+1))

	var memoryIDs []string
	err := s.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.Prefix = prefix
		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Seek(startKey); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()
			val, err := item.ValueCopy(nil)
			if err != nil {
				return err
			}
			memoryIDs = append(memoryIDs, string(val))
		}
		return nil
	})

	return memoryIDs, err
}
