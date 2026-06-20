package kernels

import (
	"runtime"
	"testing"
)

// TestNumWorkers exercises the worker-count clamps: the GOMAXPROCS cap, the
// one-worker-per-line cap, and the floor of 1.
func TestNumWorkers(t *testing.T) {
	old := runtime.GOMAXPROCS(0)
	defer runtime.GOMAXPROCS(old)

	runtime.GOMAXPROCS(4)
	if w := numWorkers(2); w != 2 { // fewer lines than cores -> capped to lines
		t.Errorf("numWorkers(2) = %d, want 2", w)
	}
	if w := numWorkers(100); w != 4 { // many lines -> capped to GOMAXPROCS
		t.Errorf("numWorkers(100) = %d, want 4", w)
	}

	runtime.GOMAXPROCS(1)
	if w := numWorkers(100); w != 1 {
		t.Errorf("numWorkers with GOMAXPROCS=1 = %d, want 1", w)
	}
}

// TestParallelLinesSerial covers the w<=1 serial body path of parallelLines and
// the multi-band path, checking every line index is visited exactly once.
func TestParallelLines(t *testing.T) {
	for _, w := range []int{1, 2, 3, 8} {
		const n = 17
		hits := make([]int, n)
		parallelLines(n, w, func(lo, hi int) {
			for i := lo; i < hi; i++ {
				hits[i]++
			}
		})
		for i, c := range hits {
			if c != 1 {
				t.Fatalf("w=%d line %d visited %d times", w, i, c)
			}
		}
	}
}

// TestForLinesThreshold covers both branches of forLines: serial when the work
// is below ParThreshold and parallel above it.
func TestForLinesThreshold(t *testing.T) {
	withThreshold(1<<30, func() { // force serial
		var calls int
		forLines(10, 10, func(lo, hi int) {
			calls++
			if lo != 0 || hi != 10 {
				t.Fatalf("serial band [%d,%d), want [0,10)", lo, hi)
			}
		})
		if calls != 1 {
			t.Fatalf("serial forLines made %d calls, want 1", calls)
		}
	})
	withThreshold(1, func() { // force parallel
		hits := make([]int, 10)
		forLines(10, 10, func(lo, hi int) {
			for i := lo; i < hi; i++ {
				hits[i]++
			}
		})
		for i, c := range hits {
			if c != 1 {
				t.Fatalf("parallel line %d visited %d times", i, c)
			}
		}
	})
}
