package images

import (
	"image"
	"image/color"
	"testing"
)

func TestThreshold(t *testing.T) {
	// Luminance: black=0, mid-gray(128)=128, white=255.
	src := image.NewRGBA(image.Rect(0, 0, 3, 1))
	src.SetRGBA(0, 0, color.RGBA{0, 0, 0, 200})
	src.SetRGBA(1, 0, color.RGBA{128, 128, 128, 200})
	src.SetRGBA(2, 0, color.RGBA{255, 255, 255, 200})
	out := Threshold(src, 128)
	// strictly greater than 128: only white passes.
	if px(out, 0, 0) != (color.RGBA{0, 0, 0, 200}) {
		t.Fatalf("black -> %v, want black", px(out, 0, 0))
	}
	if px(out, 1, 0) != (color.RGBA{0, 0, 0, 200}) {
		t.Fatalf("mid (==t) -> %v, want black (strict >)", px(out, 1, 0))
	}
	if px(out, 2, 0) != (color.RGBA{255, 255, 255, 200}) {
		t.Fatalf("white -> %v, want white", px(out, 2, 0))
	}
}

func TestOtsuThresholdBimodal(t *testing.T) {
	// Half the pixels at luminance ~30, half at ~200: Otsu should land between.
	img := image.NewRGBA(image.Rect(0, 0, 10, 1))
	for x := 0; x < 5; x++ {
		img.SetRGBA(x, 0, color.RGBA{30, 30, 30, 255})
	}
	for x := 5; x < 10; x++ {
		img.SetRGBA(x, 0, color.RGBA{200, 200, 200, 255})
	}
	t0 := OtsuThreshold(img)
	if t0 < 30 || t0 > 200 {
		t.Fatalf("otsu threshold %d not between the two modes", t0)
	}
	out := Otsu(img)
	// Dark pixels -> black, bright pixels -> white.
	if px(out, 0, 0).R != 0 || px(out, 9, 0).R != 255 {
		t.Fatalf("otsu split wrong: dark=%d bright=%d", px(out, 0, 0).R, px(out, 9, 0).R)
	}
}

func TestOtsuThresholdSingleValue(t *testing.T) {
	// A constant image: no between-class variance is positive, returns 0.
	img := solid(4, 4, color.RGBA{100, 100, 100, 255})
	if got := OtsuThreshold(img); got != 0 {
		t.Fatalf("constant image otsu = %d, want 0", got)
	}
}

func TestOtsuThresholdAllWhite(t *testing.T) {
	// Exercises the wF==0 early break: every pixel is white (luminance 255).
	img := solid(3, 3, color.RGBA{255, 255, 255, 255})
	if got := OtsuThreshold(img); got != 0 {
		t.Fatalf("all-white otsu = %d, want 0", got)
	}
}

func TestHSVRoundTrip(t *testing.T) {
	// A spread of colours must survive RGB->HSV->RGB within a small tolerance
	// (the byte encoding of hue/sat/val is lossy).
	src := image.NewRGBA(image.Rect(0, 0, 8, 1))
	cols := []color.RGBA{
		{255, 0, 0, 255}, {0, 255, 0, 255}, {0, 0, 255, 255}, // hue 0/120/240
		{255, 255, 0, 128}, {0, 255, 255, 255}, {255, 0, 255, 255}, // yellow/cyan/magenta -> hue sextants 1/3/5
		{10, 20, 30, 255}, {200, 200, 200, 255},
	}
	for x, c := range cols {
		src.SetRGBA(x, 0, c)
	}
	round := HSVToRGB(RGBToHSV(src))
	for x, c := range cols {
		got := px(round, x, 0)
		if absDiff(got.R, c.R) > 4 || absDiff(got.G, c.G) > 4 || absDiff(got.B, c.B) > 4 {
			t.Fatalf("hsv round-trip x=%d: got %v want ~%v", x, got, c)
		}
		if got.A != c.A {
			t.Fatalf("hsv round-trip dropped alpha at x=%d: %d vs %d", x, got.A, c.A)
		}
	}
}

func TestRGBToHSVPrimaries(t *testing.T) {
	// Pure red has hue 0; pure green hue 120 (->85 in byte form); blue hue 240
	// (->170). Saturation and value are full (255).
	src := image.NewRGBA(image.Rect(0, 0, 3, 1))
	src.SetRGBA(0, 0, color.RGBA{255, 0, 0, 255})
	src.SetRGBA(1, 0, color.RGBA{0, 255, 0, 255})
	src.SetRGBA(2, 0, color.RGBA{0, 0, 255, 255})
	hsv := RGBToHSV(src)
	if h := px(hsv, 0, 0); h.R != 0 || h.G != 255 || h.B != 255 {
		t.Fatalf("red hsv = %v, want {0,255,255}", h)
	}
	if h := px(hsv, 1, 0).R; absDiff(h, 85) > 1 {
		t.Fatalf("green hue byte = %d, want ~85", h)
	}
	if h := px(hsv, 2, 0).R; absDiff(h, 170) > 1 {
		t.Fatalf("blue hue byte = %d, want ~170", h)
	}
}

func TestHSVToRGBGrayscale(t *testing.T) {
	// Zero saturation -> R=G=B=value regardless of hue (exercises delta==0).
	src := image.NewRGBA(image.Rect(0, 0, 1, 1))
	src.SetRGBA(0, 0, color.RGBA{0, 0, 200, 255}) // H=0, S=0, V~200
	out := HSVToRGB(src)
	got := px(out, 0, 0)
	if got.R != got.G || got.G != got.B {
		t.Fatalf("zero-sat HSV not gray: %v", got)
	}
}

func TestColorOpsDoNotMutateInput(t *testing.T) {
	src := distinct(4, 4)
	cp := make([]uint8, len(src.Pix))
	copy(cp, src.Pix)
	_ = Threshold(src, 100)
	_ = Otsu(src)
	_ = RGBToHSV(src)
	_ = HSVToRGB(src)
	for i := range cp {
		if src.Pix[i] != cp[i] {
			t.Fatalf("a color op mutated input at byte %d", i)
		}
	}
}

func absDiff(a, b uint8) int {
	if a > b {
		return int(a - b)
	}
	return int(b - a)
}
