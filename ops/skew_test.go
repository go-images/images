// Copyright (c) 2026, the go-images authors
// All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package ops

import (
	"errors"
	"image"
	"image/color"
	"math"
	"testing"
)

// The ground truth here is PLANTED, not borrowed: a synthetic page of text
// lines is rotated by an angle we chose, and the question is what comes back.
// That is the one measurement a skew detector can be held to exactly — there is
// no corpus whose "true" angles are known to a twentieth of a degree.
//
// [TestSkewAgainstTesseract] adds the other half: an independent judge
// (tesseract, which knows nothing of this package) reading the text off the
// page before and after, which measures the thing a caller actually wants.

// textPage draws a page of text-like lines: dark bars on white paper, in
// blocks, with ragged right edges and inter-word gaps, so that the projection
// profile has the peaks and troughs a real page has rather than a single
// uniform block.
func textPage(w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for i := range img.Pix {
		img.Pix[i] = 0xff
	}
	ink := color.RGBA{0x14, 0x14, 0x14, 0xff}

	margin := w / 10
	lineH, gap := 6, 8
	// A deterministic pseudo-random sequence: the page must be the same page
	// on every run, or a flaky recovery would be unattributable.
	seed := uint32(0x5EED)
	next := func(n int) int {
		seed = seed*1664525 + 1013904223
		return int(seed>>16) % n
	}
	for y := margin; y+lineH < h-margin; y += lineH + gap {
		right := w - margin - next(w/6)
		for x := margin; x < right; {
			word := 3 + next(9)
			for dx := 0; dx < word && x < right; dx, x = dx+1, x+1 {
				for dy := 0; dy < lineH; dy++ {
					img.Set(x, y+dy, ink)
				}
			}
			x += 2 + next(3) // the space between words
		}
	}
	return img
}

// overWhite composites img onto opaque white paper. [Rotate] leaves the
// corners it did not cover transparent; a scanner hands back paper there, so
// the test must too — otherwise it would be measuring the detector's handling
// of transparency, which TestSkewTransparentMarginIsPaper does on purpose.
func overWhite(img image.Image) *image.RGBA {
	b := img.Bounds()
	out := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			r, g, bl, a := img.At(b.Min.X+x, b.Min.Y+y).RGBA()
			inv := 0xffff - a
			out.Set(x, y, color.RGBA64{uint16(r + inv), uint16(g + inv), uint16(bl + inv), 0xffff})
		}
	}
	return out
}

// plantedAngles spans the search: zero, a fraction of a degree (below the
// coarse step, so only the fine pass can find it), and both signs out to the
// tilt a page gets from being dropped into a scanner.
var plantedAngles = []float64{-8, -5, -2.5, -0.75, 0, 0.75, 2.5, 5, 8}

func TestSkewAngleRecoversAPlantedAngle(t *testing.T) {
	page := textPage(600, 800)
	for _, criterion := range []SkewCriterion{Variance, AdjacentDifference} {
		name := map[SkewCriterion]string{Variance: "Variance", AdjacentDifference: "AdjacentDifference"}[criterion]
		t.Run(name, func(t *testing.T) {
			for _, want := range plantedAngles {
				tilted := overWhite(Rotate(page, want, true))
				got, err := SkewAngle(tilted, SkewOptions{Criterion: criterion})
				if err != nil {
					t.Fatalf("planted %+.2f°: %v", want, err)
				}
				// The fine step is 0.05°, so the best achievable is half a
				// step away from the truth; 0.15° leaves room for the
				// resampling the rotation itself did.
				if math.Abs(got-want) > 0.15 {
					t.Errorf("planted %+.2f°, recovered %+.2f° (off by %.2f°)", want, got, got-want)
				} else {
					t.Logf("planted %+.2f°, recovered %+.2f° (off by %+.3f°)", want, got, got-want)
				}
			}
		})
	}
}

