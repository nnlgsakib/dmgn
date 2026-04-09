package sync

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	libp2p_host "github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"

	"github.com/nnlgsakib/dmgn/pkg/memory"
	"github.com/nnlgsakib/dmgn/pkg/storage"
)

// SyncProtocol is the protocol ID for delta sync.
const SyncProtocol = protocol.ID("/dmgn/memory/sync/1.0.0")

// syncRequest is sent by the initiator to begin delta sync.
type syncRequest struct {
	SenderPeerID  string            `json:"sender_peer_id"`
	VersionVector map[string]uint64 `json:"version_vector"`
}

// syncResponse is sent by the responder with missing memories + their version vector.
type syncResponse struct {
	SenderPeerID  string            `json:"sender_peer_id"`
	VersionVector map[string]uint64 `json:"version_vector"`
	Memories      []json.RawMessage `json:"memories"`
}

// DeltaSyncManager handles version-vector-based reconnection sync.
type DeltaSyncManager struct {
	host         libp2p_host.Host
	vv           *VersionVector
	vvStore      *VClockStore
	store        *storage.Store
	localPeerID  string
	syncInterval time.Duration
	cancel       context.CancelFunc
	done         chan struct{}
	onReceive    func(mem *memory.Memory)
}

// NewDeltaSyncManager creates a new delta sync manager.
func NewDeltaSyncManager(host libp2p_host.Host, vv *VersionVector, vvStore *VClockStore,
	store *storage.Store, localPeerID string, syncInterval time.Duration,
	onReceive func(mem *memory.Memory)) *DeltaSyncManager {
	return &DeltaSyncManager{
		host:         host,
		vv:           vv,
		vvStore:      vvStore,
		store:        store,
		localPeerID:  localPeerID,
		syncInterval: syncInterval,
		onReceive:    onReceive,
		done:         make(chan struct{}),
	}
}

// RegisterHandler registers the /dmgn/memory/sync/1.0.0 stream handler.
func (dm *DeltaSyncManager) RegisterHandler() {
	dm.host.SetStreamHandler(SyncProtocol, dm.handleStream)
}

// handleStream handles an incoming delta sync request.
func (dm *DeltaSyncManager) handleStream(s network.Stream) {
	defer s.Close()

	// Read the sync request
	var req syncRequest
	decoder := json.NewDecoder(s)
	if err := decoder.Decode(&req); err != nil {
		return
	}

	// Build the remote's version vector
	remoteVV := NewVersionVector()
	for k, v := range req.VersionVector {
		remoteVV.Set(k, v)
	}

	// Find what we have that they're missing
	theirMissing := remoteVV.MissingFrom(dm.vv)
	memories := dm.collectMissingMemories(theirMissing)

	// Send response with our version vector + missing memories
	resp := syncResponse{
		SenderPeerID:  dm.localPeerID,
		VersionVector: dm.vv.Entries(),
		Memories:      memories,
	}

	encoder := json.NewEncoder(s)
	encoder.Encode(resp)

	// Read their follow-up (memories they have that we're missing)
	var followUp syncResponse
	if err := decoder.Decode(&followUp); err != nil && err != io.EOF {
		return
	}

	dm.processReceivedMemories(followUp.Memories)
}

// SyncWithPeer initiates delta sync with a specific peer.
func (dm *DeltaSyncManager) SyncWithPeer(ctx context.Context, peerID peer.ID) error {
	s, err := dm.host.NewStream(ctx, peerID, SyncProtocol)
	if err != nil {
		return fmt.Errorf("open sync stream: %w", err)
	}
	defer s.Close()

	// Send our version vector
	req := syncRequest{
		SenderPeerID:  dm.localPeerID,
		VersionVector: dm.vv.Entries(),
	}

	encoder := json.NewEncoder(s)
	if err := encoder.Encode(req); err != nil {
		return fmt.Errorf("send sync request: %w", err)
	}

	// Read response
	var resp syncResponse
	decoder := json.NewDecoder(s)
	if err := decoder.Decode(&resp); err != nil {
		return fmt.Errorf("read sync response: %w", err)
	}

	// Process received memories
	dm.processReceivedMemories(resp.Memories)

	// Build their version vector and find what they need from us
	remoteVV := NewVersionVector()
	for k, v := range resp.VersionVector {
		remoteVV.Set(k, v)
	}
	theirMissing := remoteVV.MissingFrom(dm.vv)
	memories := dm.collectMissingMemories(theirMissing)

	// Send follow-up with what they're missing
	followUp := syncResponse{
		SenderPeerID:  dm.localPeerID,
		VersionVector: dm.vv.Entries(),
		Memories:      memories,
	}
	if err := encoder.Encode(followUp); err != nil {
		return fmt.Errorf("send follow-up: %w", err)
	}

	// Merge version vectors
	dm.vv.Merge(remoteVV)
	dm.vvStore.Save(dm.localPeerID, dm.vv)

	return nil
}

// Start begins periodic sync and registers the connected notifiee.
func (dm *DeltaSyncManager) Start(ctx context.Context) {
	ctx, dm.cancel = context.WithCancel(ctx)
	go func() {
		defer close(dm.done)
		ticker := time.NewTicker(dm.syncInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				dm.syncAllPeers(ctx)
			}
		}
	}()
}

// Stop shuts down the delta sync manager.
func (dm *DeltaSyncManager) Stop() {
	if dm.cancel != nil {
		dm.cancel()
		<-dm.done
	}
}

// syncAllPeers syncs with all currently connected peers.
func (dm *DeltaSyncManager) syncAllPeers(ctx context.Context) {
	peers := dm.host.Network().Peers()
	for _, p := range peers {
		if dm.host.Network().Connectedness(p) == network.Connected {
			syncCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			dm.SyncWithPeer(syncCtx, p)
			cancel()
		}
	}
}

// collectMissingMemories gathers memories the remote peer is missing.
func (dm *DeltaSyncManager) collectMissingMemories(missing map[string]uint64) []json.RawMessage {
	var memories []json.RawMessage

	for peerID, afterSeq := range missing {
		memIDs, err := dm.vvStore.GetMemoriesAfter(peerID, afterSeq)
		if err != nil {
			continue
		}
		for _, memID := range memIDs {
			mem, err := dm.store.GetMemory(memID)
			if err != nil {
				continue
			}
			data, err := json.Marshal(mem)
			if err != nil {
				continue
			}
			memories = append(memories, json.RawMessage(data))
		}
	}

	return memories
}

// processReceivedMemories stores and indexes received memories.
func (dm *DeltaSyncManager) processReceivedMemories(memories []json.RawMessage) {
	for _, raw := range memories {
		var mem memory.Memory
		if err := json.Unmarshal(raw, &mem); err != nil {
			continue
		}
		if dm.onReceive != nil {
			dm.onReceive(&mem)
		}
	}
}
