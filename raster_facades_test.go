package images

import (
	"bytes"
	"image"
	"testing"

	"github.com/go-gfx/gfx/raster"
)

// The façade contract every test below proves: XxxRaster(src) emits byte-for-byte
// the pixels that Xxx emits for an *image.RGBA holding the identical source
// bytes, and it leaves src untouched. patternRaster and rgbaFrom (defined in
// raster_test.go) supply a source whose alpha cycles through 0 with non-zero
// colour above it, so any path that confused straight and premultiplied bytes
// would diverge on exactly those pixels.

// facadeGeoms exercises square, both non-square orientations (to catch a
// transposed rotate or flip), and single-row / single-column edges.
var facadeGeoms = []struct{ w, h int }{
	{4, 4}, {5, 3}, {3, 5}, {7, 1}, {1, 6},
}

// unaryFacade pairs a raster-typed façade with the image.RGBA operation it must
// reproduce, both reduced to the same one-argument shape (bound parameters are
// captured in the closures).
type unaryFacade struct {
	name   string
	raster func(*raster.Image) *raster.Image
	rgba   func(image.Image) *image.RGBA
}

func unaryFacades() []unaryFacade {
	return []unaryFacade{
		{"Grayscale", GrayscaleRaster, func(i image.Image) *image.RGBA { return Grayscale(i) }},
		{"Invert", InvertRaster, func(i image.Image) *image.RGBA { return Invert(i) }},
		{"AdjustBrightness", func(r *raster.Image) *raster.Image { return AdjustBrightnessRaster(r, 40) },
			func(i image.Image) *image.RGBA { return AdjustBrightness(i, 40) }},
		{"AdjustContrast", func(r *raster.Image) *raster.Image { return AdjustContrastRaster(r, 1.7) },
			func(i image.Image) *image.RGBA { return AdjustContrast(i, 1.7) }},
		{"Threshold", func(r *raster.Image) *raster.Image { return ThresholdRaster(r, 100) },
			func(i image.Image) *image.RGBA { return Threshold(i, 100) }},
		{"Otsu", OtsuRaster, func(i image.Image) *image.RGBA { return Otsu(i) }},
		{"RGBToHSV", RGBToHSVRaster, func(i image.Image) *image.RGBA { return RGBToHSV(i) }},
		{"HSVToRGB", HSVToRGBRaster, func(i image.Image) *image.RGBA { return HSVToRGB(i) }},
		{"Sobel", SobelRaster, func(i image.Image) *image.RGBA { return Sobel(i) }},
		{"SobelX", SobelXRaster, func(i image.Image) *image.RGBA { return SobelX(i) }},
		{"SobelY", SobelYRaster, func(i image.Image) *image.RGBA { return SobelY(i) }},
		{"Prewitt", PrewittRaster, func(i image.Image) *image.RGBA { return Prewitt(i) }},
		{"Scharr", ScharrRaster, func(i image.Image) *image.RGBA { return Scharr(i) }},
		{"SobelMag", SobelMagRaster, func(i image.Image) *image.RGBA { return SobelMag(i) }},
		{"Laplacian", LaplacianRaster, func(i image.Image) *image.RGBA { return Laplacian(i) }},
		{"Sharpen", SharpenRaster, func(i image.Image) *image.RGBA { return Sharpen(i) }},
		{"FlipHorizontal", FlipHorizontalRaster, func(i image.Image) *image.RGBA { return FlipHorizontal(i) }},
		{"FlipVertical", FlipVerticalRaster, func(i image.Image) *image.RGBA { return FlipVertical(i) }},
		{"Rotate90", Rotate90Raster, func(i image.Image) *image.RGBA { return Rotate90(i) }},
		{"Rotate180", Rotate180Raster, func(i image.Image) *image.RGBA { return Rotate180(i) }},
		{"Rotate270", Rotate270Raster, func(i image.Image) *image.RGBA { return Rotate270(i) }},
	}
}

// TestUnaryRasterFacadesMatchRGBA is the byte-parity control for every non-error
// façade: over each geometry and the colour-under-transparency source, the
// raster path must equal the image.RGBA path pixel for pixel, must carry the
// operation's own output dimensions (Rotate90/270 transpose), and must not
// mutate src.
func TestUnaryRasterFacadesMatchRGBA(t *testing.T) {
	for _, f := range unaryFacades() {
		for _, g := range facadeGeoms {
			src := patternRaster(g.w, g.h)
			before := append([]uint8(nil), src.Pix...)
			ref := rgbaFrom(src)

			want := f.rgba(ref)
			got := f.raster(src)

			wb := want.Bounds()
			if got.W != wb.Dx() || got.H != wb.Dy() {
				t.Fatalf("%s %dx%d: got %dx%d, want %dx%d", f.name, g.w, g.h, got.W, got.H, wb.Dx(), wb.Dy())
			}
			if !bytes.Equal(got.Pix, want.Pix) {
				t.Fatalf("%s %dx%d: raster path diverged from image.RGBA path", f.name, g.w, g.h)
			}
			if !bytes.Equal(src.Pix, before) {
				t.Fatalf("%s %dx%d: façade mutated its source", f.name, g.w, g.h)
			}
		}
	}
}

