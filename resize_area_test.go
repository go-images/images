package images

import (
	"image"
	"image/color"
	"math"
	"math/rand"
	"testing"
)

// areaReference is the DEFINITION of an area resize, written directly: for each
// destination pixel, walk the source rectangle it covers and average, weighting
// each source pixel by how much of it falls inside. It is deliberately the
// naive two-dimensional form -- no separation, no precomputed spans -- so it
// shares no code with the implementation it checks.
func areaReference(src *image.RGBA, dstW, dstH int) *image.RGBA {
	b := src.Bounds()
	srcW, srcH := b.Dx(), b.Dy()
	sx := float64(srcW) / float64(dstW)
	sy := float64(srcH) / float64(dstH)
	out := image.NewRGBA(image.Rect(0, 0, dstW, dstH))

	span := func(i int, scale float64, n int) (int, int) {
		lo, hi := float64(i)*scale, float64(i)*scale+scale
		i0, i1 := int(math.Floor(lo)), int(math.Ceil(hi))
		if i0 < 0 {
			i0 = 0
		}
		if i1 > n {
			i1 = n
		}
		if i1 <= i0 {
			i1 = i0 + 1
		}
		return i0, i1
	}
	overlap := func(j int, i int, scale float64) float64 {
		lo, hi := float64(i)*scale, float64(i)*scale+scale
		w := math.Min(hi, float64(j+1)) - math.Max(lo, float64(j))
		if w < 0 {
			return 0
		}
		return w
	}

	for y := 0; y < dstH; y++ {
		y0, y1 := span(y, sy, srcH)
		for x := 0; x < dstW; x++ {
			x0, x1 := span(x, sx, srcW)
			var acc [4]float64
			var sum float64
			for j := y0; j < y1; j++ {
				wy := overlap(j, y, sy)
				for i := x0; i < x1; i++ {
					w := wy * overlap(i, x, sx)
					si := src.PixOffset(b.Min.X+i, b.Min.Y+j)
					acc[0] += float64(src.Pix[si]) * w
					acc[1] += float64(src.Pix[si+1]) * w
					acc[2] += float64(src.Pix[si+2]) * w
					acc[3] += float64(src.Pix[si+3]) * w
					sum += w
				}
			}
			di := out.PixOffset(x, y)
			for c := 0; c < 4; c++ {
				out.Pix[di+c] = uint8(math.Round(math.Max(0, math.Min(255, acc[c]/sum))))
			}
		}
	}
	return out
}

func randomRGBA(w, h int, seed int64) *image.RGBA {
	rng := rand.New(rand.NewSource(seed))
	im := image.NewRGBA(image.Rect(0, 0, w, h))
	for i := range im.Pix {
		im.Pix[i] = uint8(rng.Intn(256))
	}
	return im
}

// The separable implementation must agree with the two-dimensional definition.
// Rounding order differs between them -- the separable form rounds once at the
// end of two float64 passes -- so a one-level difference is allowed and
// anything more is a bug.
func TestResizeAreaMatchesTheDefinition(t *testing.T) {
	for _, tc := range []struct {
		name                   string
		srcW, srcH, dstW, dstH int
	}{
		{"halved", 16, 12, 8, 6},
		{"quartered", 16, 16, 4, 4},
		{"reduced unevenly", 17, 13, 5, 4},
		{"reduced to one pixel", 9, 7, 1, 1},
		{"reduced in x only", 16, 5, 3, 5},
		{"reduced in y only", 5, 16, 5, 3},
		{"enlarged", 4, 3, 12, 9},
		{"enlarged unevenly", 5, 4, 13, 9},
		{"reduced in x, enlarged in y", 16, 3, 4, 9},
		{"unchanged", 7, 6, 7, 6},
	} {
		t.Run(tc.name, func(t *testing.T) {
			src := randomRGBA(tc.srcW, tc.srcH, int64(tc.srcW*31+tc.srcH))
			got, err := Resize(src, tc.dstW, tc.dstH, Area)
			if err != nil {
				t.Fatal(err)
			}
			want := areaReference(src, tc.dstW, tc.dstH)
			for i := range want.Pix {
				if d := int(got.Pix[i]) - int(want.Pix[i]); d > 1 || d < -1 {
					px := i / 4
					t.Fatalf("pixel %d,%d channel %d = %d, the definition says %d",
						px%tc.dstW, px/tc.dstW, i%4, got.Pix[i], want.Pix[i])
				}
			}
		})
	}
}

// At an integer ratio the area average IS the block mean, which is what
// scikit-image's downscale_local_mean computes. Checked on values whose mean is
// exact, so no rounding tolerance is needed to state it.
func TestResizeAreaIsTheBlockMeanAtIntegerRatios(t *testing.T) {
	// 4x4 of four 2x2 blocks, each block holding values whose mean is exact.
	src := image.NewRGBA(image.Rect(0, 0, 4, 4))
	blocks := [][4]uint8{{10, 20, 30, 40}, {100, 100, 100, 100}, {0, 0, 8, 8}, {200, 202, 204, 206}}
	for b, vals := range blocks {
		bx, by := (b%2)*2, (b/2)*2
		for k, v := range vals {
			src.SetRGBA(bx+k%2, by+k/2, color.RGBA{v, v, v, 255})
		}
	}
	out, err := Resize(src, 2, 2, Area)
	if err != nil {
		t.Fatal(err)
	}
	want := []uint8{25, 100, 4, 203}
	for b, w := range want {
		got := px(out, b%2, b/2)
		if got.R != w || got.G != w || got.B != w {
			t.Errorf("block %d = %v, want the mean %d", b, got, w)
		}
		if got.A != 255 {
			t.Errorf("block %d alpha = %d, want 255", b, got.A)
		}
	}
}

// Enlarging puts every destination pixel inside a single source pixel, so the
// area average has nothing to average and must agree with nearest-neighbour.
func TestResizeAreaEnlargingEqualsNearest(t *testing.T) {
	src := randomRGBA(5, 4, 7)
	area, err := Resize(src, 15, 12, Area)
	if err != nil {
		t.Fatal(err)
	}
	near, err := Resize(src, 15, 12, NearestNeighbor)
	if err != nil {
		t.Fatal(err)
	}
	for i := range near.Pix {
		if area.Pix[i] != near.Pix[i] {
			t.Fatalf("byte %d: area %d, nearest %d", i, area.Pix[i], near.Pix[i])
		}
	}
}

// Reducing is the case the mode exists for: nearest-neighbour keeps one pixel
// in sixteen and can miss a feature entirely, while the average cannot.
func TestResizeAreaKeepsWhatNearestThrowsAway(t *testing.T) {
	// White, with one black column that nearest-neighbour steps straight over.
	src := image.NewRGBA(image.Rect(0, 0, 8, 1))
	for x := 0; x < 8; x++ {
		c := color.RGBA{255, 255, 255, 255}
		if x == 1 {
			c = color.RGBA{0, 0, 0, 255}
		}
		src.SetRGBA(x, 0, c)
	}
	near, err := Resize(src, 2, 1, NearestNeighbor)
	if err != nil {
		t.Fatal(err)
	}
	if px(near, 0, 0).R != 255 {
		t.Fatal("the premise is wrong: nearest did not step over the black column")
	}
	area, err := Resize(src, 2, 1, Area)
	if err != nil {
		t.Fatal(err)
	}
	if got := px(area, 0, 0).R; got != 191 {
		t.Errorf("left half = %d, want 191 (three white and one black, averaged)", got)
	}
}
