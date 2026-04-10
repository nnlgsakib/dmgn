package storage

import (
	"testing"

	dmgnpb "github.com/nnlgsakib/dmgn/proto/dmgn/v1"
	"google.golang.org/protobuf/proto"
)

func TestSaveEdgeWithMeta(t *testing.T) {
	dir := t.TempDir()
	store, err := New(Options{DataDir: dir})
	if err != nil {
		t.Fatalf("New store failed: %v", err)
	}
	defer store.Close()

	edge := &dmgnpb.Edge{
		FromId:        "mem-001",
		ToId:          "mem-002",
		Weight:        0.85,
		EdgeType:      "references",
		Timestamp:     1234567890,
		CreatorPeerId: "peer-A",
	}

	if err := store.SaveEdgeWithMeta(edge); err != nil {
		t.Fatalf("SaveEdgeWithMeta: %v", err)
	}

	// Verify it was saved
	data, err := store.GetEdgeProto("mem-001", "mem-002")
	if err != nil {
		t.Fatalf("GetEdgeProto: %v", err)
	}

	got := &dmgnpb.Edge{}
	if err := proto.Unmarshal(data, got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if got.FromId != "mem-001" {
		t.Errorf("FromId: expected mem-001, got %s", got.FromId)
	}
	if got.ToId != "mem-002" {
		t.Errorf("ToId: expected mem-002, got %s", got.ToId)
	}
	if got.Weight != 0.85 {
		t.Errorf("Weight: expected 0.85, got %f", got.Weight)
	}
	if got.EdgeType != "references" {
		t.Errorf("EdgeType: expected references, got %s", got.EdgeType)
	}
	if got.CreatorPeerId != "peer-A" {
		t.Errorf("CreatorPeerId: expected peer-A, got %s", got.CreatorPeerId)
	}
}

func TestGetEdgeProtoNotFound(t *testing.T) {
	dir := t.TempDir()
	store, err := New(Options{DataDir: dir})
	if err != nil {
		t.Fatalf("New store failed: %v", err)
	}
	defer store.Close()

	_, err = store.GetEdgeProto("nonexistent", "also-nonexistent")
	if err == nil {
		t.Fatal("expected error for non-existent edge")
	}
}

func TestSaveEdgeWithMetaOverwrite(t *testing.T) {
	dir := t.TempDir()
	store, err := New(Options{DataDir: dir})
	if err != nil {
		t.Fatalf("New store failed: %v", err)
	}
	defer store.Close()

	edge1 := &dmgnpb.Edge{
		FromId:   "m1",
		ToId:     "m2",
		Weight:   0.5,
		EdgeType: "related",
	}
	edge2 := &dmgnpb.Edge{
		FromId:   "m1",
		ToId:     "m2",
		Weight:   0.9,
		EdgeType: "strong",
	}

	store.SaveEdgeWithMeta(edge1)
	store.SaveEdgeWithMeta(edge2)

	data, err := store.GetEdgeProto("m1", "m2")
	if err != nil {
		t.Fatalf("GetEdgeProto: %v", err)
	}

	got := &dmgnpb.Edge{}
	proto.Unmarshal(data, got)

	if got.Weight != 0.9 {
		t.Errorf("expected overwritten weight 0.9, got %f", got.Weight)
	}
	if got.EdgeType != "strong" {
		t.Errorf("expected overwritten type strong, got %s", got.EdgeType)
	}
}

func TestEdgeAndLegacyAddEdgeCoexist(t *testing.T) {
	dir := t.TempDir()
	store, err := New(Options{DataDir: dir})
	if err != nil {
		t.Fatalf("New store failed: %v", err)
	}
	defer store.Close()

	// Old-style edge via AddEdge
	if err := store.AddEdge("m1", "m2", 1.0, "related"); err != nil {
		t.Fatalf("AddEdge: %v", err)
	}

	// GetEdges (old API) should still work
	edges, err := store.GetEdges("m1")
	if err != nil {
		t.Fatalf("GetEdges: %v", err)
	}
	if len(edges) != 1 || edges[0] != "m2" {
		t.Errorf("expected [m2], got %v", edges)
	}

	// GetEdgeProto on legacy edge should reconstruct
	data, err := store.GetEdgeProto("m1", "m2")
	if err != nil {
		t.Fatalf("GetEdgeProto on legacy: %v", err)
	}

	got := &dmgnpb.Edge{}
	if err := proto.Unmarshal(data, got); err != nil {
		t.Fatalf("Unmarshal legacy: %v", err)
	}
	if got.FromId != "m1" || got.ToId != "m2" {
		t.Errorf("legacy edge reconstructed incorrectly: %+v", got)
	}
}
