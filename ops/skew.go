// Copyright (c) 2026, the go-images authors
// All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package ops

import (
	"errors"
	"fmt"
	"image"
	"math"

	"github.com/go-images/images/internal/kernels"
)

// Skew detection for scanned pages: how far a document is tilted, so that it
// can be put straight.
//
// ⛔ ONLY THE ANGLE IS NEW HERE. This package could already binarise
// ([OtsuThreshold]) and rotate by an arbitrary angle ([Rotate], [Warp]); what
// it could not do was MEASURE the tilt, so nothing could decide how far to
// turn a page. The detection is the missing half, not a new subsystem.
//
// # Bibliography
//
// Baird, H. (1987), "The skew angle of printed documents", Proceedings of the
// Society of Photographic Scientists and Engineers, pp. 21–24: simulate
// rotation by a shear, and take the angle that MAXIMISES the variance of the
// number of marked pixels per scan line. When the page is straight, every text
// line falls on its own rows and the profile is spiky; tilted, the lines smear
// across rows and the profile flattens.
//
// Postl, W. (1986), "Detection of linear oblique structures and skew scans in
// digitized documents": the same projection, scored instead by the sum of
// squared differences between ADJACENT profile elements.
//
// Mascaro, Cavalcanti et al., Pattern Recognition Letters 31 (2010) 1403–1411,
// which surveys both: "Baird's projection profile method achieved the best
// result due to its high accuracy in dealing with documents containing clear
// separation between the text lines" — which is what a page of text is.
//
// Both criteria are implemented, and [SkewCriterion] says which to use,
// because the survey's ranking is about their corpus and ours is measurable:
// the tests plant a known angle and report what each recovers.

// A SkewCriterion scores a projection profile. The angle whose profile scores
// highest is the skew.
type SkewCriterion int

const (
	// Variance is Baird's: the variance of the marked-pixel count per scan
	// line. The default.
	Variance SkewCriterion = iota
	// AdjacentDifference is Postl's: the sum of squared differences between
	// neighbouring profile elements.
	AdjacentDifference
)

// SkewOptions tunes the search. The zero value is usable and means the
// defaults named on each field.
type SkewOptions struct {
	// MaxAngle bounds the search, in degrees either side of level. Zero means
	// 10, which covers a page dropped crookedly into a scanner; beyond about
	// 15 the shear approximation stops standing in for a rotation.
	MaxAngle float64

	// Coarse and Fine are the two search steps, in degrees. Zero means 1 and
	// 0.05: the coarse pass finds the basin, the fine pass finds the bottom of
	// it.
	Coarse, Fine float64

	// Criterion scores the profile. The zero value is [Variance].
	Criterion SkewCriterion

	// MaxPixels caps the working image; a larger one is scaled down before
	// anything is measured. Zero means 2 000 000.
	//
	// ⛔ IT IS A BOUND ON UNTRUSTED INPUT, not only a speed setting. The
	// search evaluates the profile once per candidate angle, so cost is
	// pixels × angles: a 100-megapixel scan would be some sixty passes over
	// a hundred million pixels because someone handed us a large file.
	//
	// ⛔ LOWERING IT TOO FAR BREAKS THE MEASUREMENT SILENTLY, and that is a
	// limit rather than a bug to be fixed here. The projection resolves text
	// lines, and a line plus its gap is one cycle, so the reduced page needs
	// at least two samples per line pitch — Nyquist. Measured by
	// TestMeasureTheDownscaleFilter, as the worst error over five planted
	// angles on pages whose lines sit fourteen pixels apart:
	//
	//	px per line   700×900   1400×1800
	//	        8.8    0.000°      0.000°
	//	        3.9    0.050°      0.000°
	//	        2.8    0.150°      0.050°
	//	        2.5    0.250°      4.350°
	//	        2.0   16.150°     16.050°
	//
	// ⛔ TWO COLUMNS BECAUSE ONE WOULD HAVE BEEN A FALSE GENERALISATION. I
	// wrote this table from the larger page, called the pixels per text line
	// the one quantity that matters, and then halved the page expecting the
	// same figures: at 2.5 px per line the smaller page is off by a quarter of
	// a degree and the larger by over four. The number of rows the profile is
	// built from counts too, because more of them make the peak stand out of
	// more evidence. Two samples per line is where both collapse; the margin
	// above it is not a property of the ratio alone.
	//
	// The default leaves an A4 page scanned at 300 DPI at some 1200×1700,
	// which is five to ten pixels per line of body text. Below about three the
	// angle is not to be trusted, and the function does NOT say so: see
	// TestMeasureTheConfidenceOfThePeak, which measured the obvious confidence
	// statistic and found that no single threshold on it tells the sound pages
	// from the broken ones across both page sizes.
	MaxPixels int
}

