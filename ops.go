package images

import (
	"fmt"
	"image"

	"github.com/go-images/images/internal/kernels"
)

// ResizeMode selects the interpolation used by Resize.
type ResizeMode int

const (
	// NearestNeighbor selects the source pixel nearest to each destination
	// pixel. It is fast and exact for integer scale factors but blocky.
	NearestNeighbor ResizeMode = iota
	// Bilinear linearly interpolates the four nearest source pixels. It is
	// smoother than nearest-neighbour at the cost of more arithmetic.
	Bilinear
)

// Grayscale returns a copy of img with every pixel replaced by its
// luminance-weighted gray value (Rec. 601 coefficients). Alpha is preserved.
func Grayscale(img image.Image) *image.RGBA {
	src := ToRGBA(img)
	dst := newLike(src)
	kernels.Grayscale(dst.Pix, src.Pix)
	return dst
}

// Invert returns a copy of img with the R, G and B channels negated. Alpha is
// preserved.
func Invert(img image.Image) *image.RGBA {
	src := ToRGBA(img)
	dst := newLike(src)
	kernels.Invert(dst.Pix, src.Pix)
	return dst
}

// AdjustBrightness returns a copy of img with delta added to the R, G and B
// channels, clamped to [0, 255]. Alpha is preserved.
func AdjustBrightness(img image.Image, delta float64) *image.RGBA {
	src := ToRGBA(img)
	dst := newLike(src)
	kernels.AdjustBrightness(dst.Pix, src.Pix, delta)
	return dst
}

// AdjustContrast returns a copy of img with the R, G and B channels scaled
// about the mid-point (128) by factor, clamped to [0, 255]. A factor of 1
// leaves the image unchanged; values above 1 increase contrast, values in
// [0, 1) reduce it. Alpha is preserved.
func AdjustContrast(img image.Image, factor float64) *image.RGBA {
	src := ToRGBA(img)
	dst := newLike(src)
	kernels.AdjustContrast(dst.Pix, src.Pix, factor)
	return dst
}

// Resize returns img scaled to w by h pixels using the given mode. It returns
// an error if w or h is not positive.
func Resize(img image.Image, w, h int, mode ResizeMode) (*image.RGBA, error) {
	if w <= 0 || h <= 0 {
		return nil, fmt.Errorf("images: resize: dimensions must be positive, got %dx%d", w, h)
	}
	src := ToRGBA(img)
	b := src.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	switch mode {
	case NearestNeighbor:
		kernels.ResizeNearest(dst.Pix, src.Pix, b.Dx(), b.Dy(), w, h)
	case Bilinear:
		kernels.ResizeBilinear(dst.Pix, src.Pix, b.Dx(), b.Dy(), w, h)
	default:
		return nil, fmt.Errorf("images: resize: unknown mode %d", mode)
	}
	return dst, nil
}

// Kernel is a 2-D convolution kernel: Weights is a row-major slice of length
// Width*Height, and both Width and Height must be odd.
type Kernel struct {
	Width   int
	Height  int
	Weights []float64
}

// Convolve returns img convolved with k, using clamp-to-edge addressing at the
// borders. The R, G and B channels are convolved and clamped to [0, 255];
// alpha is preserved. It returns an error if k has non-positive or even
// dimensions, or if the length of k.Weights does not match Width*Height.
func Convolve(img image.Image, k Kernel) (*image.RGBA, error) {
	if k.Width <= 0 || k.Height <= 0 {
		return nil, fmt.Errorf("images: convolve: kernel dimensions must be positive, got %dx%d", k.Width, k.Height)
	}
	if k.Width%2 == 0 || k.Height%2 == 0 {
		return nil, fmt.Errorf("images: convolve: kernel dimensions must be odd, got %dx%d", k.Width, k.Height)
	}
	if len(k.Weights) != k.Width*k.Height {
		return nil, fmt.Errorf("images: convolve: have %d weights, want %d for a %dx%d kernel", len(k.Weights), k.Width*k.Height, k.Width, k.Height)
	}
	src := ToRGBA(img)
	b := src.Bounds()
	dst := newLike(src)
	kernels.Convolve(dst.Pix, src.Pix, b.Dx(), b.Dy(), k.Weights, k.Width, k.Height)
	return dst, nil
}

// GaussianBlur returns img blurred by a Gaussian of standard deviation sigma,
// implemented as a separable convolution with clamp-to-edge borders. It returns
// an error if sigma is not positive.
func GaussianBlur(img image.Image, sigma float64) (*image.RGBA, error) {
	if sigma <= 0 {
		return nil, fmt.Errorf("images: gaussian blur: sigma must be positive, got %g", sigma)
	}
	src := ToRGBA(img)
	b := src.Bounds()
	dst := newLike(src)
	tmp := newLike(src)
	k := kernels.GaussianKernel1D(sigma)
	kernels.ConvolveSeparable(dst.Pix, tmp.Pix, src.Pix, b.Dx(), b.Dy(), k)
	return dst, nil
}
