package sharding

import (
	"crypto/rand"
	"fmt"
)

// GF(2^8) arithmetic using AES irreducible polynomial x^8 + x^4 + x^3 + x + 1 (0x1b)

// gfMul multiplies two elements in GF(2^8) using schoolbook method.
func gfMul(a, b uint8) uint8 {
	var p uint8
	for i := 0; i < 8; i++ {
		if b&1 != 0 {
			p ^= a
		}
		carry := a & 0x80
		a <<= 1
		if carry != 0 {
			a ^= 0x1b
		}
		b >>= 1
	}
	return p
}

// gfInv computes the multiplicative inverse in GF(2^8) using Fermat's little theorem:
// a^(-1) = a^254 since the multiplicative group has order 255.
func gfInv(a uint8) uint8 {
	if a == 0 {
		panic("inverse of zero in GF(2^8)")
	}
	// Compute a^254 via repeated square-and-multiply
	// 254 = 11111110 in binary
	r := a
	for i := 0; i < 6; i++ {
		r = gfMul(r, r) // square
		r = gfMul(r, a) // multiply by a
	}
	r = gfMul(r, r) // final square (bit 1 of 254 is 0)
	return r
}

// gfDiv divides a by b in GF(2^8).
func gfDiv(a, b uint8) uint8 {
	if b == 0 {
		panic("division by zero in GF(2^8)")
	}
	if a == 0 {
		return 0
	}
	return gfMul(a, gfInv(b))
}

// Split splits a secret byte slice into n shares with threshold k.
// Any k shares can reconstruct the secret; k-1 shares reveal nothing.
// Each share is len(secret)+1 bytes: the first byte is the x-coordinate (1..n).
func Split(secret []byte, n, k int) ([][]byte, error) {
	if k < 2 {
		return nil, fmt.Errorf("threshold must be >= 2, got %d", k)
	}
	if n < k {
		return nil, fmt.Errorf("shares (%d) must be >= threshold (%d)", n, k)
	}
	if n > 255 {
		return nil, fmt.Errorf("shares must be <= 255, got %d", n)
	}
	if len(secret) == 0 {
		return nil, fmt.Errorf("secret must not be empty")
	}

	shares := make([][]byte, n)
	for i := range shares {
		shares[i] = make([]byte, len(secret)+1)
		shares[i][0] = uint8(i + 1) // x-coordinate: 1..n
	}

	// For each byte of the secret, generate a random polynomial of degree k-1
	// and evaluate at x=1..n
	coeffs := make([]byte, k-1)
	for byteIdx, secretByte := range secret {
		// Random coefficients for degree 1..k-1
		if _, err := rand.Read(coeffs); err != nil {
			return nil, fmt.Errorf("failed to generate random coefficients: %w", err)
		}

		for i := 0; i < n; i++ {
			x := uint8(i + 1)
			// Evaluate polynomial: secret + c1*x + c2*x^2 + ... + c_{k-1}*x^{k-1}
			y := secretByte
			xPow := x
			for _, c := range coeffs {
				y ^= gfMul(c, xPow) // GF addition is XOR
				xPow = gfMul(xPow, x)
			}
			shares[i][byteIdx+1] = y
		}
	}

	return shares, nil
}

// Combine reconstructs the original secret from k or more shares using
// Lagrange interpolation over GF(2^8).
func Combine(shares [][]byte) ([]byte, error) {
	if len(shares) < 2 {
		return nil, fmt.Errorf("need at least 2 shares, got %d", len(shares))
	}

	// All shares must be the same length
	shareLen := len(shares[0])
	for i, s := range shares {
		if len(s) != shareLen {
			return nil, fmt.Errorf("share %d has length %d, expected %d", i, len(s), shareLen)
		}
		if shareLen < 2 {
			return nil, fmt.Errorf("share %d is too short", i)
		}
	}

	secretLen := shareLen - 1
	secret := make([]byte, secretLen)

	// Extract x-coordinates
	xs := make([]uint8, len(shares))
	for i, s := range shares {
		xs[i] = s[0]
	}

	// Lagrange interpolation at x=0 for each byte position
	for byteIdx := 0; byteIdx < secretLen; byteIdx++ {
		var value uint8
		for i := 0; i < len(shares); i++ {
			yi := shares[i][byteIdx+1]
			// Compute Lagrange basis polynomial L_i(0)
			var basis uint8 = 1
			for j := 0; j < len(shares); j++ {
				if i == j {
					continue
				}
				// L_i(0) *= (0 - x_j) / (x_i - x_j) = x_j / (x_i ^ x_j)
				num := xs[j]
				den := xs[i] ^ xs[j] // GF subtraction = XOR
				if den == 0 {
					return nil, fmt.Errorf("duplicate share x-coordinate: %d", xs[i])
				}
				basis = gfMul(basis, gfDiv(num, den))
			}
			value ^= gfMul(yi, basis) // GF addition = XOR
		}
		secret[byteIdx] = value
	}

	return secret, nil
}
