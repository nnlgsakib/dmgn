package memory

import (
	"testing"

	dmgnpb "github.com/nnlgsakib/dmgn/proto/dmgn/v1"
	"google.golang.org/protobuf/proto"
)

func TestMemoryProtoRoundtrip(t *testing.T) {
	original := &Memory{
		ID:               "mem-abc123",
		Timestamp:        1700000000000,
		Type:             TypeConversation,
		Embedding:        []float32{0.1, 0.2, 0.3, 0.99, -0.5},
		EncryptedPayload: []byte("encrypted-data-here"),
		Links:            []string{"link-1", "link-2", "link-3"},
		MerkleProof:      "deadbeefcafe",
		Metadata: map[string]string{
			"source": "test",
			"tag":    "roundtrip",
		},
	}

	// Convert to proto
	pb := original.ToProto()
	if pb.Id != original.ID {
		t.Fatalf("ToProto: Id mismatch: got %q, want %q", pb.Id, original.ID)
	}

	// Marshal to bytes
	data, err := proto.Marshal(pb)
	if err != nil {
		t.Fatalf("proto.Marshal: %v", err)
	}
	t.Logf("Proto marshal size: %d bytes", len(data))

	// Unmarshal from bytes
	pb2 := &dmgnpb.Memory{}
	if err := proto.Unmarshal(data, pb2); err != nil {
		t.Fatalf("proto.Unmarshal: %v", err)
	}

	// Convert back to Go Memory
	restored := MemoryFromProto(pb2)

	// Verify all fields
	if restored.ID != original.ID {
		t.Errorf("ID: got %q, want %q", restored.ID, original.ID)
	}
	if restored.Timestamp != original.Timestamp {
		t.Errorf("Timestamp: got %d, want %d", restored.Timestamp, original.Timestamp)
	}
	if restored.Type != original.Type {
		t.Errorf("Type: got %q, want %q", restored.Type, original.Type)
	}
	if len(restored.Embedding) != len(original.Embedding) {
		t.Fatalf("Embedding length: got %d, want %d", len(restored.Embedding), len(original.Embedding))
	}
	for i, v := range restored.Embedding {
		if v != original.Embedding[i] {
			t.Errorf("Embedding[%d]: got %f, want %f", i, v, original.Embedding[i])
		}
	}
	if string(restored.EncryptedPayload) != string(original.EncryptedPayload) {
		t.Errorf("EncryptedPayload: got %q, want %q", restored.EncryptedPayload, original.EncryptedPayload)
	}
	if len(restored.Links) != len(original.Links) {
		t.Fatalf("Links length: got %d, want %d", len(restored.Links), len(original.Links))
	}
	for i, v := range restored.Links {
		if v != original.Links[i] {
			t.Errorf("Links[%d]: got %q, want %q", i, v, original.Links[i])
		}
	}
	if restored.MerkleProof != original.MerkleProof {
		t.Errorf("MerkleProof: got %q, want %q", restored.MerkleProof, original.MerkleProof)
	}
	if len(restored.Metadata) != len(original.Metadata) {
		t.Fatalf("Metadata length: got %d, want %d", len(restored.Metadata), len(original.Metadata))
	}
	for k, v := range original.Metadata {
		if restored.Metadata[k] != v {
			t.Errorf("Metadata[%q]: got %q, want %q", k, restored.Metadata[k], v)
		}
	}
}
