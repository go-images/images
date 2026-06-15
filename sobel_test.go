package images

import (
	"image"
	"image/color"
	"math"
	"testing"
)

// gray returns an origin-anchored w*h RGBA whose every pixel is the gray value
// v (R=G=B=v, fully opaque). For such pixels the Rec. 601 luminance equals v
// exactly, so the Sobel responses can be derived analytically from v.
func gray(w, h int, vals [][]uint8) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			v := vals[y][x]
			img.SetRGBA(x, y, color.RGBA{v, v, v, 255})
		}
	}
	return img
}

func TestSobelUniform(t *testing.T) {
	// A flat field has zero gradient everywhere, so the edge map is black.
	src := solid(4, 4, color.RGBA{77, 77, 77, 200})
	out := Sobel(src)
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			if got := px(out, x, y); got != (color.RGBA{0, 0, 0, 200}) {
				t.Fatalf("uniform edge at %d,%d = %v, want black with alpha 200", x, y, got)
			}
		}
	}
}

func TestSobelVerticalEdge(t *testing.T) {
	// Left column 0, right two columns 255: a single vertical edge.
	// gx = 1020 at the two left columns (clamp-to-edge), 0 at the rightmost;
	// gy = 0 everywhere, so magnitude == |gx|, clamped to 255.
	src := gray(3, 3, [][]uint8{
		{0, 255, 255},
		{0, 255, 255},
		{0, 255, 255},
	})
	out := Sobel(src)
	want := [][]uint8{
		{255, 255, 0},
		{255, 255, 0},
		{255, 255, 0},
	}
	for y := 0; y < 3; y++ {
		for x := 0; x < 3; x++ {
			if got := px(out, x, y).R; got != want[y][x] {
				t.Fatalf("vertical edge at %d,%d = %d, want %d", x, y, got, want[y][x])
			}
		}
	}
}

func TestSobelMagnitudeNonSaturating(t *testing.T) {
	// A single bright pixel (value 10) on black. At (0,0) the clamp-to-edge
	// neighbourhood gives gx=10, gy=10, so magnitude = sqrt(200) = 14.142 -> 14.
	// At the centre (1,1) both gradients are 0. At (1,0) gx=20, gy=0 -> 20.
	src := gray(3, 3, [][]uint8{
		{0, 0, 0},
		{0, 10, 0},
		{0, 0, 0},
	})
	out := Sobel(src)
	if got := px(out, 0, 0).R; got != 14 {
		t.Fatalf("magnitude at 0,0 = %d, want 14 (sqrt(200))", got)
	}
	if got := px(out, 1, 0).R; got != 20 {
		t.Fatalf("magnitude at 1,0 = %d, want 20", got)
	}
	if got := px(out, 1, 1).R; got != 0 {
		t.Fatalf("magnitude at centre = %d, want 0", got)
	}
	// R, G and B must all carry the same magnitude (grayscale output).
	c := px(out, 0, 0)
	if c.R != c.G || c.G != c.B {
		t.Fatalf("output not grayscale at 0,0: %v", c)
	}
}

func TestSobelXDirectional(t *testing.T) {
	// Vertical edge (left 0, right 255): gx = +1020 at the left columns.
	// Scaled by 1/8 and offset by 128: 1020/8+128 = 255.5 -> clamps to 255.
	// The rightmost column has gx=0 -> 128 (mid-grey).
	src := gray(3, 3, [][]uint8{
		{0, 255, 255},
		{0, 255, 255},
		{0, 255, 255},
	})
	out := SobelX(src)
	if got := px(out, 0, 0).R; got != 255 {
		t.Fatalf("SobelX at left edge = %d, want 255", got)
	}
	if got := px(out, 2, 0).R; got != 128 {
		t.Fatalf("SobelX at flat right = %d, want 128 (mid-grey)", got)
	}
	// SobelY of a purely vertical edge is flat mid-grey (gy=0 everywhere).
	outY := SobelY(src)
	for y := 0; y < 3; y++ {
		for x := 0; x < 3; x++ {
			if got := px(outY, x, y).R; got != 128 {
				t.Fatalf("SobelY of vertical edge at %d,%d = %d, want 128", x, y, got)
			}
		}
	}
}

