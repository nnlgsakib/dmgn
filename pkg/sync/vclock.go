package sync

import (
	"encoding/json"
	"sync"
)

// VersionVector tracks the latest sequence number from each known peer.
type VersionVector struct {
	entries map[string]uint64 // peer_id -> latest sequence number
	mu      sync.RWMutex
}

// NewVersionVector creates an empty version vector.
func NewVersionVector() *VersionVector {
	return &VersionVector{
		entries: make(map[string]uint64),
	}
}

// Increment increments the local peer's sequence number and returns the new value.
func (vv *VersionVector) Increment(peerID string) uint64 {
	vv.mu.Lock()
	defer vv.mu.Unlock()
	vv.entries[peerID]++
	return vv.entries[peerID]
}

// Get returns the sequence number for a peer (0 if unknown).
func (vv *VersionVector) Get(peerID string) uint64 {
	vv.mu.RLock()
	defer vv.mu.RUnlock()
	return vv.entries[peerID]
}

// Set sets the sequence number for a peer.
func (vv *VersionVector) Set(peerID string, seq uint64) {
	vv.mu.Lock()
	defer vv.mu.Unlock()
	vv.entries[peerID] = seq
}

// Merge updates the vector with another vector's entries (take max of each).
func (vv *VersionVector) Merge(other *VersionVector) {
	vv.mu.Lock()
	defer vv.mu.Unlock()
	other.mu.RLock()
	defer other.mu.RUnlock()

	for k, v := range other.entries {
		if v > vv.entries[k] {
			vv.entries[k] = v
		}
	}
}

// MissingFrom returns peer entries where other has newer data than this vector.
// Key is peer_id, value is the sequence this vector has (so caller should fetch seq > value).
func (vv *VersionVector) MissingFrom(other *VersionVector) map[string]uint64 {
	vv.mu.RLock()
	defer vv.mu.RUnlock()
	other.mu.RLock()
	defer other.mu.RUnlock()

	missing := make(map[string]uint64)
	for k, otherSeq := range other.entries {
		mySeq := vv.entries[k]
		if otherSeq > mySeq {
			missing[k] = mySeq
		}
	}
	return missing
}

// Entries returns a copy of the internal map.
func (vv *VersionVector) Entries() map[string]uint64 {
	vv.mu.RLock()
	defer vv.mu.RUnlock()
	out := make(map[string]uint64, len(vv.entries))
	for k, v := range vv.entries {
		out[k] = v
	}
	return out
}

// Marshal serializes the version vector to JSON bytes.
func (vv *VersionVector) Marshal() ([]byte, error) {
	vv.mu.RLock()
	defer vv.mu.RUnlock()
	return json.Marshal(vv.entries)
}

// UnmarshalVersionVector deserializes a version vector from JSON bytes.
func UnmarshalVersionVector(data []byte) (*VersionVector, error) {
	entries := make(map[string]uint64)
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, err
	}
	return &VersionVector{entries: entries}, nil
}

// Clone returns a deep copy of the version vector.
func (vv *VersionVector) Clone() *VersionVector {
	vv.mu.RLock()
	defer vv.mu.RUnlock()
	clone := &VersionVector{entries: make(map[string]uint64, len(vv.entries))}
	for k, v := range vv.entries {
		clone.entries[k] = v
	}
	return clone
}
