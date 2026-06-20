package kernels

import (
	"math/rand"
	"testing"
)

// withThreshold runs f with ParThreshold forced to v, restoring it after, so a
// test can drive both the serial (below-threshold) and the parallel
// (above-threshold) branches of the separable passes deterministically.
func withThreshold(v int, f func()) {
	old := ParThreshold
	ParThreshold = v
	defer func() { ParThreshold = old }()
	f()
}

func randRGBA(w, h int, seed int64) []uint8 {
	r := rand.New(rand.NewSource(seed))
	p := make([]uint8, w*h*4)
	for i := range p {
		p[i] = uint8(r.Intn(256))
	}
	return p
}

// gaussNaive is an independent O(radius) separable reference matching the
// production ConvolveSeparable's two-pass, byte-rounded-intermediate semantics.
func gaussNaive(src []uint8, w, h int, k []float64) []uint8 {
	radius := len(k) / 2
	tmp := make([]uint8, len(src))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			var sr, sg, sb float64
			for i := -radius; i <= radius; i++ {
				sx := clampIndex(x+i, w)
				wt := k[i+radius]
				si := (y*w + sx) * 4
				sr += wt * float64(src[si])
				sg += wt * float64(src[si+1])
				sb += wt * float64(src[si+2])
			}
			di := (y*w + x) * 4
			tmp[di] = ClampByte(sr)
			tmp[di+1] = ClampByte(sg)
			tmp[di+2] = ClampByte(sb)
			tmp[di+3] = src[di+3]
		}
	}
	dst := make([]uint8, len(src))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			var sr, sg, sb float64
			for i := -radius; i <= radius; i++ {
				sy := clampIndex(y+i, h)
				wt := k[i+radius]
				si := (sy*w + x) * 4
				sr += wt * float64(tmp[si])
				sg += wt * float64(tmp[si+1])
				sb += wt * float64(tmp[si+2])
			}
			di := (y*w + x) * 4
			dst[di] = ClampByte(sr)
			dst[di+1] = ClampByte(sg)
			dst[di+2] = ClampByte(sb)
			dst[di+3] = tmp[di+3]
		}
	}
	return dst
}

// morphNaive is an independent reference for the separable grayscale morphology.
func morphNaive(src []uint8, w, h, radius int, op morphOp) []uint8 {
	red := func(acc, v uint8) uint8 {
		if op == morphMax {
			if v > acc {
				return v
			}
			return acc
		}
		if v < acc {
			return v
		}
		return acc
	}
	pass := func(in []uint8, vertical bool) []uint8 {
		out := make([]uint8, len(in))
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				di := (y*w + x) * 4
				var a [3]uint8
				for c := 0; c < 3; c++ {
					var sx, sy int
					if vertical {
						sx, sy = x, clampIndex(y-radius, h)
					} else {
						sx, sy = clampIndex(x-radius, w), y
					}
					a[c] = in[(sy*w+sx)*4+c]
				}
				for t := -radius + 1; t <= radius; t++ {
					var sx, sy int
					if vertical {
						sx, sy = x, clampIndex(y+t, h)
					} else {
						sx, sy = clampIndex(x+t, w), y
					}
					si := (sy*w + sx) * 4
					for c := 0; c < 3; c++ {
						a[c] = red(a[c], in[si+c])
					}
				}
				out[di] = a[0]
				out[di+1] = a[1]
				out[di+2] = a[2]
				out[di+3] = in[di+3]
			}
		}
		return out
	}
	return pass(pass(src, false), true)
}

func eq(t *testing.T, name string, got, want []uint8) {
	t.Helper()
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("%s mismatch at byte %d: got %d want %d", name, i, got[i], want[i])
		}
	}
}

