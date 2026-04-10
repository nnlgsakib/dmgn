package sync

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"strings"
	"time"

	libp2p_host "github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"

	"github.com/nnlgsakib/dmgn/pkg/memory"
	dmgnpb "github.com/nnlgsakib/dmgn/proto/dmgn/v1"
	"github.com/nnlgsakib/dmgn/pkg/storage"
	"google.golang.org/protobuf/proto"
)

// SyncProtocol is the protocol ID for delta sync.
const SyncProtocol = protocol.ID("/dmgn/memory/sync/2.0.0")

const maxSyncMsgLen = 16 * 1024 * 1024 // 16 MB

// DeltaSyncManager handles version-vector-based reconnection sync.
type DeltaSyncManager struct {
	host         libp2p_host.Host
	vv           *VersionVector
	edgeVV       *VersionVector
	vvStore      *VClockStore
	store        *storage.Store
	localPeerID  string
	syncInterval time.Duration
	cancel       context.CancelFunc
	done         chan struct{}
	onReceive    func(mem *memory.Memory)
	onEdgeReceive func(edge *dmgnpb.Edge)
}

// NewDeltaSyncManager creates a new delta sync manager.
func NewDeltaSyncManager(host libp2p_host.Host, vv *VersionVector, edgeVV *VersionVector, vvStore *VClockStore,
	store *storage.Store, localPeerID string, syncInterval time.Duration,
	onReceive func(mem *memory.Memory), onEdgeReceive func(edge *dmgnpb.Edge)) *DeltaSyncManager {
	return &DeltaSyncManager{
		host:          host,
		vv:            vv,
		edgeVV:        edgeVV,
		vvStore:       vvStore,
		store:         store,
		localPeerID:   localPeerID,
		syncInterval:  syncInterval,
		onReceive:     onReceive,
		onEdgeReceive: onEdgeReceive,
		done:          make(chan struct{}),
	}
}

// RegisterHandler registers the /dmgn/memory/sync/2.0.0 stream handler.
func (dm *DeltaSyncManager) RegisterHandler() {
	dm.host.SetStreamHandler(SyncProtocol, dm.handleStream)
}

// handleStream handles an incoming delta sync request.
func (dm *DeltaSyncManager) handleStream(s network.Stream) {
	defer s.Close()

	// Read the sync request
	req := &dmgnpb.SyncRequest{}
	if err := readSyncMsg(s, req); err != nil {
		return
	}

	// Build the remote's version vectors
	remoteVV := NewVersionVector()
	for k, v := range req.VersionVector {
		remoteVV.Set(k, v)
	}
	remoteEdgeVV := NewVersionVector()
	for k, v := range req.EdgeVersionVector {
		remoteEdgeVV.Set(k, v)
	}

	// Find what we have that they're missing
	theirMissing := remoteVV.MissingFrom(dm.vv)
	memories := dm.collectMissingMemories(theirMissing)
	theirEdgeMissing := remoteEdgeVV.MissingFrom(dm.edgeVV)
	edges := dm.collectMissingEdges(theirEdgeMissing)

	// Send response with our version vectors + missing data
	resp := &dmgnpb.SyncResponse{
		SenderPeerId:      dm.localPeerID,
		VersionVector:     dm.vv.Entries(),
		Memories:          memories,
		Edges:             edges,
		EdgeVersionVector: dm.edgeVV.Entries(),
	}

	writeSyncMsg(s, resp)

	// Read their follow-up (data they have that we're missing)
	followUp := &dmgnpb.SyncResponse{}
	if err := readSyncMsg(s, followUp); err != nil {
		return
	}

	dm.processReceivedMemories(followUp.Memories)
	dm.processReceivedEdges(followUp.Edges)
}