// TestSkewCriteriaCompared reports which criterion recovers planted angles
// better ON OUR PAGES. The survey (Mascaro et al. 2010) ranks Baird's variance
// first on their corpus; that is a fact about their corpus, so it is measured
// here rather than inherited.
func TestSkewCriteriaCompared(t *testing.T) {
	page := textPage(600, 800)
	worst := map[SkewCriterion]float64{}
	var total = map[SkewCriterion]float64{}
	for _, criterion := range []SkewCriterion{Variance, AdjacentDifference} {
		for _, want := range plantedAngles {
			tilted := overWhite(Rotate(page, want, true))
			got, err := SkewAngle(tilted, SkewOptions{Criterion: criterion})
			if err != nil {
				t.Fatalf("planted %+.2f°: %v", want, err)
			}
			e := math.Abs(got - want)
			total[criterion] += e
			if e > worst[criterion] {
				worst[criterion] = e
			}
		}
	}
	n := float64(len(plantedAngles))
	t.Logf("Variance:           mean |error| %.3f°, worst %.3f°", total[Variance]/n, worst[Variance])
	t.Logf("AdjacentDifference: mean |error| %.3f°, worst %.3f°", total[AdjacentDifference]/n, worst[AdjacentDifference])
	if worst[Variance] > 0.15 {
		t.Errorf("the default criterion is the worse one here: Variance worst %.3f°", worst[Variance])
	}
}

// TestSkewCriterionIsActuallyWired is the control for the test above. Both
// criteria recover every planted angle to the same step, which is the reading
// one would expect of two sound detectors — and is ALSO exactly what a
// [SkewOptions.Criterion] field that nothing consults would produce. The two
// readings are indistinguishable from the angles alone, so this asserts on the
// score itself: the two criteria must put different numbers on the same
// profile.
func TestSkewCriterionIsActuallyWired(t *testing.T) {
	page := textPage(300, 400)
	marks, w, h, err := skewMask(page, 2_000_000)
	if err != nil {
		t.Fatal(err)
	}
	v := skewScore(marks, w, h, 3, SkewOptions{Criterion: Variance})
	d := skewScore(marks, w, h, 3, SkewOptions{Criterion: AdjacentDifference})
	if v == d {
		t.Fatalf("both criteria scored %v: the Criterion field is not being read", v)
	}
	t.Logf("same profile at +3°: Variance %.1f, AdjacentDifference %.1f", v, d)
}

// TestSkewAngleOnANoisyPage is the other thing a clean synthetic page cannot
// show. A scan has speckle, grey paper and soft edges; a detector that only
// works on bars drawn at full black is not a detector of scanned pages.
func TestSkewAngleOnANoisyPage(t *testing.T) {
	page := textPage(600, 800)
	// Grey paper, grey ink, and salt-and-pepper speckle over both.
	seed := uint32(0xC0FFEE)
	for i := 0; i < len(page.Pix); i += 4 {
		seed = seed*1664525 + 1013904223
		n := int(seed>>16) % 1000
		v := int(page.Pix[i])
		switch {
		case v > 128: // paper: grey, not white
			v = 228
		default: // ink: grey, not black
			v = 60
		}
		if n < 30 { // 3% speckle, either way
			if n%2 == 0 {
				v = 0
			} else {
				v = 255
			}
		}
		page.Pix[i], page.Pix[i+1], page.Pix[i+2] = uint8(v), uint8(v), uint8(v)
	}
	for _, want := range []float64{-4.5, 1.25, 7} {
		tilted := overWhite(Rotate(page, want, true))
		got, err := SkewAngle(tilted, SkewOptions{})
		if err != nil {
			t.Fatalf("planted %+.2f°: %v", want, err)
		}
		if math.Abs(got-want) > 0.25 {
			t.Errorf("noisy page, planted %+.2f°, recovered %+.2f°", want, got)
		} else {
			t.Logf("noisy page, planted %+.2f°, recovered %+.2f° (off by %+.3f°)", want, got, got-want)
		}
	}
}

