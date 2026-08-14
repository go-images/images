package images

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"testing"

	ico "github.com/sergeymakinen/go-ico"
	"golang.org/x/image/bmp"
	"golang.org/x/image/tiff"
)

// This file proves Wave-2 decoding of eight container formats through
// github.com/go-gfx/gfx/codec while keeping the historical PNG/JPEG path
// byte-for-byte unchanged, and keeping the whole library's premultiplied-at-input
// alpha convention consistent across every format.
//
// All fixtures are synthetic and generated in-test (never personal data); any
// naming is English.

// ---------------------------------------------------------------------------
// Synthetic fixtures
// ---------------------------------------------------------------------------

// straightGradient builds a w*h straight-alpha NRGBA image whose alpha VARIES
// across the frame, so premultiplied-vs-straight divergence is exercised (a
// colour under partial transparency is the discriminating input). The content is
// a deterministic ramp.
func straightGradient(w, h int) *image.NRGBA {
	m := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			m.SetNRGBA(x, y, color.NRGBA{
				R: uint8(10 + x*29),
				G: uint8(20 + y*31),
				B: uint8(200 - x*7),
				A: uint8((x*37 + y*53) & 0xff), // sweeps 0..255, includes fully transparent
			})
		}
	}
	return m
}

// opaqueGradient is straightGradient forced fully opaque (A=255), for formats
// (GIF, BMP-24, JPEG) whose reference encoders do not preserve partial alpha.
func opaqueGradient(w, h int) *image.NRGBA {
	m := straightGradient(w, h)
	for i := 3; i < len(m.Pix); i += 4 {
		m.Pix[i] = 0xff
	}
	return m
}

// palettedFrom quantises img into a Paletted image using a fixed palette that
// contains its exact colours, so a GIF round-trip is lossless for our ramp.
func palettedFrom(t *testing.T, img *image.NRGBA) *image.Paletted {
	t.Helper()
	seen := map[color.NRGBA]bool{}
	var pal color.Palette
	b := img.Bounds()
	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			c := img.NRGBAAt(x, y)
			if !seen[c] {
				seen[c] = true
				pal = append(pal, c)
			}
		}
	}
	if len(pal) > 256 {
		t.Fatalf("paletted fixture needs %d colours (>256); shrink it", len(pal))
	}
	p := image.NewPaletted(b, pal)
	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			p.Set(x, y, img.NRGBAAt(x, y))
		}
	}
	return p
}

// webpBase64 is a synthetic 6x4 lossless WebP gradient. golang.org/x/image has
// no WebP encoder, so these bytes were produced once from a generated gradient
// and embedded verbatim (neutral, synthetic content). The tests below prove the
// reference decoder we ship decodes them; this is the identical fixture used by
// go-gfx/gfx/codec's own suite.
const webpBase64 = "UklGRooBAABXRUJQVlA4TH4BAAAvBcAAAE1kRP/DRYBMMwAAAAAAAAAcAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAACA4AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAABBAJAMAAAAAAgAAAAAAAAAAAAAAAAAAAAAAAMAAAAAAAAAAEEAkAwAAAAAAAAAAAAAAAAAAAAIAAAAAAAAAwAAAAAAAAACICRDq/gAA"

func webpBytes(t *testing.T) []byte {
	t.Helper()
	b, err := base64.StdEncoding.DecodeString(webpBase64)
	if err != nil {
		t.Fatalf("webp base64: %v", err)
	}
	return b
}

// makeICO encodes a synthetic multi-size .ico from generated opaque gradients.
func makeICO(t *testing.T, sizes ...int) []byte {
	t.Helper()
	imgs := make([]image.Image, len(sizes))
	for i, s := range sizes {
		imgs[i] = opaqueGradient(s, s)
	}
	var buf bytes.Buffer
	if err := ico.EncodeAll(&buf, imgs); err != nil {
		t.Fatalf("ico encode: %v", err)
	}
	return buf.Bytes()
}

