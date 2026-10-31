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
	mask, w, h, err := skewMask(img, opt.MaxPixels)
	if err != nil {
		return 0, err
	}

	// ⛔ THE SEARCH SHEARS RATHER THAN ROTATES, which is Baird's trick and not
	// only a saving. A rotation resamples, and resampling BLURS the very line
	// structure being measured — the sharper the profile, the better the peak,
	// so interpolating before scoring works against the measurement. A shear
	// moves whole pixels between rows and leaves the counts exact.
	best, bestScore := 0.0, math.Inf(-1)
	for a := -opt.MaxAngle; a <= opt.MaxAngle+1e-9; a += opt.Coarse {
		if s := skewScore(mask, w, h, a, opt); s > bestScore {
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
		if s := skewScore(mask, w, h, a, opt); s > bestScore {
			best, bestScore = a, s
		}
	}
	return best, nil
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
	b := img.Bounds()
	if b.Dx() <= 0 || b.Dy() <= 0 {
		return nil, 0, 0, ErrSkewEmpty
	}
	work := img
	if n := b.Dx() * b.Dy(); n > maxPixels {
		// One scale for both axes: the skew angle of a page is not preserved
		// by squashing one direction more than the other.
		s := math.Sqrt(float64(maxPixels) / float64(n))
		w := max(1, int(float64(b.Dx())*s))
		h := max(1, int(float64(b.Dy())*s))
		small, err := Resize(img, w, h, Bilinear)
		if err != nil {
			return nil, 0, 0, fmt.Errorf("ops: scaling the page down to %d×%d before measuring its skew: %w", w, h, err)
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
	if len(marks) == 0 || len(marks) == w*h {
		return nil, 0, 0, fmt.Errorf("%w: %d of %d pixels are marked", ErrSkewBlank, len(marks), w*h)
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
