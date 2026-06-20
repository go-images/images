package images

import (
	"image"
	"image/color"
	"testing"
)

func TestCannyBadArgs(t *testing.T) {
	src := solid(4, 4, color.RGBA{1, 2, 3, 255})
	if _, err := Canny(src, 0, 1, 2); err == nil {
		t.Fatal("expected error for sigma=0")
	}
	if _, err := Canny(src, -1, 1, 2); err == nil {
		t.Fatal("expected error for negative sigma")
	}
	if _, err := Canny(src, 1, -1, 2); err == nil {
		t.Fatal("expected error for negative low threshold")
	}
	if _, err := Canny(src, 1, 1, -2); err == nil {
		t.Fatal("expected error for negative high threshold")
	}
	if _, err := Canny(src, 1, 5, 2); err == nil {
		t.Fatal("expected error for high < low")
	}
}

func TestCannyUniformHasNoEdges(t *testing.T) {
	// A flat field has zero gradient: the edge map is entirely black.
	src := solid(16, 16, color.RGBA{100, 100, 100, 255})
	out, err := Canny(src, 1.0, 1, 2)
	if err != nil {
		t.Fatal(err)
	}
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			if got := px(out, x, y); got != (color.RGBA{0, 0, 0, 255}) {
				t.Fatalf("uniform Canny at %d,%d = %v, want black", x, y, got)
			}
		}
	}
}

func TestCannyFindsStepEdge(t *testing.T) {
	// A vertical step edge (left half dark, right half bright) should yield a
	// connected run of edge pixels near the boundary column and none in the flat
	// interiors. Use a large image so the smoothing/border region is a minority.
	const w, h = 40, 24
	src := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			v := uint8(20)
			if x >= w/2 {
				v = 220
			}
			src.SetRGBA(x, y, color.RGBA{v, v, v, 255})
		}
	}
	out, err := Canny(src, 1.0, 10, 30)
	if err != nil {
		t.Fatal(err)
	}
	// There must be edge pixels, and they must cluster around the centre column.
	var edges int
	for y := 2; y < h-2; y++ {
		for x := 0; x < w; x++ {
			if px(out, x, y).R == 255 {
				edges++
				if x < w/2-3 || x > w/2+2 {
					t.Fatalf("edge pixel far from the step at %d,%d", x, y)
				}
			}
		}
	}
	if edges == 0 {
		t.Fatal("Canny found no edge on a clear step edge")
	}
	// Output is binary: every pixel is pure black or pure white, opaque.
	for i := 0; i < len(out.Pix); i += 4 {
		v := out.Pix[i]
		if (v != 0 && v != 255) || out.Pix[i+1] != v || out.Pix[i+2] != v || out.Pix[i+3] != 255 {
			t.Fatalf("non-binary Canny output at byte %d: %v", i, out.Pix[i:i+4])
		}
	}
}

func TestCannyHysteresisThresholdMonotone(t *testing.T) {
	// On a noisy textured image, raising the high threshold can only remove
	// edges, never add them: every pixel kept at a higher high threshold is also
	// a confirmed seed at a lower one (the low threshold and NMS are unchanged),
	// so the strict edge set shrinks monotonically. This pins the double-
	// threshold logic without depending on a particular synthetic gradient.
	src := benchImage(48, 36)
	count := func(low, high float64) int {
		out, err := Canny(src, 1.0, low, high)
		if err != nil {
			t.Fatal(err)
		}
		n := 0
		for i := 0; i < len(out.Pix); i += 4 {
			if out.Pix[i] == 255 {
				n++
			}
		}
		return n
	}
	loose := count(10, 20)
	tight := count(10, 80)
	if loose == 0 {
		t.Fatal("expected some Canny edges on a textured image")
	}
	if tight > loose {
		t.Fatalf("raising the high threshold added edges: %d > %d", tight, loose)
	}
	// A high threshold so large nothing seeds it yields no edges at all,
	// exercising the count==0 early-out path.
	if got := count(10, 1e9); got != 0 {
		t.Fatalf("unreachable high threshold still produced %d edges", got)
	}
}

// BenchmarkCanny measures the full Canny pipeline (Gaussian + Sobel + NMS +
// hysteresis) at a realistic size.
func BenchmarkCanny(b *testing.B) {
	src := benchImage(512, 512)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = Canny(src, 1.0, 20, 40)
	}
}