func (o SkewOptions) withDefaults() SkewOptions {
	if o.MaxAngle == 0 {
		o.MaxAngle = 10
	}
	if o.Coarse == 0 {
		o.Coarse = 1
	}
	if o.Fine == 0 {
		o.Fine = 0.05
	}
	if o.MaxPixels == 0 {
		o.MaxPixels = 2_000_000
	}
	return o
}

// Errors returned by the skew functions.
var (
	ErrSkewEmpty = errors.New("ops: the image has no pixels")
	ErrSkewBlank = errors.New("ops: the image has nothing marked on it, so it has no skew")

	// ErrSkewAtLimit says the best score was found at an edge of the search,
	// so the answer is a BOUND and not a measurement: the true maximum is at
	// that angle or beyond it, and nothing here can tell which.
	//
	// ⛔ FOUND ON REAL SCANS, AND NOT BY ANY SYNTHETIC TEST. Page 3 of
	// bdrc-W1KG4230 in the pdfscans corpus is an overscanned frame that comes
	// out 99.2% ink: it has no text lines, so the profile has no peak, and the
	// score simply grows with the shear. Measured, the reported angle tracked
	// the search exactly — −5.00° at MaxAngle 5, −10.00° at 10, −20.00° at 20,
	// −35.00° at 35 — and the caller was told −10.00° as if it had been read
	// off the page. Deskewing then turned a page by ten degrees for no reason,
	// and a second measurement still said −10.00°.
	//
	// A caller who means "find it however far it has gone" raises MaxAngle; a
	// caller who gets this on a page of text is being told the page is not a
	// page of text.
	ErrSkewAtLimit = errors.New("ops: the best angle is at the edge of the search, so it is a bound and not a measurement")
)

// SkewAngle is how far the page is tilted, in degrees, in [Rotate]'s own
// convention: a positive angle means the text runs UP to the right
// (counter-clockwise, as scikit-image names it), so a page produced by
// Rotate(img, a, …) measures a, and Rotate(img, -SkewAngle(img), …) puts it
// back. [Deskew] does exactly that.
//
// It reports [ErrSkewBlank] for a page with nothing on it: a blank sheet has no
// skew, and returning zero would be indistinguishable from a page measured and
// found level.
func SkewAngle(img image.Image, opt SkewOptions) (float64, error) {
	opt = opt.withDefaults()
	marks, w, h, err := skewMask(img, opt.MaxPixels)
	if err != nil {
		return 0, err
	}
	best := searchSkew(marks, w, h, opt)

	// ⛔ A MAXIMUM IS LOCATED ONLY IF IT IS BRACKETED, and the test is whether
	// the score still RISES on the way out — not whether the answer sits
	// exactly on the boundary. Measured: a page tilted 9° searched to ±2°
	// comes back at −1.95°, a fine step short of the edge, because the
	// discretised scores wobble as they climb. An equality test on the bound
	// missed it, and that near miss is the common shape rather than the
	// exception.
	//
	// So when the best falls in the outermost coarse interval, the score is
	// evaluated one coarse step BEYOND the bound — as a test, never as an
	// answer. If it is higher, the maximum is outside the range the caller
	// allowed and what came back is a bound. The probe costs one projection,
	// and only on the pages that need it.
	//
	// The bound is returned alongside the error, because "at least this far"
	// is what the caller needs in order to decide to widen the search.
	if math.Abs(best) > opt.MaxAngle-opt.Coarse {
		outward := math.Copysign(opt.MaxAngle+opt.Coarse, best)
		if skewScore(marks, w, h, outward, opt) > skewScore(marks, w, h, best, opt) {
			return best, fmt.Errorf("%w: still rising at %+.2f° with the search bounded at %g°",
				ErrSkewAtLimit, best, opt.MaxAngle)
		}
	}
	return best, nil
}

