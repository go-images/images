package images

import (
	"image"
	"math"
)

// This file implements arbitrary affine warping and rotation, reproducing
// scikit-image's skimage.transform.warp / rotate for order 0 (nearest) and
// order 1 (bilinear) interpolation. The coordinate and sampling conventions
// match scikit-image exactly: an affine matrix maps output (col, row) to input
// (col, row), bilinear interpolation blends the four surrounding pixels, and an
// out-of-bounds neighbour contributes the constant fill value (mode "constant")
// or the clamped edge pixel (mode "edge"). Because bilinear interpolation is
// linear, sampling the 8-bit channels directly and quantising with
// round-half-to-even yields the same bytes scikit-image produces from its float
// pipeline.

// Interp selects the interpolation order used when sampling between pixels.
type Interp int

const (
	// InterpNearest samples the single nearest pixel (scikit-image order 0).
	InterpNearest Interp = iota
	// InterpBilinear blends the four surrounding pixels (scikit-image order 1).
	InterpBilinear
)

// BorderMode selects how samples that fall outside the input image are filled.
type BorderMode int

const (
	// BorderConstant fills out-of-bounds samples with a constant value
	// (scikit-image mode "constant" with cval).
	BorderConstant BorderMode = iota
	// BorderEdge clamps out-of-bounds coordinates to the nearest edge pixel
	// (scikit-image mode "edge").
	BorderEdge
)

// Affine is a 2-D affine transform. It maps a point (x, y) — with x the column
// and y the row, matching scikit-image's coordinate order — to
//
//	x' = A*x + B*y + C
//	y' = D*x + E*y + F
//
// which is the top two rows of the homogeneous 3x3 matrix whose bottom row is
// (0, 0, 1).
type Affine struct {
	A, B, C, D, E, F float64
}

// Identity is the affine transform that leaves every point unchanged.
func Identity() Affine { return Affine{A: 1, E: 1} }

// Translation returns a transform that shifts a point by (tx, ty).
func Translation(tx, ty float64) Affine { return Affine{A: 1, E: 1, C: tx, F: ty} }

// Scaling returns a transform that scales x by sx and y by sy about the origin.
func Scaling(sx, sy float64) Affine { return Affine{A: sx, E: sy} }

// Rotation returns a transform that rotates a point about the origin by theta
// radians, using scikit-image's convention
// ([[cos, -sin], [sin, cos]] acting on (x, y)).
func Rotation(theta float64) Affine {
	s, c := math.Sincos(theta)
	return Affine{A: c, B: -s, D: s, E: c}
}

// Then returns the composition that applies t first and then u: the result maps
// p to u.Apply(t.Apply(p)).
func (t Affine) Then(u Affine) Affine {
	return Affine{
		A: u.A*t.A + u.B*t.D,
		B: u.A*t.B + u.B*t.E,
		C: u.A*t.C + u.B*t.F + u.C,
		D: u.D*t.A + u.E*t.D,
		E: u.D*t.B + u.E*t.E,
		F: u.D*t.C + u.E*t.F + u.F,
	}
}

// Apply maps the point (x, y) through the transform.
func (t Affine) Apply(x, y float64) (float64, float64) {
	return t.A*x + t.B*y + t.C, t.D*x + t.E*y + t.F
}

// Invert returns the inverse transform. It panics only for a singular matrix
// (zero determinant), which no rotation, translation, or non-degenerate scaling
// produces.
func (t Affine) Invert() Affine {
	det := t.A*t.E - t.B*t.D
	inv := 1 / det
	return Affine{
		A: t.E * inv,
		B: -t.B * inv,
		C: (t.B*t.F - t.C*t.E) * inv,
		D: -t.D * inv,
		E: t.A * inv,
		F: (t.C*t.D - t.A*t.F) * inv,
	}
}

// Warp resamples img through the inverse coordinate map inv onto an
// outW-by-outH output, matching skimage.transform.warp. inv maps an output
// pixel's (col, row) to the input (col, row) to sample, so it is the inverse of
// the geometric transform being applied to the image (exactly as scikit-image's
// warp takes an inverse map). interp selects nearest or bilinear sampling; mode
// and cval select the out-of-bounds behaviour. cval fills the R, G and B
// channels outside the image; the alpha channel is filled with 0 there, so
// out-of-bounds regions are transparent regardless of cval.
func Warp(img image.Image, inv Affine, outW, outH int, interp Interp, mode BorderMode, cval float64) *image.RGBA {
	src := ToRGBA(img)
	b := src.Bounds()
	sw, sh := b.Dx(), b.Dy()
	dst := image.NewRGBA(image.Rect(0, 0, outW, outH))
	for oy := 0; oy < outH; oy++ {
		for ox := 0; ox < outW; ox++ {
			sx, sy := inv.Apply(float64(ox), float64(oy))
			di := dst.PixOffset(ox, oy)
			if interp == InterpNearest {
				sampleNearest(dst.Pix[di:di+4], src.Pix, sw, sh, sx, sy, mode, cval)
			} else {
				sampleBilinear(dst.Pix[di:di+4], src.Pix, sw, sh, sx, sy, mode, cval)
			}
		}
	}
	return dst
}

