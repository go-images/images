package images

import (
	"bytes"
	"image"
	"image/color"
	"testing"

	"github.com/go-gfx/gfx/raster"
)

// patternRaster builds a w*h raster.Image whose bytes vary across all four
// channels, deliberately including pixels whose alpha is 0 while their colour
// channels are non-zero (colour "hidden under" full transparency). A path that
// silently interpreted the straight bytes as premultiplied — dividing colour
// back out of alpha, or multiplying it in — would corrupt exactly those pixels,
// so they are the discriminating input for the straight-vs-premultiplied
// question the adapter must get right.
func patternRaster(w, h int) *raster.Image {
	r := raster.New(w, h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			i := (y*w + x) * 4
			r.Pix[i] = uint8((x*37 + y*11) & 0xff)
			r.Pix[i+1] = uint8((x*5 + y*97) & 0xff)
			r.Pix[i+2] = uint8((x*3 + y*3 + 200) & 0xff)
			// Alpha cycles through 0 (with non-zero colour above), mid, opaque.
			r.Pix[i+3] = []uint8{0, 64, 200, 255}[(x+y)%4]
		}
	}
	return r
}

// rgbaFrom returns an independent *image.RGBA holding a copy of r's bytes. It is
// the "old path" reference: the established image.RGBA pipeline fed the exact
// same source pixels, sharing no memory with the raster under test.
func rgbaFrom(r *raster.Image) *image.RGBA {
	pix := make([]uint8, len(r.Pix))
	copy(pix, r.Pix)
	return &image.RGBA{Pix: pix, Stride: 4 * r.W, Rect: image.Rect(0, 0, r.W, r.H)}
}

// TestResizeRasterMatchesResize is the control: for every mode and geometry, and
// for source pixels that include colour-under-transparency, the raster-typed
// ResizeRaster must emit byte-for-byte the same pixels as the established
// image.RGBA Resize on an independent copy of the same source. This proves that
// adopting go-gfx's raster.Image as the interchange type — via the AsRGBA /
// AsRaster adapters — changes no output byte, i.e. the two buffer types are
// interchangeable for this operation.
func TestResizeRasterMatchesResize(t *testing.T) {
	modes := []struct {
		name string
		mode ResizeMode
	}{
		{"Nearest", NearestNeighbor},
		{"Bilinear", Bilinear},
		{"Area", Area},
	}
	geoms := []struct{ sw, sh, dw, dh int }{
		{7, 5, 3, 2}, // downscale, non-integer ratio
		{7, 5, 2, 5}, // downscale one axis only
		{3, 2, 7, 5}, // upscale, non-integer ratio
		{4, 4, 4, 4}, // identity size
		{8, 1, 3, 1}, // single row
		{1, 6, 1, 2}, // single column
		{6, 6, 2, 2}, // integer downscale
	}
	for _, m := range modes {
		for _, g := range geoms {
			src := patternRaster(g.sw, g.sh)
			ref := rgbaFrom(src)

			want, err := Resize(ref, g.dw, g.dh, m.mode)
			if err != nil {
				t.Fatalf("%s %+v: reference Resize: %v", m.name, g, err)
			}
			got, err := ResizeRaster(src, g.dw, g.dh, m.mode)
			if err != nil {
				t.Fatalf("%s %+v: ResizeRaster: %v", m.name, g, err)
			}
			if got.W != g.dw || got.H != g.dh {
				t.Fatalf("%s %+v: got %dx%d, want %dx%d", m.name, g, got.W, got.H, g.dw, g.dh)
			}
			if !bytes.Equal(got.Pix, want.Pix) {
				t.Fatalf("%s %+v: raster path diverged from image.RGBA path", m.name, g)
			}
		}
	}
}

// TestResizeRasterControlIsSensitive controls the instrument: the byte-for-byte
// comparison used above must actually be able to fail. A single perturbed byte
// in the reference output has to make the equality check report a difference —
// otherwise a green TestResizeRasterMatchesResize would prove nothing.
func TestResizeRasterControlIsSensitive(t *testing.T) {
	src := patternRaster(7, 5)
	want, err := Resize(rgbaFrom(src), 3, 2, Bilinear)
	if err != nil {
		t.Fatalf("reference Resize: %v", err)
	}
	got, err := ResizeRaster(src, 3, 2, Bilinear)
	if err != nil {
		t.Fatalf("ResizeRaster: %v", err)
	}
	if !bytes.Equal(got.Pix, want.Pix) {
		t.Fatalf("precondition: paths must match before perturbation")
	}
	want.Pix[0] ^= 0xff
	if bytes.Equal(got.Pix, want.Pix) {
		t.Fatalf("comparison is insensitive: a corrupted reference still compared equal")
	}
}

