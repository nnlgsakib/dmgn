package storage

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/dgraph-io/badger/v4"
	"github.com/nnlgsakib/dmgn/pkg/memory"
	dmgnpb "github.com/nnlgsakib/dmgn/proto/dmgn/v1"
	"google.golang.org/protobuf/proto"
)

const (
	PrefixMemory    = "m:"
	PrefixTimeIndex = "t:"
	PrefixEdge      = "e:"
	PrefixMeta      = "meta:"
)

type Store struct {
	db           *badger.DB
	graph        *memory.Graph
	datadir      string
	maxRetention int
}

type Options struct {
	DataDir            string
	MaxTableSize       int64
	ValueLogMaxEntries uint
	MaxRetention       int
}

func DefaultOptions(dataDir string) Options {
	return Options{
		DataDir:            dataDir,
		MaxTableSize:       64 << 20,
		ValueLogMaxEntries: 100000,
	}
}

func New(opts Options) (*Store, error) {
	badgerOpts := badger.DefaultOptions(opts.DataDir)
	badgerOpts = badgerOpts.WithLogger(nil)
	badgerOpts.ValueLogMaxEntries = uint32(opts.ValueLogMaxEntries)

	db, err := badger.Open(badgerOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to open badger db: %w", err)
	}

	store := &Store{
		db:           db,
		graph:        memory.NewGraph(),
		datadir:      opts.DataDir,
		maxRetention: opts.MaxRetention,
	}

	if err := store.loadGraph(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to load graph: %w", err)
	}

	return store, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) DataDir() string {
	return s.datadir
}

func (s *Store) SaveMemory(m *memory.Memory) error {
	err := s.db.Update(func(txn *badger.Txn) error {
		memKey := []byte(PrefixMemory + m.ID)
		memData, err := m.ToJSON()
		if err != nil {
			return fmt.Errorf("failed to serialize memory: %w", err)
		}

		if err := txn.Set(memKey, memData); err != nil {
			return fmt.Errorf("failed to store memory: %w", err)
		}

		timeKey := makeTimeKey(m.Timestamp, m.ID)
		if err := txn.Set(timeKey, []byte(m.ID)); err != nil {
			return fmt.Errorf("failed to store time index: %w", err)
		}

		for _, linkID := range m.Links {
			edgeKey := []byte(PrefixEdge + m.ID + ":" + linkID)
			if err := txn.Set(edgeKey, []byte{}); err != nil {
				return fmt.Errorf("failed to store edge: %w", err)
			}
		}

		return nil
	})
	if err != nil {
		return err
	}

	if s.maxRetention > 0 {
		if _, err := s.EnforceRetention(); err != nil {
			return fmt.Errorf("failed to enforce retention: %w", err)
		}
	}

	return nil
}

func (s *Store) GetMemory(id string) (*memory.Memory, error) {
	var m *memory.Memory

	err := s.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(PrefixMemory + id))
		if err == badger.ErrKeyNotFound {
			return fmt.Errorf("memory not found: %s", id)
		}
		if err != nil {
			return fmt.Errorf("failed to get memory: %w", err)
		}

		return item.Value(func(val []byte) error {
			var err error
			m, err = memory.FromJSON(val)
			return err
		})
	})

	if err != nil {
		return nil, err
	}

	return m, nil
}

func (s *Store) GetMemoriesByTime(start, end int64, limit int) ([]*memory.Memory, error) {
	var memories []*memory.Memory
	count := 0

	err := s.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = true

		it := txn.NewIterator(opts)
		defer it.Close()

		// With inverted timestamps (^timestamp): newest = smallest key, oldest = largest key.
		// Forward iteration gives newest-first ordering.
		seekKey := makeTimeKey(end, "")
		boundKey := makeTimeKey(start, "\xff")

		prefix := []byte(PrefixTimeIndex)
		for it.Seek(seekKey); it.Valid() && count < limit; it.Next() {
			item := it.Item()
			key := item.Key()

			if !hasPrefix(key, prefix) {
				break
			}

			if string(key) > string(boundKey) {
				break
			}

			var memID string
			if err := item.Value(func(val []byte) error {
				memID = string(val)
				return nil
			}); err != nil {
				return err
			}

			mem, err := s.GetMemory(memID)
			if err != nil {
				continue
			}

			memories = append(memories, mem)
			count++
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return memories, nil
}

func hasPrefix(key, prefix []byte) bool {
	if len(key) < len(prefix) {
		return false
	}
	for i := range prefix {
		if key[i] != prefix[i] {
			return false
		}
	}
	return true
}

func (s *Store) GetRecentMemories(limit int) ([]*memory.Memory, error) {
	now := time.Now().UnixNano()
	start := int64(0)
	return s.GetMemoriesByTime(start, now, limit)
}

func (s *Store) AddEdge(fromID, toID string, weight float32, edgeType string) error {
	return s.db.Update(func(txn *badger.Txn) error {
		edgeKey := []byte(PrefixEdge + fromID + ":" + toID)
		edgeData := fmt.Sprintf("%f:%s", weight, edgeType)
		return txn.Set(edgeKey, []byte(edgeData))
	})
}

func (s *Store) GetEdges(fromID string) ([]string, error) {
	var edges []string
	prefix := []byte(PrefixEdge + fromID + ":")

	err := s.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = false
		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			key := it.Item().Key()
			keyStr := string(key)
			if len(keyStr) > len(PrefixEdge)+len(fromID)+1 {
				toID := keyStr[len(PrefixEdge)+len(fromID)+1:]
				edges = append(edges, toID)
			}
		}

		return nil
	})

	return edges, err
}

