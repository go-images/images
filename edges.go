package images

import (
	"fmt"
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

// Canny returns the binary Canny edge map of img: white (255,255,255) edges on
// an opaque black background. It implements the classic Canny pipeline, matching
// the algorithm of skimage.feature.canny:
//
//  1. smooth the Rec. 601 luminance with a Gaussian of standard deviation sigma
//     (clamp-to-edge borders);
//  2. estimate gradients with the Sobel operator; the edge strength is the
//     gradient norm;
//  3. thin to 1-pixel ridges by non-maximum suppression with bilinear
//     interpolation along the gradient direction;
//  4. link edges by hysteresis: keep every ridge pixel with magnitude >= high,
//     plus every ridge pixel with magnitude >= low that is 8-connected to a kept
//     one.
//
// low and high are absolute thresholds on the Sobel gradient magnitude (the
// smoothed luminance is in [0,255], so the magnitudes are on that scale). It
// returns an error if sigma is not positive, if either threshold is negative, or
// if high < low.
func Canny(img image.Image, sigma, low, high float64) (*image.RGBA, error) {
	if sigma <= 0 {
		return nil, fmt.Errorf("images: canny: sigma must be positive, got %g", sigma)
	}
	if low < 0 || high < 0 {
		return nil, fmt.Errorf("images: canny: thresholds must be non-negative, got low=%g high=%g", low, high)
	}
	if high < low {
		return nil, fmt.Errorf("images: canny: high threshold %g must be >= low threshold %g", high, low)
	}
	src := ToRGBA(img)
	b := src.Bounds()
	dst := newLike(src)
	smoothed := kernels.GaussianPlane(src.Pix, b.Dx(), b.Dy(), sigma)
	kernels.Canny(dst.Pix, smoothed, b.Dx(), b.Dy(), low, high)
	return dst, nil
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
