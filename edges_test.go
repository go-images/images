package images

import (
	"image"
	"image/color"
	"math"
	"testing"
)

// edgeNaive is an independent reference for the normalised separable edge
// operators (Prewitt/Scharr/SobelMag). It convolves the float luminance plane
// with the explicit 3x3 derivative kernels (clamp-to-edge) and returns the
// byte-clamped magnitude sqrt((gx^2+gy^2)/2), the same definition the production
// kernel uses but written without the smoothing-triple factoring.
func edgeNaive(src []uint8, w, h int, a, b, c float64) []uint8 {
	lum := make([]float64, w*h)
	for i := 0; i < w*h; i++ {
		j := i * 4
		lum[i] = 0.299*float64(src[j]) + 0.587*float64(src[j+1]) + 0.114*float64(src[j+2])
	}
	at := func(x, y int) float64 {
		if x < 0 {
			x = 0
		}
		if x >= w {
			x = w - 1
		}
		if y < 0 {
			y = 0
		}
		if y >= h {
			y = h - 1
		}
		return lum[y*w+x]
	}
	round := func(v float64) uint8 {
		switch {
		case v <= 0:
			return 0
		case v >= 255:
			return 255
		default:
			return uint8(v + 0.5)
		}
	}
	dst := make([]uint8, len(src))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			// gx = derivative across columns, smoothed down rows.
			gx := a*(at(x-1, y-1)-at(x+1, y-1)) +
				b*(at(x-1, y)-at(x+1, y)) +
				c*(at(x-1, y+1)-at(x+1, y+1))
			gy := a*(at(x-1, y-1)-at(x-1, y+1)) +
				b*(at(x, y-1)-at(x, y+1)) +
				c*(at(x+1, y-1)-at(x+1, y+1))
			m := math.Sqrt((gx*gx + gy*gy) / 2)
			v := round(m)
			j := (y*w + x) * 4
			dst[j], dst[j+1], dst[j+2], dst[j+3] = v, v, v, src[j+3]
		}
	}
	return dst
}

func TestEdgeOperatorsUniform(t *testing.T) {
	// A flat field has zero gradient and zero Laplacian everywhere.
	src := solid(4, 4, color.RGBA{77, 77, 77, 200})
	for name, out := range map[string]*image.RGBA{
		"Prewitt": Prewitt(src), "Scharr": Scharr(src), "SobelMag": SobelMag(src),
	} {
		for y := 0; y < 4; y++ {
			for x := 0; x < 4; x++ {
				if got := px(out, x, y); got != (color.RGBA{0, 0, 0, 200}) {
					t.Fatalf("%s uniform at %d,%d = %v, want black alpha 200", name, x, y, got)
				}
			}
		}
	}
	// Laplacian of a flat field is the 128 mid-grey offset, alpha preserved.
	lap := Laplacian(src)
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			if got := px(lap, x, y); got != (color.RGBA{128, 128, 128, 200}) {
				t.Fatalf("Laplacian uniform at %d,%d = %v, want mid-grey alpha 200", x, y, got)
			}
		}
	}
}

func TestEdgeOperatorsMatchNaive(t *testing.T) {
	const w, h = 17, 13
	src := benchImage(w, h)
	cases := []struct {
		name    string
		got     *image.RGBA
		a, b, c float64
	}{
		{"Prewitt", Prewitt(src), 1.0 / 3, 1.0 / 3, 1.0 / 3},
		{"Scharr", Scharr(src), 0.1875, 0.625, 0.1875},
		{"SobelMag", SobelMag(src), 0.25, 0.5, 0.25},
	}
	for _, c := range cases {
		want := edgeNaive(src.Pix, w, h, c.a, c.b, c.c)
		for i := range want {
			if c.got.Pix[i] != want[i] {
				t.Fatalf("%s disagrees with naive at byte %d: got %d want %d",
					c.name, i, c.got.Pix[i], want[i])
			}
		}
	}
}

func TestSobelMagVerticalEdge(t *testing.T) {
	// Left column 0, right two columns 255: a single vertical edge. The
	// normalised Sobel x-response at the edge is the full contrast 255, smoothed
	// to 1.0; gy = 0, so magnitude = sqrt(255^2/2) = 180.3 -> 180.
	src := gray(3, 3, [][]uint8{
		{0, 255, 255},
		{0, 255, 255},
		{0, 255, 255},
	})
	out := SobelMag(src)
	if got := px(out, 0, 0).R; got != 180 {
		t.Fatalf("SobelMag at vertical edge = %d, want 180 (sqrt(255^2/2))", got)
	}
	if got := px(out, 2, 0).R; got != 0 {
		t.Fatalf("SobelMag at flat right = %d, want 0", got)
	}
}

func TestLaplacianResponds(t *testing.T) {
	// A single bright pixel (value 100) on black. At the centre the Laplacian is
	// 4*100 - 0 = 400 -> clamps to 255 (offset 128 then clamp). The four
	// edge-adjacent neighbours each see one bright neighbour: 0 - 100 + 128 = 28.
	src := gray(3, 3, [][]uint8{
		{0, 0, 0},
		{0, 100, 0},
		{0, 0, 0},
	})
	out := Laplacian(src)
	if got := px(out, 1, 1).R; got != 255 {
		t.Fatalf("Laplacian centre = %d, want 255 (4*100+128 clamped)", got)
	}
	if got := px(out, 0, 1).R; got != 28 {
		t.Fatalf("Laplacian left neighbour = %d, want 28 (-100+128)", got)
	}
	// Output is grayscale.
	c := px(out, 1, 1)
	if c.R != c.G || c.G != c.B {
		t.Fatalf("Laplacian output not grayscale: %v", c)
	}
}

func TestEdgeOperatorsDoNotMutateInput(t *testing.T) {
	src := benchImage(6, 6)
	cp := make([]uint8, len(src.Pix))
	copy(cp, src.Pix)
	_ = Prewitt(src)
	_ = Scharr(src)
	_ = SobelMag(src)
	_ = Laplacian(src)
	for i := range cp {
		if src.Pix[i] != cp[i] {
			t.Fatalf("edge operator mutated input at byte %d", i)
		}
	}
}

// BenchmarkScharr measures the normalised separable Scharr edge operator at a
// realistic size; it shares the fused luminance/gradient kernel with Prewitt and
// SobelMag and fans out across cores above the parallel threshold.
func BenchmarkScharr(b *testing.B) {
	src := benchImage(512, 512)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Scharr(src)
	}
}