// SaveEdgeWithMeta stores an edge with full metadata as serialized proto.
func (s *Store) SaveEdgeWithMeta(edge *dmgnpb.Edge) error {
	return s.db.Update(func(txn *badger.Txn) error {
		edgeKey := []byte(PrefixEdge + edge.FromId + ":" + edge.ToId)
		data, err := proto.Marshal(edge)
		if err != nil {
			return fmt.Errorf("marshal edge: %w", err)
		}
		return txn.Set(edgeKey, data)
	})
}

// GetEdgeProto retrieves an edge as serialized proto bytes.
func (s *Store) GetEdgeProto(fromID, toID string) ([]byte, error) {
	edgeKey := []byte(PrefixEdge + fromID + ":" + toID)
	var data []byte

	err := s.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(edgeKey)
		if err != nil {
			return err
		}
		data, err = item.ValueCopy(nil)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("get edge %s:%s: %w", fromID, toID, err)
	}

	// Verify it's valid proto (not legacy format)
	edge := &dmgnpb.Edge{}
	if err := proto.Unmarshal(data, edge); err != nil {
		// Legacy format — reconstruct
		edge = &dmgnpb.Edge{
			FromId:   fromID,
			ToId:     toID,
			Weight:   1.0,
			EdgeType: "related",
		}
		data, _ = proto.Marshal(edge)
	}

	return data, nil
}

func (s *Store) DeleteMemory(id string) error {
	return s.db.Update(func(txn *badger.Txn) error {
		memKey := []byte(PrefixMemory + id)
		if err := txn.Delete(memKey); err != nil {
			return fmt.Errorf("failed to delete memory: %w", err)
		}

		m, err := s.GetMemory(id)
		if err == nil {
			timeKey := makeTimeKey(m.Timestamp, id)
			txn.Delete(timeKey)
		}

		edgePrefix := []byte(PrefixEdge + id + ":")
	opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = false
		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Seek(edgePrefix); it.ValidForPrefix(edgePrefix); it.Next() {
			key := it.Item().Key()
			txn.Delete(key)
		}

		return nil
	})
}

func (s *Store) GetStats() (map[string]int64, error) {
	stats := map[string]int64{
		"memory_count": 0,
		"edge_count":   0,
	}

	err := s.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = false

		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Rewind(); it.Valid(); it.Next() {
			key := string(it.Item().Key())
			if len(key) > 2 {
				prefix := key[:2]
				switch prefix {
				case PrefixMemory:
					stats["memory_count"]++
				case PrefixEdge:
					stats["edge_count"]++
				}
			}
		}

		return nil
	})

	return stats, err
}

func (s *Store) BatchSave(memories []*memory.Memory) error {
	return s.db.Update(func(txn *badger.Txn) error {
		for _, m := range memories {
			memKey := []byte(PrefixMemory + m.ID)
			memData, err := m.ToJSON()
			if err != nil {
				return fmt.Errorf("failed to serialize memory %s: %w", m.ID, err)
			}

			if err := txn.Set(memKey, memData); err != nil {
				return fmt.Errorf("failed to store memory %s: %w", m.ID, err)
			}

			timeKey := makeTimeKey(m.Timestamp, m.ID)
			if err := txn.Set(timeKey, []byte(m.ID)); err != nil {
				return fmt.Errorf("failed to store time index for %s: %w", m.ID, err)
			}
		}

		return nil
	})
}

func (s *Store) loadGraph() error {
	return s.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = true
		it := txn.NewIterator(opts)
		defer it.Close()

		prefix := []byte(PrefixMemory)
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()
			if err := item.Value(func(val []byte) error {
				m, err := memory.FromJSON(val)
				if err != nil {
					return nil
				}
				s.graph.AddNode(m)
				return nil
			}); err != nil {
				return err
			}
		}

		edgePrefix := []byte(PrefixEdge)
		it2 := txn.NewIterator(opts)
		defer it2.Close()

		for it2.Seek(edgePrefix); it2.ValidForPrefix(edgePrefix); it2.Next() {
			item := it2.Item()
			key := string(item.Key())

			if len(key) > len(PrefixEdge) {
				parts := splitEdgeKey(key[len(PrefixEdge):])
				if len(parts) == 2 {
					fromID, toID := parts[0], parts[1]
					_ = s.graph.AddEdge(fromID, toID, 1.0, "default")
				}
			}
		}

		return nil
	})
}

func (s *Store) GetGraph() *memory.Graph {
	return s.graph
}

func makeTimeKey(timestamp int64, id string) []byte {
	key := make([]byte, 0, 8+len(id)+2)
	key = append(key, []byte(PrefixTimeIndex)...)
	
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, uint64(^timestamp))
	key = append(key, b...)
	
	if id != "" {
		key = append(key, ':')
		key = append(key, []byte(id)...)
	}
	
	return key
}

func splitEdgeKey(key string) []string {
	for i := 0; i < len(key); i++ {
		if key[i] == ':' {
			return []string{key[:i], key[i+1:]}
		}
	}
	return []string{key}
}

func (s *Store) Backup(path string) error {
	f, err := os.Create(filepath.Join(path, "backup.bdg"))
	if err != nil {
		return fmt.Errorf("failed to create backup file: %w", err)
	}
	defer f.Close()
	_, err = s.db.Backup(f, 0)
	return err
}

func (s *Store) Path() string {
	return s.datadir
}

// DB returns the underlying BadgerDB instance for cross-package use.
func (s *Store) DB() *badger.DB {
	return s.db
}
