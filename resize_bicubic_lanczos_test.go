package images

import (
	"bytes"
	"flag"
	"fmt"
	"image"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-gfx/gfx/raster"
	"github.com/go-gfx/gfx/resample"
)

// updateGolden regenerates the checked-in bicubic/lanczos golden outputs. Run
//
//	go test -run TestResizeBicubicLanczosGolden -update-golden ./
//
// to refresh them after a deliberate resample change; the plain run compares
// against them and fails on any unexpected drift.
var updateGolden = flag.Bool("update-golden", false, "rewrite bicubic/lanczos golden files")

// hqFixture builds a deterministic srcW*srcH straight-alpha RGBA image whose
// channels vary independently and whose alpha cycles through fully transparent
// (over non-zero colour), several mid values and opaque — the input that
// discriminates a correct premultiplied-alpha resize from a naive one.
func hqFixture(srcW, srcH int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, srcW, srcH))
	for y := 0; y < srcH; y++ {
		for x := 0; x < srcW; x++ {
			i := img.PixOffset(x, y)
			img.Pix[i] = uint8((x*29 + y*13 + 7) & 0xff)
			img.Pix[i+1] = uint8((x*7 + y*53 + 90) & 0xff)
			img.Pix[i+2] = uint8((x*17 + y*3 + 140) & 0xff)
			img.Pix[i+3] = []uint8{0, 48, 96, 160, 255}[(x+2*y)%5]
		}
	}
	return img
}

// hqModes pairs each new public mode with the premultiplied resample mode it
// must delegate to.
var hqModes = []struct {
	name  string
	imode ResizeMode
	rmode resample.Mode
}{
	{"Bicubic", Bicubic, resample.Bicubic},
	{"Lanczos", Lanczos, resample.Lanczos},
}

var hqGeoms = []struct{ sw, sh, dw, dh int }{
	{8, 6, 3, 2},     // reduce, non-integer
	{6, 4, 13, 9},    // enlarge, non-integer
	{5, 5, 5, 5},     // identity
	{16, 12, 4, 3},   // integer reduce (4:1)
	{4, 3, 16, 12},   // integer enlarge (1:4)
	{9, 7, 5, 11},    // reduce one axis, enlarge the other
	{1, 1, 4, 4},     // 1x1 enlarged
	{7, 7, 1, 1},     // reduce to a single pixel
	{32, 20, 11, 27}, // larger coprime mix
}

// TestResizeBicubicLanczosDelegates is the wiring proof: the public Bicubic and
// Lanczos modes must produce exactly the bytes of resample.ResizePremultiplied
// with the matching filter, over the geometry sweep — confirming Resize selects
// the right filter, uses the premultiplied variant, and round-trips through the
// zero-copy raster adapters without altering a byte.
func TestResizeBicubicLanczosDelegates(t *testing.T) {
	for _, m := range hqModes {
		for _, g := range hqGeoms {
			t.Run(fmt.Sprintf("%s_%dx%d_to_%dx%d", m.name, g.sw, g.sh, g.dw, g.dh), func(t *testing.T) {
				img := hqFixture(g.sw, g.sh)

				want, err := resample.ResizePremultiplied(AsRaster(img), g.dw, g.dh, m.rmode)
				if err != nil {
					t.Fatalf("resample.ResizePremultiplied: %v", err)
				}
				got, err := Resize(img, g.dw, g.dh, m.imode)
				if err != nil {
					t.Fatalf("Resize %s: %v", m.name, err)
				}
				if got.Rect.Dx() != g.dw || got.Rect.Dy() != g.dh {
					t.Fatalf("%s: got %v, want %dx%d", m.name, got.Rect, g.dw, g.dh)
				}
				if !bytes.Equal(got.Pix, want.Pix) {
					t.Fatalf("%s %dx%d->%dx%d: public Resize diverged from resample.ResizePremultiplied",
						m.name, g.sw, g.sh, g.dw, g.dh)
				}
			})
		}
	}
}

