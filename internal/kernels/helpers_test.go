package kernels

import "testing"

// TestInRangeSpan covers every clamp branch of inRangeSpan, including shifts
// larger than the line width in both directions (where the in-range span is
// empty and lo/hi are clamped back into [0,width]).
func TestInRangeSpan(t *testing.T) {
	cases := []struct {
		t, width int
		lo, hi   int
	}{
		{0, 5, 0, 5},  // no shift: whole line in range
		{-2, 5, 2, 5}, // left border of width 2
		{2, 5, 0, 3},  // right border of width 2
		{-9, 5, 5, 5}, // shift past start: lo clamped to width, empty span
		{9, 5, 0, 0},  // shift past end: hi clamped to 0, empty span
		{-5, 5, 5, 5}, // exactly width to the left: empty
		{5, 5, 0, 0},  // exactly width to the right: empty
	}
	for _, c := range cases {
		lo, hi := inRangeSpan(c.t, c.width)
		if lo != c.lo || hi != c.hi {
			t.Errorf("inRangeSpan(%d,%d) = (%d,%d), want (%d,%d)", c.t, c.width, lo, hi, c.lo, c.hi)
		}
		if lo < 0 || hi > c.width || lo > hi {
			t.Errorf("inRangeSpan(%d,%d) = (%d,%d) out of [0,%d] or lo>hi", c.t, c.width, lo, hi, c.width)
		}
	}
}

// vanHerkOracle is a brute-force O(n*k) windowed extremum with clamp-to-edge
// borders, used to validate the O(1) van Herk implementation directly.
func vanHerkOracle(sig []uint8, radius int, op morphOp) []uint8 {
	n := len(sig)
	out := make([]uint8, n)
	clamp := func(i int) uint8 {
		if i < 0 {
			i = 0
		}
		if i >= n {
			i = n - 1
		}
		return sig[i]
	}
	for x := 0; x < n; x++ {
		acc := clamp(x - radius)
		for t := -radius + 1; t <= radius; t++ {
			v := clamp(x + t)
			if op == morphMax {
				if v > acc {
					acc = v
				}
			} else if v < acc {
				acc = v
			}
		}
		out[x] = acc
	}
	return out
}

// TestVanHerk1D validates the O(1) running min/max against a brute-force oracle
// across signal lengths, radii (exercising full and partial trailing blocks),
// the empty signal, and both reduction directions.
func TestVanHerk1D(t *testing.T) {
	seeds := [][]uint8{
		{},
		{42},
		{5, 1, 9, 3, 7, 2, 8, 4, 6, 0},
		{255, 0, 255, 0, 255, 0, 255},
		{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13},
	}
	for si, sig := range seeds {
		for _, radius := range []int{1, 2, 3, 4, 5} {
			for _, op := range []morphOp{morphMin, morphMax} {
				var sc morphScratch
				sc.ensure(len(sig) + 2*radius)
				sc.fillPad(len(sig), radius, func(i int) uint8 { return sig[i] })
				got := make([]uint8, len(sig))
				sc.vanHerk1D(got, len(sig), radius, op)
				want := vanHerkOracle(sig, radius, op)
				for i := range want {
					if got[i] != want[i] {
						t.Fatalf("seed %d radius %d op %v: at %d got %d want %d",
							si, radius, op, i, got[i], want[i])
					}
				}
			}
		}
	}
}

// TestMorphScratchEnsureReuse exercises both the grow and the in-place reuse
// branches of morphScratch.ensure.
func TestMorphScratchEnsureReuse(t *testing.T) {
	var sc morphScratch
	sc.ensure(8)
	if len(sc.pad) != 8 || len(sc.pref) != 8 || len(sc.suf) != 8 {
		t.Fatalf("ensure(8): lengths %d %d %d", len(sc.pad), len(sc.pref), len(sc.suf))
	}
	// Shrink-and-reuse: smaller request keeps the backing array.
	sc.ensure(4)
	if len(sc.pad) != 4 || cap(sc.pad) < 8 {
		t.Fatalf("ensure(4) reuse: len %d cap %d", len(sc.pad), cap(sc.pad))
	}
	// Grow: larger request reallocates.
	sc.ensure(16)
	if len(sc.pad) != 16 || len(sc.pref) != 16 || len(sc.suf) != 16 {
		t.Fatalf("ensure(16): lengths %d %d %d", len(sc.pad), len(sc.pref), len(sc.suf))
	}
}
