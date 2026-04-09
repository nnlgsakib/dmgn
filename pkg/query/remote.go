package query

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	libp2p_host "github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"

	dmgnpb "github.com/nnlgsakib/dmgn/proto/dmgn/v1"
)

// RemoteQueryOrchestrator fans out queries to connected peers.
type RemoteQueryOrchestrator struct {
	host         libp2p_host.Host
	engine       *QueryEngine
	localPeerID  string
	queryTimeout time.Duration
}

// NewRemoteQueryOrchestrator creates a new orchestrator.
func NewRemoteQueryOrchestrator(h libp2p_host.Host, engine *QueryEngine,
	localPeerID string, queryTimeout time.Duration) *RemoteQueryOrchestrator {
	return &RemoteQueryOrchestrator{
		host:         h,
		engine:       engine,
		localPeerID:  localPeerID,
		queryTimeout: queryTimeout,
	}
}

type peerResultSet struct {
	peerID  string
	results []QueryResult
}

// SearchAll performs a local search and fans out to all connected peers.
// Results are merged with source diversity and deduplication.
func (ro *RemoteQueryOrchestrator) SearchAll(ctx context.Context, req QueryRequest) ([]QueryResult, error) {
	var mu sync.Mutex
	var allPeerResults []peerResultSet

	// Start local search
	localResults, err := ro.engine.SearchLocal(req)
	if err != nil {
		return nil, fmt.Errorf("local search: %w", err)
	}
	// Tag local results
	for i := range localResults {
		localResults[i].SourcePeer = ro.localPeerID
	}
	allPeerResults = append(allPeerResults, peerResultSet{
		peerID:  ro.localPeerID,
		results: localResults,
	})

	// Fan out to connected peers
	connectedPeers := ro.host.Network().Peers()
	var wg sync.WaitGroup

	queryID := fmt.Sprintf("q-%d", time.Now().UnixNano())
	var pbFilters *dmgnpb.QueryFilters
	if req.Filters.Type != "" || req.Filters.After != 0 || req.Filters.Before != 0 {
		pbFilters = &dmgnpb.QueryFilters{
			Type:   req.Filters.Type,
			After:  req.Filters.After,
			Before: req.Filters.Before,
		}
	}
	protoReq := &dmgnpb.QueryRequest{
		QueryId:   queryID,
		Embedding: req.Embedding,
		TextQuery: req.TextQuery,
		Limit:     int32(req.Limit * 2), // ask for more to allow dedup
		Filters:   pbFilters,
	}

	for _, p := range connectedPeers {
		if ro.host.Network().Connectedness(p) != network.Connected {
			continue
		}

		wg.Add(1)
		go func(pid peer.ID) {
			defer wg.Done()

			peerCtx, cancel := context.WithTimeout(ctx, ro.queryTimeout)
			defer cancel()

			resp, err := QueryPeer(peerCtx, ro.host, pid, protoReq)
			if err != nil || resp == nil {
				return
			}

			// Convert proto results to internal types
			var peerResults []QueryResult
			for _, r := range resp.Results {
				peerResults = append(peerResults, QueryResult{
					MemoryID:   r.MemoryId,
					Score:      r.Score,
					Type:       r.Type,
					Timestamp:  r.Timestamp,
					Snippet:    r.Snippet,
					SourcePeer: r.SourcePeer,
				})
			}

			mu.Lock()
			allPeerResults = append(allPeerResults, peerResultSet{
				peerID:  pid.String(),
				results: peerResults,
			})
			mu.Unlock()
		}(p)
	}

	wg.Wait()

	return mergeWithDiversity(allPeerResults, req.Limit), nil
}

// mergeWithDiversity merges results from multiple peers with source diversity.
// Deduplicates by memory_id (keep highest score) and interleaves from different peers.
func mergeWithDiversity(peerResults []peerResultSet, limit int) []QueryResult {
	// Deduplicate: keep highest score per memory_id
	best := make(map[string]QueryResult)
	for _, pr := range peerResults {
		for _, r := range pr.results {
			if existing, ok := best[r.MemoryID]; !ok || r.Score > existing.Score {
				best[r.MemoryID] = r
			}
		}
	}

	// Group deduped results by source peer
	byPeer := make(map[string][]QueryResult)
	for _, r := range best {
		byPeer[r.SourcePeer] = append(byPeer[r.SourcePeer], r)
	}

	// Sort each peer's results by score
	peerIDs := make([]string, 0, len(byPeer))
	for pid, results := range byPeer {
		peerIDs = append(peerIDs, pid)
		sort.Slice(results, func(i, j int) bool {
			return results[i].Score > results[j].Score
		})
		byPeer[pid] = results
	}
	sort.Strings(peerIDs) // deterministic order

	// Round-robin interleave from each peer
	var merged []QueryResult
	indices := make(map[string]int)
	for len(merged) < limit {
		added := false
		for _, pid := range peerIDs {
			idx := indices[pid]
			if idx < len(byPeer[pid]) {
				merged = append(merged, byPeer[pid][idx])
				indices[pid] = idx + 1
				added = true
				if len(merged) >= limit {
					break
				}
			}
		}
		if !added {
			break
		}
	}

	return merged
}
