// Package benchmarks holds the performance-parity benchmark suite that compares
// go-images against scikit-image / OpenCV. It is a separate Go module (see
// go.mod) so it never participates in the parent module's 100%-coverage gate.
//
// The benchmark names are structured "<Op>/<Size>/<Mode>" so the companion
// Python harness (run.py) can pair each Go result with the matching reference.
// Run with:
//
//	go test -bench=. -benchmem -cpu=1 -count=5 ./...
package benchmarks

import (
	"fmt"
	"image"
	"math/rand"
	"testing"

	img "github.com/go-images/images"
)

// sizes benchmarked. Each is square; the pixel count drives Mpix/s.
var sizes = []int{512, 1024, 4096}

// makeRGBA builds a deterministic w*h RGBA test image (opaque) seeded so every
// run and the Python harness operate on statistically identical data. The exact
// pixels differ from numpy's PRNG, but correctness is checked separately
// (verify.py); here we only need a representative, reproducible workload.
func makeRGBA(w, h int) *image.RGBA {
	rng := rand.New(rand.NewSource(int64(w*131 + h)))
	im := image.NewRGBA(image.Rect(0, 0, w, h))
	for i := 0; i < len(im.Pix); i += 4 {
		im.Pix[i] = uint8(rng.Intn(256))
		im.Pix[i+1] = uint8(rng.Intn(256))
		im.Pix[i+2] = uint8(rng.Intn(256))
		im.Pix[i+3] = 255
	}
	return im
}

// report attaches Mpix/s to a benchmark given the side length.
func report(b *testing.B, side int) {
	mpix := float64(side*side) * float64(b.N) / 1e6
	b.ReportMetric(mpix/b.Elapsed().Seconds(), "Mpix/s")
}

func benchSizes(b *testing.B, fn func(b *testing.B, src *image.RGBA, side int)) {
	for _, s := range sizes {
		src := makeRGBA(s, s)
		b.Run(fmt.Sprintf("%d", s), func(b *testing.B) {
			b.ReportAllocs()
			fn(b, src, s)
		})
	}
}

func BenchmarkBoxBlur_r2(b *testing.B) {
	benchSizes(b, func(b *testing.B, src *image.RGBA, side int) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = img.BoxBlur(src, 2)
		}
		report(b, side)
	})
}

func BenchmarkGaussianBlur_s2(b *testing.B) {
	benchSizes(b, func(b *testing.B, src *image.RGBA, side int) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = img.GaussianBlur(src, 2.0)
		}
		report(b, side)
	})
}

func BenchmarkSobel(b *testing.B) {
	benchSizes(b, func(b *testing.B, src *image.RGBA, side int) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = img.Sobel(src)
		}
		report(b, side)
	})
}

func BenchmarkErode_r3(b *testing.B) {
	benchSizes(b, func(b *testing.B, src *image.RGBA, side int) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = img.Erode(src, 3)
		}
		report(b, side)
	})
}

func BenchmarkDilate_r3(b *testing.B) {
	benchSizes(b, func(b *testing.B, src *image.RGBA, side int) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = img.Dilate(src, 3)
		}
		report(b, side)
	})
}

func BenchmarkOpen_r3(b *testing.B) {
	benchSizes(b, func(b *testing.B, src *image.RGBA, side int) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = img.Open(src, 3)
		}
		report(b, side)
	})
}

func BenchmarkClose_r3(b *testing.B) {
	benchSizes(b, func(b *testing.B, src *image.RGBA, side int) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = img.Close(src, 3)
		}
		report(b, side)
	})
}

func BenchmarkFlipHorizontal(b *testing.B) {
	benchSizes(b, func(b *testing.B, src *image.RGBA, side int) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = img.FlipHorizontal(src)
		}
		report(b, side)
	})
}

func BenchmarkRotate90(b *testing.B) {
	benchSizes(b, func(b *testing.B, src *image.RGBA, side int) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = img.Rotate90(src)
		}
		report(b, side)
	})
}

func BenchmarkCrop(b *testing.B) {
	benchSizes(b, func(b *testing.B, src *image.RGBA, side int) {
		r := image.Rect(0, 0, side/2, side/2)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = img.Crop(src, r)
		}
		report(b, side)
	})
}

func BenchmarkRGBToHSV(b *testing.B) {
	benchSizes(b, func(b *testing.B, src *image.RGBA, side int) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = img.RGBToHSV(src)
		}
		report(b, side)
	})
}

func BenchmarkOtsu(b *testing.B) {
	benchSizes(b, func(b *testing.B, src *image.RGBA, side int) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = img.Otsu(src)
		}
		report(b, side)
	})
}

func BenchmarkGrayscale(b *testing.B) {
	benchSizes(b, func(b *testing.B, src *image.RGBA, side int) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = img.Grayscale(src)
		}
		report(b, side)
	})
}
