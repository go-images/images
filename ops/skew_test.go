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

// skewPlanted is the angle set the two budget measurements use, kept in one
// place because BOTH of their findings depend on which angles are in it.
var skewPlanted = []float64{-7, -2.5, 0, 1.25, 6}

// skewPitch is the distance between the text lines [textPage] draws, which is
// the quantity the budget measurements are really about.
const skewPitch = 6 + 8

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

// TestMeasureTheDownscaleFilter chooses the filter the bound uses, instead of
// taking the obvious one. Bilinear reads four neighbours whatever the scale
// factor, so reducing a page sevenfold samples it sparsely and can step over a
// thin stroke; Area averages the whole footprint, which is what a reduction
// asks for. Which matters HERE is a question about recovering planted angles,
// so it is measured.
func TestMeasureTheDownscaleFilter(t *testing.T) {
	// The budgets are the five rows quoted in the MaxPixels documentation,
	// chosen to bracket the collapse rather than to sample evenly: the whole
	// point is where it breaks, and every row costs a full search under -race.
	//
	// ⛔ DO NOT TRIM skewPlanted TO SAVE TIME. I shortened it to three angles
	// and the finding CHANGED: budget 80 000 reported 0.000° instead of
	// 4.350°, because the angle that failed there was one of the two I had
	// dropped. A collapse that only some angles witness is still a collapse.
	atHalf := map[string]float64{}
	for _, c := range skewCorpora(t) {
		for i, budget := range c.budgets {
			for _, mode := range []struct {
				name string
				m    ResizeMode
			}{{"Bilinear", Bilinear}, {"Area", Area}} {
				worst, rw, rh := c.measure(t, budget, mode.m)
				t.Logf("%-9s budget %7d (%dx%d, %.1f px per line), %-8s: worst |error| over %d angles %.3f°",
					c.name, budget, rw, rh, c.perLine(rh), mode.name, len(skewPlanted), worst)
				if i == 3 && mode.m == skewDownscale { // the 2.5 px-per-line row
					atHalf[c.name] = worst
				}
			}
		}
	}

	// ⛔ THE FINDING THIS ASSERTS IS NOT THE ONE I SET OUT TO MEASURE. I had
	// written that the collapse is about pixels per text line and nothing
	// else, and halved the page to make the test cheap — expecting the same
	// figures. At 2.5 pixels per line the small page is off by a quarter of a
	// degree and the large one by over four: the number of ROWS the profile is
	// built from matters as well, because more of them make the peak stand out
	// of more evidence. Same ratio, different answer, so the quantity is not
	// sufficient on its own.
	t.Logf("at 2.5 px per line: %s %.3f°, %s %.3f°", "700x900", atHalf["700x900"], "1400x1800", atHalf["1400x1800"])
	if atHalf["1400x1800"] < 1 || atHalf["700x900"] > 1 {
		t.Errorf("pixels per text line now predicts the collapse on its own "+
			"(small %.3f°, large %.3f° at the same 2.5 px per line): the documented limit can be stated in that one quantity",
			atHalf["700x900"], atHalf["1400x1800"])
	}
}

// A skewCorpus is a page, its tilted copies and the pixel budgets that reduce
// it to a known number of pixels per text line. Both corpora are built ONCE
// per test and share the same five ratios, so a row of one lines up with the
// same row of the other.
//
// ⛔ The copies are rotated outside the budget and filter loops. The first
// version rotated inside them — fifty rotations of a 2.5-megapixel page — and
// pushed the package past `go test`'s ten-minute timeout under -race, which is
// a CI failure waiting to happen rather than a slow test.
type skewCorpus struct {
	name    string
	page    *image.RGBA
	tilted  []*image.RGBA
	budgets []int
}

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
	return []skewCorpus{
		build("700x900", 700, 900, []int{250_000, 50_000, 25_000, 20_000, 12_500}),
		// Four times the pixels and four times each budget, so every row has
		// the same pixels per text line as the row above it.
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

// TestMeasureTheConfidenceOfThePeak asked whether the detector can SEE its own
// failure, and the answer was NO — which is why [SkewOptions.MaxPixels]
// documents a limit instead of the code refusing to cross it.
//
// TestMeasureTheDownscaleFilter shows the measurement collapsing below about
// 2.8 pixels per text line — the Nyquist limit of the line pitch, since a line
// and its gap are one cycle — and returning an angle 16° wrong without
// complaint. The obvious confidence is the ratio of the best score to the mean
// over all candidate angles: a towering peak where the lines are resolvable, a
// flat landscape where they alias.
//
// ⛔ MEASURED, AND IT DOES NOT SEPARATE: 1.66 at a budget that is right to
// 0.05°, 1.75 at one that is 4.35° wrong. A threshold on this statistic would
// reject sound pages and accept broken ones, so no gate was built on it. This
// test asserts the NON-separation, so that if a change to the scoring ever
// makes the two populations pull apart, it fails and says a gate has become
// possible.
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
	t.Logf("over both corpora: lowest ratio among sound budgets %.2f, highest among broken ones %.2f",
		lowestSound, highestBroken)
	if lowestSound > highestBroken {
		t.Errorf("peak/mean now SEPARATES sound from broken across both page sizes (%.2f > %.2f): "+
			"a confidence gate has become possible, and MaxPixels should stop documenting a limit it could refuse",
			lowestSound, highestBroken)
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
