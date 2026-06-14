package images

import (
	"image"
	"image/color"
	"testing"
)

// solid returns an origin-anchored w*h RGBA filled with c.
func solid(w, h int, c color.RGBA) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetRGBA(x, y, c)
		}
	}
	return img
}

func px(img *image.RGBA, x, y int) color.RGBA {
	return img.RGBAAt(x, y)
}

func TestToRGBAPassthrough(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 2, 2))
	if got := ToRGBA(src); got != src {
		t.Fatalf("expected the same *image.RGBA to be returned unchanged")
	}
}

func TestToRGBANonOrigin(t *testing.T) {
	src := image.NewRGBA(image.Rect(3, 5, 5, 7)) // 2x2, not origin-anchored
	src.SetRGBA(3, 5, color.RGBA{10, 20, 30, 40})
	got := ToRGBA(src)
	if got == src {
		t.Fatalf("expected a copy for a non-origin image")
	}
	b := got.Bounds()
	if b.Dx() != 2 || b.Dy() != 2 || b.Min.X != 0 || b.Min.Y != 0 {
		t.Fatalf("unexpected bounds %v", b)
	}
	if got.RGBAAt(0, 0) != (color.RGBA{10, 20, 30, 40}) {
		t.Fatalf("pixel not copied correctly: %v", got.RGBAAt(0, 0))
	}
}

func TestToRGBAConvertFromNRGBA(t *testing.T) {
	// A non-RGBA image type forces the conversion path.
	src := image.NewNRGBA(image.Rect(0, 0, 1, 1))
	src.SetNRGBA(0, 0, color.NRGBA{255, 128, 0, 255})
	got := ToRGBA(src)
	if got.RGBAAt(0, 0) != (color.RGBA{255, 128, 0, 255}) {
		t.Fatalf("conversion wrong: %v", got.RGBAAt(0, 0))
	}
}

func TestGrayscale(t *testing.T) {
	// Pure red: 0.299*255 = 76.245 -> 76.
	src := solid(1, 1, color.RGBA{255, 0, 0, 200})
	out := Grayscale(src)
	got := px(out, 0, 0)
	if got != (color.RGBA{76, 76, 76, 200}) {
		t.Fatalf("grayscale red = %v, want {76,76,76,200}", got)
	}
	// White stays white.
	white := Grayscale(solid(1, 1, color.RGBA{255, 255, 255, 255}))
	if px(white, 0, 0) != (color.RGBA{255, 255, 255, 255}) {
		t.Fatalf("grayscale white = %v", px(white, 0, 0))
	}
}

func TestInvert(t *testing.T) {
	src := solid(1, 1, color.RGBA{0, 100, 255, 123})
	out := Invert(src)
	if px(out, 0, 0) != (color.RGBA{255, 155, 0, 123}) {
		t.Fatalf("invert = %v, want {255,155,0,123}", px(out, 0, 0))
	}
	// Input must not be mutated.
	if px(src, 0, 0) != (color.RGBA{0, 100, 255, 123}) {
		t.Fatalf("input was mutated: %v", px(src, 0, 0))
	}
}

func TestAdjustBrightness(t *testing.T) {
	src := solid(1, 1, color.RGBA{100, 200, 50, 255})
	out := AdjustBrightness(src, 60)
	// 100+60=160, 200+60=260->255, 50+60=110.
	if px(out, 0, 0) != (color.RGBA{160, 255, 110, 255}) {
		t.Fatalf("brightness up = %v", px(out, 0, 0))
	}
	// Clamp at the low bound.
	down := AdjustBrightness(src, -300)
	if px(down, 0, 0) != (color.RGBA{0, 0, 0, 255}) {
		t.Fatalf("brightness down = %v, want black", px(down, 0, 0))
	}
}

func TestAdjustContrast(t *testing.T) {
	// factor 1 is a no-op.
	src := solid(1, 1, color.RGBA{200, 100, 128, 255})
	if got := px(AdjustContrast(src, 1), 0, 0); got != (color.RGBA{200, 100, 128, 255}) {
		t.Fatalf("contrast 1 = %v, want unchanged", got)
	}
	// factor 2: (200-128)*2+128=272->255, (100-128)*2+128=72, mid stays 128.
	hi := AdjustContrast(src, 2)
	if px(hi, 0, 0) != (color.RGBA{255, 72, 128, 255}) {
		t.Fatalf("contrast 2 = %v", px(hi, 0, 0))
	}
	// factor 0 collapses to mid-grey.
	lo := AdjustContrast(src, 0)
	if px(lo, 0, 0) != (color.RGBA{128, 128, 128, 255}) {
		t.Fatalf("contrast 0 = %v, want mid", px(lo, 0, 0))
	}
}

func TestResizeNearest(t *testing.T) {
	// 2x1 image: left red, right blue. Upscale to 4x1.
	src := image.NewRGBA(image.Rect(0, 0, 2, 1))
	src.SetRGBA(0, 0, color.RGBA{255, 0, 0, 255})
	src.SetRGBA(1, 0, color.RGBA{0, 0, 255, 255})
	out, err := Resize(src, 4, 1, NearestNeighbor)
	if err != nil {
		t.Fatal(err)
	}
	want := []color.RGBA{
		{255, 0, 0, 255}, {255, 0, 0, 255}, {0, 0, 255, 255}, {0, 0, 255, 255},
	}
	for x, w := range want {
		if px(out, x, 0) != w {
			t.Fatalf("nearest x=%d = %v, want %v", x, px(out, x, 0), w)
		}
	}
}