// makeICNS assembles a well-formed .icns container from PNG representations, the
// same shape go-gfx/gfx/codec demuxes.
func makeICNS(t *testing.T, sizes ...int) []byte {
	t.Helper()
	type chunk struct {
		typ     string
		payload []byte
	}
	types := []string{"ic07", "ic08", "ic09", "ic10", "ic11", "ic12", "ic13", "ic14"}
	var chunks []chunk
	for i, s := range sizes {
		var buf bytes.Buffer
		if err := png.Encode(&buf, opaqueGradient(s, s)); err != nil {
			t.Fatalf("icns png encode: %v", err)
		}
		chunks = append(chunks, chunk{types[i%len(types)], buf.Bytes()})
	}
	var body []byte
	for _, c := range chunks {
		hdr := make([]byte, 8)
		copy(hdr, c.typ)
		binary.BigEndian.PutUint32(hdr[4:], uint32(8+len(c.payload)))
		body = append(body, hdr...)
		body = append(body, c.payload...)
	}
	out := make([]byte, 8+len(body))
	copy(out, "icns")
	binary.BigEndian.PutUint32(out[4:], uint32(len(out)))
	copy(out[8:], body)
	return out
}

// ---------------------------------------------------------------------------
// CONTROL: PNG/JPEG byte-identical to the OLD decode path
// ---------------------------------------------------------------------------

// oldDecode reproduces VERBATIM the pre-Wave-2 Decode implementation: the exact
// two lines this change replaced. The parity tests below diff the new Decode
// against THIS, not against a variant of the new code.
func oldDecode(data []byte) (*image.RGBA, error) {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("images: decode: %w", err)
	}
	return ToRGBA(img), nil
}

// diffPix returns a human-readable first mismatch between two RGBA images, or ""
// if identical (bounds and every byte).
func diffPix(a, b *image.RGBA) string {
	if a.Bounds() != b.Bounds() {
		return fmt.Sprintf("bounds differ: %v vs %v", a.Bounds(), b.Bounds())
	}
	if len(a.Pix) != len(b.Pix) {
		return fmt.Sprintf("pix length differ: %d vs %d", len(a.Pix), len(b.Pix))
	}
	for i := range a.Pix {
		if a.Pix[i] != b.Pix[i] {
			return fmt.Sprintf("byte %d differ: %d vs %d (pixel %d channel %d)", i, a.Pix[i], b.Pix[i], i/4, i%4)
		}
	}
	return ""
}

// pngVariants returns a sweep of synthetic PNG encodings covering the source
// image types the standard PNG encoder emits, INCLUDING colour under partial
// transparency (the discriminating alpha input).
func pngVariants(t *testing.T) map[string][]byte {
	t.Helper()
	out := map[string][]byte{}
	enc := func(name string, m image.Image) {
		var buf bytes.Buffer
		if err := png.Encode(&buf, m); err != nil {
			t.Fatalf("png encode %s: %v", name, err)
		}
		out[name] = buf.Bytes()
	}
	// NRGBA with varying alpha (decodes to *image.NRGBA): the premult trap input.
	enc("nrgba_alpha", straightGradient(9, 7))
	// Fully opaque NRGBA.
	enc("nrgba_opaque", opaqueGradient(9, 7))
	// Pre-built RGBA (decodes to *image.RGBA path in ToRGBA).
	rgba := image.NewRGBA(image.Rect(0, 0, 5, 5))
	for y := 0; y < 5; y++ {
		for x := 0; x < 5; x++ {
			rgba.SetRGBA(x, y, color.RGBA{uint8(x * 40), uint8(y * 40), 0x33, 0xff})
		}
	}
	enc("rgba", rgba)
	// Grayscale.
	gray := image.NewGray(image.Rect(0, 0, 6, 4))
	for i := range gray.Pix {
		gray.Pix[i] = uint8(i * 9)
	}
	enc("gray", gray)
	// Paletted with a transparent entry (colour under transparency again).
	pal := color.Palette{
		color.NRGBA{0, 0, 0, 0},
		color.NRGBA{200, 30, 40, 128},
		color.NRGBA{10, 220, 60, 255},
	}
	pimg := image.NewPaletted(image.Rect(0, 0, 6, 6), pal)
	for y := 0; y < 6; y++ {
		for x := 0; x < 6; x++ {
			pimg.SetColorIndex(x, y, uint8((x+y)%3))
		}
	}
	enc("paletted_alpha", pimg)
	// 16-bit NRGBA64 with alpha.
	n64 := image.NewNRGBA64(image.Rect(0, 0, 4, 4))
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			n64.SetNRGBA64(x, y, color.NRGBA64{uint16(x * 8000), uint16(y * 8000), 0x4000, uint16(x * 16000)})
		}
	}
	enc("nrgba64_alpha", n64)
	return out
}

