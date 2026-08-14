package images

import (
	"bytes"
	"fmt"
	"math"
	"testing"

	"github.com/go-gfx/gfx/raster"
	"github.com/go-gfx/gfx/resample"
)

// This file is the control run behind Wave 3 of the 2-D unification: resolving
// the duplicated resize kernels between this library and go-gfx's resample
// package. Before Resize was rewired to delegate its Nearest, Bilinear and Area
// modes to resample, those three kernels lived here AND in resample as
// byte-for-byte copies. Delegation is only safe if the two produce identical
// bytes, so this file reproduces the kernels this library USED TO run —
// verbatim, from git history, sharing no code with either production path — and
// proves resample reproduces them exactly over a wide sweep of geometries and
// alpha patterns. Comparing the new production path against itself would prove
// nothing (the trap in feedback-prove-against-replaced-code), so the reference
// below is the OLD code, not a re-expression of the new code.
//
// Table established by TestResizeDedupSweep (see the PR body):
//
//	Nearest  (go-images) vs Nearest  (resample) -> byte-identical
//	Bilinear (go-images) vs Bilinear (resample) -> byte-identical
//	Area     (go-images) vs Box      (resample) -> byte-identical
//
// All three delegate; no kernel is kept. If a future resample change diverged
// on any ratio, this control would go red.

// refClampByte is images' old kernels.ClampByte, reproduced verbatim.
func refClampByte(v float64) uint8 {
	if v <= 0 {
		return 0
	}
	if v >= 255 {
		return 255
	}
	return uint8(v + 0.5)
}

// refClampIndex is the old kernels.clampIndex, reproduced verbatim.
func refClampIndex(i, n int) int {
	if i < 0 {
		return 0
	}
	if i >= n {
		return n - 1
	}
	return i
}

// refResizeNearest is the old kernels.ResizeNearest, reproduced verbatim.
func refResizeNearest(dst, src []uint8, srcW, srcH, dstW, dstH int) {
	for y := 0; y < dstH; y++ {
		sy := y * srcH / dstH
		for x := 0; x < dstW; x++ {
			sx := x * srcW / dstW
			si := (sy*srcW + sx) * 4
			di := (y*dstW + x) * 4
			dst[di] = src[si]
			dst[di+1] = src[si+1]
			dst[di+2] = src[si+2]
			dst[di+3] = src[si+3]
		}
	}
}

// refResizeBilinear is the old kernels.ResizeBilinear, reproduced verbatim.
func refResizeBilinear(dst, src []uint8, srcW, srcH, dstW, dstH int) {
	scaleX := float64(srcW) / float64(dstW)
	scaleY := float64(srcH) / float64(dstH)
	for y := 0; y < dstH; y++ {
		fy := (float64(y)+0.5)*scaleY - 0.5
		y0 := int(math.Floor(fy))
		wy := fy - float64(y0)
		y0c := refClampIndex(y0, srcH)
		y1c := refClampIndex(y0+1, srcH)
		for x := 0; x < dstW; x++ {
			fx := (float64(x)+0.5)*scaleX - 0.5
			x0 := int(math.Floor(fx))
			wx := fx - float64(x0)
			x0c := refClampIndex(x0, srcW)
			x1c := refClampIndex(x0+1, srcW)
			i00 := (y0c*srcW + x0c) * 4
			i01 := (y0c*srcW + x1c) * 4
			i10 := (y1c*srcW + x0c) * 4
			i11 := (y1c*srcW + x1c) * 4
			di := (y*dstW + x) * 4
			for c := 0; c < 4; c++ {
				top := float64(src[i00+c])*(1-wx) + float64(src[i01+c])*wx
				bot := float64(src[i10+c])*(1-wx) + float64(src[i11+c])*wx
				dst[di+c] = refClampByte(top*(1-wy) + bot*wy)
			}
		}
	}
}

// refBoxSpans is the old kernels.boxSpans, reproduced verbatim.
func refBoxSpans(srcN, dstN int) (starts, counts []int, weights []float64) {
	scale := float64(srcN) / float64(dstN)
	starts = make([]int, dstN)
	counts = make([]int, dstN)
	weights = make([]float64, 0, dstN+srcN)
	for i := 0; i < dstN; i++ {
		lo := float64(i) * scale
		hi := lo + scale
		i0 := int(math.Floor(lo))
		i1 := max(min(int(math.Ceil(hi)), srcN), i0+1)
		starts[i], counts[i] = i0, i1-i0
		for j := i0; j < i1; j++ {
			weights = append(weights, math.Min(hi, float64(j+1))-math.Max(lo, float64(j)))
		}
	}
	return starts, counts, weights
}

