package images

import (
	"fmt"
	"image"

	"github.com/go-images/images/internal/kernels"
)

// Median returns img filtered by a square median of the given radius: every
// output channel is the median of the (2*radius+1) by (2*radius+1) source
// neighbourhood, computed independently per R, G and B (alpha preserved), with
// clamp-to-edge addressing. It matches scipy.ndimage.median_filter with a
// (2*radius+1)-square footprint and mode="nearest". The median is robust to
// outliers, so it removes salt-and-pepper noise while preserving edges far
// better than a linear blur. It returns an error if radius is not positive.
func Median(img image.Image, radius int) (*image.RGBA, error) {
	if radius <= 0 {
		return nil, fmt.Errorf("images: median: radius must be positive, got %d", radius)
	}
	src := ToRGBA(img)
	b := src.Bounds()
	dst := newLike(src)
	kernels.Median(dst.Pix, src.Pix, b.Dx(), b.Dy(), radius)
	return dst, nil
}

// UnsharpMask returns a sharpened copy of img using the unsharp-masking
// technique: dst = clamp(src + amount*(src - blurred)), where blurred is the
// Gaussian blur of img with standard deviation radius. The R, G and B channels
// are processed independently and alpha is preserved. It matches
// skimage.filters.unsharp_mask applied per channel.
//
// radius controls the scale of the detail recovered (the Gaussian sigma) and
// must be positive; amount scales how strongly that detail is added back: 0
// leaves the image unchanged, typical sharpening uses values around 0.5–2, and
// negative values soften. It returns an error if radius is not positive.
func UnsharpMask(img image.Image, radius, amount float64) (*image.RGBA, error) {
	if radius <= 0 {
		return nil, fmt.Errorf("images: unsharp mask: radius must be positive, got %g", radius)
	}
	src := ToRGBA(img)
	// radius > 0 was just validated, so GaussianBlur (which errors only on a
	// non-positive sigma) cannot fail here.
	blurred, _ := GaussianBlur(src, radius)
	dst := newLike(src)
	kernels.UnsharpMask(dst.Pix, src.Pix, blurred.Pix, amount)
	return dst, nil
}

// Sharpen returns a sharpened copy of img with sensible defaults: an unsharp
// mask with radius 1.0 and amount 1.0, i.e. it adds back the full single-pixel-
// scale detail layer. For finer control over the scale or strength use
// UnsharpMask directly.
func Sharpen(img image.Image) *image.RGBA {
	// radius 1.0 is always valid, so the error cannot occur.
	dst, _ := UnsharpMask(img, 1.0, 1.0)
	return dst
}
