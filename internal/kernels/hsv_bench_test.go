package kernels

import "testing"

// These benchmarks measure the go-gfx-backed HSV kernels against the verbatim
// pre-migration implementations (refRGBToHSV / refHSVToRGB in hsv_parity_test.go)
// so a benchstat run can confirm the delegation adds no allocation and no
// meaningful time per pixel. Bench names are paired (Old/New) for benchstat.

func benchHSV(b *testing.B, f func(dst, src []uint8)) {
	src := sweepPixels(97, 200) // 256*256 pixels
	dst := make([]uint8, len(src))
	b.SetBytes(int64(len(src)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		f(dst, src)
	}
}

func BenchmarkRGBToHSV_Old(b *testing.B) { benchHSV(b, refRGBToHSV) }
func BenchmarkRGBToHSV_New(b *testing.B) { benchHSV(b, RGBToHSV) }
func BenchmarkHSVToRGB_Old(b *testing.B) { benchHSV(b, refHSVToRGB) }
func BenchmarkHSVToRGB_New(b *testing.B) { benchHSV(b, HSVToRGB) }
