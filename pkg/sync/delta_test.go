package sync

import (
	"bytes"
	"testing"

	dmgnpb "github.com/nnlgsakib/dmgn/proto/dmgn/v1"
	"google.golang.org/protobuf/proto"
)

func TestSyncRequestMarshal(t *testing.T) {
	req := &dmgnpb.SyncRequest{
		SenderPeerId: "peer-A",
		VersionVector: map[string]uint64{
			"peer-A": 10,
			"peer-B": 5,
		},
	}

	data, err := proto.Marshal(req)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	decoded := &dmgnpb.SyncRequest{}
	if err := proto.Unmarshal(data, decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if decoded.SenderPeerId != "peer-A" {
		t.Errorf("expected peer-A, got %s", decoded.SenderPeerId)
	}
	if decoded.VersionVector["peer-A"] != 10 {
		t.Errorf("expected peer-A=10, got %d", decoded.VersionVector["peer-A"])
	}
}

func TestSyncResponseMarshal(t *testing.T) {
	resp := &dmgnpb.SyncResponse{
		SenderPeerId: "peer-B",
		VersionVector: map[string]uint64{
			"peer-A": 10,
			"peer-B": 20,
		},
		Memories: [][]byte{
			[]byte("mem-001-proto-bytes"),
			[]byte("mem-002-proto-bytes"),
		},
	}

	data, err := proto.Marshal(resp)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	decoded := &dmgnpb.SyncResponse{}
	if err := proto.Unmarshal(data, decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if len(decoded.Memories) != 2 {
		t.Errorf("expected 2 memories, got %d", len(decoded.Memories))
	}
}

func TestSyncMsgFrameRoundtrip(t *testing.T) {
	req := &dmgnpb.SyncRequest{
		SenderPeerId: "peer-frame-test",
		VersionVector: map[string]uint64{
			"peer-A": 100,
		},
	}

	var buf bytes.Buffer
	if err := writeSyncMsg(&buf, req); err != nil {
		t.Fatalf("writeSyncMsg: %v", err)
	}

	decoded := &dmgnpb.SyncRequest{}
	if err := readSyncMsg(&buf, decoded); err != nil {
		t.Fatalf("readSyncMsg: %v", err)
	}

	if decoded.SenderPeerId != "peer-frame-test" {
		t.Errorf("SenderPeerId: got %q, want %q", decoded.SenderPeerId, "peer-frame-test")
	}
	if decoded.VersionVector["peer-A"] != 100 {
		t.Errorf("VersionVector[peer-A]: got %d, want 100", decoded.VersionVector["peer-A"])
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
