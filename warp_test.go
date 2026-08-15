package images

import (
	"image"
	"image/color"
	"math"
	"testing"
)

// distinctW builds a deterministic w-by-h opaque RGBA image whose channels vary
// with position, matching the fixture used to capture the scikit-image
// references below: R = x*50+10, G = y*60+20, B = x*x+y*7+5 (each mod 256).
func distinctW(w, h int) *image.RGBA {
	m := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			m.SetRGBA(x, y, color.RGBA{
				uint8((x*50 + 10) % 256),
				uint8((y*60 + 20) % 256),
				uint8((x*x + y*7 + 5) % 256),
				255,
			})
		}
	}
	return m
}

func almost(a, b float64) bool { return math.Abs(a-b) < 1e-12 }

func TestAffinePrimitives(t *testing.T) {
	// Identity fixes every point.
	if x, y := Identity().Apply(3, 7); x != 3 || y != 7 {
		t.Fatalf("identity moved a point to %g,%g", x, y)
	}
	// Translation and scaling.
	if x, y := Translation(2, -1).Apply(5, 5); x != 7 || y != 4 {
		t.Fatalf("translation = %g,%g", x, y)
	}
	if x, y := Scaling(2, 3).Apply(4, 5); x != 8 || y != 15 {
		t.Fatalf("scaling = %g,%g", x, y)
	}
	// Rotation by 90 degrees maps (1,0) to (0,1) in scikit-image's convention.
	if x, y := Rotation(math.Pi/2).Apply(1, 0); !almost(x, 0) || !almost(y, 1) {
		t.Fatalf("rotation 90 of (1,0) = %g,%g, want 0,1", x, y)
	}
	// Then composes: apply t first, then u.
	comp := Translation(1, 2).Then(Scaling(2, 2))
	if x, y := comp.Apply(3, 4); x != 8 || y != 12 { // (3+1)*2, (4+2)*2
		t.Fatalf("compose = %g,%g, want 8,12", x, y)
	}
	// Invert undoes a transform.
	tr := Translation(3, -2).Then(Rotation(0.7)).Then(Scaling(1.5, 0.8))
	inv := tr.Invert()
	x, y := inv.Apply(tr.Apply(9, -4))
	if !almost(x, 9) || !almost(y, -4) {
		t.Fatalf("invert round-trip = %g,%g, want 9,-4", x, y)
	}
}

func TestRotateMatrixMatchesSkimage(t *testing.T) {
	// For a 5-col, 4-row image the rotate-30 composition must equal the matrix
	// scikit-image builds: centre (2.0, 1.5), [[cos,-sin,c],[sin,cos,f]].
	cx, cy := 2.0, 1.5
	theta := 30 * math.Pi / 180
	m := Translation(-cx, -cy).Then(Rotation(theta)).Then(Translation(cx, cy))
	want := Affine{
		A: 0.866025403784439, B: -0.5, C: 1.017949192431122,
		D: 0.5, E: 0.866025403784439, F: -0.799038105676658,
	}
	for _, p := range []struct {
		got, exp float64
		name     string
	}{{m.A, want.A, "A"}, {m.B, want.B, "B"}, {m.C, want.C, "C"}, {m.D, want.D, "D"}, {m.E, want.E, "E"}, {m.F, want.F, "F"}} {
		if math.Abs(p.got-p.exp) > 1e-12 {
			t.Fatalf("matrix %s = %.15g, want %.15g", p.name, p.got, p.exp)
		}
	}
}

