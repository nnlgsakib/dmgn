package storage

import (
	"fmt"

	"github.com/dgraph-io/badger/v4"
)

func (s *Store) EnforceRetention() (int, error) {
	if s.maxRetention <= 0 {
		return 0, nil
	}

	// Count total memories
	var count int
	var idsToDelete []string

	err := s.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = true

		// Count memories
		it := txn.NewIterator(opts)
		defer it.Close()

		prefix := []byte(PrefixMemory)
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			count++
		}

		if count <= s.maxRetention {
			return nil
		}

		// Iterate time index (oldest first = reverse of our inverted timestamp)
		// Our time keys use ^timestamp so the largest keys = oldest entries
		// Default iteration goes smallest to largest = newest first
		// We need oldest first, so iterate in reverse (largest keys first? No.)
		// Actually: makeTimeKey uses ^timestamp. So:
		//   newest timestamp => smallest ^timestamp => appears first in forward iteration
		//   oldest timestamp => largest ^timestamp => appears last in forward iteration
		// We want to delete the oldest. So we iterate forward and skip the first maxRetention.

		timeIt := txn.NewIterator(badger.DefaultIteratorOptions)
		defer timeIt.Close()

		timePrefix := []byte(PrefixTimeIndex)
		skipped := 0
		for timeIt.Seek(timePrefix); timeIt.ValidForPrefix(timePrefix); timeIt.Next() {
			if skipped < s.maxRetention {
				skipped++
				continue
			}

			item := timeIt.Item()
			err := item.Value(func(val []byte) error {
				idsToDelete = append(idsToDelete, string(val))
				return nil
			})
			if err != nil {
				return err
			}
		}

		return nil
	})

	if err != nil {
		return 0, fmt.Errorf("failed to scan for retention: %w", err)
	}

	if len(idsToDelete) == 0 {
		return 0, nil
	}

	// Delete excess memories
	for _, id := range idsToDelete {
		if err := s.DeleteMemory(id); err != nil {
			return 0, fmt.Errorf("failed to delete memory %s during retention: %w", id, err)
		}
	}

	return len(idsToDelete), nil
}
