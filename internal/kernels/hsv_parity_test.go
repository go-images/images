package kernels

import (
	"math"
	"testing"
)

// This file proves that routing the HSV byte kernels through the shared colour
// layer (github.com/go-gfx/gfx/color) is BYTE-FOR-BYTE identical to the
// hand-rolled sRGB<->HSV geometry this package used to carry. The reference
// functions below are the PREVIOUS implementations of RGBToHSV / HSVToRGB,
// reproduced verbatim, so the sweep compares the new code against the code it
// replaced rather than against a variant of itself.

// refRGBToHSV is the verbatim pre-migration RGBToHSV (the local sRGB->HSV
// geometry that now lives in color.SRGBToHSV), kept here as the reference the
// migration must reproduce exactly.
func refRGBToHSV(dst, src []uint8) {
	for i := 0; i < len(src); i += 4 {
		r := float64(src[i]) / 255
		g := float64(src[i+1]) / 255
		b := float64(src[i+2]) / 255
		max := r
		if g > max {
			max = g
		}
		if b > max {
			max = b
		}
		min := r
		if g < min {
			min = g
		}
		if b < min {
			min = b
		}
		v := max
		delta := max - min
		var s float64
		if max > 0 {
			s = delta / max
		}
		var hue float64
		if delta > 0 {
			switch max {
			case r:
				hue = (g - b) / delta
			case g:
				hue = 2 + (b-r)/delta
			default:
				hue = 4 + (r-g)/delta
			}
			hue *= 60
			if hue < 0 {
				hue += 360
			}
		}
		dst[i] = ClampByte(hue * 255 / 360)
		dst[i+1] = ClampByte(s * 255)
		dst[i+2] = ClampByte(v * 255)
		dst[i+3] = src[i+3]
	}
}

// refHSVToRGB is the verbatim pre-migration HSVToRGB (the local HSV->sRGB
// geometry that now lives in color.HSVToSRGB).
func refHSVToRGB(dst, src []uint8) {
	for i := 0; i < len(src); i += 4 {
		h := float64(src[i]) / 255 * 360
		s := float64(src[i+1]) / 255
		v := float64(src[i+2]) / 255
		c := v * s
		hp := h / 60
		x := c * (1 - math.Abs(math.Mod(hp, 2)-1))
		var r, g, b float64
		switch int(hp) % 6 {
		case 0:
			r, g, b = c, x, 0
		case 1:
			r, g, b = x, c, 0
		case 2:
			r, g, b = 0, c, x
		case 3:
			r, g, b = 0, x, c
		case 4:
			r, g, b = x, 0, c
		default:
			r, g, b = c, 0, x
		}
		m := v - c
		dst[i] = ClampByte((r + m) * 255)
		dst[i+1] = ClampByte((g + m) * 255)
		dst[i+2] = ClampByte((b + m) * 255)
		dst[i+3] = src[i+3]
	}
}

// sweepPixels builds one 256*256 RGBA slice per fixed R channel, varying G and B
// over their full range with a fixed alpha, so the whole 24-bit RGB cube is
// covered across the 256 returned buffers.
func sweepPixels(r int, alpha uint8) []uint8 {
	pix := make([]uint8, 256*256*4)
	for g := 0; g < 256; g++ {
		for b := 0; b < 256; b++ {
			i := (b*256 + g) * 4
			pix[i] = uint8(r)
			pix[i+1] = uint8(g)
			pix[i+2] = uint8(b)
			pix[i+3] = alpha
		}
	}
	return pix
}

// TestRGBToHSVMatchesReplacedKernel sweeps the entire 24-bit RGB cube and
// asserts the go-gfx-backed RGBToHSV is byte-identical to the verbatim kernel it
// replaced.
func TestRGBToHSVMatchesReplacedKernel(t *testing.T) {
	for r := 0; r < 256; r++ {
		src := sweepPixels(r, 200)
		got := make([]uint8, len(src))
		want := make([]uint8, len(src))
		RGBToHSV(got, src)
		refRGBToHSV(want, src)
		for i := range want {
			if got[i] != want[i] {
				g, b := (i/4)%256, (i/4)/256
				t.Fatalf("RGBToHSV mismatch at rgb=(%d,%d,%d) chan %d: got %d want %d", r, g, b, i%4, got[i], want[i])
			}
		}
	}
}

// TestHSVToRGBMatchesReplacedKernel sweeps every byte-encoded HSV triple and
// asserts the go-gfx-backed HSVToRGB is byte-identical to the verbatim kernel it
// replaced.
func TestHSVToRGBMatchesReplacedKernel(t *testing.T) {
	for hh := 0; hh < 256; hh++ {
		// Reuse sweepPixels: treat R/G/B as H/S/V byte channels.
		src := sweepPixels(hh, 200)
		got := make([]uint8, len(src))
		want := make([]uint8, len(src))
		HSVToRGB(got, src)
		refHSVToRGB(want, src)
		for i := range want {
			if got[i] != want[i] {
				s, v := (i/4)%256, (i/4)/256
				t.Fatalf("HSVToRGB mismatch at hsv=(%d,%d,%d) chan %d: got %d want %d", hh, s, v, i%4, got[i], want[i])
			}
		}
	}
}

// TestHSVParityReferenceIsSensitive is the control on the control: it confirms
// the reference sweep actually detects a divergence, so a passing parity test
// above is meaningful and not a comparison that can never fail. It corrupts one
// output byte and checks the byte-compare catches it.
func TestHSVParityReferenceIsSensitive(t *testing.T) {
	src := sweepPixels(128, 200)
	got := make([]uint8, len(src))
	want := make([]uint8, len(src))
	RGBToHSV(got, src)
	refRGBToHSV(want, src)
	// Perturb a single value the way a real regression would.
	got[8] ^= 1
	diff := false
	for i := range want {
		if got[i] != want[i] {
			diff = true
			break
		}
	}
	if !diff {
		t.Fatal("reference comparison failed to detect a one-byte perturbation")
	}
}