// oracleWarp is an independent reference sampler used to check Warp: for each
// output pixel it applies inv, then samples with the requested interpolation and
// border rule, working in [0,1] and quantising with round-half-to-even. It
// shares no code with warp.go.
func oracleWarp(src *image.RGBA, inv Affine, ow, oh int, interp Interp, mode BorderMode, cval float64) *image.RGBA {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	get := func(x, y, ch int) float64 {
		if x < 0 || x >= w || y < 0 || y >= h {
			if mode == BorderConstant {
				if ch == 3 {
					return 0
				}
				return cval / 255
			}
			if x < 0 {
				x = 0
			} else if x >= w {
				x = w - 1
			}
			if y < 0 {
				y = 0
			} else if y >= h {
				y = h - 1
			}
		}
		return float64(src.Pix[(y*w+x)*4+ch]) / 255
	}
	q := func(v float64) uint8 {
		if v <= 0 {
			return 0
		}
		if v >= 1 {
			return 255
		}
		return uint8(math.RoundToEven(v * 255))
	}
	dst := image.NewRGBA(image.Rect(0, 0, ow, oh))
	for oy := 0; oy < oh; oy++ {
		for ox := 0; ox < ow; ox++ {
			sx, sy := inv.Apply(float64(ox), float64(oy))
			di := dst.PixOffset(ox, oy)
			for ch := 0; ch < 4; ch++ {
				if interp == InterpNearest {
					dst.Pix[di+ch] = q(get(int(math.Round(sx)), int(math.Round(sy)), ch))
				} else {
					x0, y0 := int(math.Floor(sx)), int(math.Floor(sy))
					dx, dy := sx-float64(x0), sy-float64(y0)
					top := get(x0, y0, ch)*(1-dx) + get(x0+1, y0, ch)*dx
					bot := get(x0, y0+1, ch)*(1-dx) + get(x0+1, y0+1, ch)*dx
					dst.Pix[di+ch] = q(top*(1-dy) + bot*dy)
				}
			}
		}
	}
	return dst
}

func TestWarpMatchesOracle(t *testing.T) {
	src := distinctW(6, 5)
	mats := []Affine{
		{A: 0.9, B: 0.1, C: 1.5, D: -0.2, E: 1.1, F: -0.7},
		{A: 0.7071, B: -0.7071, C: 3, D: 0.7071, E: 0.7071, F: -1},
		{A: 1.3, E: 0.8, C: -2, F: 1},
	}
	for _, interp := range []Interp{InterpNearest, InterpBilinear} {
		for _, mode := range []BorderMode{BorderConstant, BorderEdge} {
			for _, cval := range []float64{0, 128} {
				for i, m := range mats {
					got := Warp(src, m, 7, 6, interp, mode, cval)
					want := oracleWarp(src, m, 7, 6, interp, mode, cval)
					for k := range got.Pix {
						if got.Pix[k] != want.Pix[k] {
							t.Fatalf("interp=%d mode=%d cval=%g mat=%d: byte %d = %d, oracle %d",
								interp, mode, cval, i, k, got.Pix[k], want.Pix[k])
						}
					}
				}
			}
		}
	}
}

func TestWarpNearestMatchesSkimage(t *testing.T) {
	src := distinctW(5, 4)
	m := Affine{A: 1.1, B: 0.2, C: 0.3, D: -0.15, E: 0.95, F: 0.5}
	got := Warp(src, m, 5, 4, InterpNearest, BorderConstant, 0)
	// scikit-image warp(order=0, mode="constant", cval=0), RGB, row-major.
	want := []uint8{
		10, 80, 12, 60, 20, 6, 160, 20, 14, 210, 20, 21, 0, 0, 0,
		60, 80, 13, 110, 80, 16, 160, 80, 21, 210, 80, 28, 0, 0, 0,
		60, 140, 20, 110, 140, 23, 160, 140, 28, 210, 140, 35, 0, 0, 0,
		60, 200, 27, 110, 200, 30, 160, 200, 35, 210, 200, 42, 0, 0, 0,
	}
	assertRGBEqual(t, got, want, 5, 4, 0)
}

func TestRotateNoResizeMatchesSkimage(t *testing.T) {
	src := distinctW(5, 4)
	got := Rotate(src, 25, false)
	if b := got.Bounds(); b.Dx() != 5 || b.Dy() != 4 {
		t.Fatalf("bounds %v, want 5x4", b)
	}
	// scikit-image rotate(25, resize=False, clip=False), RGB, row-major.
	want := []uint8{
		15, 6, 2, 69, 14, 6, 142, 28, 13, 187, 54, 22, 116, 44, 15,
		30, 32, 7, 75, 57, 11, 121, 83, 17, 166, 108, 25, 205, 130, 33,
		10, 84, 12, 54, 112, 17, 99, 137, 22, 145, 163, 29, 190, 188, 38,
		6, 78, 11, 33, 166, 23, 78, 192, 27, 89, 144, 23, 50, 59, 11,
	}
	assertRGBEqual(t, got, want, 5, 4, 0)
}

