// Copyright (c) 2026, the go-images authors
// All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

//go:build !race

package ops

import (
	"fmt"
	"image"
	"math"
	"testing"
)

// The two measurements behind the limits documented on [SkewOptions]: which
// resampling filter the pixel budget should reduce a page with, and whether
// the detector can see its own failure when the budget is too small.
//
// ⛔ THEY ARE EXCLUDED FROM THE RACE BUILD, and that is a cost decision with a
// reason rather than an exemption. Neither test starts a goroutine, so the
// race detector has nothing in them to find — while instrumenting the pixel
// loops they drive multiplies this package's runtime by some eighty-five
// times. MEASURED: the whole ops package is seven seconds here without -race
// and over six hundred with it, and CI's race job — which also carries the
// coverage gate — died on `panic: test timed out after 10m0s` with these two
// tests three minutes in and the rest of the package already seven and a half
// minutes deep. The non-race lanes still run them on every push.
//
// ⛔ THE COVERAGE GATE RUNS UNDER -race, so excluding a test must not take
// coverage with it. It does not: searchSkew and skewMaskWith are reached by
// SkewAngle, and skewScore under both criteria by
// TestSkewCriterionIsActuallyWired. Verified by running the gate without this
// file, not assumed.

// skewPlanted is the angle set both measurements use, kept in one place
// because BOTH of their findings depend on which angles are in it.
//
// ⛔ DO NOT TRIM IT TO SAVE TIME. I shortened it to three angles and a finding
// CHANGED: one budget reported 0.000° instead of 4.350°, because the angle
// that failed there was one of the two I had dropped. A collapse that only
// some angles witness is still a collapse.
var skewPlanted = []float64{-7, -2.5, 0, 1.25, 6}

// skewPitch is the distance between the text lines [textPage] draws, which is
// the quantity the budget measurements are about.
const skewPitch = 6 + 8

// A skewCorpus is a page, its tilted copies and the pixel budgets that reduce
// it to a known number of pixels per text line. All three corpora share the
// same five ratios, so a row of one lines up with the same row of the others.
//
// ⛔ The copies are rotated outside the budget and filter loops. The first
// version rotated inside them — fifty rotations of a 2.5-megapixel page — and
// that alone was most of the cost.
type skewCorpus struct {
	name    string
	page    *image.RGBA
	tilted  []*image.RGBA
	budgets []int
}

// ⛔ THREE SIZES, BECAUSE TWO GAVE TWO DIFFERENT ANSWERS. Measured on
// {350×450, 700×900} the confidence statistic below separates sound pages from
// broken ones; measured on {700×900, 1400×1800} it does not. Whether a
// threshold exists depended on which pages were in the sample, which is a
// stronger reason not to build a gate on it than either pair alone.
func skewCorpora(t *testing.T) []skewCorpus {
	t.Helper()
	build := func(name string, w, h int, budgets []int) skewCorpus {
		page := textPage(w, h)
		tilted := make([]*image.RGBA, len(skewPlanted))
		for i, a := range skewPlanted {
			tilted[i] = overWhite(Rotate(page, a, true))
		}
		return skewCorpus{name: name, page: page, tilted: tilted, budgets: budgets}
	}
	// Each corpus is four times the pixels of the one before it, and each
	// budget four times the one above, so every row holds the same pixels per
	// text line across all three.
	return []skewCorpus{
		build("350x450", 350, 450, []int{62_500, 12_500, 6_250, 5_000, 3_125}),
		build("700x900", 700, 900, []int{250_000, 50_000, 25_000, 20_000, 12_500}),
		build("1400x1800", 1400, 1800, []int{1_000_000, 200_000, 100_000, 80_000, 50_000}),
	}
}

func (c skewCorpus) perLine(reducedH int) float64 {
	return float64(skewPitch*reducedH) / float64(c.page.Rect.Dy())
}

