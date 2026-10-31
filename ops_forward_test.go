// Code generated from the exported API of ./ops; DO NOT EDIT.

package images

import (
	"image"
	"image/color"
	"reflect"
	"testing"

	"github.com/go-images/images/ops"
)

func fwdImage() *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	for i := range img.Pix {
		img.Pix[i] = uint8(i * 37)
	}
	return img
}

// fwdSkewPage is a page with horizontal lines on it, because the skew
// forwarders are the only ones that REFUSE an input: fwdImage is a four-pixel
// gradient, and both of them would come back as ErrSkewBlank — comparing two
// refusals would say nothing about what is forwarded.
func fwdSkewPage() *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, 24, 24))
	for i := range img.Pix {
		img.Pix[i] = 0xff
	}
	for y := 2; y < 22; y += 4 {
		for x := 2; x < 22; x++ {
			img.SetRGBA(x, y, color.RGBA{0, 0, 0, 0xff})
		}
	}
	return img
}

// TestForwardersAreIdentity calls every root forwarder and its ops target
// with the same arguments and requires identical results.
func TestForwardersAreIdentity(t *testing.T) {
	check := func(name string, got, want []any) {
		t.Helper()
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s: forwarder %v, ops %v", name, got, want)
		}
	}
	check("AdjustBrightness", []any{AdjustBrightness(fwdImage(), 1)}, []any{ops.AdjustBrightness(fwdImage(), 1)})
	check("AdjustBrightnessRaster", []any{AdjustBrightnessRaster(ops.AsRaster(fwdImage()), 1)}, []any{ops.AdjustBrightnessRaster(ops.AsRaster(fwdImage()), 1)})
	check("AdjustContrast", []any{AdjustContrast(fwdImage(), 1)}, []any{ops.AdjustContrast(fwdImage(), 1)})
	check("AdjustContrastRaster", []any{AdjustContrastRaster(ops.AsRaster(fwdImage()), 1)}, []any{ops.AdjustContrastRaster(ops.AsRaster(fwdImage()), 1)})
	check("AsRGBA", []any{AsRGBA(ops.AsRaster(fwdImage()))}, []any{ops.AsRGBA(ops.AsRaster(fwdImage()))})
	check("AsRaster", []any{AsRaster(fwdImage())}, []any{ops.AsRaster(fwdImage())})
	{
		a0, a1 := BoxBlur(fwdImage(), 1)
		b0, b1 := ops.BoxBlur(fwdImage(), 1)
		check("BoxBlur", []any{a0, a1}, []any{b0, b1})
	}
	{
		a0, a1 := BoxBlurRaster(ops.AsRaster(fwdImage()), 1)
		b0, b1 := ops.BoxBlurRaster(ops.AsRaster(fwdImage()), 1)
		check("BoxBlurRaster", []any{a0, a1}, []any{b0, b1})
	}
	{
		a0, a1 := Canny(fwdImage(), 1, 1, 1)
		b0, b1 := ops.Canny(fwdImage(), 1, 1, 1)
		check("Canny", []any{a0, a1}, []any{b0, b1})
	}
	{
		a0, a1 := CannyRaster(ops.AsRaster(fwdImage()), 1, 1, 1)
		b0, b1 := ops.CannyRaster(ops.AsRaster(fwdImage()), 1, 1, 1)
		check("CannyRaster", []any{a0, a1}, []any{b0, b1})
	}
	{
		a0, a1 := Close(fwdImage(), 1)
		b0, b1 := ops.Close(fwdImage(), 1)
		check("Close", []any{a0, a1}, []any{b0, b1})
	}
	{
		a0, a1 := CloseRaster(ops.AsRaster(fwdImage()), 1)
		b0, b1 := ops.CloseRaster(ops.AsRaster(fwdImage()), 1)
		check("CloseRaster", []any{a0, a1}, []any{b0, b1})
	}
	{
		a0, a1 := Convolve(fwdImage(), Kernel{Width: 1, Height: 1, Weights: []float64{1}})
		b0, b1 := ops.Convolve(fwdImage(), Kernel{Width: 1, Height: 1, Weights: []float64{1}})
		check("Convolve", []any{a0, a1}, []any{b0, b1})
	}
	{
		a0, a1 := ConvolveRaster(ops.AsRaster(fwdImage()), Kernel{Width: 1, Height: 1, Weights: []float64{1}})
		b0, b1 := ops.ConvolveRaster(ops.AsRaster(fwdImage()), Kernel{Width: 1, Height: 1, Weights: []float64{1}})
		check("ConvolveRaster", []any{a0, a1}, []any{b0, b1})
	}
	{
		a0, a1 := Crop(fwdImage(), image.Rect(0, 0, 2, 2))
		b0, b1 := ops.Crop(fwdImage(), image.Rect(0, 0, 2, 2))
		check("Crop", []any{a0, a1}, []any{b0, b1})
	}
	{
		a0, a1 := CropRaster(ops.AsRaster(fwdImage()), image.Rect(0, 0, 2, 2))
		b0, b1 := ops.CropRaster(ops.AsRaster(fwdImage()), image.Rect(0, 0, 2, 2))
		check("CropRaster", []any{a0, a1}, []any{b0, b1})
	}
	{
		a0, a1, a2 := Deskew(fwdSkewPage(), 0.5, SkewOptions{})
		b0, b1, b2 := ops.Deskew(fwdSkewPage(), 0.5, SkewOptions{})
		check("Deskew", []any{a0, a1, a2}, []any{b0, b1, b2})
	}
	{
		a0, a1 := Dilate(fwdImage(), 1)
		b0, b1 := ops.Dilate(fwdImage(), 1)
		check("Dilate", []any{a0, a1}, []any{b0, b1})
	}
	{
		a0, a1 := DilateRaster(ops.AsRaster(fwdImage()), 1)
		b0, b1 := ops.DilateRaster(ops.AsRaster(fwdImage()), 1)
		check("DilateRaster", []any{a0, a1}, []any{b0, b1})
	}
	{
		a0, a1 := Erode(fwdImage(), 1)
		b0, b1 := ops.Erode(fwdImage(), 1)
		check("Erode", []any{a0, a1}, []any{b0, b1})
	}
	{
		a0, a1 := ErodeRaster(ops.AsRaster(fwdImage()), 1)
		b0, b1 := ops.ErodeRaster(ops.AsRaster(fwdImage()), 1)
		check("ErodeRaster", []any{a0, a1}, []any{b0, b1})
	}
	check("FlipHorizontal", []any{FlipHorizontal(fwdImage())}, []any{ops.FlipHorizontal(fwdImage())})
	check("FlipHorizontalRaster", []any{FlipHorizontalRaster(ops.AsRaster(fwdImage()))}, []any{ops.FlipHorizontalRaster(ops.AsRaster(fwdImage()))})
	check("FlipVertical", []any{FlipVertical(fwdImage())}, []any{ops.FlipVertical(fwdImage())})
	check("FlipVerticalRaster", []any{FlipVerticalRaster(ops.AsRaster(fwdImage()))}, []any{ops.FlipVerticalRaster(ops.AsRaster(fwdImage()))})
	{
		a0, a1 := GaussianBlur(fwdImage(), 1)
		b0, b1 := ops.GaussianBlur(fwdImage(), 1)
		check("GaussianBlur", []any{a0, a1}, []any{b0, b1})
	}
	{
		a0, a1 := GaussianBlurRaster(ops.AsRaster(fwdImage()), 1)
		b0, b1 := ops.GaussianBlurRaster(ops.AsRaster(fwdImage()), 1)
		check("GaussianBlurRaster", []any{a0, a1}, []any{b0, b1})
	}
	check("Grayscale", []any{Grayscale(fwdImage())}, []any{ops.Grayscale(fwdImage())})
	check("GrayscaleRaster", []any{GrayscaleRaster(ops.AsRaster(fwdImage()))}, []any{ops.GrayscaleRaster(ops.AsRaster(fwdImage()))})
	check("HSVToRGB", []any{HSVToRGB(fwdImage())}, []any{ops.HSVToRGB(fwdImage())})
	check("HSVToRGBRaster", []any{HSVToRGBRaster(ops.AsRaster(fwdImage()))}, []any{ops.HSVToRGBRaster(ops.AsRaster(fwdImage()))})
	check("Identity", []any{Identity()}, []any{ops.Identity()})
	check("Invert", []any{Invert(fwdImage())}, []any{ops.Invert(fwdImage())})
	check("InvertRaster", []any{InvertRaster(ops.AsRaster(fwdImage()))}, []any{ops.InvertRaster(ops.AsRaster(fwdImage()))})
	check("Laplacian", []any{Laplacian(fwdImage())}, []any{ops.Laplacian(fwdImage())})
	check("LaplacianRaster", []any{LaplacianRaster(ops.AsRaster(fwdImage()))}, []any{ops.LaplacianRaster(ops.AsRaster(fwdImage()))})
	{
		a0, a1 := Median(fwdImage(), 1)
		b0, b1 := ops.Median(fwdImage(), 1)
		check("Median", []any{a0, a1}, []any{b0, b1})
	}
	{
		a0, a1 := MedianRaster(ops.AsRaster(fwdImage()), 1)
		b0, b1 := ops.MedianRaster(ops.AsRaster(fwdImage()), 1)
		check("MedianRaster", []any{a0, a1}, []any{b0, b1})
	}
	{
		a0, a1 := Open(fwdImage(), 1)
		b0, b1 := ops.Open(fwdImage(), 1)
		check("Open", []any{a0, a1}, []any{b0, b1})
	}
	{
		a0, a1 := OpenRaster(ops.AsRaster(fwdImage()), 1)
		b0, b1 := ops.OpenRaster(ops.AsRaster(fwdImage()), 1)
		check("OpenRaster", []any{a0, a1}, []any{b0, b1})
	}
	check("Otsu", []any{Otsu(fwdImage())}, []any{ops.Otsu(fwdImage())})
	check("OtsuRaster", []any{OtsuRaster(ops.AsRaster(fwdImage()))}, []any{ops.OtsuRaster(ops.AsRaster(fwdImage()))})
	check("OtsuThreshold", []any{OtsuThreshold(fwdImage())}, []any{ops.OtsuThreshold(fwdImage())})
	check("Prewitt", []any{Prewitt(fwdImage())}, []any{ops.Prewitt(fwdImage())})
	check("PrewittRaster", []any{PrewittRaster(ops.AsRaster(fwdImage()))}, []any{ops.PrewittRaster(ops.AsRaster(fwdImage()))})
	check("RGBToHSV", []any{RGBToHSV(fwdImage())}, []any{ops.RGBToHSV(fwdImage())})
	check("RGBToHSVRaster", []any{RGBToHSVRaster(ops.AsRaster(fwdImage()))}, []any{ops.RGBToHSVRaster(ops.AsRaster(fwdImage()))})
	{
		a0, a1 := Resize(fwdImage(), 1, 1, Bilinear)
		b0, b1 := ops.Resize(fwdImage(), 1, 1, Bilinear)
		check("Resize", []any{a0, a1}, []any{b0, b1})
	}
	{
		a0, a1 := ResizeRaster(ops.AsRaster(fwdImage()), 1, 1, Bilinear)
		b0, b1 := ops.ResizeRaster(ops.AsRaster(fwdImage()), 1, 1, Bilinear)
		check("ResizeRaster", []any{a0, a1}, []any{b0, b1})
	}
	check("Rotate", []any{Rotate(fwdImage(), 1, true)}, []any{ops.Rotate(fwdImage(), 1, true)})
	check("Rotate180", []any{Rotate180(fwdImage())}, []any{ops.Rotate180(fwdImage())})
	check("Rotate180Raster", []any{Rotate180Raster(ops.AsRaster(fwdImage()))}, []any{ops.Rotate180Raster(ops.AsRaster(fwdImage()))})
	check("Rotate270", []any{Rotate270(fwdImage())}, []any{ops.Rotate270(fwdImage())})
	check("Rotate270Raster", []any{Rotate270Raster(ops.AsRaster(fwdImage()))}, []any{ops.Rotate270Raster(ops.AsRaster(fwdImage()))})
	check("Rotate90", []any{Rotate90(fwdImage())}, []any{ops.Rotate90(fwdImage())})
	check("Rotate90Raster", []any{Rotate90Raster(ops.AsRaster(fwdImage()))}, []any{ops.Rotate90Raster(ops.AsRaster(fwdImage()))})
	check("Rotation", []any{Rotation(1)}, []any{ops.Rotation(1)})
	check("Scaling", []any{Scaling(1, 1)}, []any{ops.Scaling(1, 1)})
	check("Scharr", []any{Scharr(fwdImage())}, []any{ops.Scharr(fwdImage())})
	check("ScharrRaster", []any{ScharrRaster(ops.AsRaster(fwdImage()))}, []any{ops.ScharrRaster(ops.AsRaster(fwdImage()))})
	check("Sharpen", []any{Sharpen(fwdImage())}, []any{ops.Sharpen(fwdImage())})
	check("SharpenRaster", []any{SharpenRaster(ops.AsRaster(fwdImage()))}, []any{ops.SharpenRaster(ops.AsRaster(fwdImage()))})
	{
		a0, a1 := SkewAngle(fwdSkewPage(), SkewOptions{})
		b0, b1 := ops.SkewAngle(fwdSkewPage(), SkewOptions{})
		check("SkewAngle", []any{a0, a1}, []any{b0, b1})
	}
	check("Sobel", []any{Sobel(fwdImage())}, []any{ops.Sobel(fwdImage())})
	check("SobelMag", []any{SobelMag(fwdImage())}, []any{ops.SobelMag(fwdImage())})
	check("SobelMagRaster", []any{SobelMagRaster(ops.AsRaster(fwdImage()))}, []any{ops.SobelMagRaster(ops.AsRaster(fwdImage()))})
	check("SobelRaster", []any{SobelRaster(ops.AsRaster(fwdImage()))}, []any{ops.SobelRaster(ops.AsRaster(fwdImage()))})
	check("SobelX", []any{SobelX(fwdImage())}, []any{ops.SobelX(fwdImage())})
	check("SobelXRaster", []any{SobelXRaster(ops.AsRaster(fwdImage()))}, []any{ops.SobelXRaster(ops.AsRaster(fwdImage()))})
	check("SobelY", []any{SobelY(fwdImage())}, []any{ops.SobelY(fwdImage())})
	check("SobelYRaster", []any{SobelYRaster(ops.AsRaster(fwdImage()))}, []any{ops.SobelYRaster(ops.AsRaster(fwdImage()))})
	check("Threshold", []any{Threshold(fwdImage(), 128)}, []any{ops.Threshold(fwdImage(), 128)})
	check("ThresholdRaster", []any{ThresholdRaster(ops.AsRaster(fwdImage()), 128)}, []any{ops.ThresholdRaster(ops.AsRaster(fwdImage()), 128)})
	check("ToRGBA", []any{ToRGBA(fwdImage())}, []any{ops.ToRGBA(fwdImage())})
	check("Translation", []any{Translation(1, 1)}, []any{ops.Translation(1, 1)})
	check("Transpose", []any{Transpose(fwdImage())}, []any{ops.Transpose(fwdImage())})
	check("Transverse", []any{Transverse(fwdImage())}, []any{ops.Transverse(fwdImage())})
	{
		a0, a1 := UnsharpMask(fwdImage(), 1, 1)
		b0, b1 := ops.UnsharpMask(fwdImage(), 1, 1)
		check("UnsharpMask", []any{a0, a1}, []any{b0, b1})
	}
	{
		a0, a1 := UnsharpMaskRaster(ops.AsRaster(fwdImage()), 1, 1)
		b0, b1 := ops.UnsharpMaskRaster(ops.AsRaster(fwdImage()), 1, 1)
		check("UnsharpMaskRaster", []any{a0, a1}, []any{b0, b1})
	}
	check("Warp", []any{Warp(fwdImage(), Rotation(0.5), 1, 1, Interp(1), BorderMode(1), 1)}, []any{ops.Warp(fwdImage(), Rotation(0.5), 1, 1, Interp(1), BorderMode(1), 1)})
}