// searchSkew is the coarse-then-fine maximisation, split out so a test can
// drive it over a mask built differently — see TestMeasureTheDownscaleFilter.
// opt must already have had withDefaults applied.
func searchSkew(marks []skewPoint, w, h int, opt SkewOptions) float64 {
	// ⛔ THE SEARCH SHEARS RATHER THAN ROTATES, which is Baird's trick and not
	// only a saving. A rotation resamples, and resampling BLURS the very line
	// structure being measured — the sharper the profile, the better the peak,
	// so interpolating before scoring works against the measurement. A shear
	// moves whole pixels between rows and leaves the counts exact.
	best, bestScore := 0.0, math.Inf(-1)
	for a := -opt.MaxAngle; a <= opt.MaxAngle+1e-9; a += opt.Coarse {
		if s := skewScore(marks, w, h, a, opt); s > bestScore {
			best, bestScore = a, s
		}
	}
	// ⛔ THE FINE PASS IS CLAMPED TO MaxAngle. Found by a mutant: when the
	// coarse best lands on an edge of the search — which is what happens to a
	// page tilted FURTHER than the caller allowed — a refinement of best±Coarse
	// reaches a full step beyond MaxAngle, and the function returns an angle
	// from outside the range it was asked for. The caller's bound is a promise
	// about the answer, not only about where to look.
	lo := math.Max(best-opt.Coarse, -opt.MaxAngle)
	hi := math.Min(best+opt.Coarse, opt.MaxAngle)
	for a := lo; a <= hi+1e-9; a += opt.Fine {
		if s := skewScore(marks, w, h, a, opt); s > bestScore {
			best, bestScore = a, s
		}
	}
	return best
}

// Deskew measures the page's tilt and puts it straight, returning the
// corrected image and the angle that was found.
//
// A tilt smaller than threshold degrees is left alone and reported: turning a
// page by a hundredth of a degree resamples every pixel to no visible purpose,
// and a scan that is already level should come back untouched rather than
// slightly softer.
func Deskew(img image.Image, threshold float64, opt SkewOptions) (*image.RGBA, float64, error) {
	angle, err := SkewAngle(img, opt)
	if err != nil {
		return nil, 0, err
	}
	if math.Abs(angle) < threshold {
		return ToRGBA(img), angle, nil
	}
	return Rotate(img, -angle, false), angle, nil
}

// satAdd is c+inv clamped to 255.
func satAdd(c uint8, inv uint16) uint8 {
	if s := uint16(c) + inv; s < 255 {
		return uint8(s)
	}
	return 255
}

// A skewPoint is one marked pixel. int32 rather than int because the budget
// caps a side well below 2³¹ and the pair then fits one eight-byte slot, which
// halves what the inner loop pulls through the cache.
type skewPoint struct{ x, y int32 }

// skewMask reduces the image to the coordinates of its marked pixels, scaled
// down to the pixel budget. It returns them with the working width and height,
// which the projection needs to size its profile.
func skewMask(img image.Image, maxPixels int) ([]skewPoint, int, int, error) {
	return skewMaskWith(img, maxPixels, skewDownscale)
}

// ⛔ Area, NOT Bilinear — and the measurement does NOT say Area is better. It
// says the choice costs nothing, which is a weaker and truer claim.
//
// The argument for Area is that it is the only one of the three whose support
// grows with the scale factor: Bilinear reads four neighbours however far the
// page is being reduced, so it samples sparsely and can step over a thin
// stroke. TestMeasureTheDownscaleFilter put that to fifteen budgets across
// three page sizes and found Area better in 3 rows and worse in 6 — every
// difference one step of the search, 0.05°, except one row where Area was
// better by 0.15°. The two filters collapse together once the line pitch
// aliases.
//
// ⛔ I CLAIMED THE OPPOSITE FIRST. On two page sizes the tally was "better in
// three, worse in none" and I wrote that down as Area winning; the third
// corpus turned it round. Nine rows were not enough to rank two filters whose
// difference is one quantisation step.
const skewDownscale = Area

