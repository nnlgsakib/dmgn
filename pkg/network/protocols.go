package network

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"time"

	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"

	dmgnpb "github.com/nnlgsakib/dmgn/proto/dmgn/v1"
	"github.com/nnlgsakib/dmgn/pkg/sharding"
	"google.golang.org/protobuf/proto"
)

const (
	StoreProtocol = protocol.ID("/dmgn/memory/store/2.0.0")
	FetchProtocol = protocol.ID("/dmgn/memory/fetch/2.0.0")

	storeTimeout = 30 * time.Second
	fetchTimeout = 15 * time.Second
	maxHeaderLen = 4096
	maxShardLen  = 10 * 1024 * 1024 // 10 MB
)

// StorageBackend abstracts shard persistence for protocol handlers.
type StorageBackend interface {
	SaveShard(shard *sharding.Shard) error
	GetShard(memoryID string, shardIndex int) (*sharding.Shard, error)
}


// writeProtoFrame writes a length-prefixed protobuf message followed by optional data.
func writeProtoFrame(w io.Writer, msg proto.Message, data []byte) error {
	msgBytes, err := proto.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal proto: %w", err)
	}

	// Write 4-byte message length (big-endian)
	lenBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(lenBuf, uint32(len(msgBytes)))
	if _, err := w.Write(lenBuf); err != nil {
		return fmt.Errorf("write proto length: %w", err)
	}

	// Write message
	if _, err := w.Write(msgBytes); err != nil {
		return fmt.Errorf("write proto message: %w", err)
	}

	// Write data if present
	if len(data) > 0 {
		if _, err := w.Write(data); err != nil {
			return fmt.Errorf("write data: %w", err)
		}
	}

	return nil
}

// readProtoFrame reads a length-prefixed protobuf message and optional data of dataLen bytes.
func readProtoFrame(r io.Reader, msg proto.Message, dataLen int) ([]byte, error) {
	// Read 4-byte message length
	lenBuf := make([]byte, 4)
	if _, err := io.ReadFull(r, lenBuf); err != nil {
		return nil, fmt.Errorf("read proto length: %w", err)
	}
	msgLen := binary.BigEndian.Uint32(lenBuf)

	if msgLen > maxHeaderLen {
		return nil, fmt.Errorf("proto message too large: %d bytes", msgLen)
	}

	// Read message
	msgBytes := make([]byte, msgLen)
	if _, err := io.ReadFull(r, msgBytes); err != nil {
		return nil, fmt.Errorf("read proto message: %w", err)
	}

	if err := proto.Unmarshal(msgBytes, msg); err != nil {
		return nil, fmt.Errorf("unmarshal proto: %w", err)
	}

	// Read data if expected
	var data []byte
	if dataLen > 0 {
		if dataLen > maxShardLen {
			return nil, fmt.Errorf("data too large: %d bytes", dataLen)
		}
		data = make([]byte, dataLen)
		if _, err := io.ReadFull(r, data); err != nil {
			return nil, fmt.Errorf("read data: %w", err)
		}
	}

	return data, nil
}

// readProtoHeaderOnly reads just the length-prefixed protobuf header (no data).
func readProtoHeaderOnly(r io.Reader, msg proto.Message) error {
	_, err := readProtoFrame(r, msg, 0)
	return err
}

// RegisterStoreHandler registers the /dmgn/memory/store/2.0.0 stream handler.
func (h *Host) RegisterStoreHandler(store StorageBackend) {
	h.host.SetStreamHandler(StoreProtocol, func(s network.Stream) {
		defer s.Close()
		s.SetDeadline(time.Now().Add(storeTimeout))

		// Read request header
		req := &dmgnpb.StoreRequest{}
		if err := readProtoHeaderOnly(s, req); err != nil {
			writeProtoFrame(s, &dmgnpb.StoreResponse{Status: "error", Message: err.Error()}, nil)
			return
		}

		if req.DataLen <= 0 || int(req.DataLen) > maxShardLen {
			writeProtoFrame(s, &dmgnpb.StoreResponse{Status: "error", Message: "invalid data length"}, nil)
			return
		}

		// Read shard data
		data := make([]byte, int(req.DataLen))
		if _, err := io.ReadFull(s, data); err != nil {
			writeProtoFrame(s, &dmgnpb.StoreResponse{Status: "error", Message: "failed to read data"}, nil)
			return
		}

		// Validate checksum
		checksum := sha256.Sum256(data)
		if hex.EncodeToString(checksum[:]) != req.Checksum {
			writeProtoFrame(s, &dmgnpb.StoreResponse{Status: "error", Message: "checksum mismatch"}, nil)
			return
		}

		// Store the shard
		shard := &sharding.Shard{
			MemoryID:    req.MemoryId,
			ShardIndex:  int(req.ShardIndex),
			TotalShards: int(req.TotalShards),
			Threshold:   int(req.Threshold),
			Data:        data,
			Checksum:    req.Checksum,
			OwnerPeerID: s.Conn().RemotePeer().String(),
			ReceivedAt:  time.Now().Unix(),
		}

		if err := store.SaveShard(shard); err != nil {
			writeProtoFrame(s, &dmgnpb.StoreResponse{Status: "error", Message: "storage failed"}, nil)
			return
		}

		writeProtoFrame(s, &dmgnpb.StoreResponse{Status: "ok"}, nil)
	})
}