// measure returns the worst absolute error over the planted angles at this
// budget, with the reduced dimensions the budget produced.
func (c skewCorpus) measure(t *testing.T, budget int, mode ResizeMode) (worst float64, rw, rh int) {
	t.Helper()
	for i, want := range skewPlanted {
		marks, w, h, err := skewMaskWith(c.tilted[i], budget, mode)
		if err != nil {
			t.Fatalf("%s at %d: %v", c.name, budget, err)
		}
		if e := math.Abs(searchSkew(marks, w, h, SkewOptions{}.withDefaults()) - want); e > worst {
			worst = e
		}
	}
	_, rw, rh, err := skewMaskWith(c.page, budget, mode)
	if err != nil {
		t.Fatalf("%s at %d: %v", c.name, budget, err)
	}
	return worst, rw, rh
}

// theNyquistRow is the index of the 2.5-pixels-per-line budget: just above two
// samples per line pitch, which is where the collapse begins.
const theNyquistRow = 3

// TestMeasureTheDownscaleFilter asks what the filter choice is worth, rather
// than taking the obvious one. Bilinear reads four neighbours whatever the
// scale factor, so a large reduction samples the page sparsely and can step
// over a thin stroke; Area's support grows with the factor.
//
// ⛔ THE ANSWER IS "NOTHING MEASURABLE", and the way that answer arrived is
// the point. On two page sizes the tally was Area better in three rows and
// worse in none, and I wrote that into the code as Area winning. The third
// corpus made it better in three and worse in six. The differences are one
// step of the search, 0.05°, in all but one row — too small to rank two
// filters with, and nine rows were not enough to notice that.
//
// So this asserts the agreement rather than a winner: in the rows where the
// angle is usable at all, the two filters must stay within four search steps
// of each other. If a change ever makes one properly better, this fails and
// the comment on skewDownscale needs rewriting.
func TestMeasureTheDownscaleFilter(t *testing.T) {
	atNyquist := map[string]float64{}
	var areaBetter, areaWorse int
	var widestGap float64
	var widestRow string
	for _, c := range skewCorpora(t) {
		for i, budget := range c.budgets {
			var byMode [2]float64
			for m, mode := range []struct {
				name string
				m    ResizeMode
			}{{"Bilinear", Bilinear}, {"Area", Area}} {
				worst, rw, rh := c.measure(t, budget, mode.m)
				byMode[m] = worst
				t.Logf("%-9s budget %7d (%dx%d, %.1f px per line), %-8s: worst |error| over %d angles %.3f°",
					c.name, budget, rw, rh, c.perLine(rh), mode.name, len(skewPlanted), worst)
				if i == theNyquistRow && mode.m == skewDownscale {
					atNyquist[c.name] = worst
				}
			}
			switch {
			case byMode[1] < byMode[0]:
				areaBetter++
			case byMode[1] > byMode[0]:
				areaWorse++
			}
			// Only the rows whose angle is usable: once the pitch aliases both
			// filters are wrong by sixteen degrees and the gap between two
			// wrong answers means nothing.
			if math.Min(byMode[0], byMode[1]) < 1 {
				if gap := math.Abs(byMode[1] - byMode[0]); gap > widestGap {
					widestGap, widestRow = gap, fmt.Sprintf("%s at %d", c.name, budget)
				}
			}
		}
	}
	t.Logf("Area better in %d rows, worse in %d; widest gap where the angle is usable %.3f° (%s)",
		areaBetter, areaWorse, widestGap, widestRow)
	if widestGap > 0.2 {
		t.Errorf("the two filters now differ by %.3f° at %s, more than four search steps: "+
			"one of them is properly better and the note on skewDownscale should say which",
			widestGap, widestRow)
	}

	// ⛔ THE FINDING THIS ASSERTS IS NOT THE ONE I SET OUT TO MEASURE. I had
	// written that the collapse is about pixels per text line and nothing
	// else, and shrank the pages to make the test cheap — expecting the same
	// figures. At the same 2.5 pixels per line the three sizes disagree by
	// more than an order of magnitude, because the number of ROWS the profile
	// is built from matters as well: more of them make the peak stand out of
	// more evidence. Same ratio, different answer, so the quantity is not
	// sufficient on its own, and the documentation carries a column per size
	// rather than one curve.
	lo, hi := math.Inf(1), math.Inf(-1)
	for _, v := range atNyquist {
		lo, hi = math.Min(lo, v), math.Max(hi, v)
	}
	t.Logf("at 2.5 px per line the three sizes span %.3f° to %.3f°", lo, hi)
	if hi < 4*lo {
		t.Errorf("pixels per text line now predicts the collapse to within a factor of four "+
			"(%.3f° to %.3f° at the same 2.5 px per line): the documented limit can be stated in that one quantity",
			lo, hi)
	}
}