func skewMaskWith(img image.Image, maxPixels int, mode ResizeMode) ([]skewPoint, int, int, error) {
	b := img.Bounds()
	if b.Dx() <= 0 || b.Dy() <= 0 {
		return nil, 0, 0, ErrSkewEmpty
	}
	work := img
	if n := b.Dx() * b.Dy(); n > maxPixels {
		// One scale for both axes: the skew angle of a page is not preserved
		// by squashing one direction more than the other.
		//
		// ⛔ The dimensions are NOT clamped up to one. A budget too small to
		// hold a single row of the page rounds a side to zero, and Resize
		// refuses that — which is the right answer, because a caller asking
		// for a page in one pixel is asking for something that cannot carry an
		// angle. Clamping to 1×1 would instead hand the search a mask of one
		// pixel and report an angle measured on nothing.
		s := math.Sqrt(float64(maxPixels) / float64(n))
		w, h := int(float64(b.Dx())*s), int(float64(b.Dy())*s)
		small, err := Resize(img, w, h, mode)
		if err != nil {
			return nil, 0, 0, fmt.Errorf("ops: scaling the page down to %d×%d to fit a %d-pixel budget: %w", w, h, maxPixels, err)
		}
		work, b = small, small.Bounds()
	}

	w, h := b.Dx(), b.Dy()
	src := ToRGBA(work)

	// ⛔ TRANSPARENT IS PAPER, NOT INK. image.RGBA stores premultiplied alpha,
	// so a fully transparent pixel is (0,0,0) — black, the darkest thing there
	// is, which any threshold reads as a mark. A page that has already been
	// through [Rotate] carries transparent triangles in its corners, so feeding
	// one back here would measure two large black wedges instead of the text.
	// Compositing over white (c + (255-a), which is alpha-over-white on
	// premultiplied channels) makes "nothing there" read as blank paper, which
	// is what it means. ToRGBA may hand back the caller's own pixels, so this
	// goes into a buffer of our own rather than over theirs.
	// Row by row through PixOffset rather than straight down src.Pix: an
	// *image.RGBA anchored at the origin can still be a sub-image of a wider
	// one, whose Stride is then larger than 4*w, and ToRGBA hands such an image
	// back untouched.
	//
	// ⛔ AND IT SATURATES. On premultiplied channels c ≤ a, so c + (255-a)
	// cannot exceed 255 and a plain add would do. But [AsRGBA] views a
	// go-gfx raster.Image — which is STRAIGHT alpha — as an *image.RGBA
	// without copying, so this can be handed (255,255,255,0): a plain add
	// wraps that to 254 and turns a transparent white pixel into ink. Clamping
	// is the right answer under both conventions, because transparent over
	// white is white either way.
	flat := make([]uint8, 4*w*h)
	for y := 0; y < h; y++ {
		row := src.Pix[src.PixOffset(0, y) : src.PixOffset(0, y)+4*w]
		out := flat[4*w*y : 4*w*(y+1)]
		for i := 0; i < len(row); i += 4 {
			inv := uint16(255 - row[i+3])
			out[i] = satAdd(row[i], inv)
			out[i+1] = satAdd(row[i+1], inv)
			out[i+2] = satAdd(row[i+2], inv)
			out[i+3] = 255
		}
	}

	// ⛔ Otsu's threshold is read from THIS image rather than assumed: a scan
	// of newsprint and a scan of a laser print do not share a grey level, and
	// a fixed threshold turns one of them into a blank page — which the
	// detection would then report as "no skew" rather than as "I could not
	// see anything".
	//
	// ⛔ And it is read from `flat`, the SAME bytes the mask below is built
	// from, with the same luma formula the histogram uses. A threshold chosen
	// on one set of pixels and applied to another is two measurements that can
	// disagree: compositing the corners to white moves the histogram, so a
	// threshold taken before the composite would be cutting a page it never
	// saw.
	t := kernels.OtsuThreshold(flat)
	marks := make([]skewPoint, 0, w*h/8) // a page is mostly paper
	for y := 0; y < h; y++ {
		row := flat[4*w*y : 4*w*(y+1)]
		for x := 0; x < w; x++ {
			i := 4 * x
			l := kernels.ClampByte(0.299*float64(row[i]) + 0.587*float64(row[i+1]) + 0.114*float64(row[i+2]))
			// kernels.Threshold calls luminance strictly greater than t the
			// foreground, so ink — the dark side — is everything at or below it.
			if l <= t {
				marks = append(marks, skewPoint{int32(x), int32(y)})
			}
		}
	}
	// ⛔ "NOTHING ON IT" IS NOT ONLY ZERO, and the floor is a MEASURED figure
	// rather than a chosen one. This check used to be len(marks) == 0, and
	// page 67 of bdrc-W1KG4230 in the pdfscans corpus slipped through it with
	// 0.00159% of the page marked — a few specks — on which the detector found
	// a confident, stable −8.00° that was pure noise, and Deskew turned the
	// page by it.
	//
	// Over 62 pages of that collection answered inside the search, the ink
	// fraction runs from 1.826% to 10.28%, median 7.84%. Page 67 is a factor
	// of ELEVEN HUNDRED below the next lowest, and nothing lies between them.
	//
	// The floor itself is borrowed from a ladder measured elsewhere in this
	// fleet (go-pdfkit/ops BlankOptions.MaxInk, on A4 at 150 DPI): one pixel
	// is 4.6e-07 of the page, an 8-point full stop 1.8e-06, and one word of
	// 12-point text 2.0e-04. A page carrying less ink than a single word has
	// no text lines to be tilted. That puts the floor twelve times above page
	// 67 and ninety times below the thinnest real page here — orders of
	// magnitude from both populations, which is what keeps it from being a
	// tuned number.
	//
	// There is deliberately no matching ceiling: an almost entirely inked page
	// is caught by the bracketing test in [SkewAngle], which needs no
	// threshold at all, and one failure should have one guard.
	const leastInk = 2e-4 // one word of 12-point text on A4 at 150 DPI
	if n := len(marks); n == w*h || float64(n) < leastInk*float64(w*h) {
		return nil, 0, 0, fmt.Errorf("%w: %d of %d pixels are marked (%.5f%%)",
			ErrSkewBlank, n, w*h, 100*float64(n)/float64(w*h))
	}
	return marks, w, h, nil
}