func TestRotateResizeMatchesSkimage(t *testing.T) {
	src := distinctW(5, 4)
	got := Rotate(src, 40, true)
	if b := got.Bounds(); b.Dx() != 6 || b.Dy() != 6 {
		t.Fatalf("bounds %v, want 6x6", b)
	}
	want := []uint8{
		0, 0, 0, 0, 0, 0, 53, 6, 5, 199, 19, 20, 59, 16, 7, 0, 0, 0,
		0, 0, 0, 43, 9, 4, 137, 25, 12, 175, 63, 21, 194, 94, 28, 34, 22, 6,
		16, 11, 3, 67, 32, 8, 105, 71, 15, 143, 109, 23, 182, 148, 32, 169, 150, 32,
		7, 29, 5, 35, 78, 12, 73, 117, 18, 111, 155, 25, 149, 194, 33, 86, 92, 18,
		1, 7, 1, 8, 105, 15, 41, 163, 22, 77, 195, 27, 39, 67, 10, 0, 0, 0,
		0, 0, 0, 2, 35, 5, 8, 166, 22, 10, 42, 6, 0, 0, 0, 0, 0, 0,
	}
	assertRGBEqual(t, got, want, 6, 6, 0)
}

func TestRotateResizeShapes(t *testing.T) {
	// scikit-image's output shape for an 11-col, 9-row image at each angle.
	src := image.NewRGBA(image.Rect(0, 0, 11, 9))
	cases := []struct {
		angle        float64
		wantW, wantH int
	}{
		{30, 14, 13}, {45, 14, 14}, {90, 9, 11},
		{17, 13, 12}, {-22.5, 13, 12}, {25, 13, 12}, {40, 14, 14},
	}
	for _, c := range cases {
		got := Rotate(src, c.angle, true)
		if b := got.Bounds(); b.Dx() != c.wantW || b.Dy() != c.wantH {
			t.Fatalf("rotate %g resize: bounds %v, want %dx%d", c.angle, got.Bounds(), c.wantW, c.wantH)
		}
	}
}

func TestRotateZeroIsIdentity(t *testing.T) {
	// A zero-degree rotation samples every output pixel at an integer input
	// coordinate, so it reproduces the source exactly (alpha included).
	src := distinctW(6, 5)
	got := Rotate(src, 0, false)
	for i := range got.Pix {
		if got.Pix[i] != src.Pix[i] {
			t.Fatalf("rotate 0 changed byte %d: %d vs %d", i, got.Pix[i], src.Pix[i])
		}
	}
}

func TestWarpNonRGBAOrigin(t *testing.T) {
	// A non-*image.RGBA source is accepted via ToRGBA.
	src := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	src.Set(0, 0, color.NRGBA{200, 100, 50, 255})
	src.Set(1, 0, color.NRGBA{10, 20, 30, 255})
	got := Warp(src, Identity(), 2, 2, InterpBilinear, BorderConstant, 0)
	if got.Pix[0] != 200 || got.Pix[1] != 100 || got.Pix[2] != 50 {
		t.Fatalf("identity warp of NRGBA lost the first pixel: %v", got.Pix[:4])
	}
}

// assertRGBEqual checks the R,G,B channels of got against a flat row-major RGB
// slice, allowing a per-channel absolute tolerance tol (0 for exact).
func assertRGBEqual(t *testing.T, got *image.RGBA, wantRGB []uint8, w, h, tol int) {
	t.Helper()
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			off := got.PixOffset(x, y)
			base := (y*w + x) * 3
			for ch := 0; ch < 3; ch++ {
				g := int(got.Pix[off+ch])
				want := int(wantRGB[base+ch])
				if d := g - want; d < -tol || d > tol {
					t.Fatalf("pixel (%d,%d) ch %d = %d, want %d (tol %d)", x, y, ch, g, want, tol)
				}
			}
		}
	}
}
