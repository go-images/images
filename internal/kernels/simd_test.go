package kernels

import (
	"math"
	"math/rand"
	"testing"
)

// The SIMD dispatch points (axpy, vminInto, vmaxInto) are validated here against
// their scalar oracles (axpyScalar, vminScalar, vmaxScalar) across every length
// residue, so the vector body, the second packed pair, and the scalar tail are
// all exercised. On architectures without a SIMD kernel the dispatch aliases the
// oracle, so the comparison trivially holds and still covers the dispatch.
//
// vmin/vmax are held bit-identical (the per-element op is the same on the finite
// values morphology uses). axpy is held bit-identical at the amd64-v1 / arm64
// baselines (the scalar oracle there is MULPD+ADDPD / FMA, which the kernel
// reproduces) and to a tight relative tolerance otherwise, because a v3 build's
// scalar oracle fuses to VFMADD — a <=0.5 ULP regrouping the kernel does not.

func randPlane(n int, seed int64) []float64 {
	r := rand.New(rand.NewSource(seed))
	a := make([]float64, n)
	for i := range a {
		a[i] = math.Floor(r.Float64() * 256) // uint8-like values
	}
	return a
}

func TestAxpySIMD(t *testing.T) {
	t.Logf("HaveSIMD = %v (%s)", HaveSIMD, SIMDName)
	r := rand.New(rand.NewSource(7))
	for _, n := range []int{0, 1, 2, 3, 4, 5, 7, 8, 9, 15, 16, 31, 64, 100, 511, 4096} {
		src := randPlane(n, int64(n)+1)
		a := r.NormFloat64()
		got := make([]float64, n)
		want := make([]float64, n)
		copy(got, randPlane(n, int64(n)+2))
		copy(want, got)
		axpy(got, src, a)
		axpyScalar(want, src, a)
		for i := range want {
			if got[i] != want[i] {
				// Allow a tight relative tolerance for the FMA-fusion mismatch a
				// GOAMD64=v3 build can introduce; bit-identical at the baseline.
				tol := 1e-13 * (math.Abs(want[i]) + 1)
				if math.Abs(got[i]-want[i]) > tol {
					t.Fatalf("axpy n=%d [%d]: %.17g vs %.17g", n, i, got[i], want[i])
				}
			}
		}
	}
}

func TestMinMaxSIMD(t *testing.T) {
	for _, n := range []int{0, 1, 2, 3, 4, 5, 7, 8, 9, 15, 16, 31, 64, 100, 511, 4096} {
		dst := randPlane(n, int64(n)*3+1)
		src := randPlane(n, int64(n)*3+2)

		gotMin := append([]float64(nil), dst...)
		wantMin := append([]float64(nil), dst...)
		vminInto(gotMin, src)
		vminScalar(wantMin, src)
		for i := range wantMin {
			if gotMin[i] != wantMin[i] {
				t.Fatalf("vmin n=%d [%d]: %v vs %v", n, i, gotMin[i], wantMin[i])
			}
		}

		gotMax := append([]float64(nil), dst...)
		wantMax := append([]float64(nil), dst...)
		vmaxInto(gotMax, src)
		vmaxScalar(wantMax, src)
		for i := range wantMax {
			if gotMax[i] != wantMax[i] {
				t.Fatalf("vmax n=%d [%d]: %v vs %v", n, i, gotMax[i], wantMax[i])
			}
		}
	}
}

// TestAxpyEmpty / TestMinMaxEmpty cover the len==0 guard in the dispatch
// wrappers (the kernels are never called with a nil base pointer).
func TestSIMDEmpty(t *testing.T) {
	axpy(nil, nil, 1.5)
	vminInto(nil, nil)
	vmaxInto(nil, nil)
}

func BenchmarkAxpy(b *testing.B) {
	n := 1 << 12
	dst := randPlane(n, 1)
	src := randPlane(n, 2)
	b.Run(SIMDName, func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			axpy(dst, src, 0.25)
		}
	})
	b.Run("scalar", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			axpyScalar(dst, src, 0.25)
		}
	})
}
