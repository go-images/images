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
	"strings"
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
	lineH, gap := 6, 8 // skewPitch apart, which the budget measurements rely on
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

// The two budget measurements, their corpora and their angle set live in
// skew_measure_test.go, which is excluded from the race build. The reason is
// written there.

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

func TestSkewStraightAlphaMarginIsPaper(t *testing.T) {
	// ⛔ The OTHER alpha convention, and the one a plain add gets wrong.
	// AsRGBA views a go-gfx raster.Image — STRAIGHT alpha — as an *image.RGBA
	// without copying, so white paper can arrive as (255,255,255,250) rather
	// than premultiplied. The composite is c + (255-a); unsaturated that is
	// (c-a-1) mod 256, which for white at α=250 is 4 — near BLACK. Every
	// pixel of paper then reads as ink.
	//
	// ⛔ AND IT ASSERTS ON THE MASK, NOT ON THE ANGLE. My first version of
	// this test planted an angle and checked the recovery, and it PASSED with
	// the wrapping add: inverting ink and paper on a page of text leaves the
	// horizontal structure intact — the gaps between lines are as horizontal
	// as the lines — so the projection peaks at the same angle either way.
	// The detector being robust to polarity is exactly why an angle cannot
	// witness a thresholding defect.
	page := textPage(400, 520)
	for i := 0; i < len(page.Pix); i += 4 {
		if page.Pix[i] > 128 {
			page.Pix[i+3] = 250 // nearly opaque paper, as an anti-aliased render gives
		}
	}
	marks, w, h, err := skewMask(page, 2_000_000)
	if err != nil {
		t.Fatal(err)
	}
	frac := float64(len(marks)) / float64(w*h)
	t.Logf("%.1f%% of the page reads as ink", 100*frac)
	if frac > 0.30 {
		t.Fatalf("%.1f%% of a page of text reads as ink: the paper is being counted", 100*frac)
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

func TestDeskewPassesOnTheRefusal(t *testing.T) {
	// A page Deskew cannot measure must not come back silently unrotated: the
	// caller asked for a page put straight and has to learn that nothing was.
	blank := image.NewRGBA(image.Rect(0, 0, 40, 40))
	for i := range blank.Pix {
		blank.Pix[i] = 0xff
	}
	out, angle, err := Deskew(blank, 0.5, SkewOptions{})
	if !errors.Is(err, ErrSkewBlank) {
		t.Fatalf("got (%v, %v, %v), want ErrSkewBlank", out, angle, err)
	}
	if out != nil || angle != 0 {
		t.Errorf("a refusal must carry no image and no angle, got %v and %v", out, angle)
	}
}

func TestSkewAngleRejectsABudgetTooSmallForAPage(t *testing.T) {
	// ⛔ The dimensions are deliberately NOT clamped up to one. A budget that
	// rounds a side to zero is a caller asking for a page in a pixel, and an
	// angle measured on a 1×1 mask would be a number with nothing behind it.
	page := textPage(600, 800)
	_, err := SkewAngle(page, SkewOptions{MaxPixels: 1})
	if err == nil {
		t.Fatal("a one-pixel budget was accepted")
	}
	if !strings.Contains(err.Error(), "budget") {
		t.Errorf("the error should say the budget is the cause: %v", err)
	}
	t.Logf("refused: %v", err)
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

// overInkedPage is the shape of the real failure: an overscanned frame, almost
// all ink, with no text lines in it. Page 3 of bdrc-W1KG4230 in the pdfscans
// corpus renders at 99.2% ink and page 68 at 99.7%.
func overInkedPage(w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	seed := uint32(0x5CA4)
	for i := 0; i < len(img.Pix); i += 4 {
		seed = seed*1664525 + 1013904223
		v := uint8(10 + seed>>28) // dark, with a little grain
		if seed>>16%256 < 2 {     // and a scattering of light specks
			v = 230
		}
		img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = v, v, v, 0xff
	}
	return img
}

func TestSkewAngleRefusesAPageWithNoPeakToFind(t *testing.T) {
	// ⛔ THE DEFECT REAL SCANS FOUND AND NO SYNTHETIC TEST DID. A page with no
	// text lines has no peak in its profile, so the score simply grows with
	// the shear and the search returns its own edge — and the caller was told
	// that edge as though it had been read off the page. Measured on
	// bdrc-W1KG4230 page 3: −5.00° at MaxAngle 5, −10.00° at 10, −20.00° at
	// 20, −35.00° at 35. Deskew then turned the page by ten degrees for no
	// reason, and measuring the result still said −10.00°.
	page := overInkedPage(400, 520)
	for _, max := range []float64{5, 10, 20} {
		got, err := SkewAngle(page, SkewOptions{MaxAngle: max})
		if !errors.Is(err, ErrSkewAtLimit) {
			t.Errorf("MaxAngle %g: got (%+.2f°, %v), want ErrSkewAtLimit", max, got, err)
		}
		// The bound is returned WITH the error, because "at least this far" is
		// what a caller needs in order to decide to widen the search — and it
		// must still be inside the range they asked for.
		if got < -max || got > max {
			t.Errorf("MaxAngle %g: the bound reported was %+.3f°, outside the range asked for", max, got)
		}
	}
	// The control: a page that does have lines is measured, not refused, so
	// the refusal is about the absence of a peak and not about the check
	// firing on everything.
	if _, err := SkewAngle(overWhite(Rotate(textPage(400, 520), 3, true)), SkewOptions{}); err != nil {
		t.Errorf("a page of text was refused: %v", err)
	}
}

func TestSkewAngleCannotSeeATiltJustOutsideTheSearch(t *testing.T) {
	// ⛔ THE LIMIT OF THE CHECK ABOVE, WRITTEN DOWN RATHER THAN HIDDEN. The
	// test for a bound is whether the score is still RISING on the way out,
	// which catches a page with no peak at all. It does NOT catch a page whose
	// real peak sits just beyond the edge: at 2.5° with the search stopped at
	// 2°, the scores at 2° and at 3° straddle the peak and neither dominates,
	// so the landscape inside the range looks like any other.
	//
	// This is a genuine limitation and the honest statement of it is that
	// MaxAngle must be wide enough for the pages being measured, not that the
	// function will tell you when it was not. The default of 10° covers a page
	// dropped crookedly into a scanner, which is what it is for.
	page := textPage(400, 520)
	got, err := SkewAngle(overWhite(Rotate(page, 2.5, true)), SkewOptions{MaxAngle: 2})
	if err != nil {
		t.Logf("a tilt of 2.5° searched to ±2° is now refused (%v) — better than when this "+
			"was written, and the limitation above can be struck out", err)
	} else if math.Abs(got-2) > 0.1 {
		t.Errorf("searched to ±2°, returned %+.3f°: not the bound and not the truth", got)
	}
	// Whatever else, the answer stays inside the range the caller asked for.
	if got < -2 || got > 2 {
		t.Errorf("returned %+.3f° from a search bounded at 2°", got)
	}
	// And the same page measures correctly once the search is wide enough.
	wide, err := SkewAngle(overWhite(Rotate(page, 2.5, true)), SkewOptions{})
	if err != nil {
		t.Fatalf("the same page with the default search: %v", err)
	}
	if math.Abs(wide-2.5) > 0.15 {
		t.Errorf("the same page with the default search measured %+.2f°, want +2.50°", wide)
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
