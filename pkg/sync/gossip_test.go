package sync

import (
	"testing"

	dmgnpb "github.com/nnlgsakib/dmgn/proto/dmgn/v1"
	"google.golang.org/protobuf/proto"
)

func TestGossipMessageEnvelope(t *testing.T) {
	msg := &dmgnpb.GossipMessage{
		Type:         "new_memory",
		Memory:       []byte("test-memory-bytes"),
		SenderPeerId: "12D3KooWTest",
		Timestamp:    1700000000,
		Sequence:     42,
	}

	data, err := proto.Marshal(msg)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var decoded dmgnpb.GossipMessage
	if err := proto.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if decoded.Type != "new_memory" {
		t.Errorf("expected type 'new_memory', got '%s'", decoded.Type)
	}
	if decoded.SenderPeerId != "12D3KooWTest" {
		t.Errorf("expected sender '12D3KooWTest', got '%s'", decoded.SenderPeerId)
	}
	if decoded.Sequence != 42 {
		t.Errorf("expected sequence 42, got %d", decoded.Sequence)
	}
	if string(decoded.Memory) != "test-memory-bytes" {
		t.Errorf("memory payload mismatch")
	}
}

func TestGossipMessageInvalidProto(t *testing.T) {
	var msg dmgnpb.GossipMessage
	err := proto.Unmarshal([]byte{0xff, 0xff, 0xff}, &msg)
	if err == nil {
		t.Error("expected error for invalid proto data")
	}
}