// RegisterFetchHandler registers the /dmgn/memory/fetch/2.0.0 stream handler.
func (h *Host) RegisterFetchHandler(store StorageBackend) {
	h.host.SetStreamHandler(FetchProtocol, func(s network.Stream) {
		defer s.Close()
		s.SetDeadline(time.Now().Add(fetchTimeout))

		// Read request
		req := &dmgnpb.FetchRequest{}
		if err := readProtoHeaderOnly(s, req); err != nil {
			writeProtoFrame(s, &dmgnpb.FetchResponse{Status: "error", Message: err.Error()}, nil)
			return
		}

		// Look up shard
		shard, err := store.GetShard(req.MemoryId, int(req.ShardIndex))
		if err != nil {
			writeProtoFrame(s, &dmgnpb.FetchResponse{Status: "error", Message: "shard not found"}, nil)
			return
		}

		// Send response header + data
		resp := &dmgnpb.FetchResponse{
			Status:      "ok",
			MemoryId:    shard.MemoryID,
			ShardIndex:  int32(shard.ShardIndex),
			TotalShards: int32(shard.TotalShards),
			Threshold:   int32(shard.Threshold),
			Checksum:    shard.Checksum,
			DataLen:     int32(len(shard.Data)),
		}

		if err := writeProtoFrame(s, resp, shard.Data); err != nil {
			return
		}
	})
}

// SendShard opens a stream to the target peer and sends a shard via the store protocol.
func (h *Host) SendShard(ctx context.Context, peerID peer.ID, shard *sharding.Shard) error {
	ctx, cancel := context.WithTimeout(ctx, storeTimeout)
	defer cancel()

	s, err := h.host.NewStream(ctx, peerID, StoreProtocol)
	if err != nil {
		return fmt.Errorf("open stream: %w", err)
	}
	defer s.Close()

	checksum := sha256.Sum256(shard.Data)
	req := &dmgnpb.StoreRequest{
		MemoryId:    shard.MemoryID,
		ShardIndex:  int32(shard.ShardIndex),
		TotalShards: int32(shard.TotalShards),
		Threshold:   int32(shard.Threshold),
		Checksum:    hex.EncodeToString(checksum[:]),
		DataLen:     int32(len(shard.Data)),
	}

	// Write header
	if err := writeProtoFrame(s, req, nil); err != nil {
		return fmt.Errorf("write request: %w", err)
	}

	// Write shard data
	if _, err := s.Write(shard.Data); err != nil {
		return fmt.Errorf("write shard data: %w", err)
	}

	// Read response
	resp := &dmgnpb.StoreResponse{}
	if err := readProtoHeaderOnly(s, resp); err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	if resp.Status != "ok" {
		return fmt.Errorf("store rejected: %s", resp.Message)
	}

	return nil
}

// FetchShard opens a stream to the target peer and fetches a shard via the fetch protocol.
func (h *Host) FetchShard(ctx context.Context, peerID peer.ID, memoryID string, shardIndex int) (*sharding.Shard, error) {
	ctx, cancel := context.WithTimeout(ctx, fetchTimeout)
	defer cancel()

	s, err := h.host.NewStream(ctx, peerID, FetchProtocol)
	if err != nil {
		return nil, fmt.Errorf("open stream: %w", err)
	}
	defer s.Close()

	req := &dmgnpb.FetchRequest{
		MemoryId:   memoryID,
		ShardIndex: int32(shardIndex),
	}

	if err := writeProtoFrame(s, req, nil); err != nil {
		return nil, fmt.Errorf("write request: %w", err)
	}

	// Read response header
	resp := &dmgnpb.FetchResponse{}
	if err := readProtoHeaderOnly(s, resp); err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.Status != "ok" {
		return nil, fmt.Errorf("fetch failed: %s", resp.Message)
	}

	if resp.DataLen <= 0 || int(resp.DataLen) > maxShardLen {
		return nil, fmt.Errorf("invalid data length: %d", resp.DataLen)
	}

	// Read shard data
	data := make([]byte, int(resp.DataLen))
	if _, err := io.ReadFull(s, data); err != nil {
		return nil, fmt.Errorf("read shard data: %w", err)
	}

	// Validate checksum
	checksum := sha256.Sum256(data)
	if hex.EncodeToString(checksum[:]) != resp.Checksum {
		return nil, fmt.Errorf("checksum mismatch")
	}

	return &sharding.Shard{
		MemoryID:    resp.MemoryId,
		ShardIndex:  int(resp.ShardIndex),
		TotalShards: int(resp.TotalShards),
		Threshold:   int(resp.Threshold),
		Data:        data,
		Checksum:    resp.Checksum,
	}, nil
}
