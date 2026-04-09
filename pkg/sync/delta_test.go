package sync

import (
	"encoding/json"
	"testing"
)

func TestSyncRequestMarshal(t *testing.T) {
	req := syncRequest{
		SenderPeerID: "peer-A",
		VersionVector: map[string]uint64{
			"peer-A": 10,
			"peer-B": 5,
		},
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var decoded syncRequest
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if decoded.SenderPeerID != "peer-A" {
		t.Errorf("expected peer-A, got %s", decoded.SenderPeerID)
	}
	if decoded.VersionVector["peer-A"] != 10 {
		t.Errorf("expected peer-A=10, got %d", decoded.VersionVector["peer-A"])
	}
}

func TestSyncResponseMarshal(t *testing.T) {
	resp := syncResponse{
		SenderPeerID: "peer-B",
		VersionVector: map[string]uint64{
			"peer-A": 10,
			"peer-B": 20,
		},
		Memories: []json.RawMessage{
			json.RawMessage(`{"id":"mem-001"}`),
			json.RawMessage(`{"id":"mem-002"}`),
		},
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var decoded syncResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if len(decoded.Memories) != 2 {
		t.Errorf("expected 2 memories, got %d", len(decoded.Memories))
	}
}

func TestCollectMissingLogic(t *testing.T) {
	// Test that version vector comparison correctly identifies missing entries
	local := NewVersionVector()
	local.Set("A", 5)
	local.Set("B", 10)

	remote := NewVersionVector()
	remote.Set("A", 3)
	remote.Set("B", 15)
	remote.Set("C", 7)

	// What remote is missing from local
	remoteMissing := remote.MissingFrom(local)
	if _, ok := remoteMissing["A"]; !ok {
		t.Error("remote should be missing A (local=5, remote=3)")
	}
	if remoteMissing["A"] != 3 {
		t.Errorf("expected remote A seq=3, got %d", remoteMissing["A"])
	}

	// What local is missing from remote
	localMissing := local.MissingFrom(remote)
	if _, ok := localMissing["B"]; !ok {
		t.Error("local should be missing B (local=10, remote=15)")
	}
	if _, ok := localMissing["C"]; !ok {
		t.Error("local should be missing C (local doesn't have it)")
	}
	if _, ok := localMissing["A"]; ok {
		t.Error("local should NOT be missing A (local=5, remote=3)")
	}
}