// TestMeasureTheConfidenceOfThePeak asked whether the detector can SEE its own
// failure, and the answer was NO — which is why [SkewOptions.MaxPixels]
// documents a limit instead of the code refusing to cross it.
//
// TestMeasureTheDownscaleFilter shows the measurement collapsing at two
// samples per text line — the Nyquist limit of the line pitch, since a line
// and its gap are one cycle — and returning an angle 16° wrong without
// complaint. The obvious confidence is the ratio of the best score to the mean
// over all candidate angles: a towering peak where the lines are resolvable, a
// flat landscape where they alias.
//
// ⛔ MEASURED, AND IT DOES NOT SEPARATE — once there are three page sizes in
// the sample. On two of them it did, which is the trap: a threshold fitted to
// a pair would have looked principled. Over all three there is a sound budget
// scoring below a broken one, so any cut either rejects sound pages or accepts
// an angle 4° wrong. This test asserts the NON-separation, so that if a change
// to the scoring ever makes the populations pull apart it fails and says a
// gate has become possible.
func TestMeasureTheConfidenceOfThePeak(t *testing.T) {
	opt := SkewOptions{}.withDefaults()
	lowestSound, highestBroken := math.Inf(1), math.Inf(-1)
	for _, c := range skewCorpora(t) {
		for _, budget := range c.budgets {
			var worstErr float64
			lowestRatio := math.Inf(1)
			for i, want := range skewPlanted {
				marks, w, h, err := skewMaskWith(c.tilted[i], budget, skewDownscale)
				if err != nil {
					t.Fatal(err)
				}
				var sum, best float64
				n := 0
				for a := -opt.MaxAngle; a <= opt.MaxAngle+1e-9; a += opt.Coarse {
					s := skewScore(marks, w, h, a, opt)
					sum += s
					best = math.Max(best, s)
					n++
				}
				lowestRatio = math.Min(lowestRatio, best/(sum/float64(n)))
				if e := math.Abs(searchSkew(marks, w, h, opt) - want); e > worstErr {
					worstErr = e
				}
			}
			// Sound means an angle still worth applying; above a degree the
			// correction would be worse than leaving the page alone.
			sound := worstErr < 1.0
			t.Logf("%-9s budget %7d: worst |error| %6.3f° (%s), lowest peak/mean %5.2f",
				c.name, budget, worstErr, map[bool]string{true: "sound", false: "BROKEN"}[sound], lowestRatio)
			if sound {
				lowestSound = math.Min(lowestSound, lowestRatio)
			} else {
				highestBroken = math.Max(highestBroken, lowestRatio)
			}
		}
	}
	t.Logf("over all three corpora: lowest ratio among sound budgets %.2f, highest among broken ones %.2f",
		lowestSound, highestBroken)
	if lowestSound > highestBroken {
		t.Errorf("peak/mean now SEPARATES sound from broken across every page size (%.2f > %.2f): "+
			"a confidence gate has become possible, and MaxPixels should stop documenting a limit it could refuse",
			lowestSound, highestBroken)
	}
}
