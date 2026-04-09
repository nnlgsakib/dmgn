package network

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"

	"github.com/nnlgsakib/dmgn/pkg/sharding"
)

const (
	StoreProtocol = protocol.ID("/dmgn/memory/store/1.0.0")
	FetchProtocol = protocol.ID("/dmgn/memory/fetch/1.0.0")

	storeTimeout = 30 * time.Second
	fetchTimeout = 15 * time.Second
	maxHeaderLen = 4096
	maxShardLen  = 10 * 1024 * 1024 // 10 MB
)

// StoreRequest is the JSON header sent before shard data on a store stream.
type StoreRequest struct {
	MemoryID    string `json:"memory_id"`
	ShardIndex  int    `json:"shard_index"`
	TotalShards int    `json:"total_shards"`
	Threshold   int    `json:"threshold"`
	Checksum    string `json:"checksum"`
	DataLen     int    `json:"data_len"`
}

// StoreResponse is the JSON response after a store operation.
type StoreResponse struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

// FetchRequest is the JSON header for a fetch stream.
type FetchRequest struct {
	MemoryID   string `json:"memory_id"`
	ShardIndex int    `json:"shard_index"`
}

// FetchResponse is the JSON header sent before shard data on a fetch response.
type FetchResponse struct {
	Status      string `json:"status"`
	MemoryID    string `json:"memory_id,omitempty"`
	ShardIndex  int    `json:"shard_index,omitempty"`
	TotalShards int    `json:"total_shards,omitempty"`
	Threshold   int    `json:"threshold,omitempty"`
	Checksum    string `json:"checksum,omitempty"`
	DataLen     int    `json:"data_len,omitempty"`
	Message     string `json:"message,omitempty"`
}

// StorageBackend abstracts shard persistence for protocol handlers.
type StorageBackend interface {
	SaveShard(shard *sharding.Shard) error
	GetShard(memoryID string, shardIndex int) (*sharding.Shard, error)
}

// writeFrame writes a length-prefixed JSON header followed by optional data.
func writeFrame(w io.Writer, header interface{}, data []byte) error {
	headerBytes, err := json.Marshal(header)
	if err != nil {
		return fmt.Errorf("marshal header: %w", err)
	}

	// Write 4-byte header length (big-endian)
	lenBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(lenBuf, uint32(len(headerBytes)))
	if _, err := w.Write(lenBuf); err != nil {
		return fmt.Errorf("write header length: %w", err)
	}

	// Write header
	if _, err := w.Write(headerBytes); err != nil {
		return fmt.Errorf("write header: %w", err)
	}

	// Write data if present
	if len(data) > 0 {
		if _, err := w.Write(data); err != nil {
			return fmt.Errorf("write data: %w", err)
		}
	}

	return nil
}

// readFrame reads a length-prefixed JSON header and optional data of dataLen bytes.
func readFrame(r io.Reader, dataLen int) ([]byte, []byte, error) {
	// Read 4-byte header length
	lenBuf := make([]byte, 4)
	if _, err := io.ReadFull(r, lenBuf); err != nil {
		return nil, nil, fmt.Errorf("read header length: %w", err)
	}
	headerLen := binary.BigEndian.Uint32(lenBuf)

	if headerLen > maxHeaderLen {
		return nil, nil, fmt.Errorf("header too large: %d bytes", headerLen)
	}

	// Read header
	headerBytes := make([]byte, headerLen)
	if _, err := io.ReadFull(r, headerBytes); err != nil {
		return nil, nil, fmt.Errorf("read header: %w", err)
	}

	// Read data if expected
	var data []byte
	if dataLen > 0 {
		if dataLen > maxShardLen {
			return nil, nil, fmt.Errorf("data too large: %d bytes", dataLen)
		}
		data = make([]byte, dataLen)
		if _, err := io.ReadFull(r, data); err != nil {
			return nil, nil, fmt.Errorf("read data: %w", err)
		}
	}

	return headerBytes, data, nil
}

// readHeaderOnly reads just the length-prefixed header (no data).
func readHeaderOnly(r io.Reader) ([]byte, error) {
	headerBytes, _, err := readFrame(r, 0)
	return headerBytes, err
}