// SyncWithPeer initiates delta sync with a specific peer.
func (dm *DeltaSyncManager) SyncWithPeer(ctx context.Context, peerID peer.ID) error {
	s, err := dm.host.NewStream(ctx, peerID, SyncProtocol)
	if err != nil {
		return fmt.Errorf("open sync stream: %w", err)
	}
	defer s.Close()

	// Send our version vectors
	req := &dmgnpb.SyncRequest{
		SenderPeerId:      dm.localPeerID,
		VersionVector:     dm.vv.Entries(),
		EdgeVersionVector: dm.edgeVV.Entries(),
	}

	if err := writeSyncMsg(s, req); err != nil {
		return fmt.Errorf("send sync request: %w", err)
	}

	// Read response
	resp := &dmgnpb.SyncResponse{}
	if err := readSyncMsg(s, resp); err != nil {
		return fmt.Errorf("read sync response: %w", err)
	}

	// Process received data
	dm.processReceivedMemories(resp.Memories)
	dm.processReceivedEdges(resp.Edges)

	// Build their version vectors and find what they need from us
	remoteVV := NewVersionVector()
	for k, v := range resp.VersionVector {
		remoteVV.Set(k, v)
	}
	remoteEdgeVV := NewVersionVector()
	for k, v := range resp.EdgeVersionVector {
		remoteEdgeVV.Set(k, v)
	}
	theirMissing := remoteVV.MissingFrom(dm.vv)
	memories := dm.collectMissingMemories(theirMissing)
	theirEdgeMissing := remoteEdgeVV.MissingFrom(dm.edgeVV)
	edges := dm.collectMissingEdges(theirEdgeMissing)

	// Send follow-up with what they're missing
	followUp := &dmgnpb.SyncResponse{
		SenderPeerId:      dm.localPeerID,
		VersionVector:     dm.vv.Entries(),
		Memories:          memories,
		Edges:             edges,
		EdgeVersionVector: dm.edgeVV.Entries(),
	}
	if err := writeSyncMsg(s, followUp); err != nil {
		return fmt.Errorf("send follow-up: %w", err)
	}

	// Merge version vectors
	dm.vv.Merge(remoteVV)
	dm.edgeVV.Merge(remoteEdgeVV)
	dm.vvStore.Save(dm.localPeerID, dm.vv)
	dm.vvStore.Save("edge:"+dm.localPeerID, dm.edgeVV)

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
func (dm *DeltaSyncManager) collectMissingMemories(missing map[string]uint64) [][]byte {
	var memories [][]byte

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
			data, err := proto.Marshal(mem.ToProto())
			if err != nil {
				continue
			}
			memories = append(memories, data)
		}
	}

	return memories
}

// processReceivedMemories stores and indexes received memories.
func (dm *DeltaSyncManager) processReceivedMemories(memories [][]byte) {
	for _, raw := range memories {
		pb := &dmgnpb.Memory{}
		if err := proto.Unmarshal(raw, pb); err != nil {
			continue
		}
		mem := memory.MemoryFromProto(pb)
		if dm.onReceive != nil {
			dm.onReceive(mem)
		}
	}
}

// collectMissingEdges gathers edges the remote peer is missing.
func (dm *DeltaSyncManager) collectMissingEdges(missing map[string]uint64) [][]byte {
	var edges [][]byte

	for peerID, afterSeq := range missing {
		edgeKeys, err := dm.vvStore.GetEdgesAfter(peerID, afterSeq)
		if err != nil {
			continue
		}
		for _, edgeKey := range edgeKeys {
			parts := strings.SplitN(edgeKey, ":", 2)
			if len(parts) != 2 {
				continue
			}
			edgeData, err := dm.store.GetEdgeProto(parts[0], parts[1])
			if err != nil {
				continue
			}
			edges = append(edges, edgeData)
		}
	}

	return edges
}

// processReceivedEdges deserializes and applies received edges.
func (dm *DeltaSyncManager) processReceivedEdges(edges [][]byte) {
	for _, raw := range edges {
		pb := &dmgnpb.Edge{}
		if err := proto.Unmarshal(raw, pb); err != nil {
			continue
		}
		if dm.onEdgeReceive != nil {
			dm.onEdgeReceive(pb)
		}
	}
}

// writeSyncMsg writes a length-prefixed protobuf message to a stream.
func writeSyncMsg(w io.Writer, msg proto.Message) error {
	data, err := proto.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal sync message: %w", err)
	}
	lenBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(lenBuf, uint32(len(data)))
	if _, err := w.Write(lenBuf); err != nil {
		return fmt.Errorf("write sync length: %w", err)
	}
	if _, err := w.Write(data); err != nil {
		return fmt.Errorf("write sync message: %w", err)
	}
	return nil
}

// readSyncMsg reads a length-prefixed protobuf message from a stream.
func readSyncMsg(r io.Reader, msg proto.Message) error {
	lenBuf := make([]byte, 4)
	if _, err := io.ReadFull(r, lenBuf); err != nil {
		return fmt.Errorf("read sync length: %w", err)
	}
	msgLen := binary.BigEndian.Uint32(lenBuf)
	if msgLen > maxSyncMsgLen {
		return fmt.Errorf("sync message too large: %d bytes", msgLen)
	}
	data := make([]byte, msgLen)
	if _, err := io.ReadFull(r, data); err != nil {
		return fmt.Errorf("read sync message: %w", err)
	}
	if err := proto.Unmarshal(data, msg); err != nil {
		return fmt.Errorf("unmarshal sync message: %w", err)
	}
	return nil
}
