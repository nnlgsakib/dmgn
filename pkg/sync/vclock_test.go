package sync

import (
	"testing"
)

func TestIncrement(t *testing.T) {
	vv := NewVersionVector()
	seq := vv.Increment("peer-A")
	if seq != 1 {
		t.Errorf("expected 1, got %d", seq)
	}
	seq = vv.Increment("peer-A")
	if seq != 2 {
		t.Errorf("expected 2, got %d", seq)
	}
	if vv.Get("peer-A") != 2 {
		t.Errorf("expected Get to return 2")
	}
}

func TestGet_Unknown(t *testing.T) {
	vv := NewVersionVector()
	if vv.Get("unknown") != 0 {
		t.Error("expected 0 for unknown peer")
	}
}

func TestMerge(t *testing.T) {
	vv1 := NewVersionVector()
	vv1.Set("A", 3)
	vv1.Set("B", 5)

	vv2 := NewVersionVector()
	vv2.Set("A", 7)
	vv2.Set("C", 2)

	vv1.Merge(vv2)

	if vv1.Get("A") != 7 {
		t.Errorf("expected A=7 after merge, got %d", vv1.Get("A"))
	}
	if vv1.Get("B") != 5 {
		t.Errorf("expected B=5 unchanged, got %d", vv1.Get("B"))
	}
	if vv1.Get("C") != 2 {
		t.Errorf("expected C=2 from merge, got %d", vv1.Get("C"))
	}
}

func TestMissingFrom(t *testing.T) {
	local := NewVersionVector()
	local.Set("A", 3)
	local.Set("B", 5)

	remote := NewVersionVector()
	remote.Set("A", 7) // remote has newer
	remote.Set("B", 5) // same
	remote.Set("C", 2) // local doesn't have

	missing := local.MissingFrom(remote)

	if _, ok := missing["A"]; !ok {
		t.Error("expected A in missing")
	}
	if missing["A"] != 3 {
		t.Errorf("expected A=3 (local seq), got %d", missing["A"])
	}
	if _, ok := missing["B"]; ok {
		t.Error("B should not be in missing (same seq)")
	}
	if _, ok := missing["C"]; !ok {
		t.Error("expected C in missing")
	}
	if missing["C"] != 0 {
		t.Errorf("expected C=0 (local doesn't have), got %d", missing["C"])
	}
}

func TestMarshalUnmarshal(t *testing.T) {
	vv := NewVersionVector()
	vv.Set("A", 10)
	vv.Set("B", 20)

	data, err := vv.Marshal()
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	vv2, err := UnmarshalVersionVector(data)
	if err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if vv2.Get("A") != 10 || vv2.Get("B") != 20 {
		t.Errorf("round-trip failed: A=%d, B=%d", vv2.Get("A"), vv2.Get("B"))
	}
}

func TestClone(t *testing.T) {
	vv := NewVersionVector()
	vv.Set("A", 5)

	clone := vv.Clone()
	clone.Set("A", 99)

	if vv.Get("A") != 5 {
		t.Error("original should not be modified by clone")
	}
	if clone.Get("A") != 99 {
		t.Error("clone should have updated value")
	}
}
