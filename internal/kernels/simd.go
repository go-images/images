package kernels

// SIMD dispatch core.
//
// The hot separable kernels (Gaussian convolution, grayscale morphology) are
// expressed over contiguous float64 channel planes so their inner loops map
// directly onto vector registers. This file holds the scalar oracles those
// kernels reduce to; the per-architecture files (simd_amd64.go, simd_arm64.go,
// simd_s390x.go, simd_generic.go) bind the three dispatch points — axpy,
// vminInto, vmaxInto — to a go-asmgen SIMD kernel where one is shipped and to
// these oracles everywhere else. The result is identical on every target: the
// SIMD kernels are validated bit-for-bit against the oracles (the per-element
// computation is the same; only the lane width differs, and on finite,
// uint8-derived data there are no NaN or signed-zero edge cases).

// axpyScalar computes dst[i] += a*src[i] for every i, the multiply-accumulate at
// the heart of the separable Gaussian pass (one kernel tap weight a applied to a
// whole shifted line). It is the oracle the SIMD axpy kernels reproduce.
//
// The single fused multiply-add form dst[i] = dst[i] + a*src[i] is exactly what
// the gc compiler emits for this loop on FMA-baseline targets (amd64-v3, arm64,
// s390x), so the vector kernels use the corresponding packed FMA and stay
// bit-identical to this oracle.
func axpyScalar(dst, src []float64, a float64) {
	for i := range dst {
		dst[i] += a * src[i]
	}
}

// vminScalar / vmaxScalar compute the elementwise minimum / maximum of dst and
// src into dst (dst[i] = min/max(dst[i], src[i])), the per-window reduction step
// of the separable morphology pass. They are the oracles the SIMD min/max
// kernels reproduce. Morphology runs on finite uint8-derived values, so there
// are no NaN or signed-zero cases where packed MIN/MAX would diverge from the
// scalar comparison.
func vminScalar(dst, src []float64) {
	for i := range dst {
		if src[i] < dst[i] {
			dst[i] = src[i]
		}
	}
}

func vmaxScalar(dst, src []float64) {
	for i := range dst {
		if src[i] > dst[i] {
			dst[i] = src[i]
		}
	}
}
