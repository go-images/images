package images

import (
	"image"
	"image/color"
	"testing"
)

// boxBlurNaive is an independent O(radius) reference for the separable
// sliding-window BoxBlur kernel: for every pixel it directly averages the
// (2*radius+1)^2 clamp-to-edge neighbourhood. It exists only as the correctness
// oracle for the production running-sum implementation.
func boxBlurNaive(src []uint8, w, h, radius int) []uint8 {
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
	inv := 1.0 / float64(win*win)
	dst := make([]uint8, len(src))
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
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			var sr, sg, sb float64
			for dy := -radius; dy <= radius; dy++ {
				sy := clamp(y+dy, h)
				for dx := -radius; dx <= radius; dx++ {
					sx := clamp(x+dx, w)
					si := (sy*w + sx) * 4
					sr += float64(src[si])
					sg += float64(src[si+1])
					sb += float64(src[si+2])
				}
			}
			di := (y*w + x) * 4
			dst[di] = round(sr * inv)
			dst[di+1] = round(sg * inv)
			dst[di+2] = round(sb * inv)
			dst[di+3] = src[di+3]
		}
	}
	return dst
}

func TestBoxBlurUniform(t *testing.T) {
	// A uniform image stays uniform under any radius, including at the borders.
	src := solid(5, 5, color.RGBA{40, 80, 160, 200})
	for _, r := range []int{1, 2, 3} {
		out, err := BoxBlur(src, r)
		if err != nil {
			t.Fatal(err)
		}
		for y := 0; y < 5; y++ {
			for x := 0; x < 5; x++ {
				if px(out, x, y) != (color.RGBA{40, 80, 160, 200}) {
					t.Fatalf("radius %d changed uniform pixel %d,%d: %v", r, x, y, px(out, x, y))
				}
			}
		}
	}
}

func TestBoxBlurAverages(t *testing.T) {
	// A 3x1 image 0,90,0 box-blurred with radius 1: each output is the mean of
	// the clamped 3-wide window. Vertical pass is a no-op (height 1, clamp).
	// x=0 window {0,0,90}=30; x=1 window {0,90,0}=30; x=2 window {90,0,0}=30.
	src := image.NewRGBA(image.Rect(0, 0, 3, 1))
	src.SetRGBA(0, 0, color.RGBA{0, 0, 0, 255})
	src.SetRGBA(1, 0, color.RGBA{90, 90, 90, 255})
	src.SetRGBA(2, 0, color.RGBA{0, 0, 0, 255})
	out, err := BoxBlur(src, 1)
	if err != nil {
		t.Fatal(err)
	}
	for x := 0; x < 3; x++ {
		if got := px(out, x, 0).R; got != 30 {
			t.Fatalf("box blur x=%d = %d, want 30", x, got)
		}
	}
}

func TestBoxBlurMatchesNaive(t *testing.T) {
	const w, h = 19, 11
	src := benchImage(w, h)
	for _, r := range []int{1, 2, 4, 7} {
		got, err := BoxBlur(src, r)
		if err != nil {
			t.Fatal(err)
		}
		want := boxBlurNaive(src.Pix, w, h, r)
		for i := range want {
			if got.Pix[i] != want[i] {
				t.Fatalf("radius %d: BoxBlur disagrees with naive at byte %d: got %d want %d",
					r, i, got.Pix[i], want[i])
			}
		}
	}
}

func TestBoxBlurBadRadius(t *testing.T) {
	src := solid(3, 3, color.RGBA{1, 2, 3, 4})
	if _, err := BoxBlur(src, 0); err == nil {
		t.Fatal("expected error for radius=0")
	}
	if _, err := BoxBlur(src, -2); err == nil {
		t.Fatal("expected error for negative radius")
	}
}

func TestBoxBlurDoesNotMutateInput(t *testing.T) {
	src := benchImage(8, 8)
	cp := make([]uint8, len(src.Pix))
	copy(cp, src.Pix)
	if _, err := BoxBlur(src, 2); err != nil {
		t.Fatal(err)
	}
	for i := range cp {
		if src.Pix[i] != cp[i] {
			t.Fatalf("BoxBlur mutated input at byte %d", i)
		}
	}
}

// BenchmarkBoxBlur measures the separable running-sum box filter at a realistic
// size; the per-pixel cost is independent of the radius.
func BenchmarkBoxBlur(b *testing.B) {
	src := benchImage(512, 512)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = BoxBlur(src, 5)
	}
}

// BenchmarkGaussianBlur measures the separable Gaussian at a realistic size.
func BenchmarkGaussianBlur(b *testing.B) {
	src := benchImage(512, 512)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = GaussianBlur(src, 3.0)
	}
}