// TestConvolveSeparableMatchesNaive checks the SIMD/multicore separable Gaussian
// against the naive reference for several sizes (covering even/odd widths and
// images both below and above the parallel threshold) and at both threshold
// settings, so the serial path, the parallel path, and the border-clamp columns
// are all exercised. The result must match bit-for-bit at the amd64-v1 / arm64
// baselines the CI runs.
func TestConvolveSeparableMatchesNaive(t *testing.T) {
	sizes := []struct{ w, h int }{{1, 1}, {3, 1}, {1, 3}, {7, 5}, {16, 16}, {33, 17}}
	for _, s := range sizes {
		src := randRGBA(s.w, s.h, int64(s.w*100+s.h))
		for _, sigma := range []float64{0.6, 1.0, 2.0} {
			k := GaussianKernel1D(sigma)
			want := gaussNaive(src, s.w, s.h, k)
			for _, par := range []int{1 << 30, 1} { // serial then parallel
				withThreshold(par, func() {
					dst := make([]uint8, len(src))
					tmp := make([]uint8, len(src))
					ConvolveSeparable(dst, tmp, src, s.w, s.h, k)
					eq(t, "gauss", dst, want)
				})
			}
		}
	}
}

func TestMorphMatchesNaive(t *testing.T) {
	sizes := []struct{ w, h int }{{1, 1}, {3, 1}, {1, 3}, {7, 5}, {16, 16}, {33, 17}}
	for _, s := range sizes {
		src := randRGBA(s.w, s.h, int64(s.w*7+s.h*13))
		for _, radius := range []int{1, 2, 3} {
			wantE := morphNaive(src, s.w, s.h, radius, morphMin)
			wantD := morphNaive(src, s.w, s.h, radius, morphMax)
			for _, par := range []int{1 << 30, 1} {
				withThreshold(par, func() {
					gotE := make([]uint8, len(src))
					Erode(gotE, src, s.w, s.h, radius)
					eq(t, "erode", gotE, wantE)
					gotD := make([]uint8, len(src))
					Dilate(gotD, src, s.w, s.h, radius)
					eq(t, "dilate", gotD, wantD)
				})
			}
		}
	}
}

// TestSeparableParallelMatchesSerial checks that, for a larger image, the
// parallel path produces exactly the serial result for every separable op
// (independent of worker count and band boundaries).
func TestSeparableParallelMatchesSerial(t *testing.T) {
	const w, h = 40, 30
	src := randRGBA(w, h, 999)
	k := GaussianKernel1D(1.5)

	run := func(par int) (g, e, d, box []uint8) {
		withThreshold(par, func() {
			g = make([]uint8, len(src))
			tmp := make([]uint8, len(src))
			ConvolveSeparable(g, tmp, src, w, h, k)
			e = make([]uint8, len(src))
			Erode(e, src, w, h, 2)
			d = make([]uint8, len(src))
			Dilate(d, src, w, h, 2)
			box = make([]uint8, len(src))
			BoxBlur(box, src, w, h, 3)
		})
		return
	}
	gS, eS, dS, boxS := run(1 << 30)
	gP, eP, dP, boxP := run(1)
	eq(t, "gauss par", gP, gS)
	eq(t, "erode par", eP, eS)
	eq(t, "dilate par", dP, dS)
	eq(t, "box par", boxP, boxS)
}

// TestSobelParallelMatchesSerial covers the multicore Sobel paths.
func TestSobelParallelMatchesSerial(t *testing.T) {
	const w, h = 40, 30
	src := randRGBA(w, h, 4242)
	run := func(par int) (s, sx, sy []uint8) {
		withThreshold(par, func() {
			s = make([]uint8, len(src))
			Sobel(s, src, w, h)
			sx = make([]uint8, len(src))
			SobelX(sx, src, w, h)
			sy = make([]uint8, len(src))
			SobelY(sy, src, w, h)
		})
		return
	}
	sS, sxS, syS := run(1 << 30)
	sP, sxP, syP := run(1)
	eq(t, "sobel", sP, sS)
	eq(t, "sobelx", sxP, sxS)
	eq(t, "sobely", syP, syS)
}