// errFacade pairs an error-returning raster façade with the image.RGBA operation
// it reproduces, both bound to valid parameters so the success path is compared.
type errFacade struct {
	name   string
	raster func(*raster.Image) (*raster.Image, error)
	rgba   func(image.Image) (*image.RGBA, error)
}

func errFacades() []errFacade {
	sharpen := Kernel{Width: 3, Height: 3, Weights: []float64{0, -1, 0, -1, 5, -1, 0, -1, 0}}
	return []errFacade{
		{"BoxBlur", func(r *raster.Image) (*raster.Image, error) { return BoxBlurRaster(r, 2) },
			func(i image.Image) (*image.RGBA, error) { return BoxBlur(i, 2) }},
		{"GaussianBlur", func(r *raster.Image) (*raster.Image, error) { return GaussianBlurRaster(r, 1.5) },
			func(i image.Image) (*image.RGBA, error) { return GaussianBlur(i, 1.5) }},
		{"Median", func(r *raster.Image) (*raster.Image, error) { return MedianRaster(r, 1) },
			func(i image.Image) (*image.RGBA, error) { return Median(i, 1) }},
		{"UnsharpMask", func(r *raster.Image) (*raster.Image, error) { return UnsharpMaskRaster(r, 1.2, 0.8) },
			func(i image.Image) (*image.RGBA, error) { return UnsharpMask(i, 1.2, 0.8) }},
		{"Convolve", func(r *raster.Image) (*raster.Image, error) { return ConvolveRaster(r, sharpen) },
			func(i image.Image) (*image.RGBA, error) { return Convolve(i, sharpen) }},
		{"Canny", func(r *raster.Image) (*raster.Image, error) { return CannyRaster(r, 1.0, 5, 15) },
			func(i image.Image) (*image.RGBA, error) { return Canny(i, 1.0, 5, 15) }},
		{"Erode", func(r *raster.Image) (*raster.Image, error) { return ErodeRaster(r, 1) },
			func(i image.Image) (*image.RGBA, error) { return Erode(i, 1) }},
		{"Dilate", func(r *raster.Image) (*raster.Image, error) { return DilateRaster(r, 1) },
			func(i image.Image) (*image.RGBA, error) { return Dilate(i, 1) }},
		{"Open", func(r *raster.Image) (*raster.Image, error) { return OpenRaster(r, 1) },
			func(i image.Image) (*image.RGBA, error) { return Open(i, 1) }},
		{"Close", func(r *raster.Image) (*raster.Image, error) { return CloseRaster(r, 1) },
			func(i image.Image) (*image.RGBA, error) { return Close(i, 1) }},
	}
}

// errFacadeGeoms keep every dimension at least 3 so the radius-2 and Canny cases
// have a genuine neighbourhood on both axes.
var errFacadeGeoms = []struct{ w, h int }{
	{5, 4}, {4, 5}, {6, 6}, {8, 3}, {3, 8},
}

// TestErrRasterFacadesMatchRGBA is the byte-parity control for the
// error-returning façades on valid input: the success path must equal the
// image.RGBA path pixel for pixel and leave src untouched.
func TestErrRasterFacadesMatchRGBA(t *testing.T) {
	for _, f := range errFacades() {
		for _, g := range errFacadeGeoms {
			src := patternRaster(g.w, g.h)
			before := append([]uint8(nil), src.Pix...)
			ref := rgbaFrom(src)

			want, err := f.rgba(ref)
			if err != nil {
				t.Fatalf("%s %dx%d: reference: %v", f.name, g.w, g.h, err)
			}
			got, err := f.raster(src)
			if err != nil {
				t.Fatalf("%s %dx%d: façade: %v", f.name, g.w, g.h, err)
			}
			wb := want.Bounds()
			if got.W != wb.Dx() || got.H != wb.Dy() {
				t.Fatalf("%s %dx%d: got %dx%d, want %dx%d", f.name, g.w, g.h, got.W, got.H, wb.Dx(), wb.Dy())
			}
			if !bytes.Equal(got.Pix, want.Pix) {
				t.Fatalf("%s %dx%d: raster path diverged from image.RGBA path", f.name, g.w, g.h)
			}
			if !bytes.Equal(src.Pix, before) {
				t.Fatalf("%s %dx%d: façade mutated its source", f.name, g.w, g.h)
			}
		}
	}
}

