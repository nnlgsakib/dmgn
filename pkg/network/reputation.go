package network

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"sync"
	"time"

	"github.com/dgraph-io/badger/v4"
)

const reputationPrefix = "rep:"

// PeerReputation tracks a peer's reliability metrics.
type PeerReputation struct {
	PeerID           string  `json:"peer_id"`
	UptimeRatio      float64 `json:"uptime_ratio"`
	AvgLatencyMs     float64 `json:"avg_latency_ms"`
	SyncSuccessRate  float64 `json:"sync_success_rate"`
	DataAvailability float64 `json:"data_availability"`
	Score            float64 `json:"score"`
	LastSeen         int64   `json:"last_seen"` // Unix nano
	InteractionCount int64   `json:"interaction_count"`
	SuccessCount     int64   `json:"success_count"`
	FailureCount     int64   `json:"failure_count"`
}

// ReputationManager tracks and computes peer reputation scores.
type ReputationManager struct {
	db    *badger.DB
	cache map[string]*PeerReputation
	mu    sync.RWMutex
}

// NewReputationManager creates a new reputation manager backed by BadgerDB.
func NewReputationManager(db *badger.DB) *ReputationManager {
	rm := &ReputationManager{
		db:    db,
		cache: make(map[string]*PeerReputation),
	}
	rm.loadAll()
	return rm
}

// RecordInteraction records a peer interaction and updates the reputation score.
func (rm *ReputationManager) RecordInteraction(peerID string, latencyMs float64, success bool) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	rep, ok := rm.cache[peerID]
	if !ok {
		rep = &PeerReputation{
			PeerID:      peerID,
			UptimeRatio: 0.5,
		}
	}

	rep.InteractionCount++
	rep.LastSeen = time.Now().UnixNano()

	if success {
		rep.SuccessCount++
	} else {
		rep.FailureCount++
	}

	// Update rolling average latency
	if rep.InteractionCount == 1 {
		rep.AvgLatencyMs = latencyMs
	} else {
		alpha := 0.1 // exponential moving average
		rep.AvgLatencyMs = alpha*latencyMs + (1-alpha)*rep.AvgLatencyMs
	}

	// Update success rate
	total := rep.SuccessCount + rep.FailureCount
	if total > 0 {
		rep.SyncSuccessRate = float64(rep.SuccessCount) / float64(total)
		rep.DataAvailability = float64(rep.SuccessCount) / float64(total)
	}

	// Compute score
	rep.Score = computeScore(rep)

	rm.cache[peerID] = rep
	rm.save(peerID, rep)
}

// GetScore returns the reputation score for a peer (0.0 to 1.0).
func (rm *ReputationManager) GetScore(peerID string) float64 {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	rep, ok := rm.cache[peerID]
	if !ok {
		return 0.5 // neutral default
	}
	return rep.Score
}

// GetReputation returns the full reputation record for a peer.
func (rm *ReputationManager) GetReputation(peerID string) *PeerReputation {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	rep, ok := rm.cache[peerID]
	if !ok {
		return nil
	}
	// Return a copy
	copy := *rep
	return &copy
}

// GetTopPeers returns the top N peers sorted by score descending.
func (rm *ReputationManager) GetTopPeers(n int) []PeerReputation {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	peers := make([]PeerReputation, 0, len(rm.cache))
	for _, rep := range rm.cache {
		peers = append(peers, *rep)
	}

	sort.Slice(peers, func(i, j int) bool {
		return peers[i].Score > peers[j].Score
	})

	if n > 0 && len(peers) > n {
		peers = peers[:n]
	}
	return peers
}

// DecayScores moves all scores toward 0.5 (neutral) by the given factor.
// factor in (0, 1): higher = more decay. Call periodically.
func (rm *ReputationManager) DecayScores(factor float64) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	for peerID, rep := range rm.cache {
		rep.Score = rep.Score + factor*(0.5-rep.Score)
		rm.cache[peerID] = rep
		rm.save(peerID, rep)
	}
}

// PeerCount returns the number of tracked peers.
func (rm *ReputationManager) PeerCount() int {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	return len(rm.cache)
}

// computeScore calculates the weighted reputation score.
// Formula: 0.3*uptime + 0.3*latencyScore + 0.2*syncRate + 0.2*availability
func computeScore(rep *PeerReputation) float64 {
	latencyScore := math.Max(0, 1.0-rep.AvgLatencyMs/5000.0)
	score := 0.3*rep.UptimeRatio + 0.3*latencyScore + 0.2*rep.SyncSuccessRate + 0.2*rep.DataAvailability
	return math.Min(1.0, math.Max(0.0, score))
}

func (rm *ReputationManager) save(peerID string, rep *PeerReputation) {
	if rm.db == nil {
		return
	}
	data, err := json.Marshal(rep)
	if err != nil {
		return
	}
	rm.db.Update(func(txn *badger.Txn) error {
		return txn.Set([]byte(reputationPrefix+peerID), data)
	})
}

func (rm *ReputationManager) load(peerID string) (*PeerReputation, error) {
	if rm.db == nil {
		return nil, fmt.Errorf("no database")
	}
	var rep PeerReputation
	err := rm.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(reputationPrefix + peerID))
		if err != nil {
			return err
		}
		return item.Value(func(val []byte) error {
			return json.Unmarshal(val, &rep)
		})
	})
	if err != nil {
		return nil, err
	}
	return &rep, nil
}

func (rm *ReputationManager) loadAll() {
	if rm.db == nil {
		return
	}
	rm.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = true
		it := txn.NewIterator(opts)
		defer it.Close()

		prefix := []byte(reputationPrefix)
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()
			item.Value(func(val []byte) error {
				var rep PeerReputation
				if err := json.Unmarshal(val, &rep); err == nil {
					rm.cache[rep.PeerID] = &rep
				}
				return nil
			})
		}
		return nil
	})
}
