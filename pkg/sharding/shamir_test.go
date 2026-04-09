package sharding

import (
	"bytes"
	"crypto/rand"
	"testing"
)

func TestSplitCombineRoundTrip(t *testing.T) {
	secret := []byte("hello world, this is a secret message for DMGN")

	shares, err := Split(secret, 5, 3)
	if err != nil {
		t.Fatalf("Split failed: %v", err)
	}

	if len(shares) != 5 {
		t.Fatalf("expected 5 shares, got %d", len(shares))
	}

	// Reconstruct from first 3 shares
	reconstructed, err := Combine(shares[:3])
	if err != nil {
		t.Fatalf("Combine failed: %v", err)
	}

	if !bytes.Equal(secret, reconstructed) {
		t.Errorf("reconstructed secret does not match original")
	}
}

func TestThresholdMinimum(t *testing.T) {
	secret := []byte("threshold test secret")

	shares, err := Split(secret, 5, 3)
	if err != nil {
		t.Fatalf("Split failed: %v", err)
	}

	// k shares should work
	result, err := Combine(shares[:3])
	if err != nil {
		t.Fatalf("Combine with k shares failed: %v", err)
	}
	if !bytes.Equal(secret, result) {
		t.Error("k shares did not reconstruct correctly")
	}

	// k-1 shares should produce wrong result (not error — SSS produces garbage)
	result2, err := Combine(shares[:2])
	if err != nil {
		t.Fatalf("Combine with k-1 shares returned error: %v", err)
	}
	if bytes.Equal(secret, result2) {
		t.Error("k-1 shares should NOT reconstruct the original secret")
	}
}

func TestDifferentPayloadSizes(t *testing.T) {
	sizes := []int{1, 16, 256, 1024, 65536}

	for _, size := range sizes {
		t.Run("", func(t *testing.T) {
			secret := make([]byte, size)
			if _, err := rand.Read(secret); err != nil {
				t.Fatalf("rand.Read failed: %v", err)
			}

			shares, err := Split(secret, 5, 3)
			if err != nil {
				t.Fatalf("Split failed for size %d: %v", size, err)
			}

			reconstructed, err := Combine(shares[:3])
			if err != nil {
				t.Fatalf("Combine failed for size %d: %v", size, err)
			}

			if !bytes.Equal(secret, reconstructed) {
				t.Errorf("round-trip failed for size %d", size)
			}
		})
	}
}

func TestInvalidParameters(t *testing.T) {
	secret := []byte("test")

	tests := []struct {
		name string
		n, k int
	}{
		{"k=0", 5, 0},
		{"k=1", 5, 1},
		{"k>n", 3, 5},
		{"n>255", 256, 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Split(secret, tt.n, tt.k)
			if err == nil {
				t.Errorf("expected error for n=%d, k=%d", tt.n, tt.k)
			}
		})
	}

	// Empty secret
	_, err := Split([]byte{}, 5, 3)
	if err == nil {
		t.Error("expected error for empty secret")
	}
}

func TestCombineWithDifferentShareSubsets(t *testing.T) {
	secret := []byte("any subset of k shares should work")

	shares, err := Split(secret, 5, 3)
	if err != nil {
		t.Fatalf("Split failed: %v", err)
	}

	// Test different subsets of 3 shares
	subsets := [][]int{
		{0, 1, 2},
		{0, 1, 3},
		{0, 2, 4},
		{1, 3, 4},
		{2, 3, 4},
	}

	for _, subset := range subsets {
		selected := make([][]byte, len(subset))
		for i, idx := range subset {
			selected[i] = shares[idx]
		}

		result, err := Combine(selected)
		if err != nil {
			t.Fatalf("Combine failed for subset %v: %v", subset, err)
		}
		if !bytes.Equal(secret, result) {
			t.Errorf("subset %v did not reconstruct correctly", subset)
		}
	}
}

func TestAllSharesReconstruct(t *testing.T) {
	secret := []byte("all n shares should also work")

	shares, err := Split(secret, 5, 3)
	if err != nil {
		t.Fatalf("Split failed: %v", err)
	}

	result, err := Combine(shares)
	if err != nil {
		t.Fatalf("Combine with all shares failed: %v", err)
	}
	if !bytes.Equal(secret, result) {
		t.Error("all shares did not reconstruct correctly")
	}
}
