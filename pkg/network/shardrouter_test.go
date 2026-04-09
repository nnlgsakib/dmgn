package network

import (
	"testing"
)

func TestShardCIDDeterministic(t *testing.T) {
	cid1 := ShardCID("mem001", 0)
	cid2 := ShardCID("mem001", 0)

	if cid1 != cid2 {
		t.Error("same input should produce same CID")
	}

	cid3 := ShardCID("mem001", 1)
	if cid1 == cid3 {
		t.Error("different shard index should produce different CID")
	}

	cid4 := ShardCID("mem002", 0)
	if cid1 == cid4 {
		t.Error("different memory ID should produce different CID")
	}
}
