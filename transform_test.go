package images

import (
	"image"
	"image/color"
	"testing"
)

// distinct returns a w*h RGBA where each pixel encodes its own coordinates,
// so any rearrangement can be checked exactly: R=x, G=y, B=x+y, A=255.
func distinct(w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetRGBA(x, y, color.RGBA{uint8(x), uint8(y), uint8(x + y), 255})
		}
	}
	return img
}

func TestFlipHorizontal(t *testing.T) {
	src := distinct(3, 2)
	out := FlipHorizontal(src)
	for y := 0; y < 2; y++ {
		for x := 0; x < 3; x++ {
			if px(out, x, y) != px(src, 2-x, y) {
				t.Fatalf("flipH %d,%d = %v, want %v", x, y, px(out, x, y), px(src, 2-x, y))
			}
		}
	}
}

func TestFlipVertical(t *testing.T) {
	src := distinct(3, 2)
	out := FlipVertical(src)
	for y := 0; y < 2; y++ {
		for x := 0; x < 3; x++ {
			if px(out, x, y) != px(src, x, 1-y) {
				t.Fatalf("flipV %d,%d = %v, want %v", x, y, px(out, x, y), px(src, x, 1-y))
			}
		}
	}
}

func TestRotate90(t *testing.T) {
	// 3x2 source -> 2x3 result. Source (x,y) -> dst (y, w-1-x), w=3.
	src := distinct(3, 2)
	out := Rotate90(src)
	if b := out.Bounds(); b.Dx() != 2 || b.Dy() != 3 {
		t.Fatalf("rotate90 bounds %v, want 2x3", b)
	}
	for y := 0; y < 2; y++ {
		for x := 0; x < 3; x++ {
			if px(out, y, 2-x) != px(src, x, y) {
				t.Fatalf("rotate90 source %d,%d misplaced", x, y)
			}
		}
	}
}

func TestRotate180(t *testing.T) {
	src := distinct(3, 2)
	out := Rotate180(src)
	for y := 0; y < 2; y++ {
		for x := 0; x < 3; x++ {
			if px(out, x, y) != px(src, 2-x, 1-y) {
				t.Fatalf("rotate180 %d,%d wrong", x, y)
			}
		}
	}
}

func TestRotate270(t *testing.T) {
	// 3x2 source -> 2x3 result. Source (x,y) -> dst (h-1-y, x), h=2.
	src := distinct(3, 2)
	out := Rotate270(src)
	if b := out.Bounds(); b.Dx() != 2 || b.Dy() != 3 {
		t.Fatalf("rotate270 bounds %v, want 2x3", b)
	}
	for y := 0; y < 2; y++ {
		for x := 0; x < 3; x++ {
			if px(out, 1-y, x) != px(src, x, y) {
				t.Fatalf("rotate270 source %d,%d misplaced", x, y)
			}
		}
	}
}

func TestRotate90ComposesTo360(t *testing.T) {
	// Four 90-degree rotations return to the original.
	src := distinct(4, 3)
	out := Rotate90(Rotate90(Rotate90(Rotate90(src))))
	for y := 0; y < 3; y++ {
		for x := 0; x < 4; x++ {
			if px(out, x, y) != px(src, x, y) {
				t.Fatalf("4x rotate90 changed %d,%d", x, y)
			}
		}
	}
}

func TestCrop(t *testing.T) {
	src := distinct(5, 5)
	out, err := Crop(src, image.Rect(1, 2, 4, 5)) // 3x3
	if err != nil {
		t.Fatal(err)
	}
	if b := out.Bounds(); b.Dx() != 3 || b.Dy() != 3 {
		t.Fatalf("crop bounds %v, want 3x3", b)
	}
	for y := 0; y < 3; y++ {
		for x := 0; x < 3; x++ {
			if px(out, x, y) != px(src, x+1, y+2) {
				t.Fatalf("crop %d,%d = %v, want %v", x, y, px(out, x, y), px(src, x+1, y+2))
			}
		}
	}
}

func TestCropErrors(t *testing.T) {
	src := distinct(4, 4)
	cases := []struct {
		name string
		r    image.Rectangle
	}{
		{"empty", image.Rect(2, 2, 2, 2)},
		{"neg-x", image.Rect(-1, 0, 2, 2)},
		{"neg-y", image.Rect(0, -1, 2, 2)},
		{"over-x", image.Rect(2, 2, 5, 4)},
		{"over-y", image.Rect(0, 0, 4, 5)},
	}
	for _, c := range cases {
		if _, err := Crop(src, c.r); err == nil {
			t.Fatalf("%s: expected error for %v", c.name, c.r)
		}
	}
}

func TestTransformsDoNotMutateInput(t *testing.T) {
	src := distinct(3, 3)
	cp := make([]uint8, len(src.Pix))
	copy(cp, src.Pix)
	_ = FlipHorizontal(src)
	_ = FlipVertical(src)
	_ = Rotate90(src)
	_ = Rotate180(src)
	_ = Rotate270(src)
	_, _ = Crop(src, image.Rect(0, 0, 2, 2))
	for i := range cp {
		if src.Pix[i] != cp[i] {
			t.Fatalf("a transform mutated input at byte %d", i)
		}
	}
}