func TestResizeBilinear(t *testing.T) {
	// 2x1: black then white. Upscale to 4x1 — interior pixels interpolate.
	src := image.NewRGBA(image.Rect(0, 0, 2, 1))
	src.SetRGBA(0, 0, color.RGBA{0, 0, 0, 255})
	src.SetRGBA(1, 0, color.RGBA{255, 255, 255, 255})
	out, err := Resize(src, 4, 1, Bilinear)
	if err != nil {
		t.Fatal(err)
	}
	// Ends clamp to black/white; middle pixels are intermediate and monotone.
	if px(out, 0, 0).R != 0 {
		t.Fatalf("bilinear left = %v, want black", px(out, 0, 0))
	}
	if px(out, 3, 0).R != 255 {
		t.Fatalf("bilinear right = %v, want white", px(out, 3, 0))
	}
	if !(px(out, 1, 0).R < px(out, 2, 0).R) {
		t.Fatalf("bilinear not monotone: %d then %d", px(out, 1, 0).R, px(out, 2, 0).R)
	}
}

func TestResizeBadDimensions(t *testing.T) {
	src := solid(2, 2, color.RGBA{1, 2, 3, 4})
	if _, err := Resize(src, 0, 4, NearestNeighbor); err == nil {
		t.Fatal("expected error for w=0")
	}
	if _, err := Resize(src, 4, -1, NearestNeighbor); err == nil {
		t.Fatal("expected error for h<0")
	}
}

func TestResizeUnknownMode(t *testing.T) {
	src := solid(2, 2, color.RGBA{1, 2, 3, 4})
	if _, err := Resize(src, 2, 2, ResizeMode(99)); err == nil {
		t.Fatal("expected error for unknown mode")
	}
}

func TestConvolveIdentity(t *testing.T) {
	src := solid(3, 3, color.RGBA{10, 20, 30, 255})
	src.SetRGBA(1, 1, color.RGBA{200, 100, 50, 255})
	id := Kernel{Width: 3, Height: 3, Weights: []float64{0, 0, 0, 0, 1, 0, 0, 0, 0}}
	out, err := Convolve(src, id)
	if err != nil {
		t.Fatal(err)
	}
	for y := 0; y < 3; y++ {
		for x := 0; x < 3; x++ {
			if px(out, x, y) != px(src, x, y) {
				t.Fatalf("identity changed pixel %d,%d: %v vs %v", x, y, px(out, x, y), px(src, x, y))
			}
		}
	}
}

func TestConvolveBoxBlurClampedEdges(t *testing.T) {
	// A uniform image box-blurred with a normalised 3x3 box stays uniform,
	// even at the borders, thanks to clamp-to-edge addressing.
	src := solid(3, 3, color.RGBA{120, 120, 120, 255})
	w := make([]float64, 9)
	for i := range w {
		w[i] = 1.0 / 9.0
	}
	box := Kernel{Width: 3, Height: 3, Weights: w}
	out, err := Convolve(src, box)
	if err != nil {
		t.Fatal(err)
	}
	for y := 0; y < 3; y++ {
		for x := 0; x < 3; x++ {
			if px(out, x, y) != (color.RGBA{120, 120, 120, 255}) {
				t.Fatalf("box blur changed uniform pixel %d,%d: %v", x, y, px(out, x, y))
			}
		}
	}
}

func TestConvolveErrors(t *testing.T) {
	src := solid(2, 2, color.RGBA{1, 2, 3, 4})
	cases := []struct {
		name string
		k    Kernel
	}{
		{"nonpositive", Kernel{Width: 0, Height: 3, Weights: nil}},
		{"nonpositive-height", Kernel{Width: 3, Height: -1, Weights: nil}},
		{"even-width", Kernel{Width: 2, Height: 3, Weights: make([]float64, 6)}},
		{"even-height", Kernel{Width: 3, Height: 2, Weights: make([]float64, 6)}},
		{"length-mismatch", Kernel{Width: 3, Height: 3, Weights: make([]float64, 8)}},
	}
	for _, c := range cases {
		if _, err := Convolve(src, c.k); err == nil {
			t.Fatalf("%s: expected error", c.name)
		}
	}
}

func TestGaussianBlur(t *testing.T) {
	// A uniform image blurred by a Gaussian stays uniform.
	src := solid(5, 5, color.RGBA{80, 90, 100, 255})
	out, err := GaussianBlur(src, 1.0)
	if err != nil {
		t.Fatal(err)
	}
	for y := 0; y < 5; y++ {
		for x := 0; x < 5; x++ {
			if px(out, x, y) != (color.RGBA{80, 90, 100, 255}) {
				t.Fatalf("gaussian changed uniform pixel %d,%d: %v", x, y, px(out, x, y))
			}
		}
	}
	// A bright centre on a dark field should diffuse: centre dims, neighbours brighten.
	spike := solid(5, 5, color.RGBA{0, 0, 0, 255})
	spike.SetRGBA(2, 2, color.RGBA{255, 255, 255, 255})
	blurred, err := GaussianBlur(spike, 1.0)
	if err != nil {
		t.Fatal(err)
	}
	if blurred.RGBAAt(2, 2).R >= 255 {
		t.Fatalf("gaussian centre not dimmed: %v", blurred.RGBAAt(2, 2))
	}
	if blurred.RGBAAt(2, 1).R == 0 {
		t.Fatalf("gaussian did not spread to neighbour: %v", blurred.RGBAAt(2, 1))
	}
}

func TestGaussianBlurBadSigma(t *testing.T) {
	src := solid(3, 3, color.RGBA{1, 2, 3, 4})
	if _, err := GaussianBlur(src, 0); err == nil {
		t.Fatal("expected error for sigma=0")
	}
	if _, err := GaussianBlur(src, -1); err == nil {
		t.Fatal("expected error for negative sigma")
	}
}
