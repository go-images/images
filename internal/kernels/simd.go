package kernels

import "github.com/go-simd/floats"

// axpy computes dst[i] += a*src[i] for i < len(src), the multiply-accumulate at
// the heart of the separable Gaussian pass (one kernel tap weight a applied to a
// whole shifted line). It is floats.Axpy (github.com/go-simd/floats), the one
// shared implementation: AVX2 FMA on amd64, NEON FMLA on arm64, portable
// math.FMA elsewhere. It is fused (one rounding per element) and bit-identical
// on every architecture. src must not be longer than dst.
func axpy(dst, src []float64, a float64) {
	floats.Axpy(a, src, dst)
}
