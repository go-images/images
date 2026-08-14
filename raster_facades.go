package images

import (
	"image"

	"github.com/go-gfx/gfx/raster"
)

// Raster-typed façades over the dense image operations of this package.
//
// Every processing function in go-images takes an image.Image and returns a
// freshly allocated *image.RGBA. Callers already working in go-gfx's shared 2-D
// substrate (raster.Image) would otherwise have to spell the conversion out at
// each call site. Each façade below removes that boilerplate: XxxRaster is
// exactly AsRaster(Xxx(AsRGBA(src))).
//
// Because AsRGBA views src's buffer without copying and every operation reads
// its source and writes a brand-new dense, origin-anchored destination, AsRaster
// of that destination is likewise a zero-copy view. The façade therefore adds no
// per-pixel work and does not touch src's bytes; its only overhead is O(1) per
// call — two small header allocations (the aliasing *image.RGBA view of src and
// the returned *raster.Image view of the result), independent of image size. The
// output bytes are identical to the *image.RGBA path, proven byte-for-byte in
// raster_facades_test.go against an independent image.RGBA reference.
//
// The scalar analysis OtsuThreshold (which returns a uint8, not an image) has no
// façade — it takes an image.Image already, and there is nothing to convert on
// the way out. Resize's façade, ResizeRaster, lives in raster.go beside the
// adapters it is built from.

// GrayscaleRaster is the raster-typed façade for Grayscale.
func GrayscaleRaster(src *raster.Image) *raster.Image {
	return AsRaster(Grayscale(AsRGBA(src)))
}

// InvertRaster is the raster-typed façade for Invert.
func InvertRaster(src *raster.Image) *raster.Image {
	return AsRaster(Invert(AsRGBA(src)))
}

// AdjustBrightnessRaster is the raster-typed façade for AdjustBrightness.
func AdjustBrightnessRaster(src *raster.Image, delta float64) *raster.Image {
	return AsRaster(AdjustBrightness(AsRGBA(src), delta))
}

// AdjustContrastRaster is the raster-typed façade for AdjustContrast.
func AdjustContrastRaster(src *raster.Image, factor float64) *raster.Image {
	return AsRaster(AdjustContrast(AsRGBA(src), factor))
}

// ThresholdRaster is the raster-typed façade for Threshold.
func ThresholdRaster(src *raster.Image, t uint8) *raster.Image {
	return AsRaster(Threshold(AsRGBA(src), t))
}

// OtsuRaster is the raster-typed façade for Otsu.
func OtsuRaster(src *raster.Image) *raster.Image {
	return AsRaster(Otsu(AsRGBA(src)))
}

// RGBToHSVRaster is the raster-typed façade for RGBToHSV.
func RGBToHSVRaster(src *raster.Image) *raster.Image {
	return AsRaster(RGBToHSV(AsRGBA(src)))
}

// HSVToRGBRaster is the raster-typed façade for HSVToRGB.
func HSVToRGBRaster(src *raster.Image) *raster.Image {
	return AsRaster(HSVToRGB(AsRGBA(src)))
}

// SobelRaster is the raster-typed façade for Sobel.
func SobelRaster(src *raster.Image) *raster.Image {
	return AsRaster(Sobel(AsRGBA(src)))
}

// SobelXRaster is the raster-typed façade for SobelX.
func SobelXRaster(src *raster.Image) *raster.Image {
	return AsRaster(SobelX(AsRGBA(src)))
}

// SobelYRaster is the raster-typed façade for SobelY.
func SobelYRaster(src *raster.Image) *raster.Image {
	return AsRaster(SobelY(AsRGBA(src)))
}

// PrewittRaster is the raster-typed façade for Prewitt.
func PrewittRaster(src *raster.Image) *raster.Image {
	return AsRaster(Prewitt(AsRGBA(src)))
}

// ScharrRaster is the raster-typed façade for Scharr.
func ScharrRaster(src *raster.Image) *raster.Image {
	return AsRaster(Scharr(AsRGBA(src)))
}

// SobelMagRaster is the raster-typed façade for SobelMag.
func SobelMagRaster(src *raster.Image) *raster.Image {
	return AsRaster(SobelMag(AsRGBA(src)))
}

// LaplacianRaster is the raster-typed façade for Laplacian.
func LaplacianRaster(src *raster.Image) *raster.Image {
	return AsRaster(Laplacian(AsRGBA(src)))
}

// SharpenRaster is the raster-typed façade for Sharpen.
func SharpenRaster(src *raster.Image) *raster.Image {
	return AsRaster(Sharpen(AsRGBA(src)))
}

// FlipHorizontalRaster is the raster-typed façade for FlipHorizontal.
func FlipHorizontalRaster(src *raster.Image) *raster.Image {
	return AsRaster(FlipHorizontal(AsRGBA(src)))
}

