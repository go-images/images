// Code generated from the exported API of ./ops; DO NOT EDIT.
// ops_forward_complete_test.go fails when ./ops gains a symbol this file lacks.

package images

import (
	"image"

	"github.com/go-gfx/gfx/raster"

	"github.com/go-images/images/ops"
)

// Affine is [ops.Affine].
type Affine = ops.Affine

// BorderMode is [ops.BorderMode].
type BorderMode = ops.BorderMode

// Interp is [ops.Interp].
type Interp = ops.Interp

// Kernel is [ops.Kernel].
type Kernel = ops.Kernel

// ResizeMode is [ops.ResizeMode].
type ResizeMode = ops.ResizeMode

// SkewCriterion is [ops.SkewCriterion].
type SkewCriterion = ops.SkewCriterion

// SkewOptions is [ops.SkewOptions].
type SkewOptions = ops.SkewOptions

// The constants of [ops], under their names here.
const (
	AdjacentDifference = ops.AdjacentDifference
	Area               = ops.Area
	Bicubic            = ops.Bicubic
	Bilinear           = ops.Bilinear
	BorderConstant     = ops.BorderConstant
	BorderEdge         = ops.BorderEdge
	InterpBilinear     = ops.InterpBilinear
	InterpNearest      = ops.InterpNearest
	Lanczos            = ops.Lanczos
	NearestNeighbor    = ops.NearestNeighbor
	Variance           = ops.Variance
)

// The errors of [ops], under their names here.
var (
	ErrSkewBlank = ops.ErrSkewBlank
	ErrSkewEmpty = ops.ErrSkewEmpty
)

// AdjustBrightness forwards to [ops.AdjustBrightness], the codec-free implementation.
func AdjustBrightness(img image.Image, delta float64) *image.RGBA {
	return ops.AdjustBrightness(img, delta)
}

// AdjustBrightnessRaster forwards to [ops.AdjustBrightnessRaster], the codec-free implementation.
func AdjustBrightnessRaster(src *raster.Image, delta float64) *raster.Image {
	return ops.AdjustBrightnessRaster(src, delta)
}

// AdjustContrast forwards to [ops.AdjustContrast], the codec-free implementation.
func AdjustContrast(img image.Image, factor float64) *image.RGBA {
	return ops.AdjustContrast(img, factor)
}

// AdjustContrastRaster forwards to [ops.AdjustContrastRaster], the codec-free implementation.
func AdjustContrastRaster(src *raster.Image, factor float64) *raster.Image {
	return ops.AdjustContrastRaster(src, factor)
}

// AsRGBA forwards to [ops.AsRGBA], the codec-free implementation.
func AsRGBA(r *raster.Image) *image.RGBA {
	return ops.AsRGBA(r)
}

// AsRaster forwards to [ops.AsRaster], the codec-free implementation.
func AsRaster(img *image.RGBA) *raster.Image {
	return ops.AsRaster(img)
}

// BoxBlur forwards to [ops.BoxBlur], the codec-free implementation.
func BoxBlur(img image.Image, radius int) (*image.RGBA, error) {
	return ops.BoxBlur(img, radius)
}

// BoxBlurRaster forwards to [ops.BoxBlurRaster], the codec-free implementation.
func BoxBlurRaster(src *raster.Image, radius int) (*raster.Image, error) {
	return ops.BoxBlurRaster(src, radius)
}

// Canny forwards to [ops.Canny], the codec-free implementation.
func Canny(img image.Image, sigma, low, high float64) (*image.RGBA, error) {
	return ops.Canny(img, sigma, low, high)
}

// CannyRaster forwards to [ops.CannyRaster], the codec-free implementation.
func CannyRaster(src *raster.Image, sigma, low, high float64) (*raster.Image, error) {
	return ops.CannyRaster(src, sigma, low, high)
}

// Close forwards to [ops.Close], the codec-free implementation.
func Close(img image.Image, radius int) (*image.RGBA, error) {
	return ops.Close(img, radius)
}

// CloseRaster forwards to [ops.CloseRaster], the codec-free implementation.
func CloseRaster(src *raster.Image, radius int) (*raster.Image, error) {
	return ops.CloseRaster(src, radius)
}

// Convolve forwards to [ops.Convolve], the codec-free implementation.
func Convolve(img image.Image, k Kernel) (*image.RGBA, error) {
	return ops.Convolve(img, k)
}