func TestSkewAngleIsRotatesConvention(t *testing.T) {
	// ⛔ A SIGN ERROR IS INVISIBLE TO A SYMMETRIC TEST: recovering 5° from a
	// page tilted by -5° passes any check written on |error| over a symmetric
	// set of angles, and doubles the skew when Deskew turns the page. This
	// asserts the sign on its own, against the rotation that produced it.
	page := textPage(400, 520)
	got, err := SkewAngle(overWhite(Rotate(page, 6, true)), SkewOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if got <= 0 {
		t.Fatalf("Rotate(page, +6) must measure positive, got %+.2f° — the sign is inverted", got)
	}
}

func TestDeskewPutsThePageStraight(t *testing.T) {
	page := textPage(600, 800)
	for _, planted := range []float64{-6.5, 3.25} {
		tilted := overWhite(Rotate(page, planted, true))
		out, angle, err := Deskew(tilted, 0.1, SkewOptions{})
		if err != nil {
			t.Fatalf("planted %+.2f°: %v", planted, err)
		}
		if math.Abs(angle-planted) > 0.15 {
			t.Errorf("planted %+.2f°, reported %+.2f°", planted, angle)
		}
		// The measurement that matters: what is left on the page afterwards.
		residual, err := SkewAngle(overWhite(out), SkewOptions{})
		if err != nil {
			t.Fatalf("re-measuring the deskewed page: %v", err)
		}
		if math.Abs(residual) > 0.2 {
			t.Errorf("planted %+.2f°: %+.2f° of skew is still on the page", planted, residual)
		} else {
			t.Logf("planted %+.2f° → reported %+.2f°, residual %+.3f°", planted, angle, residual)
		}
	}
}

func TestDeskewLeavesALevelPageAlone(t *testing.T) {
	page := textPage(300, 400)
	out, angle, err := Deskew(page, 0.5, SkewOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(angle) >= 0.5 {
		t.Fatalf("a page drawn level measured %+.2f°", angle)
	}
	// Untouched means byte-identical, not merely close: a page that is already
	// straight must not come back softened by a resample.
	want := ToRGBA(page)
	for y := 0; y < 400; y++ {
		for x := 0; x < 300; x++ {
			i := want.PixOffset(x, y)
			j := out.PixOffset(x, y)
			if out.Pix[j] != want.Pix[i] || out.Pix[j+3] != want.Pix[i+3] {
				t.Fatalf("pixel (%d,%d) changed: %v vs %v", x, y, out.Pix[j:j+4], want.Pix[i:i+4])
			}
		}
	}
}

func TestSkewTransparentMarginIsPaper(t *testing.T) {
	// ⛔ THE CONTROL FOR THE PREMULTIPLIED-ALPHA TRAP. Rotate leaves the
	// corners transparent, which in premultiplied RGBA is byte-for-byte black.
	// Reading those as ink puts two large wedges of "text" in the corners, and
	// the wedges are far bigger than the page's own lines, so the measurement
	// becomes a measurement of the wedges. Feeding Rotate's output straight
	// back in — with no white composite — must still find the planted angle.
	page := textPage(600, 800)
	tilted := Rotate(page, 4.5, true) // transparent corners, deliberately
	got, err := SkewAngle(tilted, SkewOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(got-4.5) > 0.15 {
		t.Fatalf("planted +4.50°, recovered %+.2f° from a page with transparent corners", got)
	}
}

func TestSkewAngleRejectsAPageWithNothingOnIt(t *testing.T) {
	blank := image.NewRGBA(image.Rect(0, 0, 50, 50))
	for i := range blank.Pix {
		blank.Pix[i] = 0xff
	}
	if _, err := SkewAngle(blank, SkewOptions{}); !errors.Is(err, ErrSkewBlank) {
		t.Fatalf("a blank page: got %v, want ErrSkewBlank", err)
	}
	// Uniformly black is just as empty of structure, and must not report 0°.
	solid := image.NewRGBA(image.Rect(0, 0, 50, 50))
	for i := 3; i < len(solid.Pix); i += 4 {
		solid.Pix[i] = 0xff
	}
	if _, err := SkewAngle(solid, SkewOptions{}); !errors.Is(err, ErrSkewBlank) {
		t.Fatalf("a solid black page: got %v, want ErrSkewBlank", err)
	}
}

func TestSkewAngleRejectsAnEmptyImage(t *testing.T) {
	if _, err := SkewAngle(image.NewRGBA(image.Rect(0, 0, 0, 0)), SkewOptions{}); !errors.Is(err, ErrSkewEmpty) {
		t.Fatal("an image with no pixels must report ErrSkewEmpty")
	}
}

func TestSkewAngleBoundsTheWorkingImage(t *testing.T) {
	// A small budget forces the scale-down path; the angle must survive it,
	// because that path is what a 100-megapixel scan takes.
	page := textPage(600, 800)
	tilted := overWhite(Rotate(page, -3.5, true))
	got, err := SkewAngle(tilted, SkewOptions{MaxPixels: 40_000})
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(got+3.5) > 0.3 {
		t.Fatalf("planted -3.50°, recovered %+.2f° from a 40 000-pixel budget", got)
	}
	t.Logf("scaled from %d to at most 40 000 pixels, recovered %+.2f°", 600*800, got)
}

func TestSkewAngleStaysInsideTheSearch(t *testing.T) {
	// ⛔ THE CASE THAT FINDS THE UNCLAMPED REFINEMENT, and the reason it is
	// planted just OUTSIDE MaxAngle rather than far outside. The coarse pass
	// then puts its best on the boundary, because that is the closest it is
	// allowed to the truth — and a refinement of best±Coarse from there runs a
	// full coarse step beyond the bound. An angle planted in the middle of the
	// range cannot see this: the refinement never touches an edge, which is why
	// the first version of this test passed over the bug and only a mutant
	// elsewhere exposed it.
	page := textPage(400, 520)
	for _, tilt := range []float64{2.5, -2.5, 9, -9} {
		tilted := overWhite(Rotate(page, tilt, true))
		got, err := SkewAngle(tilted, SkewOptions{MaxAngle: 2})
		if err != nil {
			t.Fatal(err)
		}
		if got < -2 || got > 2 {
			t.Errorf("tilted %+.2f° with MaxAngle 2: returned %+.3f°, outside the range asked for", tilt, got)
		}
	}
}

// TestSkewScoreIgnoresThePadding measures the bias the padded variance would
// carry, rather than leaving the claim in a comment. Both formulas are computed
// here from the same profile: the one the code uses, over the page's own rows,
// and the one over the whole padded profile.
func TestSkewScoreIgnoresThePadding(t *testing.T) {
	marks, w, h, err := skewMask(textPage(400, 520), 2_000_000)
	if err != nil {
		t.Fatal(err)
	}
	// variance over the page's rows, and over the padded profile, at one angle
	both := func(angleDeg float64) (page, padded float64) {
		tan := math.Tan(angleDeg * math.Pi / 180)
		pad := int(math.Abs(tan)*float64(w)) + 2
		n := h + 2*pad
		profile := make([]int, n)
		for _, p := range marks {
			profile[pad+int(p.y)+int(float64(p.x)*tan)]++
		}
		variance := func(from, count int) float64 {
			var sum, sumSq float64
			for i := from; i < from+count; i++ {
				v := float64(profile[i])
				sum += v
				sumSq += v * v
			}
			mean := sum / float64(count)
			return sumSq/float64(count) - mean*mean
		}
		return variance(pad, h), variance(0, n)
	}

	// The page is level, so 0° is the truth and every other angle must score
	// below it under a sound criterion.
	page0, padded0 := both(0)
	page8, padded8 := both(8)
	t.Logf("level page: over the page's rows  0°=%.1f  8°=%.1f (ratio %.3f)", page0, page8, page8/page0)
	t.Logf("level page: over the padded profile 0°=%.1f  8°=%.1f (ratio %.3f)", padded0, padded8, padded8/padded0)

	if page8 >= page0 {
		t.Errorf("a level page scores %.1f at 8° and only %.1f at 0°", page8, page0)
	}
	// The claim in the comment, and the reason it is a MEASUREMENT: counting
	// the padding leaves the wrong angle LESS penalised — it flattens the
	// search rather than sharpening it. I first reasoned my way to the
	// opposite sign and this assertion is what corrected it.
	if padded8/padded0 <= page8/page0 {
		t.Errorf("padding was expected to leave 8° relatively higher: padded ratio %.3f, unpadded %.3f",
			padded8/padded0, page8/page0)
	}
	// And the score the code reports is the unpadded one.
	if got := skewScore(marks, w, h, 8, SkewOptions{Criterion: Variance}); math.Abs(got-page8) > 1e-9 {
		t.Errorf("skewScore at 8° is %.6f, the page-rows variance is %.6f", got, page8)
	}
}

func FuzzSkewAngle(f *testing.F) {
	f.Add([]byte{1, 2, 3, 4, 5, 6, 7, 8})
	f.Add([]byte{})
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) == 0 {
			return
		}
		// Shape an image out of the bytes: the first two choose the
		// dimensions, the rest are pixels (tiled if short).
		w := 1 + int(data[0])%40
		h := 1 + int(data[len(data)-1])%40
		img := image.NewRGBA(image.Rect(0, 0, w, h))
		for i := range img.Pix {
			img.Pix[i] = data[i%len(data)]
		}
		angle, err := SkewAngle(img, SkewOptions{})
		if err != nil {
			return // a refusal is a valid answer, and the only other one
		}
		if angle < -10.001 || angle > 10.001 {
			t.Fatalf("%dx%d: angle %v outside the default search", w, h, angle)
		}
		if math.IsNaN(angle) {
			t.Fatalf("%dx%d: NaN", w, h)
		}
	})
}

func BenchmarkSkewAngle(b *testing.B) {
	page := overWhite(Rotate(textPage(1200, 1600), 3, true))
	b.ReportAllocs()
	for b.Loop() {
		if _, err := SkewAngle(page, SkewOptions{}); err != nil {
			b.Fatal(err)
		}
	}
}