// TestResizeBicubicLanczosAreDistinct controls the switch wiring: on a
// non-trivial image each high-quality mode must produce a DIFFERENT result from
// Bilinear and from the other high-quality mode. If a case in Resize silently
// fell through to the wrong filter, the outputs would coincide and this would
// catch it — the failure mode a byte-equality "delegates" test cannot see on its
// own.
func TestResizeBicubicLanczosAreDistinct(t *testing.T) {
	img := hqFixture(16, 12)
	bil, err := Resize(img, 7, 9, Bilinear)
	if err != nil {
		t.Fatal(err)
	}
	bic, err := Resize(img, 7, 9, Bicubic)
	if err != nil {
		t.Fatal(err)
	}
	lan, err := Resize(img, 7, 9, Lanczos)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(bic.Pix, bil.Pix) {
		t.Fatalf("Bicubic collapsed onto Bilinear")
	}
	if bytes.Equal(lan.Pix, bil.Pix) {
		t.Fatalf("Lanczos collapsed onto Bilinear")
	}
	if bytes.Equal(bic.Pix, lan.Pix) {
		t.Fatalf("Bicubic and Lanczos produced identical output")
	}
}

// TestResizeBicubicLanczosOpaqueEqualsStraight documents and checks the stated
// equivalence: on a fully opaque image the premultiplied colour path is
// identical to the plain straight-alpha filter, so nothing is lost by routing
// opaque images through the premultiplied modes.
func TestResizeBicubicLanczosOpaqueEqualsStraight(t *testing.T) {
	img := hqFixture(12, 9)
	for i := 3; i < len(img.Pix); i += 4 {
		img.Pix[i] = 255 // force fully opaque
	}
	for _, m := range hqModes {
		straight, err := resample.Resize(AsRaster(img), 5, 4, m.rmode)
		if err != nil {
			t.Fatal(err)
		}
		got, err := Resize(img, 5, 4, m.imode)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got.Pix, straight.Pix) {
			t.Fatalf("%s: premultiplied path differs from straight on a fully opaque image", m.name)
		}
	}
}

// TestResizeBicubicLanczosGolden pins the exact output bytes for a fixed fixture
// and geometry to checked-in golden files, so a change in resample's numerical
// output (a different kernel, rounding, or premultiply convention) is caught as
// a visible, reviewable diff rather than passing silently.
func TestResizeBicubicLanczosGolden(t *testing.T) {
	const sw, sh, dw, dh = 10, 7, 6, 5
	img := hqFixture(sw, sh)
	for _, m := range hqModes {
		got, err := Resize(img, dw, dh, m.imode)
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join("testdata", fmt.Sprintf("resize_%s_golden.rgba", m.name))
		if *updateGolden {
			if err := os.WriteFile(path, got.Pix, 0o644); err != nil {
				t.Fatalf("write golden: %v", err)
			}
			t.Logf("wrote %s (%d bytes)", path, len(got.Pix))
			continue
		}
		want, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read golden (run with -update-golden to create): %v", err)
		}
		if len(want) != dw*dh*4 {
			t.Fatalf("%s golden is %d bytes, want %d", m.name, len(want), dw*dh*4)
		}
		if !bytes.Equal(got.Pix, want) {
			t.Fatalf("%s: output drifted from golden %s", m.name, path)
		}
	}
}

// TestResizeUnknownModeErrors covers the default arm of the switch: a mode
// outside the defined set is rejected.
func TestResizeUnknownModeErrors(t *testing.T) {
	img := hqFixture(4, 4)
	if _, err := Resize(img, 2, 2, ResizeMode(99)); err == nil {
		t.Fatalf("expected an error for an unknown mode")
	}
}

// TestResizeBicubicLanczosRasterAlias confirms the whole path stays zero-copy
// off a raster source: feeding an AsRGBA view of a raster and reading back an
// AsRaster view reproduces resample's own raster output exactly.
func TestResizeBicubicLanczosRasterAlias(t *testing.T) {
	src := &raster.Image{Pix: hqFixture(9, 6).Pix, W: 9, H: 6}
	want, err := resample.ResizePremultiplied(src, 4, 3, resample.Bicubic)
	if err != nil {
		t.Fatal(err)
	}
	got, err := Resize(AsRGBA(src), 4, 3, Bicubic)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got.Pix, want.Pix) {
		t.Fatalf("raster-sourced Bicubic diverged from resample")
	}
}
