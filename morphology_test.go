package images

import (
	"image"
	"image/color"
	"testing"
)

// morphNaive is an independent O(radius^2) reference for separable square
// grayscale morphology: for every pixel it takes the per-channel min (erode) or
// max (dilate) over the clamp-to-edge (2*radius+1)^2 neighbourhood.
func morphNaive(src []uint8, w, h, radius int, dilate bool) []uint8 {
	clamp := func(i, n int) int {
		if i < 0 {
			return 0
		}
		if i >= n {
			return n - 1
		}
		return i
	}
	dst := make([]uint8, len(src))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			var acc [3]uint8
			first := true
			for dy := -radius; dy <= radius; dy++ {
				sy := clamp(y+dy, h)
				for dx := -radius; dx <= radius; dx++ {
					sx := clamp(x+dx, w)
					si := (sy*w + sx) * 4
					for c := 0; c < 3; c++ {
						v := src[si+c]
						if first {
							acc[c] = v
						} else if dilate {
							if v > acc[c] {
								acc[c] = v
							}
						} else if v < acc[c] {
							acc[c] = v
						}
					}
					first = false
				}
			}
			di := (y*w + x) * 4
			dst[di], dst[di+1], dst[di+2] = acc[0], acc[1], acc[2]
			dst[di+3] = src[di+3]
		}
	}
	return dst
}

func TestErodeDilateMatchNaive(t *testing.T) {
	const w, h = 17, 13
	src := benchImage(w, h)
	for _, r := range []int{1, 2, 3} {
		ero, err := Erode(src, r)
		if err != nil {
			t.Fatal(err)
		}
		dil, err := Dilate(src, r)
		if err != nil {
			t.Fatal(err)
		}
		wantE := morphNaive(src.Pix, w, h, r, false)
		wantD := morphNaive(src.Pix, w, h, r, true)
		for i := range wantE {
			if ero.Pix[i] != wantE[i] {
				t.Fatalf("erode r=%d byte %d: got %d want %d", r, i, ero.Pix[i], wantE[i])
			}
			if dil.Pix[i] != wantD[i] {
				t.Fatalf("dilate r=%d byte %d: got %d want %d", r, i, dil.Pix[i], wantD[i])
			}
		}
	}
}

func TestDilateGrowsWhiteDot(t *testing.T) {
	// Single white pixel on black: dilation with radius 1 grows it to a 3x3 block.
	src := solid(5, 5, color.RGBA{0, 0, 0, 255})
	src.SetRGBA(2, 2, color.RGBA{255, 255, 255, 255})
	out, err := Dilate(src, 1)
	if err != nil {
		t.Fatal(err)
	}
	for y := 1; y <= 3; y++ {
		for x := 1; x <= 3; x++ {
			if px(out, x, y).R != 255 {
				t.Fatalf("dilate did not fill %d,%d", x, y)
			}
		}
	}
	if px(out, 0, 0).R != 0 {
		t.Fatalf("dilate leaked to corner: %v", px(out, 0, 0))
	}
}

func TestErodeShrinksWhiteBlock(t *testing.T) {
	// 3x3 white block centred on black: erosion radius 1 leaves only the centre.
	src := solid(5, 5, color.RGBA{0, 0, 0, 255})
	for y := 1; y <= 3; y++ {
		for x := 1; x <= 3; x++ {
			src.SetRGBA(x, y, color.RGBA{255, 255, 255, 255})
		}
	}
	out, err := Erode(src, 1)
	if err != nil {
		t.Fatal(err)
	}
	if px(out, 2, 2).R != 255 {
		t.Fatalf("erode removed the centre: %v", px(out, 2, 2))
	}
	if px(out, 1, 1).R != 0 {
		t.Fatalf("erode kept a corner of the block: %v", px(out, 1, 1))
	}
}

func TestOpenRemovesSpeckle(t *testing.T) {
	// A lone white speckle is removed by opening; a solid block survives.
	src := solid(7, 7, color.RGBA{0, 0, 0, 255})
	src.SetRGBA(1, 1, color.RGBA{255, 255, 255, 255}) // speckle
	for y := 3; y <= 5; y++ {
		for x := 3; x <= 5; x++ {
			src.SetRGBA(x, y, color.RGBA{255, 255, 255, 255}) // 3x3 block
		}
	}
	out, err := Open(src, 1)
	if err != nil {
		t.Fatal(err)
	}
	if px(out, 1, 1).R != 0 {
		t.Fatalf("opening did not remove speckle: %v", px(out, 1, 1))
	}
	if px(out, 4, 4).R != 255 {
		t.Fatalf("opening removed the solid block centre: %v", px(out, 4, 4))
	}
}

func TestCloseFillsHole(t *testing.T) {
	// A solid white field with a single black hole: closing fills the hole.
	src := solid(7, 7, color.RGBA{255, 255, 255, 255})
	src.SetRGBA(3, 3, color.RGBA{0, 0, 0, 255})
	out, err := Close(src, 1)
	if err != nil {
		t.Fatal(err)
	}
	if px(out, 3, 3).R != 255 {
		t.Fatalf("closing did not fill hole: %v", px(out, 3, 3))
	}
}

func TestMorphologyBadRadius(t *testing.T) {
	src := solid(3, 3, color.RGBA{1, 2, 3, 4})
	for _, fn := range []struct {
		name string
		f    func(image.Image, int) (*image.RGBA, error)
	}{
		{"erode", Erode}, {"dilate", Dilate}, {"open", Open}, {"close", Close},
	} {
		if _, err := fn.f(src, 0); err == nil {
			t.Fatalf("%s: expected error for radius 0", fn.name)
		}
		if _, err := fn.f(src, -1); err == nil {
			t.Fatalf("%s: expected error for negative radius", fn.name)
		}
	}
}

func TestMorphologyDoesNotMutateInput(t *testing.T) {
	src := benchImage(8, 8)
	cp := make([]uint8, len(src.Pix))
	copy(cp, src.Pix)
	_, _ = Erode(src, 2)
	_, _ = Dilate(src, 2)
	_, _ = Open(src, 2)
	_, _ = Close(src, 2)
	for i := range cp {
		if src.Pix[i] != cp[i] {
			t.Fatalf("a morphology op mutated input at byte %d", i)
		}
	}
}
