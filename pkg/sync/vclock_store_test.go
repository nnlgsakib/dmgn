package sync

import (
	"os"
	"testing"

	badger "github.com/dgraph-io/badger/v4"
)

func openTestDB(t *testing.T) *badger.DB {
	t.Helper()
	dir, err := os.MkdirTemp("", "vclock-test-*")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })

	opts := badger.DefaultOptions(dir).WithLogger(nil)
	db, err := badger.Open(opts)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestSaveLoadVersionVector(t *testing.T) {
	db := openTestDB(t)
	store := NewVClockStore(db)

	vv := NewVersionVector()
	vv.Set("peer-A", 10)
	vv.Set("peer-B", 20)

	if err := store.Save("local-peer", vv); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := store.Load("local-peer")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if loaded.Get("peer-A") != 10 || loaded.Get("peer-B") != 20 {
		t.Errorf("loaded values wrong: A=%d, B=%d", loaded.Get("peer-A"), loaded.Get("peer-B"))
	}
}

func TestLoadNonExistent(t *testing.T) {
	db := openTestDB(t)
	store := NewVClockStore(db)

	vv, err := store.Load("nonexistent")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if vv.Get("anything") != 0 {
		t.Error("expected empty vector for nonexistent peer")
	}
}

func TestSaveSequence(t *testing.T) {
	db := openTestDB(t)
	store := NewVClockStore(db)

	if err := store.SaveSequence("peer-A", 1, "mem-001"); err != nil {
		t.Fatalf("SaveSequence 1: %v", err)
	}
	if err := store.SaveSequence("peer-A", 2, "mem-002"); err != nil {
		t.Fatalf("SaveSequence 2: %v", err)
	}
	if err := store.SaveSequence("peer-A", 3, "mem-003"); err != nil {
		t.Fatalf("SaveSequence 3: %v", err)
	}
}

func TestSaveEdgeSequence(t *testing.T) {
	db := openTestDB(t)
	store := NewVClockStore(db)

	if err := store.SaveEdgeSequence("peer-A", 1, "mem-001:mem-002"); err != nil {
		t.Fatalf("SaveEdgeSequence 1: %v", err)
	}
	if err := store.SaveEdgeSequence("peer-A", 2, "mem-003:mem-004"); err != nil {
		t.Fatalf("SaveEdgeSequence 2: %v", err)
	}
	if err := store.SaveEdgeSequence("peer-A", 3, "mem-005:mem-006"); err != nil {
		t.Fatalf("SaveEdgeSequence 3: %v", err)
	}
}

func TestGetEdgesAfter(t *testing.T) {
	db := openTestDB(t)
	store := NewVClockStore(db)

	store.SaveEdgeSequence("peer-A", 1, "m1:m2")
	store.SaveEdgeSequence("peer-A", 2, "m3:m4")
	store.SaveEdgeSequence("peer-A", 3, "m5:m6")
	store.SaveEdgeSequence("peer-A", 4, "m7:m8")

	// Get edges after seq 2 (should return m5:m6, m7:m8)
	edges, err := store.GetEdgesAfter("peer-A", 2)
	if err != nil {
		t.Fatalf("GetEdgesAfter: %v", err)
	}
	if len(edges) != 2 {
		t.Fatalf("expected 2 edges, got %d", len(edges))
	}
	if edges[0] != "m5:m6" || edges[1] != "m7:m8" {
		t.Errorf("expected [m5:m6, m7:m8], got %v", edges)
	}

	// Get all (after 0)
	all, err := store.GetEdgesAfter("peer-A", 0)
	if err != nil {
		t.Fatalf("GetEdgesAfter 0: %v", err)
	}
	if len(all) != 4 {
		t.Errorf("expected 4 edges, got %d", len(all))
	}

	// Get none (after latest)
	none, err := store.GetEdgesAfter("peer-A", 4)
	if err != nil {
		t.Fatalf("GetEdgesAfter 4: %v", err)
	}
	if len(none) != 0 {
		t.Errorf("expected 0 edges, got %d", len(none))
	}
}

func TestGetMemoriesAfter(t *testing.T) {
	db := openTestDB(t)
	store := NewVClockStore(db)

	store.SaveSequence("peer-A", 1, "mem-001")
	store.SaveSequence("peer-A", 2, "mem-002")
	store.SaveSequence("peer-A", 3, "mem-003")
	store.SaveSequence("peer-A", 4, "mem-004")

	// Get memories after seq 2 (should return mem-003, mem-004)
	mems, err := store.GetMemoriesAfter("peer-A", 2)
	if err != nil {
		t.Fatalf("GetMemoriesAfter: %v", err)
	}

	if len(mems) != 2 {
		t.Fatalf("expected 2 memories, got %d", len(mems))
	}
	if mems[0] != "mem-003" || mems[1] != "mem-004" {
		t.Errorf("expected [mem-003, mem-004], got %v", mems)
	}

	// Get all (after 0)
	all, err := store.GetMemoriesAfter("peer-A", 0)
	if err != nil {
		t.Fatalf("GetMemoriesAfter 0: %v", err)
	}
	if len(all) != 4 {
		t.Errorf("expected 4 memories, got %d", len(all))
	}

	// Get none (after latest)
	none, err := store.GetMemoriesAfter("peer-A", 4)
	if err != nil {
		t.Fatalf("GetMemoriesAfter 4: %v", err)
	}
	if len(none) != 0 {
		t.Errorf("expected 0 memories, got %d", len(none))
	}
}