// FlipVerticalRaster is the raster-typed façade for FlipVertical.
func FlipVerticalRaster(src *raster.Image) *raster.Image {
	return AsRaster(FlipVertical(AsRGBA(src)))
}

// Rotate90Raster is the raster-typed façade for Rotate90.
func Rotate90Raster(src *raster.Image) *raster.Image {
	return AsRaster(Rotate90(AsRGBA(src)))
}

// Rotate180Raster is the raster-typed façade for Rotate180.
func Rotate180Raster(src *raster.Image) *raster.Image {
	return AsRaster(Rotate180(AsRGBA(src)))
}

// Rotate270Raster is the raster-typed façade for Rotate270.
func Rotate270Raster(src *raster.Image) *raster.Image {
	return AsRaster(Rotate270(AsRGBA(src)))
}

// BoxBlurRaster is the raster-typed façade for BoxBlur; it propagates BoxBlur's
// error for a non-positive radius.
func BoxBlurRaster(src *raster.Image, radius int) (*raster.Image, error) {
	out, err := BoxBlur(AsRGBA(src), radius)
	if err != nil {
		return nil, err
	}
	return AsRaster(out), nil
}

// GaussianBlurRaster is the raster-typed façade for GaussianBlur; it propagates
// GaussianBlur's error for a non-positive sigma.
func GaussianBlurRaster(src *raster.Image, sigma float64) (*raster.Image, error) {
	out, err := GaussianBlur(AsRGBA(src), sigma)
	if err != nil {
		return nil, err
	}
	return AsRaster(out), nil
}

// MedianRaster is the raster-typed façade for Median; it propagates Median's
// error for a non-positive radius.
func MedianRaster(src *raster.Image, radius int) (*raster.Image, error) {
	out, err := Median(AsRGBA(src), radius)
	if err != nil {
		return nil, err
	}
	return AsRaster(out), nil
}

// UnsharpMaskRaster is the raster-typed façade for UnsharpMask; it propagates
// UnsharpMask's error for a non-positive radius.
func UnsharpMaskRaster(src *raster.Image, radius, amount float64) (*raster.Image, error) {
	out, err := UnsharpMask(AsRGBA(src), radius, amount)
	if err != nil {
		return nil, err
	}
	return AsRaster(out), nil
}

// ConvolveRaster is the raster-typed façade for Convolve; it propagates
// Convolve's error for a kernel with non-positive, even, or mismatched
// dimensions.
func ConvolveRaster(src *raster.Image, k Kernel) (*raster.Image, error) {
	out, err := Convolve(AsRGBA(src), k)
	if err != nil {
		return nil, err
	}
	return AsRaster(out), nil
}

// CannyRaster is the raster-typed façade for Canny; it propagates Canny's error
// for a non-positive sigma, a negative threshold, or high < low.
func CannyRaster(src *raster.Image, sigma, low, high float64) (*raster.Image, error) {
	out, err := Canny(AsRGBA(src), sigma, low, high)
	if err != nil {
		return nil, err
	}
	return AsRaster(out), nil
}

// ErodeRaster is the raster-typed façade for Erode; it propagates Erode's error
// for a non-positive radius.
func ErodeRaster(src *raster.Image, radius int) (*raster.Image, error) {
	out, err := Erode(AsRGBA(src), radius)
	if err != nil {
		return nil, err
	}
	return AsRaster(out), nil
}

// DilateRaster is the raster-typed façade for Dilate; it propagates Dilate's
// error for a non-positive radius.
func DilateRaster(src *raster.Image, radius int) (*raster.Image, error) {
	out, err := Dilate(AsRGBA(src), radius)
	if err != nil {
		return nil, err
	}
	return AsRaster(out), nil
}

// OpenRaster is the raster-typed façade for Open; it propagates Open's error for
// a non-positive radius.
func OpenRaster(src *raster.Image, radius int) (*raster.Image, error) {
	out, err := Open(AsRGBA(src), radius)
	if err != nil {
		return nil, err
	}
	return AsRaster(out), nil
}

// CloseRaster is the raster-typed façade for Close; it propagates Close's error
// for a non-positive radius.
func CloseRaster(src *raster.Image, radius int) (*raster.Image, error) {
	out, err := Close(AsRGBA(src), radius)
	if err != nil {
		return nil, err
	}
	return AsRaster(out), nil
}

// CropRaster is the raster-typed façade for Crop; it propagates Crop's error for
// an empty rectangle or one extending outside the image bounds.
func CropRaster(src *raster.Image, r image.Rectangle) (*raster.Image, error) {
	out, err := Crop(AsRGBA(src), r)
	if err != nil {
		return nil, err
	}
	return AsRaster(out), nil
}