// TestDecodePNGByteIdenticalToOld sweeps a battery of PNG encodings and proves the
// new Decode is byte-for-byte identical to the replaced image.Decode+ToRGBA path,
// including colour under partial transparency.
func TestDecodePNGByteIdenticalToOld(t *testing.T) {
	for name, data := range pngVariants(t) {
		got, err := Decode(bytes.NewReader(data))
		if err != nil {
			t.Fatalf("%s: new Decode: %v", name, err)
		}
		want, err := oldDecode(data)
		if err != nil {
			t.Fatalf("%s: old Decode: %v", name, err)
		}
		if d := diffPix(got, want); d != "" {
			t.Fatalf("%s: new PNG decode diverges from old path: %s", name, d)
		}
	}
}

// TestDecodeJPEGByteIdenticalToOld proves JPEG parity with the old path over a
// sweep of sizes (JPEG is lossy, so parity is defined against the OLD decode of
// the SAME bytes, not against the source image).
func TestDecodeJPEGByteIdenticalToOld(t *testing.T) {
	for _, dim := range [][2]int{{3, 2}, {8, 6}, {17, 11}, {32, 32}} {
		var buf bytes.Buffer
		if err := jpeg.Encode(&buf, opaqueGradient(dim[0], dim[1]), &jpeg.Options{Quality: 85}); err != nil {
			t.Fatalf("jpeg encode %v: %v", dim, err)
		}
		data := buf.Bytes()
		got, err := Decode(bytes.NewReader(data))
		if err != nil {
			t.Fatalf("%v: new Decode: %v", dim, err)
		}
		want, err := oldDecode(data)
		if err != nil {
			t.Fatalf("%v: old Decode: %v", dim, err)
		}
		if d := diffPix(got, want); d != "" {
			t.Fatalf("%v: new JPEG decode diverges from old path: %s", dim, d)
		}
	}
}

// TestControlParityInstrumentHasTeeth is the instrument control: it confirms the
// byte-identity differ actually FAILS when the decode is perturbed. A straight
// (non-premultiplied) decode of a transparent PNG must differ from the old
// premultiplied path — if diffPix reported "identical" here, the parity tests
// above would be worthless.
func TestControlParityInstrumentHasTeeth(t *testing.T) {
	data := pngVariants(t)["nrgba_alpha"]
	old, err := oldDecode(data)
	if err != nil {
		t.Fatal(err)
	}
	// Perturbed decode: NRGBA (straight) WITHOUT the ToRGBA premultiply.
	src, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	nr := src.(*image.NRGBA)
	straight := &image.RGBA{Pix: append([]byte(nil), nr.Pix...), Stride: nr.Stride, Rect: nr.Rect}
	if d := diffPix(straight, old); d == "" {
		t.Fatal("instrument is blind: straight-alpha decode compared EQUAL to the premultiplied old path")
	}
	// And confirm the un-perturbed new Decode does match (teeth cut both ways).
	got, err := Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if d := diffPix(got, old); d != "" {
		t.Fatalf("un-perturbed new Decode should match old: %s", d)
	}
}

// ---------------------------------------------------------------------------
// New formats: decode correctly vs synthetic golden, alpha convention respected
// ---------------------------------------------------------------------------

// wantPremul is the golden RGBA that the library's premultiplied-at-input
// convention must yield for a straight-alpha source pixel: it is exactly what
// image/draw (via color.RGBA()) produces, matching ToRGBA on an NRGBA.
func wantPremul(c color.NRGBA) color.RGBA {
	r, g, b, a := color.NRGBA(c).RGBA()
	return color.RGBA{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8), uint8(a >> 8)}
}