// ConvolveRaster forwards to [ops.ConvolveRaster], the codec-free implementation.
func ConvolveRaster(src *raster.Image, k Kernel) (*raster.Image, error) {
	return ops.ConvolveRaster(src, k)
}

// Crop forwards to [ops.Crop], the codec-free implementation.
func Crop(img image.Image, r image.Rectangle) (*image.RGBA, error) {
	return ops.Crop(img, r)
}

// CropRaster forwards to [ops.CropRaster], the codec-free implementation.
func CropRaster(src *raster.Image, r image.Rectangle) (*raster.Image, error) {
	return ops.CropRaster(src, r)
}

// Deskew forwards to [ops.Deskew], the codec-free implementation.
func Deskew(img image.Image, threshold float64, opt SkewOptions) (*image.RGBA, float64, error) {
	return ops.Deskew(img, threshold, opt)
}

// Dilate forwards to [ops.Dilate], the codec-free implementation.
func Dilate(img image.Image, radius int) (*image.RGBA, error) {
	return ops.Dilate(img, radius)
}

// DilateRaster forwards to [ops.DilateRaster], the codec-free implementation.
func DilateRaster(src *raster.Image, radius int) (*raster.Image, error) {
	return ops.DilateRaster(src, radius)
}

// Erode forwards to [ops.Erode], the codec-free implementation.
func Erode(img image.Image, radius int) (*image.RGBA, error) {
	return ops.Erode(img, radius)
}

// ErodeRaster forwards to [ops.ErodeRaster], the codec-free implementation.
func ErodeRaster(src *raster.Image, radius int) (*raster.Image, error) {
	return ops.ErodeRaster(src, radius)
}

// FlipHorizontal forwards to [ops.FlipHorizontal], the codec-free implementation.
func FlipHorizontal(img image.Image) *image.RGBA {
	return ops.FlipHorizontal(img)
}

// FlipHorizontalRaster forwards to [ops.FlipHorizontalRaster], the codec-free implementation.
func FlipHorizontalRaster(src *raster.Image) *raster.Image {
	return ops.FlipHorizontalRaster(src)
}

// FlipVertical forwards to [ops.FlipVertical], the codec-free implementation.
func FlipVertical(img image.Image) *image.RGBA {
	return ops.FlipVertical(img)
}

// FlipVerticalRaster forwards to [ops.FlipVerticalRaster], the codec-free implementation.
func FlipVerticalRaster(src *raster.Image) *raster.Image {
	return ops.FlipVerticalRaster(src)
}

// GaussianBlur forwards to [ops.GaussianBlur], the codec-free implementation.
func GaussianBlur(img image.Image, sigma float64) (*image.RGBA, error) {
	return ops.GaussianBlur(img, sigma)
}

// GaussianBlurRaster forwards to [ops.GaussianBlurRaster], the codec-free implementation.
func GaussianBlurRaster(src *raster.Image, sigma float64) (*raster.Image, error) {
	return ops.GaussianBlurRaster(src, sigma)
}

// Grayscale forwards to [ops.Grayscale], the codec-free implementation.
func Grayscale(img image.Image) *image.RGBA {
	return ops.Grayscale(img)
}

// GrayscaleRaster forwards to [ops.GrayscaleRaster], the codec-free implementation.
func GrayscaleRaster(src *raster.Image) *raster.Image {
	return ops.GrayscaleRaster(src)
}

// HSVToRGB forwards to [ops.HSVToRGB], the codec-free implementation.
func HSVToRGB(img image.Image) *image.RGBA {
	return ops.HSVToRGB(img)
}

// HSVToRGBRaster forwards to [ops.HSVToRGBRaster], the codec-free implementation.
func HSVToRGBRaster(src *raster.Image) *raster.Image {
	return ops.HSVToRGBRaster(src)
}

// Identity forwards to [ops.Identity], the codec-free implementation.
func Identity() Affine {
	return ops.Identity()
}

// Invert forwards to [ops.Invert], the codec-free implementation.
func Invert(img image.Image) *image.RGBA {
	return ops.Invert(img)
}

// InvertRaster forwards to [ops.InvertRaster], the codec-free implementation.
func InvertRaster(src *raster.Image) *raster.Image {
	return ops.InvertRaster(src)
}

