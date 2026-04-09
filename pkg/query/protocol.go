package query

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
)

// QueryProtocol is the protocol ID for cross-peer queries.
const QueryProtocol = protocol.ID("/dmgn/memory/query/1.0.0")

const queryStreamTimeout = 5 * time.Second

// QueryProtocolRequest is the wire format for cross-peer queries.
type QueryProtocolRequest struct {
	QueryID   string       `json:"query_id"`
	Embedding []float32    `json:"embedding,omitempty"`
	TextQuery string       `json:"text_query,omitempty"`
	Limit     int          `json:"limit"`
	Filters   QueryFilters `json:"filters,omitempty"`
}

// QueryProtocolResponse is the wire format for cross-peer query results.
type QueryProtocolResponse struct {
	QueryID       string        `json:"query_id"`
	Results       []QueryResult `json:"results"`
	TotalSearched int           `json:"total_searched"`
}

// RegisterQueryHandler registers the /dmgn/memory/query/1.0.0 stream handler.
func RegisterQueryHandler(h libp2p_host.Host, engine *QueryEngine, localPeerID string) {
	h.SetStreamHandler(QueryProtocol, func(s network.Stream) {
		defer s.Close()
		s.SetDeadline(time.Now().Add(queryStreamTimeout))

		var req QueryProtocolRequest
		decoder := json.NewDecoder(s)
		if err := decoder.Decode(&req); err != nil {
			writeErrorResponse(s, req.QueryID, err)
			return
		}

		results, err := engine.SearchLocal(QueryRequest{
			Embedding: req.Embedding,
			TextQuery: req.TextQuery,
			Limit:     req.Limit,
			Filters:   req.Filters,
		})
		if err != nil {
			writeErrorResponse(s, req.QueryID, err)
			return
		}

		// Tag results with source peer
		for i := range results {
			results[i].SourcePeer = localPeerID
		}

		resp := QueryProtocolResponse{
			QueryID:       req.QueryID,
			Results:       results,
			TotalSearched: engine.index.Count(),
		}

		encoder := json.NewEncoder(s)
		encoder.Encode(resp)
	})
}

// QueryPeer sends a query to a specific peer and returns results.
func QueryPeer(ctx context.Context, h libp2p_host.Host, peerID peer.ID, req QueryProtocolRequest) (*QueryProtocolResponse, error) {
	s, err := h.NewStream(ctx, peerID, QueryProtocol)
	if err != nil {
		return nil, fmt.Errorf("open query stream to %s: %w", peerID, err)
	}
	defer s.Close()

	encoder := json.NewEncoder(s)
	if err := encoder.Encode(req); err != nil {
		return nil, fmt.Errorf("send query request: %w", err)
	}

	// Close write side to signal request complete
	if err := s.CloseWrite(); err != nil {
		return nil, fmt.Errorf("close write: %w", err)
	}

	var resp QueryProtocolResponse
	decoder := json.NewDecoder(s)
	if err := decoder.Decode(&resp); err != nil {
		if err == io.EOF {
			return &QueryProtocolResponse{QueryID: req.QueryID}, nil
		}
		return nil, fmt.Errorf("read query response: %w", err)
	}

	return &resp, nil
}

func writeErrorResponse(s network.Stream, queryID string, err error) {
	resp := QueryProtocolResponse{
		QueryID: queryID,
		Results: []QueryResult{},
	}
	encoder := json.NewEncoder(s)
	encoder.Encode(resp)
}
