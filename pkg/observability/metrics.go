package observability

import (
	"fmt"

	"go.opentelemetry.io/otel"
	otelmetric "go.opentelemetry.io/otel/metric"
)

// DMGNMetrics holds all DMGN metric instruments.
type DMGNMetrics struct {
	MemoryCount     otelmetric.Int64Counter
	QueryLatency    otelmetric.Float64Histogram
	PeerCount       otelmetric.Int64UpDownCounter
	SyncEvents      otelmetric.Int64Counter
	GossipMessages  otelmetric.Int64Counter
	VectorIndexSize otelmetric.Int64UpDownCounter
}

// NewDMGNMetrics creates all metric instruments for DMGN.
func NewDMGNMetrics() (*DMGNMetrics, error) {
	meter := otel.Meter("dmgn")

	memoryCount, err := meter.Int64Counter("dmgn.memory.count",
		otelmetric.WithDescription("Number of memories added"))
	if err != nil {
		return nil, fmt.Errorf("failed to create memory count metric: %w", err)
	}

	queryLatency, err := meter.Float64Histogram("dmgn.query.latency_ms",
		otelmetric.WithDescription("Query latency in milliseconds"))
	if err != nil {
		return nil, fmt.Errorf("failed to create query latency metric: %w", err)
	}

	peerCount, err := meter.Int64UpDownCounter("dmgn.peer.count",
		otelmetric.WithDescription("Current connected peer count"))
	if err != nil {
		return nil, fmt.Errorf("failed to create peer count metric: %w", err)
	}

	syncEvents, err := meter.Int64Counter("dmgn.sync.events",
		otelmetric.WithDescription("Number of sync events"))
	if err != nil {
		return nil, fmt.Errorf("failed to create sync events metric: %w", err)
	}

	gossipMessages, err := meter.Int64Counter("dmgn.gossip.messages",
		otelmetric.WithDescription("Number of gossip messages sent/received"))
	if err != nil {
		return nil, fmt.Errorf("failed to create gossip messages metric: %w", err)
	}

	vectorIndexSize, err := meter.Int64UpDownCounter("dmgn.vectorindex.size",
		otelmetric.WithDescription("Number of vectors in the index"))
	if err != nil {
		return nil, fmt.Errorf("failed to create vector index size metric: %w", err)
	}

	return &DMGNMetrics{
		MemoryCount:     memoryCount,
		QueryLatency:    queryLatency,
		PeerCount:       peerCount,
		SyncEvents:      syncEvents,
		GossipMessages:  gossipMessages,
		VectorIndexSize: vectorIndexSize,
	}, nil
}
