package images

import (
	"image"
	"image/color"
	"math"
	"testing"
)

// solid returns a 1x1 RGBA image of the given opaque colour.
func solid1(r, g, b uint8) *image.RGBA {
	m := image.NewRGBA(image.Rect(0, 0, 1, 1))
	m.SetRGBA(0, 0, color.RGBA{r, g, b, 255})
	return m
}

// close3 reports whether a three-channel pixel matches want within tol.
func close3(t *testing.T, name string, got []float64, want [3]float64, tol float64) {
	t.Helper()
	for i := 0; i < 3; i++ {
		if math.Abs(got[i]-want[i]) > tol {
			t.Fatalf("%s: channel %d = %.12g, want %.12g (tol %g)", name, i, got[i], want[i], tol)
		}
	}
}

// The reference tuples below are the exact outputs of scikit-image 0.26
// (skimage.color.rgb2xyz / rgb2lab), captured so the committed suite proves
// numerical parity without importing Python.

func TestRGBToXYZReference(t *testing.T) {
	cases := []struct {
		name    string
		r, g, b uint8
		xyz     [3]float64
	}{
		{"red", 255, 0, 0, [3]float64{0.412453, 0.212671, 0.019334}},
		{"green", 0, 255, 0, [3]float64{0.35758, 0.71516, 0.119193}},
		{"blue", 0, 0, 255, [3]float64{0.180423, 0.072169, 0.950227}},
		{"white", 255, 255, 255, [3]float64{0.950456, 1.0, 1.088754}},
		{"black", 0, 0, 0, [3]float64{0, 0, 0}},
		{"gray", 128, 128, 128, [3]float64{0.20516590749625624, 0.21586050011389926, 0.2350189829410083}},
	}
	for _, c := range cases {
		got := RGBToXYZ(solid1(c.r, c.g, c.b))
		close3(t, "xyz "+c.name, got.Pix, c.xyz, 1e-12)
	}
}

func TestRGBToLabReference(t *testing.T) {
	cases := []struct {
		name    string
		r, g, b uint8
		lab     [3]float64
	}{
		{"red", 255, 0, 0, [3]float64{53.2405879437449, 80.0923082256922, 67.2027510444287}},
		{"green", 0, 255, 0, [3]float64{87.73509948831895, -86.18302974439501, 83.17970317538452}},
		{"blue", 0, 0, 255, [3]float64{32.29567256501352, 79.18559091176553, -107.85730020669489}},
		// White is not exactly (100,0,0): scikit-image's matrix and D65 white
		// carry the same tiny offset, and matching it proves we use both.
		{"white", 255, 255, 255, [3]float64{100.0, -0.0024549378620508655, 0.004653421154054982}},
		{"black", 0, 0, 0, [3]float64{0, 0, 0}},
		{"gray", 128, 128, 128, [3]float64{53.58501345216902, -0.0014726455530578164, 0.0027914514965754478}},
	}
	for _, c := range cases {
		got := RGBToLab(solid1(c.r, c.g, c.b))
		close3(t, "lab "+c.name, got.Pix, c.lab, 1e-11)
	}
}

// rgbSet is a fixed palette used for the round-trip byte-parity tests.
var rgbSet = [][3]uint8{
	{255, 0, 0}, {0, 255, 0}, {0, 0, 255}, {255, 255, 255},
	{0, 0, 0}, {128, 64, 192}, {10, 20, 30}, {200, 150, 90},
}

// buildPalette lays rgbSet out as a 1xN opaque image.
func buildPalette() *image.RGBA {
	m := image.NewRGBA(image.Rect(0, 0, len(rgbSet), 1))
	for i, c := range rgbSet {
		m.SetRGBA(i, 0, color.RGBA{c[0], c[1], c[2], 255})
	}
	return m
}

func TestLabRoundTripBytes(t *testing.T) {
	// scikit-image lab2rgb(rgb2lab(x)) recovers each of these exactly; so must we.
	m := buildPalette()
	back := LabToRGB(RGBToLab(m))
	if b := back.Bounds(); b.Dx() != len(rgbSet) || b.Dy() != 1 {
		t.Fatalf("bounds %v", b)
	}
	for i, want := range rgbSet {
		off := back.PixOffset(i, 0)
		if back.Pix[off] != want[0] || back.Pix[off+1] != want[1] || back.Pix[off+2] != want[2] {
			t.Fatalf("Lab round-trip %d: got %v, want %v",
				i, back.Pix[off:off+3], want)
		}
		if back.Pix[off+3] != 255 {
			t.Fatalf("Lab round-trip %d: alpha = %d, want 255", i, back.Pix[off+3])
		}
	}
}

func TestXYZRoundTripBytes(t *testing.T) {
	m := buildPalette()
	back := XYZToRGB(RGBToXYZ(m))
	for i, want := range rgbSet {
		off := back.PixOffset(i, 0)
		if back.Pix[off] != want[0] || back.Pix[off+1] != want[1] || back.Pix[off+2] != want[2] {
			t.Fatalf("XYZ round-trip %d: got %v, want %v", i, back.Pix[off:off+3], want)
		}
	}
}

