package images

import (
	"image"
	"image/color"
	"sort"
	"testing"
)

// medianNaive is an independent reference for the square median filter: for each
// pixel and channel it sorts the (2*radius+1)^2 clamp-to-edge neighbourhood and
// takes the middle element.
func medianNaive(src []uint8, w, h, radius int) []uint8 {
	clamp := func(i, n int) int {
		if i < 0 {
			return 0
		}
		if i >= n {
			return n - 1
		}
		return i
	}
	win := 2*radius + 1
	count := win * win
	mid := count / 2
	dst := make([]uint8, len(src))
	buf := make([]int, count)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			di := (y*w + x) * 4
			for c := 0; c < 3; c++ {
				n := 0
				for dy := -radius; dy <= radius; dy++ {
					sy := clamp(y+dy, h)
					for dx := -radius; dx <= radius; dx++ {
						sx := clamp(x+dx, w)
						buf[n] = int(src[(sy*w+sx)*4+c])
						n++
					}
				}
				sort.Ints(buf)
				dst[di+c] = uint8(buf[mid])
			}
			dst[di+3] = src[di+3]
		}
	}
	return dst
}

func TestMedianMatchesNaive(t *testing.T) {
	const w, h = 21, 15
	src := benchImage(w, h)
	for _, r := range []int{1, 2, 3} {
		got, err := Median(src, r)
		if err != nil {
			t.Fatal(err)
		}
		want := medianNaive(src.Pix, w, h, r)
		for i := range want {
			if got.Pix[i] != want[i] {
				t.Fatalf("radius %d: Median disagrees with naive at byte %d: got %d want %d",
					r, i, got.Pix[i], want[i])
			}
		}
	}
}

func TestMedianRemovesImpulse(t *testing.T) {
	// A single white speck on a uniform gray field is removed by a radius-1
	// median (the speck is one of nine samples; the median stays gray).
	src := solid(5, 5, color.RGBA{80, 80, 80, 255})
	src.SetRGBA(2, 2, color.RGBA{255, 255, 255, 255})
	out, err := Median(src, 1)
	if err != nil {
		t.Fatal(err)
	}
	if got := px(out, 2, 2); got != (color.RGBA{80, 80, 80, 255}) {
		t.Fatalf("median did not remove impulse: %v", got)
	}
}

func TestMedianBadRadius(t *testing.T) {
	src := solid(3, 3, color.RGBA{1, 2, 3, 4})
	if _, err := Median(src, 0); err == nil {
		t.Fatal("expected error for radius=0")
	}
	if _, err := Median(src, -1); err == nil {
		t.Fatal("expected error for negative radius")
	}
}

func TestMedianAllByteValuesAtTop(t *testing.T) {
	// Exercise the counting-selection upper bins: a window whose median is 255.
	src := solid(3, 3, color.RGBA{255, 255, 255, 255})
	out, err := Median(src, 1)
	if err != nil {
		t.Fatal(err)
	}
	if got := px(out, 1, 1).R; got != 255 {
		t.Fatalf("median of all-255 window = %d, want 255", got)
	}
}

func TestUnsharpMaskZeroAmountIsIdentity(t *testing.T) {
	// amount 0 recovers no detail, so the result equals the source exactly.
	src := benchImage(12, 9)
	out, err := UnsharpMask(src, 1.5, 0)
	if err != nil {
		t.Fatal(err)
	}
	for i := range src.Pix {
		if out.Pix[i] != src.Pix[i] {
			t.Fatalf("unsharp amount=0 changed byte %d: got %d want %d", i, out.Pix[i], src.Pix[i])
		}
	}
}

func TestUnsharpMaskSharpensEdge(t *testing.T) {
	// On a step edge, unsharp masking overshoots: the bright side of the edge
	// gets brighter than the original and the dark side darker (the classic
	// sharpening halo). Check the bright plateau pixel adjacent to the edge
	// exceeds its source value.
	const w, h = 9, 3
	src := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			v := uint8(60)
			if x >= w/2 {
				v = 180
			}
			src.SetRGBA(x, y, color.RGBA{v, v, v, 255})
		}
	}
	out, err := UnsharpMask(src, 1.0, 1.5)
	if err != nil {
		t.Fatal(err)
	}
	if got := px(out, w/2, 1).R; got <= 180 {
		t.Fatalf("unsharp did not overshoot bright edge: %d, want > 180", got)
	}
	if got := px(out, w/2-1, 1).R; got >= 60 {
		t.Fatalf("unsharp did not undershoot dark edge: %d, want < 60", got)
	}
}

func TestUnsharpMaskBadRadius(t *testing.T) {
	src := solid(3, 3, color.RGBA{1, 2, 3, 4})
	if _, err := UnsharpMask(src, 0, 1); err == nil {
		t.Fatal("expected error for radius=0")
	}
	if _, err := UnsharpMask(src, -1, 1); err == nil {
		t.Fatal("expected error for negative radius")
	}
}

func TestSharpenRuns(t *testing.T) {
	// Sharpen is UnsharpMask(1.0, 1.0); it must leave a flat field unchanged
	// (zero detail) and preserve dimensions and alpha.
	src := solid(6, 4, color.RGBA{90, 90, 90, 123})
	out := Sharpen(src)
	if out.Bounds() != src.Bounds() {
		t.Fatalf("Sharpen changed bounds: %v", out.Bounds())
	}
	if got := px(out, 3, 2); got != (color.RGBA{90, 90, 90, 123}) {
		t.Fatalf("Sharpen altered a flat field: %v", got)
	}
}

// BenchmarkMedian measures the square median filter at a realistic size; the
// counting-selection inner loop is O(window + 256) per pixel and fans out across
// cores above the parallel threshold.
func BenchmarkMedian(b *testing.B) {
	src := benchImage(512, 512)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = Median(src, 2)
	}
}