// Laplacian forwards to [ops.Laplacian], the codec-free implementation.
func Laplacian(img image.Image) *image.RGBA {
	return ops.Laplacian(img)
}

// LaplacianRaster forwards to [ops.LaplacianRaster], the codec-free implementation.
func LaplacianRaster(src *raster.Image) *raster.Image {
	return ops.LaplacianRaster(src)
}

// Median forwards to [ops.Median], the codec-free implementation.
func Median(img image.Image, radius int) (*image.RGBA, error) {
	return ops.Median(img, radius)
}

// MedianRaster forwards to [ops.MedianRaster], the codec-free implementation.
func MedianRaster(src *raster.Image, radius int) (*raster.Image, error) {
	return ops.MedianRaster(src, radius)
}

// Open forwards to [ops.Open], the codec-free implementation.
func Open(img image.Image, radius int) (*image.RGBA, error) {
	return ops.Open(img, radius)
}

// OpenRaster forwards to [ops.OpenRaster], the codec-free implementation.
func OpenRaster(src *raster.Image, radius int) (*raster.Image, error) {
	return ops.OpenRaster(src, radius)
}

// Otsu forwards to [ops.Otsu], the codec-free implementation.
func Otsu(img image.Image) *image.RGBA {
	return ops.Otsu(img)
}

// OtsuRaster forwards to [ops.OtsuRaster], the codec-free implementation.
func OtsuRaster(src *raster.Image) *raster.Image {
	return ops.OtsuRaster(src)
}

// OtsuThreshold forwards to [ops.OtsuThreshold], the codec-free implementation.
func OtsuThreshold(img image.Image) uint8 {
	return ops.OtsuThreshold(img)
}

// Prewitt forwards to [ops.Prewitt], the codec-free implementation.
func Prewitt(img image.Image) *image.RGBA {
	return ops.Prewitt(img)
}

// PrewittRaster forwards to [ops.PrewittRaster], the codec-free implementation.
func PrewittRaster(src *raster.Image) *raster.Image {
	return ops.PrewittRaster(src)
}

// RGBToHSV forwards to [ops.RGBToHSV], the codec-free implementation.
func RGBToHSV(img image.Image) *image.RGBA {
	return ops.RGBToHSV(img)
}

// RGBToHSVRaster forwards to [ops.RGBToHSVRaster], the codec-free implementation.
func RGBToHSVRaster(src *raster.Image) *raster.Image {
	return ops.RGBToHSVRaster(src)
}

// Resize forwards to [ops.Resize], the codec-free implementation.
func Resize(img image.Image, w, h int, mode ResizeMode) (*image.RGBA, error) {
	return ops.Resize(img, w, h, mode)
}

// ResizeRaster forwards to [ops.ResizeRaster], the codec-free implementation.
func ResizeRaster(src *raster.Image, w, h int, mode ResizeMode) (*raster.Image, error) {
	return ops.ResizeRaster(src, w, h, mode)
}

// Rotate forwards to [ops.Rotate], the codec-free implementation.
func Rotate(img image.Image, angle float64, resize bool) *image.RGBA {
	return ops.Rotate(img, angle, resize)
}

// Rotate180 forwards to [ops.Rotate180], the codec-free implementation.
func Rotate180(img image.Image) *image.RGBA {
	return ops.Rotate180(img)
}

// Rotate180Raster forwards to [ops.Rotate180Raster], the codec-free implementation.
func Rotate180Raster(src *raster.Image) *raster.Image {
	return ops.Rotate180Raster(src)
}

// Rotate270 forwards to [ops.Rotate270], the codec-free implementation.
func Rotate270(img image.Image) *image.RGBA {
	return ops.Rotate270(img)
}

// Rotate270Raster forwards to [ops.Rotate270Raster], the codec-free implementation.
func Rotate270Raster(src *raster.Image) *raster.Image {
	return ops.Rotate270Raster(src)
}

// Rotate90 forwards to [ops.Rotate90], the codec-free implementation.
func Rotate90(img image.Image) *image.RGBA {
	return ops.Rotate90(img)
}

// Rotate90Raster forwards to [ops.Rotate90Raster], the codec-free implementation.
func Rotate90Raster(src *raster.Image) *raster.Image {
	return ops.Rotate90Raster(src)
}

// Rotation forwards to [ops.Rotation], the codec-free implementation.
func Rotation(theta float64) Affine {
	return ops.Rotation(theta)
}

