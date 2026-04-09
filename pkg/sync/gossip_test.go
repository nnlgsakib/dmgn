package sync

import (
	"encoding/json"
	"testing"
)

func TestGossipMessageEnvelope(t *testing.T) {
	msg := GossipMessage{
		Type:         "new_memory",
		Memory:       []byte(`{"id":"test-123"}`),
		SenderPeerID: "12D3KooWTest",
		Timestamp:    1700000000,
		Sequence:     42,
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var decoded GossipMessage
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if decoded.Type != "new_memory" {
		t.Errorf("expected type 'new_memory', got '%s'", decoded.Type)
	}
	if decoded.SenderPeerID != "12D3KooWTest" {
		t.Errorf("expected sender '12D3KooWTest', got '%s'", decoded.SenderPeerID)
	}
	if decoded.Sequence != 42 {
		t.Errorf("expected sequence 42, got %d", decoded.Sequence)
	}
	if string(decoded.Memory) != `{"id":"test-123"}` {
		t.Errorf("memory payload mismatch")
	}
}

func TestGossipMessageInvalidJSON(t *testing.T) {
	var msg GossipMessage
	err := json.Unmarshal([]byte(`{invalid`), &msg)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}