func TestSobelYDirectional(t *testing.T) {
	// Horizontal edge: top row 0, bottom two rows 100. gy = +400 at the top
	// rows: 400/8+128 = 178; bottom row gy=0 -> 128. SobelX is flat mid-grey.
	src := gray(3, 3, [][]uint8{
		{0, 0, 0},
		{100, 100, 100},
		{100, 100, 100},
	})
	out := SobelY(src)
	if got := px(out, 0, 0).R; got != 178 {
		t.Fatalf("SobelY at top edge = %d, want 178 (400/8+128)", got)
	}
	if got := px(out, 0, 2).R; got != 128 {
		t.Fatalf("SobelY at flat bottom = %d, want 128", got)
	}
	outX := SobelX(src)
	for y := 0; y < 3; y++ {
		for x := 0; x < 3; x++ {
			if got := px(outX, x, y).R; got != 128 {
				t.Fatalf("SobelX of horizontal edge at %d,%d = %d, want 128", x, y, got)
			}
		}
	}
}

func TestSobelDoesNotMutateInput(t *testing.T) {
	src := gray(3, 3, [][]uint8{
		{0, 255, 255},
		{0, 255, 255},
		{0, 255, 255},
	})
	_ = Sobel(src)
	_ = SobelX(src)
	_ = SobelY(src)
	if px(src, 0, 0) != (color.RGBA{0, 0, 0, 255}) {
		t.Fatalf("input mutated at 0,0: %v", px(src, 0, 0))
	}
	if px(src, 1, 1) != (color.RGBA{255, 255, 255, 255}) {
		t.Fatalf("input mutated at 1,1: %v", px(src, 1, 1))
	}
}

// sobelNaive is a deliberately straightforward reference: it materialises a
// float luminance plane and then convolves it twice. It exists only as the
// correctness oracle and benchmark baseline for the kernel; the production path
// fuses both passes and reads luminance on the fly.
func sobelNaive(src []uint8, w, h int) []uint8 {
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
	dst := make([]uint8, len(src))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			gx := -at(x-1, y-1) + at(x+1, y-1) - 2*at(x-1, y) + 2*at(x+1, y) - at(x-1, y+1) + at(x+1, y+1)
			gy := -at(x-1, y-1) - 2*at(x, y-1) - at(x+1, y-1) + at(x-1, y+1) + 2*at(x, y+1) + at(x+1, y+1)
			m := math.Sqrt(gx*gx + gy*gy)
			v := uint8(0)
			switch {
			case m <= 0:
				v = 0
			case m >= 255:
				v = 255
			default:
				v = uint8(m + 0.5)
			}
			j := (y*w + x) * 4
			dst[j], dst[j+1], dst[j+2], dst[j+3] = v, v, v, src[j+3]
		}
	}
	return dst
}

func TestSobelMatchesNaiveReference(t *testing.T) {
	// Differential test on a deterministic pseudo-random image: the production
	// kernel must agree with the independent naive reference pixel for pixel.
	const w, h = 17, 13
	src := image.NewRGBA(image.Rect(0, 0, w, h))
	seed := uint32(2463534242)
	next := func() uint8 {
		seed ^= seed << 13
		seed ^= seed >> 17
		seed ^= seed << 5
		return uint8(seed)
	}
	for i := 0; i < len(src.Pix); i += 4 {
		src.Pix[i], src.Pix[i+1], src.Pix[i+2], src.Pix[i+3] = next(), next(), next(), 255
	}
	got := Sobel(src)
	want := sobelNaive(src.Pix, w, h)
	for i := range want {
		if got.Pix[i] != want[i] {
			t.Fatalf("Sobel disagrees with reference at byte %d: got %d want %d", i, got.Pix[i], want[i])
		}
	}
}

// benchImage builds a deterministic w*h RGBA for benchmarking.
func benchImage(w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	seed := uint32(123456789)
	for i := 0; i < len(img.Pix); i += 4 {
		seed ^= seed << 13
		seed ^= seed >> 17
		seed ^= seed << 5
		img.Pix[i] = uint8(seed)
		img.Pix[i+1] = uint8(seed >> 8)
		img.Pix[i+2] = uint8(seed >> 16)
		img.Pix[i+3] = 255
	}
	return img
}

// BenchmarkSobel measures the fused production kernel. Its inner loop is a dense
// pass over a regular RGBA byte layout — exactly the shape a go-asmgen SIMD
// kernel (amd64/arm64/riscv64/loong64/ppc64le/s390x) would vectorise. TODO:
// drop in a SIMD Sobel behind kernels.Sobel and add a differential SIMD test.
func BenchmarkSobel(b *testing.B) {
	src := benchImage(256, 256)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Sobel(src)
	}
}

// BenchmarkSobelNaive is the unfused two-pass baseline, for comparison.
func BenchmarkSobelNaive(b *testing.B) {
	src := benchImage(256, 256)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = sobelNaive(src.Pix, 256, 256)
	}
}