// Scaling forwards to [ops.Scaling], the codec-free implementation.
func Scaling(sx, sy float64) Affine {
	return ops.Scaling(sx, sy)
}

// Scharr forwards to [ops.Scharr], the codec-free implementation.
func Scharr(img image.Image) *image.RGBA {
	return ops.Scharr(img)
}

// ScharrRaster forwards to [ops.ScharrRaster], the codec-free implementation.
func ScharrRaster(src *raster.Image) *raster.Image {
	return ops.ScharrRaster(src)
}

// Sharpen forwards to [ops.Sharpen], the codec-free implementation.
func Sharpen(img image.Image) *image.RGBA {
	return ops.Sharpen(img)
}

// SharpenRaster forwards to [ops.SharpenRaster], the codec-free implementation.
func SharpenRaster(src *raster.Image) *raster.Image {
	return ops.SharpenRaster(src)
}

// SkewAngle forwards to [ops.SkewAngle], the codec-free implementation.
func SkewAngle(img image.Image, opt SkewOptions) (float64, error) {
	return ops.SkewAngle(img, opt)
}

// Sobel forwards to [ops.Sobel], the codec-free implementation.
func Sobel(img image.Image) *image.RGBA {
	return ops.Sobel(img)
}

// SobelMag forwards to [ops.SobelMag], the codec-free implementation.
func SobelMag(img image.Image) *image.RGBA {
	return ops.SobelMag(img)
}

// SobelMagRaster forwards to [ops.SobelMagRaster], the codec-free implementation.
func SobelMagRaster(src *raster.Image) *raster.Image {
	return ops.SobelMagRaster(src)
}

// SobelRaster forwards to [ops.SobelRaster], the codec-free implementation.
func SobelRaster(src *raster.Image) *raster.Image {
	return ops.SobelRaster(src)
}

// SobelX forwards to [ops.SobelX], the codec-free implementation.
func SobelX(img image.Image) *image.RGBA {
	return ops.SobelX(img)
}

// SobelXRaster forwards to [ops.SobelXRaster], the codec-free implementation.
func SobelXRaster(src *raster.Image) *raster.Image {
	return ops.SobelXRaster(src)
}

// SobelY forwards to [ops.SobelY], the codec-free implementation.
func SobelY(img image.Image) *image.RGBA {
	return ops.SobelY(img)
}

// SobelYRaster forwards to [ops.SobelYRaster], the codec-free implementation.
func SobelYRaster(src *raster.Image) *raster.Image {
	return ops.SobelYRaster(src)
}

// Threshold forwards to [ops.Threshold], the codec-free implementation.
func Threshold(img image.Image, t uint8) *image.RGBA {
	return ops.Threshold(img, t)
}

// ThresholdRaster forwards to [ops.ThresholdRaster], the codec-free implementation.
func ThresholdRaster(src *raster.Image, t uint8) *raster.Image {
	return ops.ThresholdRaster(src, t)
}

// ToRGBA forwards to [ops.ToRGBA], the codec-free implementation.
func ToRGBA(img image.Image) *image.RGBA {
	return ops.ToRGBA(img)
}

// Translation forwards to [ops.Translation], the codec-free implementation.
func Translation(tx, ty float64) Affine {
	return ops.Translation(tx, ty)
}

// Transpose forwards to [ops.Transpose], the codec-free implementation.
func Transpose(img image.Image) *image.RGBA {
	return ops.Transpose(img)
}

// Transverse forwards to [ops.Transverse], the codec-free implementation.
func Transverse(img image.Image) *image.RGBA {
	return ops.Transverse(img)
}

// UnsharpMask forwards to [ops.UnsharpMask], the codec-free implementation.
func UnsharpMask(img image.Image, radius, amount float64) (*image.RGBA, error) {
	return ops.UnsharpMask(img, radius, amount)
}

// UnsharpMaskRaster forwards to [ops.UnsharpMaskRaster], the codec-free implementation.
func UnsharpMaskRaster(src *raster.Image, radius, amount float64) (*raster.Image, error) {
	return ops.UnsharpMaskRaster(src, radius, amount)
}

// Warp forwards to [ops.Warp], the codec-free implementation.
func Warp(img image.Image, inv Affine, outW, outH int, interp Interp, mode BorderMode, cval float64) *image.RGBA {
	return ops.Warp(img, inv, outW, outH, interp, mode, cval)
}