// refResizeArea is the old kernels.ResizeArea, reproduced verbatim.
func refResizeArea(dst, src []uint8, srcW, srcH, dstW, dstH int) {
	xs, xc, xw := refBoxSpans(srcW, dstW)
	ys, yc, yw := refBoxSpans(srcH, dstH)

	tmp := make([]float64, srcH*dstW*4)
	for y := 0; y < srcH; y++ {
		srcRow, tmpRow, wi := y*srcW*4, y*dstW*4, 0
		for x := 0; x < dstW; x++ {
			var acc [4]float64
			var sum float64
			for k := 0; k < xc[x]; k++ {
				w := xw[wi+k]
				si := srcRow + (xs[x]+k)*4
				acc[0] += float64(src[si]) * w
				acc[1] += float64(src[si+1]) * w
				acc[2] += float64(src[si+2]) * w
				acc[3] += float64(src[si+3]) * w
				sum += w
			}
			wi += xc[x]
			ti := tmpRow + x*4
			tmp[ti] = acc[0] / sum
			tmp[ti+1] = acc[1] / sum
			tmp[ti+2] = acc[2] / sum
			tmp[ti+3] = acc[3] / sum
		}
	}

	wi := 0
	for y := 0; y < dstH; y++ {
		dstRow := y * dstW * 4
		for x := 0; x < dstW; x++ {
			var acc [4]float64
			var sum float64
			for k := 0; k < yc[y]; k++ {
				w := yw[wi+k]
				ti := (ys[y]+k)*dstW*4 + x*4
				acc[0] += tmp[ti] * w
				acc[1] += tmp[ti+1] * w
				acc[2] += tmp[ti+2] * w
				acc[3] += tmp[ti+3] * w
				sum += w
			}
			di := dstRow + x*4
			dst[di] = refClampByte(acc[0] / sum)
			dst[di+1] = refClampByte(acc[1] / sum)
			dst[di+2] = refClampByte(acc[2] / sum)
			dst[di+3] = refClampByte(acc[3] / sum)
		}
		wi += yc[y]
	}
}

// dedupPattern fills a srcW*srcH straight-alpha RGBA plane whose four channels
// each vary on a different periodicity, and whose alpha deliberately cycles
// through 0 over non-zero colour, mids and opaque, so a path that confused
// straight and premultiplied bytes would diverge on exactly those pixels.
func dedupPattern(srcW, srcH int) []uint8 {
	pix := make([]uint8, srcW*srcH*4)
	for y := 0; y < srcH; y++ {
		for x := 0; x < srcW; x++ {
			i := (y*srcW + x) * 4
			pix[i] = uint8((x*37 + y*11 + 3) & 0xff)
			pix[i+1] = uint8((x*5 + y*97 + 17) & 0xff)
			pix[i+2] = uint8((x*3 + y*3 + 200) & 0xff)
			pix[i+3] = []uint8{0, 32, 64, 128, 200, 255}[(x*2+y*3)%6]
		}
	}
	return pix
}

// dedupGeoms is the geometry sweep: enlargements, reductions, both non-integer
// and integer ratios, single-row and single-column degeneracies, one-axis-only
// changes, identity, and 1x1 extremes — the ratios where a resampler's rounding
// and edge handling are most likely to disagree.
var dedupGeoms = []struct{ sw, sh, dw, dh int }{
	{7, 5, 3, 2},     // reduce, non-integer both axes
	{7, 5, 2, 5},     // reduce one axis, identity on the other
	{3, 2, 7, 5},     // enlarge, non-integer both axes
	{4, 4, 4, 4},     // identity
	{8, 1, 3, 1},     // single row, reduce
	{1, 6, 1, 2},     // single column, reduce
	{6, 6, 2, 2},     // integer reduce (3:1)
	{2, 2, 6, 6},     // integer enlarge (1:3)
	{9, 9, 4, 4},     // reduce, ratio 9:4
	{4, 4, 9, 9},     // enlarge, ratio 4:9
	{13, 7, 5, 11},   // reduce one axis, enlarge the other
	{1, 1, 5, 5},     // 1x1 source enlarged
	{5, 5, 1, 1},     // reduce to a single pixel
	{16, 16, 3, 5},   // heavy reduce, coprime target
	{3, 5, 16, 16},   // heavy enlarge, coprime source
	{32, 24, 32, 24}, // larger identity
	{31, 17, 29, 19}, // large coprime enlarge/reduce mix
	{64, 48, 21, 33}, // larger non-integer both axes
}

// modeCase pairs an images ResizeMode with the resample.Mode it must equal.
type modeCase struct {
	name  string
	imode ResizeMode
	rmode resample.Mode
	// ref runs the verbatim old kernel this mode used to run.
	ref func(dst, src []uint8, srcW, srcH, dstW, dstH int)
}

var dedupModes = []modeCase{
	{"Nearest", NearestNeighbor, resample.Nearest, refResizeNearest},
	{"Bilinear", Bilinear, resample.Bilinear, refResizeBilinear},
	{"Area", Area, resample.Box, refResizeArea},
}