// RegisterStoreHandler registers the /dmgn/memory/store/1.0.0 stream handler.
func (h *Host) RegisterStoreHandler(store StorageBackend) {
	h.host.SetStreamHandler(StoreProtocol, func(s network.Stream) {
		defer s.Close()
		s.SetDeadline(time.Now().Add(storeTimeout))

		// Read request header to get data length
		headerBytes, err := readHeaderOnly(s)
		if err != nil {
			writeFrame(s, StoreResponse{Status: "error", Message: err.Error()}, nil)
			return
		}

		var req StoreRequest
		if err := json.Unmarshal(headerBytes, &req); err != nil {
			writeFrame(s, StoreResponse{Status: "error", Message: "invalid request"}, nil)
			return
		}

		if req.DataLen <= 0 || req.DataLen > maxShardLen {
			writeFrame(s, StoreResponse{Status: "error", Message: "invalid data length"}, nil)
			return
		}

		// Read shard data
		data := make([]byte, req.DataLen)
		if _, err := io.ReadFull(s, data); err != nil {
			writeFrame(s, StoreResponse{Status: "error", Message: "failed to read data"}, nil)
			return
		}

		// Validate checksum
		checksum := sha256.Sum256(data)
		if hex.EncodeToString(checksum[:]) != req.Checksum {
			writeFrame(s, StoreResponse{Status: "error", Message: "checksum mismatch"}, nil)
			return
		}

		// Store the shard
		shard := &sharding.Shard{
			MemoryID:    req.MemoryID,
			ShardIndex:  req.ShardIndex,
			TotalShards: req.TotalShards,
			Threshold:   req.Threshold,
			Data:        data,
			Checksum:    req.Checksum,
			OwnerPeerID: s.Conn().RemotePeer().String(),
			ReceivedAt:  time.Now().Unix(),
		}

		if err := store.SaveShard(shard); err != nil {
			writeFrame(s, StoreResponse{Status: "error", Message: "storage failed"}, nil)
			return
		}

		writeFrame(s, StoreResponse{Status: "ok"}, nil)
	})
}

// RegisterFetchHandler registers the /dmgn/memory/fetch/1.0.0 stream handler.
func (h *Host) RegisterFetchHandler(store StorageBackend) {
	h.host.SetStreamHandler(FetchProtocol, func(s network.Stream) {
		defer s.Close()
		s.SetDeadline(time.Now().Add(fetchTimeout))

		// Read request
		headerBytes, err := readHeaderOnly(s)
		if err != nil {
			writeFrame(s, FetchResponse{Status: "error", Message: err.Error()}, nil)
			return
		}

		var req FetchRequest
		if err := json.Unmarshal(headerBytes, &req); err != nil {
			writeFrame(s, FetchResponse{Status: "error", Message: "invalid request"}, nil)
			return
		}

		// Look up shard
		shard, err := store.GetShard(req.MemoryID, req.ShardIndex)
		if err != nil {
			writeFrame(s, FetchResponse{Status: "error", Message: "shard not found"}, nil)
			return
		}

		// Send response header + data
		resp := FetchResponse{
			Status:      "ok",
			MemoryID:    shard.MemoryID,
			ShardIndex:  shard.ShardIndex,
			TotalShards: shard.TotalShards,
			Threshold:   shard.Threshold,
			Checksum:    shard.Checksum,
			DataLen:     len(shard.Data),
		}

		if err := writeFrame(s, resp, shard.Data); err != nil {
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
	req := StoreRequest{
		MemoryID:    shard.MemoryID,
		ShardIndex:  shard.ShardIndex,
		TotalShards: shard.TotalShards,
		Threshold:   shard.Threshold,
		Checksum:    hex.EncodeToString(checksum[:]),
		DataLen:     len(shard.Data),
	}

	// Write header
	if err := writeFrame(s, req, nil); err != nil {
		return fmt.Errorf("write request: %w", err)
	}

	// Write shard data
	if _, err := s.Write(shard.Data); err != nil {
		return fmt.Errorf("write shard data: %w", err)
	}

	// Read response
	respBytes, err := readHeaderOnly(s)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	var resp StoreResponse
	if err := json.Unmarshal(respBytes, &resp); err != nil {
		return fmt.Errorf("parse response: %w", err)
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

	req := FetchRequest{
		MemoryID:   memoryID,
		ShardIndex: shardIndex,
	}

	if err := writeFrame(s, req, nil); err != nil {
		return nil, fmt.Errorf("write request: %w", err)
	}

	// Read response header
	respBytes, err := readHeaderOnly(s)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	var resp FetchResponse
	if err := json.Unmarshal(respBytes, &resp); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	if resp.Status != "ok" {
		return nil, fmt.Errorf("fetch failed: %s", resp.Message)
	}

	if resp.DataLen <= 0 || resp.DataLen > maxShardLen {
		return nil, fmt.Errorf("invalid data length: %d", resp.DataLen)
	}

	// Read shard data
	data := make([]byte, resp.DataLen)
	if _, err := io.ReadFull(s, data); err != nil {
		return nil, fmt.Errorf("read shard data: %w", err)
	}

	// Validate checksum
	checksum := sha256.Sum256(data)
	if hex.EncodeToString(checksum[:]) != resp.Checksum {
		return nil, fmt.Errorf("checksum mismatch")
	}

	return &sharding.Shard{
		MemoryID:    resp.MemoryID,
		ShardIndex:  resp.ShardIndex,
		TotalShards: resp.TotalShards,
		Threshold:   resp.Threshold,
		Data:        data,
		Checksum:    resp.Checksum,
	}, nil
}
