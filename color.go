package images

import (
	"image"

	"github.com/go-images/images/internal/kernels"
)

// OtsuThreshold returns the gray level in [0, 255] computed by Otsu's method on
// img's Rec. 601 luminance histogram: the level that maximises the between-class
// variance of the two pixel populations split at it. It matches the value
// returned by skimage.filters.threshold_otsu on a 256-bin histogram. Pass the
// result to Threshold (foreground = luminance strictly greater than it).
func OtsuThreshold(img image.Image) uint8 {
	return kernels.OtsuThreshold(ToRGBA(img).Pix)
}

// Threshold returns a binary image: every pixel of img whose Rec. 601 luminance
// is strictly greater than t becomes white, every other pixel black. Alpha is
// preserved. Combine with OtsuThreshold for an automatically chosen level.
func Threshold(img image.Image, t uint8) *image.RGBA {
	src := ToRGBA(img)
	dst := newLike(src)
	kernels.Threshold(dst.Pix, src.Pix, t)
	return dst
}

// Otsu is a convenience wrapper that thresholds img at the level chosen by
// Otsu's method (equivalent to Threshold(img, OtsuThreshold(img))).
func Otsu(img image.Image) *image.RGBA {
	src := ToRGBA(img)
	dst := newLike(src)
	kernels.Threshold(dst.Pix, src.Pix, kernels.OtsuThreshold(src.Pix))
	return dst
}

// RGBToHSV returns a copy of img with each pixel's R, G, B replaced by a
// byte-encoded H, S, V triple: H is the hue mapped from [0,360) to [0,255], S
// and V are mapped from [0,1] to [0,255]. Alpha is preserved. HSVToRGB inverts
// the mapping (within rounding). The encoding keeps the result inside the same
// RGBA-backed representation the rest of the pipeline uses.
func RGBToHSV(img image.Image) *image.RGBA {
	src := ToRGBA(img)
	dst := newLike(src)
	kernels.RGBToHSV(dst.Pix, src.Pix)
	return dst
}

// HSVToRGB inverts RGBToHSV: it interprets each pixel's first three channels as
// byte-encoded H, S, V and returns the corresponding R, G, B. Alpha is
// preserved.
func HSVToRGB(img image.Image) *image.RGBA {
	src := ToRGBA(img)
	dst := newLike(src)
	kernels.HSVToRGB(dst.Pix, src.Pix)
	return dst
}