// TestResizeDedupSweep is the control establishing the mode->byte-identical
// table. For every shared mode and every geometry, over a source with
// colour-under-transparency, it requires that
//
//	resample.Resize(src, ...)      (the delegate now in production)
//	the verbatim OLD images kernel  (refResize*)
//	Resize(img, ...)                (the public API, post-delegation)
//
// all agree byte-for-byte. The middle term is the load-bearing one: it proves
// resample equals the code images REPLACED, not merely the code images now
// calls.
func TestResizeDedupSweep(t *testing.T) {
	for _, m := range dedupModes {
		for _, g := range dedupGeoms {
			t.Run(fmt.Sprintf("%s_%dx%d_to_%dx%d", m.name, g.sw, g.sh, g.dw, g.dh), func(t *testing.T) {
				srcPix := dedupPattern(g.sw, g.sh)

				// Reference: the verbatim old kernel.
				want := make([]uint8, g.dw*g.dh*4)
				m.ref(want, srcPix, g.sw, g.sh, g.dw, g.dh)

				// Delegate: resample operating on a raster view of the same bytes.
				rsrc := &raster.Image{Pix: append([]uint8(nil), srcPix...), W: g.sw, H: g.sh}
				rout, err := resample.Resize(rsrc, g.dw, g.dh, m.rmode)
				if err != nil {
					t.Fatalf("resample.Resize: %v", err)
				}
				if !bytes.Equal(rout.Pix, want) {
					t.Fatalf("resample %s diverged from the replaced images kernel at %dx%d->%dx%d",
						m.name, g.sw, g.sh, g.dw, g.dh)
				}

				// Public API: must match the same reference after delegation.
				img := &raster.Image{Pix: append([]uint8(nil), srcPix...), W: g.sw, H: g.sh}
				pub, err := Resize(AsRGBA(img), g.dw, g.dh, m.imode)
				if err != nil {
					t.Fatalf("Resize: %v", err)
				}
				if !bytes.Equal(pub.Pix, want) {
					t.Fatalf("public Resize %s diverged from the replaced images kernel at %dx%d->%dx%d",
						m.name, g.sw, g.sh, g.dw, g.dh)
				}
			})
		}
	}
}

// The benchmarks below are the perf control for the dedup: each delegated mode
// is timed against the verbatim old kernel it replaced, on the same machine and
// the same source, so benchstat measures exactly the cost of routing through
// resample and the raster adapters. The delegate runs the identical scalar
// kernel, so the pair must stay within noise. Size mirrors a realistic
// thumbnailing reduction (512x512 -> 200x200).
const dedupBenchSW, dedupBenchSH, dedupBenchDW, dedupBenchDH = 512, 512, 200, 200

func benchOld(b *testing.B, ref func(dst, src []uint8, srcW, srcH, dstW, dstH int)) {
	src := dedupPattern(dedupBenchSW, dedupBenchSH)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dst := make([]uint8, dedupBenchDW*dedupBenchDH*4)
		ref(dst, src, dedupBenchSW, dedupBenchSH, dedupBenchDW, dedupBenchDH)
	}
}

func benchNew(b *testing.B, mode ResizeMode) {
	img := &raster.Image{Pix: dedupPattern(dedupBenchSW, dedupBenchSH), W: dedupBenchSW, H: dedupBenchSH}
	view := AsRGBA(img)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := Resize(view, dedupBenchDW, dedupBenchDH, mode); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkResizeNearestOld(b *testing.B) { benchOld(b, refResizeNearest) }
func BenchmarkResizeNearestNew(b *testing.B) { benchNew(b, NearestNeighbor) }

func BenchmarkResizeBilinearOld(b *testing.B) { benchOld(b, refResizeBilinear) }
func BenchmarkResizeBilinearNew(b *testing.B) { benchNew(b, Bilinear) }

func BenchmarkResizeAreaOld(b *testing.B) { benchOld(b, refResizeArea) }
func BenchmarkResizeAreaNew(b *testing.B) { benchNew(b, Area) }

// TestResizeDedupControlIsSensitive controls the instrument itself: the
// byte comparison must be able to report a difference. Perturbing one reference
// byte has to break the equality, otherwise a green TestResizeDedupSweep would
// be meaningless.
func TestResizeDedupControlIsSensitive(t *testing.T) {
	srcPix := dedupPattern(7, 5)
	want := make([]uint8, 3*2*4)
	refResizeArea(want, srcPix, 7, 5, 3, 2)

	rsrc := &raster.Image{Pix: append([]uint8(nil), srcPix...), W: 7, H: 5}
	rout, err := resample.Resize(rsrc, 3, 2, resample.Box)
	if err != nil {
		t.Fatalf("resample.Resize: %v", err)
	}
	if !bytes.Equal(rout.Pix, want) {
		t.Fatalf("precondition: resample and the reference must agree before perturbation")
	}
	want[0] ^= 0xff
	if bytes.Equal(rout.Pix, want) {
		t.Fatalf("comparison is insensitive: a corrupted reference still compared equal")
	}
}