// Rotate returns img rotated by angle degrees counter-clockwise about its
// centre, reproducing skimage.transform.rotate with bilinear interpolation, the
// "constant" border and a fill value of zero. When resize is false the output
// keeps the input's dimensions (corners may be clipped); when true the output
// grows to contain the whole rotated image, exactly as scikit-image sizes it.
//
// It matches scikit-image's rotate with clip=False: the interpolated result is
// not clamped to the input's value range, so a border pixel fades toward the
// fill value rather than being pinned to the input's global minimum. (Its
// clip=True default clamps every channel to the image-wide [min, max], which
// pins a fade-to-black border up to the minimum — the less useful behaviour.)
func Rotate(img image.Image, angle float64, resize bool) *image.RGBA {
	src := ToRGBA(img)
	b := src.Bounds()
	cols, rows := b.Dx(), b.Dy()
	cx := float64(cols)/2 - 0.5
	cy := float64(rows)/2 - 0.5
	theta := angle * math.Pi / 180

	// tform maps output (col,row) to input (col,row): translate by -centre,
	// rotate, translate back — the same composition scikit-image builds and
	// passes to warp as the inverse map.
	tform := Translation(-cx, -cy).Then(Rotation(theta)).Then(Translation(cx, cy))

	outW, outH := cols, rows
	if resize {
		// Size the output to the bounding box of the rotated input corners,
		// mapped through the forward transform (tform's inverse).
		fwd := tform.Invert()
		minc, minr := math.Inf(1), math.Inf(1)
		maxc, maxr := math.Inf(-1), math.Inf(-1)
		for _, corner := range [4][2]float64{
			{0, 0}, {0, float64(rows - 1)}, {float64(cols - 1), float64(rows - 1)}, {float64(cols - 1), 0},
		} {
			x, y := fwd.Apply(corner[0], corner[1])
			minc, maxc = math.Min(minc, x), math.Max(maxc, x)
			minr, maxr = math.Min(minr, y), math.Max(maxr, y)
		}
		outW = int(math.RoundToEven(maxc - minc + 1))
		outH = int(math.RoundToEven(maxr - minr + 1))
		// Shift the sampling window so the bounding box starts at the origin.
		tform = Translation(minc, minr).Then(tform)
	}
	return Warp(src, tform, outW, outH, InterpBilinear, BorderConstant, 0)
}

// sampleNearest writes the nearest-pixel sample at (sx, sy) into out (4 bytes).
// The coordinate is rounded half away from zero, matching scikit-image's order-0
// interpolation (its Cython path uses the C library round).
func sampleNearest(out []uint8, pix []uint8, w, h int, sx, sy float64, mode BorderMode, cval float64) {
	cx := int(math.Round(sx))
	cy := int(math.Round(sy))
	for ch := 0; ch < 4; ch++ {
		out[ch] = byteClamp(pixelAt(pix, w, h, cx, cy, ch, mode, cval))
	}
}

// sampleBilinear writes the bilinear sample at (sx, sy) into out (4 bytes),
// blending the four surrounding pixels with the standard weights. The channels
// are normalised to [0, 1] before weighting, exactly as scikit-image works in
// its float pipeline (img_as_float, then img_as_ubyte on the way out); doing the
// arithmetic in the same space makes the quantised bytes match it exactly rather
// than differing in the last level from a different rounding position.
func sampleBilinear(out []uint8, pix []uint8, w, h int, sx, sy float64, mode BorderMode, cval float64) {
	x0 := int(math.Floor(sx))
	y0 := int(math.Floor(sy))
	dx := sx - float64(x0)
	dy := sy - float64(y0)
	for ch := 0; ch < 4; ch++ {
		p00 := pixelAt(pix, w, h, x0, y0, ch, mode, cval) / 255
		p01 := pixelAt(pix, w, h, x0+1, y0, ch, mode, cval) / 255
		p10 := pixelAt(pix, w, h, x0, y0+1, ch, mode, cval) / 255
		p11 := pixelAt(pix, w, h, x0+1, y0+1, ch, mode, cval) / 255
		top := p00*(1-dx) + p01*dx
		bot := p10*(1-dx) + p11*dx
		out[ch] = unitByteClamp(top*(1-dy) + bot*dy)
	}
}

// unitByteClamp quantises a channel value in [0, 1] to a byte with
// round-half-to-even, clamping the out-of-gamut ends, matching scikit-image's
// img_as_ubyte (numpy.rint) after its clip.
func unitByteClamp(v float64) uint8 {
	if v <= 0 {
		return 0
	}
	if v >= 1 {
		return 255
	}
	return uint8(math.RoundToEven(v * 255))
}

// pixelAt returns channel ch of the pixel at (x, y) as a float, applying the
// border rule: for BorderConstant an out-of-bounds coordinate yields cval for
// the colour channels and 0 for alpha (channel 3); for BorderEdge the
// coordinate is clamped into range.
func pixelAt(pix []uint8, w, h, x, y, ch int, mode BorderMode, cval float64) float64 {
	if x < 0 || x >= w || y < 0 || y >= h {
		if mode == BorderConstant {
			if ch == 3 {
				return 0
			}
			return cval
		}
		// BorderEdge: clamp into range.
		x = clampInt(x, 0, w-1)
		y = clampInt(y, 0, h-1)
	}
	return float64(pix[(y*w+x)*4+ch])
}

// clampInt clamps v to the inclusive range [lo, hi].
func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// byteClamp quantises a channel value to a byte with round-half-to-even,
// clamping to [0, 255] (matching scikit-image's img_as_ubyte after clipping).
func byteClamp(v float64) uint8 {
	if v <= 0 {
		return 0
	}
	if v >= 255 {
		return 255
	}
	return uint8(math.RoundToEven(v))
}