// skewScore projects the marked pixels along the given angle and scores the
// profile.
//
// ⛔ IT WALKS THE MARKED PIXELS, NOT THE PAGE. The search evaluates this once
// per candidate angle — 41 coarse plus up to 41 fine with the defaults — so a
// loop over every pixel pays for the paper as many times as for the ink, and a
// page is mostly paper. The marked coordinates are gathered once in [skewMask]
// and the inner loop touches nothing else.
func skewScore(marks []skewPoint, w, h int, angleDeg float64, opt SkewOptions) float64 {
	tan := math.Tan(angleDeg * math.Pi / 180)
	// The shear moves a pixel at column x up or down by x*tan, so the profile
	// has to be tall enough to hold both extremes.
	pad := int(math.Abs(tan)*float64(w)) + 2
	n := h + 2*pad
	profile := make([]int, n)
	for _, p := range marks {
		profile[pad+int(p.y)+int(float64(p.x)*tan)]++
	}

	switch opt.Criterion {
	case AdjacentDifference:
		var sum float64
		for i := 0; i+1 < n; i++ {
			d := float64(profile[i] - profile[i+1])
			sum += d * d
		}
		return sum
	default:
		// ⛔ The variance is taken over the rows the page actually occupies,
		// not over the padded profile. The padding is empty by construction
		// and its width grows with the angle, so a variance that counted it
		// would vary with the angle for a reason that has nothing to do with
		// the page — and it varies the wrong way: it FLATTENS the search,
		// leaving large angles less penalised than they should be.
		//
		// MEASURED, by TestSkewScoreIgnoresThePadding, on a level page where
		// 0° is the truth: this criterion scores 8° at 9.7% of 0°, while the
		// padded one scores it at 16.2%. Spreading a constant marked total
		// over h+2·pad bins scales the variance to r·E[v²] − r²·E[v]² with
		// r = h/(h+2·pad), and at a large angle the smeared profile has
		// E[v²] < 2·E[v]², which makes that larger than the unpadded value
		// rather than smaller.
		//
		// ⛔ I reasoned my way to the OPPOSITE sign here and the test said
		// otherwise, which is why the figures above are quoted from it.
		var sum, sumSq float64
		for i := pad; i < pad+h; i++ {
			v := float64(profile[i])
			sum += v
			sumSq += v * v
		}
		mean := sum / float64(h)
		return sumSq/float64(h) - mean*mean
	}
}
