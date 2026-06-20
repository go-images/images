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

// TestReduceScalar exercises both the min and max branches of reduceScalar for
// all three channels, in both the "source wins" and "accumulator wins" cases.
func TestReduceScalar(t *testing.T) {
	rr := []float64{10, 20, 30}
	rg := []float64{40, 50, 60}
	rb := []float64{70, 80, 90}

	// max: src column 2 (30,60,90) beats accumulator (1,1,1) on every channel.
	ar := []float64{1}
	ag := []float64{1}
	ab := []float64{1}
	reduceScalar(ar, ag, ab, rr, rg, rb, 0, 2, morphMax)
	if ar[0] != 30 || ag[0] != 60 || ab[0] != 90 {
		t.Fatalf("max src-wins: %v %v %v", ar, ag, ab)
	}
	// max: accumulator (99) beats src column 0 (10,40,70).
	ar[0], ag[0], ab[0] = 99, 99, 99
	reduceScalar(ar, ag, ab, rr, rg, rb, 0, 0, morphMax)
	if ar[0] != 99 || ag[0] != 99 || ab[0] != 99 {
		t.Fatalf("max acc-wins: %v %v %v", ar, ag, ab)
	}
	// min: src column 0 (10,40,70) beats accumulator (99).
	ar[0], ag[0], ab[0] = 99, 99, 99
	reduceScalar(ar, ag, ab, rr, rg, rb, 0, 0, morphMin)
	if ar[0] != 10 || ag[0] != 40 || ab[0] != 70 {
		t.Fatalf("min src-wins: %v %v %v", ar, ag, ab)
	}
	// min: accumulator (1) beats src column 2 (30,60,90).
	ar[0], ag[0], ab[0] = 1, 1, 1
	reduceScalar(ar, ag, ab, rr, rg, rb, 0, 2, morphMin)
	if ar[0] != 1 || ag[0] != 1 || ab[0] != 1 {
		t.Fatalf("min acc-wins: %v %v %v", ar, ag, ab)
	}
}