func TestLabToRGBOutOfGamutMatchesSkimage(t *testing.T) {
	// Out-of-gamut Lab values exercise the [0,1] clamp on both ends and the
	// negative-Z clip; the expected bytes are scikit-image's lab2rgb output.
	labs := [][3]float64{
		{50, 120, -120}, {50, 0, 120}, {100, 80, 80}, {20, -100, -100}, {90, -80, 90},
	}
	want := [][3]uint8{
		{184, 0, 255}, {147, 116, 0}, {255, 178, 100}, {0, 82, 204}, {86, 255, 0},
	}
	c := NewChannelImage(len(labs), 1)
	for i, l := range labs {
		c.Pix[3*i], c.Pix[3*i+1], c.Pix[3*i+2] = l[0], l[1], l[2]
	}
	got := LabToRGB(c)
	for i := range labs {
		off := got.PixOffset(i, 0)
		if got.Pix[off] != want[i][0] || got.Pix[off+1] != want[i][1] || got.Pix[off+2] != want[i][2] {
			t.Fatalf("out-of-gamut Lab %v: got %v, want %v",
				labs[i], got.Pix[off:off+3], want[i])
		}
	}
}

func TestAdjustGammaReference(t *testing.T) {
	// scikit-image exposure.adjust_gamma (default gain) on the palette.
	m := buildPalette()
	cases := []struct {
		gamma float64
		want  [][3]uint8
	}{
		{2.2, [][3]uint8{{255, 0, 0}, {0, 255, 0}, {0, 0, 255}, {255, 255, 255}, {0, 0, 0}, {56, 12, 137}, {0, 1, 2}, {149, 79, 26}}},
		{0.5, [][3]uint8{{255, 0, 0}, {0, 255, 0}, {0, 0, 255}, {255, 255, 255}, {0, 0, 0}, {181, 128, 221}, {50, 71, 87}, {226, 196, 151}}},
	}
	for _, c := range cases {
		out, err := AdjustGamma(m, c.gamma)
		if err != nil {
			t.Fatal(err)
		}
		for i, want := range c.want {
			off := out.PixOffset(i, 0)
			if out.Pix[off] != want[0] || out.Pix[off+1] != want[1] || out.Pix[off+2] != want[2] {
				t.Fatalf("gamma %g pixel %d: got %v, want %v",
					c.gamma, i, out.Pix[off:off+3], want)
			}
		}
	}
}

func TestAdjustGammaIdentityAndAlpha(t *testing.T) {
	// gamma 1 is the identity; alpha must pass through untouched.
	m := image.NewRGBA(image.Rect(0, 0, 2, 1))
	m.SetRGBA(0, 0, color.RGBA{10, 128, 240, 33})
	m.SetRGBA(1, 0, color.RGBA{0, 255, 7, 200})
	out, err := AdjustGamma(m, 1.0)
	if err != nil {
		t.Fatal(err)
	}
	if !equalPix(out.Pix, m.Pix) {
		t.Fatalf("gamma 1 changed pixels: %v vs %v", out.Pix, m.Pix)
	}
}

func TestAdjustGammaNegative(t *testing.T) {
	if _, err := AdjustGamma(solid1(1, 2, 3), -0.5); err == nil {
		t.Fatal("expected error for negative gamma")
	}
}

func TestSRGBLinearRoundTrip(t *testing.T) {
	// Cover both branches of each transfer function (below and above the knee)
	// and prove they invert one another. The exact knee value 0.04045 is skipped:
	// the sRGB standard's simplified constants leave a ~1e-8 discontinuity there
	// by design, so it does not round-trip to full precision.
	for _, c := range []float64{0, 0.01, 0.03, 0.05, 0.2, 0.5, 1.0} {
		lin := SRGBToLinear(c)
		back := LinearToSRGB(lin)
		if math.Abs(back-c) > 1e-12 {
			t.Fatalf("sRGB round-trip %g -> %g -> %g", c, lin, back)
		}
	}
	// Below-knee linear values map back through the linear branch of LinearToSRGB.
	if got := LinearToSRGB(0.001); math.Abs(got-12.92*0.001) > 1e-15 {
		t.Fatalf("LinearToSRGB low branch = %g", got)
	}
}

func TestDeltaE76(t *testing.T) {
	red := RGBToLab(solid1(255, 0, 0))
	green := RGBToLab(solid1(0, 255, 0))
	d, err := DeltaE76(red, green)
	if err != nil {
		t.Fatal(err)
	}
	// scikit-image color.deltaE_cie76(red, green).
	if math.Abs(d[0]-170.56559542639405) > 1e-9 {
		t.Fatalf("deltaE red-green = %.12g, want 170.56559542639405", d[0])
	}
	// A colour against itself is zero.
	self, _ := DeltaE76(red, red)
	if self[0] != 0 {
		t.Fatalf("deltaE self = %g, want 0", self[0])
	}
}

func TestDeltaE76SizeMismatch(t *testing.T) {
	a := NewChannelImage(2, 1)
	b := NewChannelImage(1, 1)
	if _, err := DeltaE76(a, b); err == nil {
		t.Fatal("expected size-mismatch error")
	}
}

func TestColorScienceIgnoresNonRGBAOrigin(t *testing.T) {
	// A non-*image.RGBA source is accepted via ToRGBA; check a known colour
	// survives the conversion path.
	src := image.NewNRGBA(image.Rect(0, 0, 1, 1))
	src.Set(0, 0, color.NRGBA{255, 0, 0, 255})
	lab := RGBToLab(src)
	close3(t, "nrgba red", lab.Pix, [3]float64{53.2405879437449, 80.0923082256922, 67.2027510444287}, 1e-11)
}

// equalPix reports whether two byte slices are identical.
func equalPix(a, b []uint8) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