// assertMatchesSource checks a decoded RGBA equals the premultiplied golden of a
// straight-alpha source, pixel for pixel.
func assertMatchesSource(t *testing.T, tag string, got *image.RGBA, src *image.NRGBA) {
	t.Helper()
	b := src.Bounds()
	if got.Bounds().Dx() != b.Dx() || got.Bounds().Dy() != b.Dy() {
		t.Fatalf("%s: dimensions %v, want %dx%d", tag, got.Bounds(), b.Dx(), b.Dy())
	}
	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			want := wantPremul(src.NRGBAAt(x, y))
			if g := got.RGBAAt(x, y); g != want {
				t.Fatalf("%s: pixel %d,%d = %v, want %v (premultiplied golden)", tag, x, y, g, want)
			}
		}
	}
}

func TestDecodeGIF(t *testing.T) {
	src := opaqueGradient(8, 6)
	var buf bytes.Buffer
	if err := gif.Encode(&buf, palettedFrom(t, src), nil); err != nil {
		t.Fatalf("gif encode: %v", err)
	}
	got, err := Decode(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("Decode gif: %v", err)
	}
	assertMatchesSource(t, "gif", got, src)
}

func TestDecodeBMP(t *testing.T) {
	src := opaqueGradient(8, 6)
	var buf bytes.Buffer
	if err := bmp.Encode(&buf, src); err != nil {
		t.Fatalf("bmp encode: %v", err)
	}
	got, err := Decode(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("Decode bmp: %v", err)
	}
	assertMatchesSource(t, "bmp", got, src)
}

// TestDecodeTIFFAlpha decodes a TIFF carrying PARTIAL alpha, proving the new
// format obeys the premultiplied-at-input convention (a colour under partial
// transparency comes out premultiplied, matching the PNG path).
func TestDecodeTIFFAlpha(t *testing.T) {
	src := straightGradient(8, 6)
	var buf bytes.Buffer
	if err := tiff.Encode(&buf, src, nil); err != nil {
		t.Fatalf("tiff encode: %v", err)
	}
	got, err := Decode(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("Decode tiff: %v", err)
	}
	assertMatchesSource(t, "tiff", got, src)
	// Spot-check that at least one pixel is genuinely under partial transparency,
	// so this test really exercises premultiplication.
	var sawPartial bool
	for i := 3; i < len(src.Pix); i += 4 {
		if src.Pix[i] != 0 && src.Pix[i] != 0xff {
			sawPartial = true
			break
		}
	}
	if !sawPartial {
		t.Fatal("fixture has no partial-alpha pixel; premult path not exercised")
	}
}

func TestDecodeWEBP(t *testing.T) {
	got, err := Decode(bytes.NewReader(webpBytes(t)))
	if err != nil {
		t.Fatalf("Decode webp: %v", err)
	}
	if got.Bounds().Dx() != 6 || got.Bounds().Dy() != 4 {
		t.Fatalf("webp dimensions %v, want 6x4", got.Bounds())
	}
	// Opaque fixture: alpha must be fully opaque everywhere.
	for i := 3; i < len(got.Pix); i += 4 {
		if got.Pix[i] != 0xff {
			t.Fatalf("webp pixel %d alpha = %d, want 255", i/4, got.Pix[i])
		}
	}
}

func TestDecodeICO(t *testing.T) {
	data := makeICO(t, 16, 32, 48)
	// Decode -> largest representation.
	got, err := Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("Decode ico: %v", err)
	}
	if got.Bounds().Dx() != 48 {
		t.Fatalf("Decode ico largest = %d, want 48", got.Bounds().Dx())
	}
	assertMatchesSource(t, "ico48", got, opaqueGradient(48, 48))
	// DecodeBest picks the smallest representation >= target.
	best, err := DecodeBest(bytes.NewReader(data), 20)
	if err != nil {
		t.Fatalf("DecodeBest ico: %v", err)
	}
	if best.Bounds().Dx() != 32 {
		t.Fatalf("DecodeBest(20) = %d, want 32", best.Bounds().Dx())
	}
}

