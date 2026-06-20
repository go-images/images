package kernels

import (
	"runtime"
	"sync"
)

// Multicore tiling.
//
// scikit-image / scipy.ndimage run single-threaded SIMD C, so the durable way a
// pure-Go library beats them on large images is to spread the work across CPU
// cores in addition to vectorising the inner loop. The parallel helpers here
// partition the rows (or columns) of an image into one contiguous band per
// worker and run the SAME per-line kernels on each band, so correctness is
// inherited unchanged from the serial path and the result is independent of the
// worker count. A size threshold keeps small images on the serial path, where
// goroutine scheduling would dominate the work.

// ParThreshold is the minimum number of output pixels at which the separable
// passes fan out across goroutines. Below it the serial path runs, because the
// goroutine launch/join cost exceeds the work saved. It is a var so tests can
// force the parallel path on small images (and pin it for deterministic
// coverage).
var ParThreshold = 1 << 14 // 16384 pixels (e.g. 128x128)

// numWorkers returns the worker count for n independent lines: at most
// GOMAXPROCS (always >= 1) and never more than n (one worker per line), so a
// short pass does not spawn idle goroutines. Callers pass n >= 1.
func numWorkers(n int) int {
	w := runtime.GOMAXPROCS(0) // always >= 1
	if w > n {                 // never more workers than independent lines
		w = n
	}
	return w
}

// parallelLines splits the line indices [0,n) into w contiguous, near-equal
// bands and runs body on each band in its own goroutine, waiting for all to
// finish. body must be safe to call concurrently on disjoint [lo,hi) line
// ranges (the separable passes write disjoint output rows/columns, so they are).
func parallelLines(n, w int, body func(lo, hi int)) {
	if w <= 1 {
		body(0, n)
		return
	}
	chunk := (n + w - 1) / w
	var wg sync.WaitGroup
	for lo := 0; lo < n; lo += chunk {
		hi := lo + chunk
		if hi > n {
			hi = n
		}
		wg.Add(1)
		go func(lo, hi int) {
			defer wg.Done()
			body(lo, hi)
		}(lo, hi)
	}
	wg.Wait()
}

// forLines runs body over the line range [0,n), fanning out across goroutines
// when the work (n lines of `lineWork` elements each) crosses ParThreshold and
// running serially otherwise. body(lo,hi) must process the half-open band of
// lines [lo,hi). The output of a run is identical regardless of the worker
// count, because each band writes a disjoint region.
func forLines(n, lineWork int, body func(lo, hi int)) {
	if n*lineWork < ParThreshold {
		body(0, n)
		return
	}
	parallelLines(n, numWorkers(n), body)
}
