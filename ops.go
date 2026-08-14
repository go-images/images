package images

import (
	"fmt"
	"image"

	"github.com/go-gfx/gfx/resample"
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
	// Area averages the source region each destination pixel covers, weighting
	// every source pixel by how much of it falls inside. It is the mode to
	// REDUCE with: nearest-neighbour discards all but one source pixel per
	// destination pixel, so shrinking by four throws away fifteen sixteenths of
	// the image and aliases what is left, while bilinear only ever looks at
	// four neighbours however far the image is shrunk. Enlarging, where each
	// destination pixel falls inside one source pixel, it reduces to
	// nearest-neighbour.
	//
	// This is PIL's Image.BOX and OpenCV's INTER_AREA; at integer ratios it
	// reduces to scikit-image's downscale_local_mean.
	Area
	// Bicubic resamples with the Keys cubic (a = -1/2, the Catmull-Rom spline),
	// a smooth four-tap kernel whose footprint widens with the reduction factor:
	// sharper than Bilinear when enlarging and a proper antialiasing low-pass
	// when reducing. It is Pillow's BICUBIC / x/image/draw's CatmullRom. The
	// colour channels are filtered in premultiplied-alpha space so a transparent
	// pixel's colour does not bleed into the visible edge of a cut-out; on a
	// fully opaque image that is exactly the plain cubic. Higher quality than the
	// three modes above, at more arithmetic.
	Bicubic
	// Lanczos resamples with the a = 3 windowed-sinc kernel: wider and sharper
	// than Bicubic at more cost, the highest-fidelity mode in either direction.
	// It is Pillow's LANCZOS / x/image/draw's Lanczos, and like Bicubic filters
	// the colour channels in premultiplied-alpha space.
	Lanczos
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
//
// The resampling kernels live once, in go-gfx's resample package (the shared 2-D
// foundation this library sits on); Resize delegates to them over a zero-copy
// raster view of the source so no kernel is duplicated here. NearestNeighbor,
// Bilinear and Area map to resample's Nearest, Bilinear and Box filtered in
// straight-alpha space — proven byte-for-byte identical to the kernels this
// library used to run (see resize_dedup_control_test.go) — while Bicubic and
// Lanczos filter the colour channels in premultiplied-alpha space.
func Resize(img image.Image, w, h int, mode ResizeMode) (*image.RGBA, error) {
	if w <= 0 || h <= 0 {
		return nil, fmt.Errorf("images: resize: dimensions must be positive, got %dx%d", w, h)
	}
	src := AsRaster(ToRGBA(img))
	// resize is the straight-alpha resampler for the three modes proven
	// byte-identical to this library's former kernels; Bicubic and Lanczos swap
	// it for the premultiplied-alpha variant. An unrecognised mode maps to an
	// out-of-range resample mode so the single error check below rejects it,
	// keeping mode validation in one place rather than duplicating resample's.
	resize := resample.Resize
	var rmode resample.Mode
	switch mode {
	case NearestNeighbor:
		rmode = resample.Nearest
	case Bilinear:
		rmode = resample.Bilinear
	case Area:
		rmode = resample.Box
	case Bicubic:
		rmode, resize = resample.Bicubic, resample.ResizePremultiplied
	case Lanczos:
		rmode, resize = resample.Lanczos, resample.ResizePremultiplied
	default:
		rmode = resample.Mode(-1)
	}
	out, err := resize(src, w, h, rmode)
	if err != nil {
		return nil, fmt.Errorf("images: resize: %w", err)
	}
	return AsRGBA(out), nil
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

// Sobel returns the Sobel gradient-magnitude edge map of img. The operator is
// applied to each pixel's Rec. 601 luminance with clamp-to-edge borders; the
// magnitude is clamped to [0, 255] and written to the R, G and B channels,
// producing a grayscale edge image. Alpha is preserved. Strong intensity
// transitions appear bright, flat regions dark.
func Sobel(img image.Image) *image.RGBA {
	src := ToRGBA(img)
	b := src.Bounds()
	dst := newLike(src)
	kernels.Sobel(dst.Pix, src.Pix, b.Dx(), b.Dy())
	return dst
}

// SobelX returns the horizontal Sobel response of img: an estimate of the
// left-to-right intensity derivative of each pixel's luminance. The signed
// response is scaled and offset so a zero gradient is mid-grey (128), a rising
// edge brighter and a falling edge darker, clamped to [0, 255] and written to
// R, G and B. Alpha is preserved; borders use clamp-to-edge addressing.
func SobelX(img image.Image) *image.RGBA {
	src := ToRGBA(img)
	b := src.Bounds()
	dst := newLike(src)
	kernels.SobelX(dst.Pix, src.Pix, b.Dx(), b.Dy())
	return dst
}

// SobelY returns the vertical Sobel response of img: an estimate of the
// top-to-bottom intensity derivative of each pixel's luminance, with the same
// scaling, offset and addressing conventions as SobelX. Alpha is preserved.
func SobelY(img image.Image) *image.RGBA {
	src := ToRGBA(img)
	b := src.Bounds()
	dst := newLike(src)
	kernels.SobelY(dst.Pix, src.Pix, b.Dx(), b.Dy())
	return dst
}

// BoxBlur returns img blurred by a square averaging filter of the given radius:
// every output pixel is the mean of the (2*radius+1) by (2*radius+1) source
// neighbourhood centred on it, computed independently per R, G and B channel
// (alpha preserved). Borders use clamp-to-edge addressing, matching
// scipy.ndimage.uniform_filter with mode="nearest" and size 2*radius+1. The
// filter is separable and evaluated with a running window sum, so its cost is
// independent of the radius. It returns an error if radius is not positive.
func BoxBlur(img image.Image, radius int) (*image.RGBA, error) {
	if radius <= 0 {
		return nil, fmt.Errorf("images: box blur: radius must be positive, got %d", radius)
	}
	src := ToRGBA(img)
	b := src.Bounds()
	dst := newLike(src)
	kernels.BoxBlur(dst.Pix, src.Pix, b.Dx(), b.Dy(), radius)
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
