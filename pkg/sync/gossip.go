package sync

import (
	"context"
	"fmt"
	"time"

	libp2p_host "github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	dmgnpb "github.com/nnlgsakib/dmgn/proto/dmgn/v1"
	"google.golang.org/protobuf/proto"
)

// GossipManager handles pubsub memory propagation.
type GossipManager struct {
	ps          *pubsub.PubSub
	topic       *pubsub.Topic
	sub         *pubsub.Subscription
	topicName   string
	localPeerID peer.ID
	onReceive   func(msg *dmgnpb.GossipMessage)
	cancel      context.CancelFunc
	done        chan struct{}
}

// NewGossipManager creates a GossipSub manager for a libp2p host.
func NewGossipManager(ctx context.Context, host libp2p_host.Host, topicName string,
	onReceive func(msg *dmgnpb.GossipMessage)) (*GossipManager, error) {

	ps, err := pubsub.NewGossipSub(ctx, host)
	if err != nil {
		return nil, fmt.Errorf("create gossipsub: %w", err)
	}

	topic, err := ps.Join(topicName)
	if err != nil {
		return nil, fmt.Errorf("join topic %s: %w", topicName, err)
	}

	sub, err := topic.Subscribe()
	if err != nil {
		topic.Close()
		return nil, fmt.Errorf("subscribe to topic %s: %w", topicName, err)
	}

	return &GossipManager{
		ps:          ps,
		topic:       topic,
		sub:         sub,
		topicName:   topicName,
		localPeerID: host.ID(),
		onReceive:   onReceive,
		done:        make(chan struct{}),
	}, nil
}

// Publish broadcasts a new memory to the gossip network.
func (gm *GossipManager) Publish(ctx context.Context, memoryBytes []byte, seq uint64) error {
	msg := &dmgnpb.GossipMessage{
		Type:         "new_memory",
		Memory:       memoryBytes,
		SenderPeerId: gm.localPeerID.String(),
		Timestamp:    time.Now().UnixNano(),
		Sequence:     seq,
	}

	data, err := proto.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal gossip message: %w", err)
	}

	return gm.topic.Publish(ctx, data)
}

// Start begins listening for incoming gossip messages in a goroutine.
func (gm *GossipManager) Start(ctx context.Context) {
	ctx, gm.cancel = context.WithCancel(ctx)
	go func() {
		defer close(gm.done)
		for {
			msg, err := gm.sub.Next(ctx)
			if err != nil {
				if ctx.Err() != nil {
					return // context cancelled — clean shutdown
				}
				continue
			}

			// Skip own messages
			if msg.ReceivedFrom == gm.localPeerID {
				continue
			}

			var gossipMsg dmgnpb.GossipMessage
			if err := proto.Unmarshal(msg.Data, &gossipMsg); err != nil {
				continue // skip malformed messages
			}

			if gossipMsg.Type != "new_memory" || len(gossipMsg.Memory) == 0 {
				continue // skip invalid messages
			}

			if gm.onReceive != nil {
				gm.onReceive(&gossipMsg)
			}
		}
	}()
}

// Stop shuts down the gossip manager.
func (gm *GossipManager) Stop() {
	if gm.cancel != nil {
		gm.cancel()
		<-gm.done
	}
	if gm.sub != nil {
		gm.sub.Cancel()
	}
	if gm.topic != nil {
		gm.topic.Close()
	}
}
