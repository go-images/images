package images

import (
	"image"

	"github.com/go-gfx/gfx/raster"
)

// AsRGBA returns an *image.RGBA that shares r's pixel buffer without copying.
//
// go-gfx's raster.Image (the shared 2-D substrate this library sits on top of)
// and the standard library's image.RGBA use the identical physical layout: a
// densely packed, origin-anchored, row-major slice of four bytes (R, G, B, A)
// per pixel with stride 4*W and no row padding. The two therefore alias one
// backing array. The only nominal difference is the alpha model — raster.Image
// is straight (non-premultiplied) whereas image.RGBA documents its bytes as
// premultiplied — but every operation in this package treats the bytes as
// straight per-channel values and preserves the alpha channel unchanged, so the
// aliased view is processed exactly as raster's own straight bytes would be.
// Writing through either view is visible through the other.
func AsRGBA(r *raster.Image) *image.RGBA {
	return &image.RGBA{
		Pix:    r.Pix,
		Stride: 4 * r.W,
		Rect:   image.Rect(0, 0, r.W, r.H),
	}
}

// AsRaster returns a raster.Image viewing img's pixels. When img is densely
// packed and origin-anchored (Rect.Min at the origin and Stride == 4*width) the
// returned raster shares img's backing array with no copy, the exact inverse of
// AsRGBA; otherwise — a sub-image, a padded stride, or a non-origin rectangle —
// the pixels are compacted into a freshly allocated, tightly packed buffer so
// the result always satisfies raster.Image's dense, origin-anchored invariant.
func AsRaster(img *image.RGBA) *raster.Image {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if b.Min.X == 0 && b.Min.Y == 0 && img.Stride == 4*w {
		return &raster.Image{Pix: img.Pix[:4*w*h], W: w, H: h}
	}
	dst := raster.New(w, h)
	for y := 0; y < h; y++ {
		so := img.PixOffset(b.Min.X, b.Min.Y+y)
		do := y * w * 4
		copy(dst.Pix[do:do+w*4], img.Pix[so:so+w*4])
	}
	return dst
}

// ResizeRaster scales src to w by h pixels using mode, accepting and returning
// go-gfx's raster.Image so callers already working in the shared substrate do
// not have to round-trip through image.RGBA. It is exactly Resize evaluated over
// an aliased view of src: Resize's freshly allocated destination is itself dense
// and origin-anchored, so both the input and the output are aliased with no
// extra copy, and the resulting pixel bytes are identical to those Resize
// produces for the same source. It returns an error if w or h is not positive,
// or for an unknown mode.
func ResizeRaster(src *raster.Image, w, h int, mode ResizeMode) (*raster.Image, error) {
	out, err := Resize(AsRGBA(src), w, h, mode)
	if err != nil {
		return nil, err
	}
	return AsRaster(out), nil
}
