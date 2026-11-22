package kernels

import (
	"math"
	"math/rand"
	"testing"
)

// axpy (floats.Axpy) is validated against a math.FMA oracle, bit for bit on
// every architecture, across every length residue so the vector body and the
// scalar tail are all exercised.

func randPlane(n int, seed int64) []float64 {
	r := rand.New(rand.NewSource(seed))
	a := make([]float64, n)
	for i := range a {
		a[i] = math.Floor(r.Float64() * 256) // uint8-like values
	}
	return a
}

func TestAxpySIMD(t *testing.T) {
	r := rand.New(rand.NewSource(7))
	for _, n := range []int{0, 1, 2, 3, 4, 5, 7, 8, 9, 15, 16, 31, 64, 100, 511, 4096} {
		src := randPlane(n, int64(n)+1)
		a := r.NormFloat64()
		got := make([]float64, n)
		want := make([]float64, n)
		copy(got, randPlane(n, int64(n)+2))
		copy(want, got)
		axpy(got, src, a)
		for i := range want {
			want[i] = math.FMA(a, src[i], want[i])
			if got[i] != want[i] {
				t.Fatalf("axpy n=%d [%d]: %.17g vs %.17g", n, i, got[i], want[i])
			}
		}
	}
}

func BenchmarkAxpy(b *testing.B) {
	n := 1 << 12
	dst := randPlane(n, 1)
	src := randPlane(n, 2)
	for i := 0; i < b.N; i++ {
		axpy(dst, src, 0.25)
	}
}
