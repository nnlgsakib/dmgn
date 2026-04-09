package query

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"time"

	libp2p_host "github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"

	dmgnpb "github.com/nnlgsakib/dmgn/proto/dmgn/v1"
	"google.golang.org/protobuf/proto"
)

// QueryProtocol is the protocol ID for cross-peer queries.
const QueryProtocol = protocol.ID("/dmgn/memory/query/2.0.0")

const queryStreamTimeout = 5 * time.Second
const maxQueryMsgLen = 4 * 1024 * 1024 // 4 MB

// RegisterQueryHandler registers the /dmgn/memory/query/2.0.0 stream handler.
func RegisterQueryHandler(h libp2p_host.Host, engine *QueryEngine, localPeerID string) {
	h.SetStreamHandler(QueryProtocol, func(s network.Stream) {
		defer s.Close()
		s.SetDeadline(time.Now().Add(queryStreamTimeout))

		req := &dmgnpb.QueryRequest{}
		if err := readQueryMsg(s, req); err != nil {
			writeErrorResponse(s, "", err)
			return
		}

		var filters QueryFilters
		if req.Filters != nil {
			filters = QueryFilters{
				Type:   req.Filters.Type,
				After:  req.Filters.After,
				Before: req.Filters.Before,
			}
		}

		results, err := engine.SearchLocal(QueryRequest{
			Embedding: req.Embedding,
			TextQuery: req.TextQuery,
			Limit:     int(req.Limit),
			Filters:   filters,
		})
		if err != nil {
			writeErrorResponse(s, req.QueryId, err)
			return
		}

		// Tag results with source peer
		for i := range results {
			results[i].SourcePeer = localPeerID
		}

		pbResults := make([]*dmgnpb.QueryResult, len(results))
		for i, r := range results {
			pbResults[i] = &dmgnpb.QueryResult{
				MemoryId:   r.MemoryID,
				Score:      r.Score,
				Type:       r.Type,
				Timestamp:  r.Timestamp,
				Snippet:    r.Snippet,
				SourcePeer: r.SourcePeer,
			}
		}

		resp := &dmgnpb.QueryResponse{
			QueryId:       req.QueryId,
			Results:       pbResults,
			TotalSearched: int32(engine.index.Count()),
		}

		writeQueryMsg(s, resp)
	})
}

// QueryPeer sends a query to a specific peer and returns results.
func QueryPeer(ctx context.Context, h libp2p_host.Host, peerID peer.ID, req *dmgnpb.QueryRequest) (*dmgnpb.QueryResponse, error) {
	s, err := h.NewStream(ctx, peerID, QueryProtocol)
	if err != nil {
		return nil, fmt.Errorf("open query stream to %s: %w", peerID, err)
	}
	defer s.Close()

	if err := writeQueryMsg(s, req); err != nil {
		return nil, fmt.Errorf("send query request: %w", err)
	}

	// Close write side to signal request complete
	if err := s.CloseWrite(); err != nil {
		return nil, fmt.Errorf("close write: %w", err)
	}

	resp := &dmgnpb.QueryResponse{}
	if err := readQueryMsg(s, resp); err != nil {
		if err == io.EOF {
			return &dmgnpb.QueryResponse{QueryId: req.QueryId}, nil
		}
		return nil, fmt.Errorf("read query response: %w", err)
	}

	return resp, nil
}

func writeErrorResponse(s network.Stream, queryID string, err error) {
	resp := &dmgnpb.QueryResponse{
		QueryId: queryID,
		Results: []*dmgnpb.QueryResult{},
	}
	writeQueryMsg(s, resp)
}

// writeQueryMsg writes a length-prefixed protobuf message.
func writeQueryMsg(w io.Writer, msg proto.Message) error {
	data, err := proto.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal query message: %w", err)
	}
	lenBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(lenBuf, uint32(len(data)))
	if _, err := w.Write(lenBuf); err != nil {
		return fmt.Errorf("write query length: %w", err)
	}
	if _, err := w.Write(data); err != nil {
		return fmt.Errorf("write query message: %w", err)
	}
	return nil
}

// readQueryMsg reads a length-prefixed protobuf message.
func readQueryMsg(r io.Reader, msg proto.Message) error {
	lenBuf := make([]byte, 4)
	if _, err := io.ReadFull(r, lenBuf); err != nil {
		return err
	}
	msgLen := binary.BigEndian.Uint32(lenBuf)
	if msgLen > maxQueryMsgLen {
		return fmt.Errorf("query message too large: %d bytes", msgLen)
	}
	data := make([]byte, msgLen)
	if _, err := io.ReadFull(r, data); err != nil {
		return err
	}
	return proto.Unmarshal(data, msg)
}