// TestCropRasterMatchesCrop covers CropRaster's success path, whose rectangle is
// geometry-dependent and so does not fit the table above: a non-origin sub-rect
// must yield the same pixels and dimensions as Crop on the reference.
func TestCropRasterMatchesCrop(t *testing.T) {
	src := patternRaster(6, 6)
	before := append([]uint8(nil), src.Pix...)
	r := image.Rect(1, 2, 5, 5) // 4x3, non-origin

	want, err := Crop(rgbaFrom(src), r)
	if err != nil {
		t.Fatalf("reference Crop: %v", err)
	}
	got, err := CropRaster(src, r)
	if err != nil {
		t.Fatalf("CropRaster: %v", err)
	}
	if got.W != r.Dx() || got.H != r.Dy() {
		t.Fatalf("got %dx%d, want %dx%d", got.W, got.H, r.Dx(), r.Dy())
	}
	if !bytes.Equal(got.Pix, want.Pix) {
		t.Fatalf("CropRaster diverged from Crop")
	}
	if !bytes.Equal(src.Pix, before) {
		t.Fatalf("CropRaster mutated its source")
	}
}

// TestRasterFacadeControlIsSensitive controls the instrument: the byte-for-byte
// comparison the parity tests rely on must be able to fail. A single perturbed
// byte in the reference output has to make the equality check report a
// difference — otherwise a green parity test would prove nothing.
func TestRasterFacadeControlIsSensitive(t *testing.T) {
	src := patternRaster(5, 4)
	want := Grayscale(rgbaFrom(src))
	got := GrayscaleRaster(src)
	if !bytes.Equal(got.Pix, want.Pix) {
		t.Fatalf("precondition: paths must match before perturbation")
	}
	want.Pix[0] ^= 0xff
	if bytes.Equal(got.Pix, want.Pix) {
		t.Fatalf("comparison is insensitive: a corrupted reference still compared equal")
	}
}

// TestErrRasterFacadesPropagateErrors covers each error-returning façade's error
// branch: an invalid parameter must surface the underlying operation's error
// (and a nil image) rather than being swallowed.
func TestErrRasterFacadesPropagateErrors(t *testing.T) {
	src := patternRaster(4, 4)
	cases := []struct {
		name string
		call func() (*raster.Image, error)
	}{
		{"BoxBlur", func() (*raster.Image, error) { return BoxBlurRaster(src, 0) }},
		{"GaussianBlur", func() (*raster.Image, error) { return GaussianBlurRaster(src, 0) }},
		{"Median", func() (*raster.Image, error) { return MedianRaster(src, 0) }},
		{"UnsharpMask", func() (*raster.Image, error) { return UnsharpMaskRaster(src, 0, 1) }},
		{"Convolve", func() (*raster.Image, error) {
			return ConvolveRaster(src, Kernel{Width: 2, Height: 2, Weights: make([]float64, 4)})
		}},
		{"Canny", func() (*raster.Image, error) { return CannyRaster(src, 0, 1, 2) }},
		{"Erode", func() (*raster.Image, error) { return ErodeRaster(src, 0) }},
		{"Dilate", func() (*raster.Image, error) { return DilateRaster(src, 0) }},
		{"Open", func() (*raster.Image, error) { return OpenRaster(src, 0) }},
		{"Close", func() (*raster.Image, error) { return CloseRaster(src, 0) }},
		{"Crop", func() (*raster.Image, error) { return CropRaster(src, image.Rect(0, 0, 0, 0)) }},
	}
	for _, c := range cases {
		out, err := c.call()
		if err == nil {
			t.Fatalf("%s: expected an error for invalid input", c.name)
		}
		if out != nil {
			t.Fatalf("%s: expected a nil image alongside the error", c.name)
		}
	}
}

// The two benchmarks below isolate the façade's only cost over the image.RGBA
// path: the O(1)-per-call header allocations (the aliasing view of src and the
// view of the result), the per-pixel work being identical. Run with -benchmem;
// benchstat should show the raster path at +2 allocs/op and the same time/op.
const facadeBenchW, facadeBenchH = 512, 512

func BenchmarkGrayscaleRGBA(b *testing.B) {
	src := rgbaFrom(patternRaster(facadeBenchW, facadeBenchH))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Grayscale(src)
	}
}

func BenchmarkGrayscaleRaster(b *testing.B) {
	src := patternRaster(facadeBenchW, facadeBenchH)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = GrayscaleRaster(src)
	}
}