func TestDecodeICNS(t *testing.T) {
	data := makeICNS(t, 32, 64, 128)
	got, err := Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("Decode icns: %v", err)
	}
	if got.Bounds().Dx() != 128 {
		t.Fatalf("Decode icns largest = %d, want 128", got.Bounds().Dx())
	}
	assertMatchesSource(t, "icns128", got, opaqueGradient(128, 128))
	best, err := DecodeBest(bytes.NewReader(data), 40)
	if err != nil {
		t.Fatalf("DecodeBest icns: %v", err)
	}
	if best.Bounds().Dx() != 64 {
		t.Fatalf("DecodeBest(40) = %d, want 64", best.Bounds().Dx())
	}
}

// TestControlGoldenInstrumentHasTeeth confirms assertMatchesSource FAILS when the
// golden is perturbed: a straight-alpha expectation must NOT match the
// premultiplied decode of a transparent-bearing TIFF.
func TestControlGoldenInstrumentHasTeeth(t *testing.T) {
	src := straightGradient(4, 4)
	var buf bytes.Buffer
	if err := tiff.Encode(&buf, src, nil); err != nil {
		t.Fatal(err)
	}
	got, err := Decode(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	// Find a partial-alpha pixel and assert the STRAIGHT value differs from the
	// premultiplied decode there.
	b := src.Bounds()
	var checked bool
	for y := 0; y < b.Dy() && !checked; y++ {
		for x := 0; x < b.Dx(); x++ {
			s := src.NRGBAAt(x, y)
			if s.A == 0 || s.A == 0xff {
				continue
			}
			straight := color.RGBA{s.R, s.G, s.B, s.A}
			if got.RGBAAt(x, y) == straight {
				t.Fatalf("instrument blind: premult decode equalled straight source at %d,%d", x, y)
			}
			checked = true
			break
		}
	}
	if !checked {
		t.Fatal("no partial-alpha pixel available to control the instrument")
	}
}

// ---------------------------------------------------------------------------
// Error paths
// ---------------------------------------------------------------------------

func TestDecodeUnknownFormat(t *testing.T) {
	if _, err := Decode(bytes.NewReader([]byte("not an image at all"))); err == nil {
		t.Fatal("expected error for unrecognised format")
	}
}

func TestDecodeCorruptPerFormat(t *testing.T) {
	cases := map[string][]byte{
		"png":  {0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n', 0, 0, 0, 0},
		"jpeg": {0xFF, 0xD8, 0xFF, 0x00, 0x11, 0x22},
		"gif":  []byte("GIF89a\x00\x00\x00\x00garbage"),
		"tiff": append([]byte("II*\x00"), []byte("\xff\xff\xff\xffcorrupt")...),
		"bmp":  append([]byte("BM"), make([]byte, 20)...),
		"ico":  {0x00, 0x00, 0x01, 0x00, 0xFF, 0xFF},
		"icns": append([]byte("icns"), []byte{0, 0, 0, 8}...),
	}
	for name, data := range cases {
		if _, err := Decode(bytes.NewReader(data)); err == nil {
			t.Errorf("%s: expected decode error for corrupt input", name)
		}
	}
}

// failReader errors on the first Read, exercising the io.ReadAll failure branch
// of Decode and DecodeBest.
type failReader struct{}

func (failReader) Read([]byte) (int, error) { return 0, errFail }

func TestDecodeReadError(t *testing.T) {
	if _, err := Decode(failReader{}); err == nil {
		t.Fatal("expected read error from Decode")
	}
	if _, err := DecodeBest(failReader{}, 32); err == nil {
		t.Fatal("expected read error from DecodeBest")
	}
}

func TestDecodeBestTargetsSize(t *testing.T) {
	// A single-image format ignores targetSize.
	src := opaqueGradient(8, 6)
	var buf bytes.Buffer
	if err := bmp.Encode(&buf, src); err != nil {
		t.Fatal(err)
	}
	got, err := DecodeBest(bytes.NewReader(buf.Bytes()), 4)
	if err != nil {
		t.Fatalf("DecodeBest bmp: %v", err)
	}
	assertMatchesSource(t, "bmp-best", got, src)
}