// TestAsRGBAAliasesBuffer proves AsRGBA is zero-copy: the returned *image.RGBA
// and the raster share one backing array, so a write through either is seen by
// the other, and the geometry is preserved.
func TestAsRGBAAliasesBuffer(t *testing.T) {
	r := patternRaster(4, 3)
	v := AsRGBA(r)
	if v.Stride != 4*r.W || v.Rect != image.Rect(0, 0, r.W, r.H) {
		t.Fatalf("unexpected view geometry: stride=%d rect=%v", v.Stride, v.Rect)
	}
	v.Pix[0] = 0x11
	if r.Pix[0] != 0x11 {
		t.Fatalf("write through image.RGBA view not seen by raster: %d", r.Pix[0])
	}
	r.Pix[1] = 0x22
	if v.Pix[1] != 0x22 {
		t.Fatalf("write through raster not seen by image.RGBA view: %d", v.Pix[1])
	}
}

// TestAsRasterZeroCopyDense proves that for a dense, origin-anchored *image.RGBA
// AsRaster shares the buffer (no copy), the exact inverse of AsRGBA.
func TestAsRasterZeroCopyDense(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 5, 4))
	img.Pix[0] = 0x33
	r := AsRaster(img)
	if r.W != 5 || r.H != 4 {
		t.Fatalf("unexpected size %dx%d", r.W, r.H)
	}
	if &r.Pix[0] != &img.Pix[0] {
		t.Fatalf("expected a zero-copy alias of the dense image buffer")
	}
	r.Pix[0] = 0x44
	if img.Pix[0] != 0x44 {
		t.Fatalf("alias write not reflected in source image: %d", img.Pix[0])
	}
}

// TestAsRasterCompactsSubImage proves the copying branch: a sub-image (padded
// stride and non-origin rectangle) is compacted into a fresh, tightly packed
// buffer whose pixels match the sub-region and which no longer aliases the
// parent.
func TestAsRasterCompactsSubImage(t *testing.T) {
	parent := image.NewRGBA(image.Rect(0, 0, 6, 6))
	for y := 0; y < 6; y++ {
		for x := 0; x < 6; x++ {
			parent.SetRGBA(x, y, color.RGBA{uint8(x), uint8(y), 7, 255})
		}
	}
	sub := parent.SubImage(image.Rect(2, 1, 5, 4)).(*image.RGBA) // 3x3, non-origin, stride 24
	r := AsRaster(sub)
	if r.W != 3 || r.H != 3 {
		t.Fatalf("unexpected size %dx%d", r.W, r.H)
	}
	if r.Pix[0] == 0 && &r.Pix[0] == &sub.Pix[sub.PixOffset(2, 1)] {
		t.Fatalf("expected a compacted copy, not an alias")
	}
	for y := 0; y < 3; y++ {
		for x := 0; x < 3; x++ {
			i := (y*3 + x) * 4
			if r.Pix[i] != uint8(x+2) || r.Pix[i+1] != uint8(y+1) || r.Pix[i+2] != 7 || r.Pix[i+3] != 255 {
				t.Fatalf("pixel (%d,%d) wrong after compaction: % d", x, y, r.Pix[i:i+4])
			}
		}
	}
	// Confirm independence from the parent.
	r.Pix[0] = 0x99
	if parent.RGBAAt(2, 1).R == 0x99 {
		t.Fatalf("compacted raster must not alias the parent image")
	}
}

// TestResizeRasterError covers the error path: invalid dimensions propagate the
// error from the underlying Resize.
func TestResizeRasterError(t *testing.T) {
	src := patternRaster(3, 3)
	if _, err := ResizeRaster(src, 0, 2, NearestNeighbor); err == nil {
		t.Fatalf("expected an error for a non-positive width")
	}
	if _, err := ResizeRaster(src, 2, 2, ResizeMode(99)); err == nil {
		t.Fatalf("expected an error for an unknown mode")
	}
}

// benchGeom is the resize used by the two benchmarks below; keeping it identical
// isolates the only difference — the raster interchange type versus image.RGBA —
// so benchstat measures the adapter's overhead and nothing else.
const benchSW, benchSH, benchDW, benchDH = 512, 512, 200, 200

func BenchmarkResizeRGBA(b *testing.B) {
	src := rgbaFrom(patternRaster(benchSW, benchSH))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := Resize(src, benchDW, benchDH, Area); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkResizeRaster(b *testing.B) {
	src := patternRaster(benchSW, benchSH)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := ResizeRaster(src, benchDW, benchDH, Area); err != nil {
			b.Fatal(err)
		}
	}
}
