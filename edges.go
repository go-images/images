package images

import (
	"image"

	"github.com/go-images/images/internal/kernels"
)

// Prewitt returns the Prewitt gradient-magnitude edge map of img. The operator
// is the separable 3x3 Prewitt kernel applied to each pixel's Rec. 601
// luminance; the magnitude sqrt((gx^2+gy^2)/2) is written to the R, G and B
// channels as a grayscale edge image (alpha preserved). It mirrors
// skimage.filters.prewitt: the directional kernels are normalised so each axis
// kernel's absolute weights sum to one, and clamp-to-edge addressing reproduces
// skimage's default reflect border for a 3-tap kernel.
func Prewitt(img image.Image) *image.RGBA {
	src := ToRGBA(img)
	b := src.Bounds()
	dst := newLike(src)
	kernels.Prewitt(dst.Pix, src.Pix, b.Dx(), b.Dy())
	return dst
}

// Scharr returns the Scharr gradient-magnitude edge map of img, matching
// skimage.filters.scharr. The Scharr smoothing triple (0.1875, 0.625, 0.1875)
// gives the best rotational symmetry of the Sobel/Prewitt/Scharr family. See
// Prewitt for the luminance, magnitude and border conventions.
func Scharr(img image.Image) *image.RGBA {
	src := ToRGBA(img)
	b := src.Bounds()
	dst := newLike(src)
	kernels.Scharr(dst.Pix, src.Pix, b.Dx(), b.Dy())
	return dst
}

// SobelMag returns the normalised Sobel gradient-magnitude edge map of img using
// the scikit-image convention (sqrt((gx^2+gy^2)/2) on luminance scaled to
// [0,1]), matching skimage.filters.sobel. It differs from Sobel, which uses the
// classic integer kernels and the unnormalised magnitude sqrt(gx^2+gy^2);
// SobelMag shares the one definition used by Prewitt and Scharr so the edge
// family is directly comparable to scikit-image.
func SobelMag(img image.Image) *image.RGBA {
	src := ToRGBA(img)
	b := src.Bounds()
	dst := newLike(src)
	kernels.SobelMag(dst.Pix, src.Pix, b.Dx(), b.Dy())
	return dst
}

// Laplacian returns the discrete Laplacian edge map of img, matching
// skimage.filters.laplace with ksize=3 (the kernel [0,-1,0; -1,4,-1; 0,-1,0]
// applied to the luminance plane). The signed second-derivative response is
// offset by 128 so a flat region is mid-grey, written to R, G and B and clamped
// to [0,255]; alpha is preserved and borders use clamp-to-edge addressing.
// Being a second-derivative operator it highlights intensity curvature (lines,
// spots, zero-crossings) rather than step edges.
func Laplacian(img image.Image) *image.RGBA {
	src := ToRGBA(img)
	b := src.Bounds()
	dst := newLike(src)
	kernels.Laplacian(dst.Pix, src.Pix, b.Dx(), b.Dy())
	return dst
}
