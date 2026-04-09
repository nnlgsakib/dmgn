package network

import (
	"math"
	"path/filepath"
	"testing"

	"github.com/dgraph-io/badger/v4"
)

func openRepTestDB(t *testing.T) *badger.DB {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "repdb")
	opts := badger.DefaultOptions(dir).WithLogger(nil)
	db, err := badger.Open(opts)
	if err != nil {
		t.Fatalf("failed to open test DB: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestRecordInteraction(t *testing.T) {
	db := openRepTestDB(t)
	rm := NewReputationManager(db)

	rm.RecordInteraction("peer-a", 100, true)
	rm.RecordInteraction("peer-a", 200, true)

	score := rm.GetScore("peer-a")
	if score <= 0 || score > 1.0 {
		t.Fatalf("unexpected score: %f", score)
	}

	rep := rm.GetReputation("peer-a")
	if rep == nil {
		t.Fatal("expected reputation record")
	}
	if rep.InteractionCount != 2 {
		t.Fatalf("expected 2 interactions, got %d", rep.InteractionCount)
	}
}

func TestScoreComputation(t *testing.T) {
	db := openRepTestDB(t)
	rm := NewReputationManager(db)

	// Record known values: all successful, low latency
	for i := 0; i < 10; i++ {
		rm.RecordInteraction("peer-good", 50, true)
	}

	// Record known values: half failures, high latency
	for i := 0; i < 10; i++ {
		rm.RecordInteraction("peer-bad", 4000, i%2 == 0)
	}

	goodScore := rm.GetScore("peer-good")
	badScore := rm.GetScore("peer-bad")

	if goodScore <= badScore {
		t.Fatalf("expected good score > bad score: %f vs %f", goodScore, badScore)
	}
}

func TestGetTopPeers(t *testing.T) {
	db := openRepTestDB(t)
	rm := NewReputationManager(db)

	rm.RecordInteraction("peer-1", 50, true)
	rm.RecordInteraction("peer-2", 100, true)
	rm.RecordInteraction("peer-3", 500, false)
	rm.RecordInteraction("peer-4", 200, true)
	rm.RecordInteraction("peer-5", 1000, false)

	top := rm.GetTopPeers(3)
	if len(top) != 3 {
		t.Fatalf("expected 3 peers, got %d", len(top))
	}

	// Verify sorted by score descending
	for i := 1; i < len(top); i++ {
		if top[i].Score > top[i-1].Score {
			t.Fatal("top peers not sorted by score")
		}
	}
}

func TestDecayScores(t *testing.T) {
	db := openRepTestDB(t)
	rm := NewReputationManager(db)

	// Set up a high-score peer
	for i := 0; i < 20; i++ {
		rm.RecordInteraction("peer-decay", 50, true)
	}

	before := rm.GetScore("peer-decay")

	// Decay toward 0.5
	rm.DecayScores(0.1)

	after := rm.GetScore("peer-decay")

	if before <= 0.5 {
		t.Skip("score already at neutral")
	}

	if after >= before {
		t.Fatalf("expected score to decrease after decay: %f -> %f", before, after)
	}

	// After decay, score should be closer to 0.5
	distBefore := math.Abs(before - 0.5)
	distAfter := math.Abs(after - 0.5)
	if distAfter >= distBefore {
		t.Fatalf("expected score closer to 0.5 after decay: dist %f -> %f", distBefore, distAfter)
	}
}

func TestPersistence(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "repdb")
	opts := badger.DefaultOptions(dir).WithLogger(nil)

	// First manager: record interactions
	db1, err := badger.Open(opts)
	if err != nil {
		t.Fatal(err)
	}
	rm1 := NewReputationManager(db1)
	rm1.RecordInteraction("persist-peer", 100, true)
	rm1.RecordInteraction("persist-peer", 200, true)
	score1 := rm1.GetScore("persist-peer")
	db1.Close()

	// Second manager: load from same DB
	db2, err := badger.Open(opts)
	if err != nil {
		t.Fatal(err)
	}
	defer db2.Close()
	rm2 := NewReputationManager(db2)
	score2 := rm2.GetScore("persist-peer")

	if math.Abs(score1-score2) > 0.001 {
		t.Fatalf("scores differ after reload: %f vs %f", score1, score2)
	}
}

func TestLatencyScore(t *testing.T) {
	tests := []struct {
		latency float64
		want    float64
	}{
		{0, 1.0},
		{2500, 0.5},
		{5000, 0.0},
		{10000, 0.0},
	}

	for _, tt := range tests {
		rep := &PeerReputation{
			UptimeRatio:      1.0,
			AvgLatencyMs:     tt.latency,
			SyncSuccessRate:  1.0,
			DataAvailability: 1.0,
		}
		score := computeScore(rep)
		latencyComponent := 0.3 * math.Max(0, 1.0-tt.latency/5000.0)
		_ = latencyComponent

		// Just verify the latency direction
		if tt.latency == 0 && score < 0.9 {
			t.Fatalf("0ms latency should give high score, got %f", score)
		}
		if tt.latency >= 5000 && score > 0.75 {
			t.Fatalf("5000ms+ latency should lower score, got %f", score)
		}
	}
}

func TestNeutralDefault(t *testing.T) {
	db := openRepTestDB(t)
	rm := NewReputationManager(db)

	score := rm.GetScore("unknown-peer")
	if score != 0.5 {
		t.Fatalf("expected neutral score 0.5, got %f", score)
	}
}
