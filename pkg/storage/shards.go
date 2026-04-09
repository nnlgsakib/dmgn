package storage

import (
	"encoding/json"
	"fmt"

	"github.com/dgraph-io/badger/v4"
	"github.com/dmgn/dmgn/pkg/sharding"
)

const PrefixShard = "shard:"

func shardKey(memoryID string, shardIndex int) []byte {
	return []byte(fmt.Sprintf("%s%s:%d", PrefixShard, memoryID, shardIndex))
}

// SaveShard persists a shard to BadgerDB.
func (s *Store) SaveShard(shard *sharding.Shard) error {
	data, err := json.Marshal(shard)
	if err != nil {
		return fmt.Errorf("failed to marshal shard: %w", err)
	}

	return s.db.Update(func(txn *badger.Txn) error {
		return txn.Set(shardKey(shard.MemoryID, shard.ShardIndex), data)
	})
}

// GetShard retrieves a specific shard by memory ID and index.
func (s *Store) GetShard(memoryID string, shardIndex int) (*sharding.Shard, error) {
	var shard sharding.Shard

	err := s.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(shardKey(memoryID, shardIndex))
		if err == badger.ErrKeyNotFound {
			return fmt.Errorf("shard not found: %s:%d", memoryID, shardIndex)
		}
		if err != nil {
			return fmt.Errorf("failed to get shard: %w", err)
		}

		return item.Value(func(val []byte) error {
			return json.Unmarshal(val, &shard)
		})
	})

	if err != nil {
		return nil, err
	}
	return &shard, nil
}

// GetShardsForMemory retrieves all locally stored shards for a memory.
func (s *Store) GetShardsForMemory(memoryID string) ([]*sharding.Shard, error) {
	var shards []*sharding.Shard
	prefix := []byte(fmt.Sprintf("%s%s:", PrefixShard, memoryID))

	err := s.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = true
		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()
			if err := item.Value(func(val []byte) error {
				var shard sharding.Shard
				if err := json.Unmarshal(val, &shard); err != nil {
					return err
				}
				shards = append(shards, &shard)
				return nil
			}); err != nil {
				return err
			}
		}
		return nil
	})

	if err != nil {
		return nil, err
	}
	return shards, nil
}

// DeleteShard removes a shard from local storage.
func (s *Store) DeleteShard(memoryID string, shardIndex int) error {
	return s.db.Update(func(txn *badger.Txn) error {
		return txn.Delete(shardKey(memoryID, shardIndex))
	})
}

// GetShardStats returns shard storage statistics.
func (s *Store) GetShardStats() (map[string]int64, error) {
	stats := map[string]int64{
		"shard_count": 0,
	}

	err := s.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = false
		it := txn.NewIterator(opts)
		defer it.Close()

		prefix := []byte(PrefixShard)
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			stats["shard_count"]++
		}
		return nil
	})

	return stats, err
}
