package images

import (
	"fmt"
	"image"

	"github.com/go-images/images/internal/kernels"
)

// FlipHorizontal returns a copy of img mirrored left-to-right (column x of the
// width-w source becomes column w-1-x). It matches numpy.fliplr.
func FlipHorizontal(img image.Image) *image.RGBA {
	src := ToRGBA(img)
	b := src.Bounds()
	dst := newLike(src)
	kernels.FlipH(dst.Pix, src.Pix, b.Dx(), b.Dy())
	return dst
}

// FlipVertical returns a copy of img mirrored top-to-bottom (row y of the
// height-h source becomes row h-1-y). It matches numpy.flipud.
func FlipVertical(img image.Image) *image.RGBA {
	src := ToRGBA(img)
	b := src.Bounds()
	dst := newLike(src)
	kernels.FlipV(dst.Pix, src.Pix, b.Dx(), b.Dy())
	return dst
}

// Rotate90 returns img rotated 90 degrees counter-clockwise (matching
// numpy.rot90 with k=1). A w-by-h image becomes h-by-w.
func Rotate90(img image.Image) *image.RGBA {
	src := ToRGBA(img)
	b := src.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, b.Dy(), b.Dx()))
	kernels.Rotate90(dst.Pix, src.Pix, b.Dx(), b.Dy())
	return dst
}

// Rotate180 returns img rotated 180 degrees (numpy.rot90 with k=2). The
// dimensions are unchanged.
func Rotate180(img image.Image) *image.RGBA {
	src := ToRGBA(img)
	b := src.Bounds()
	dst := newLike(src)
	kernels.Rotate180(dst.Pix, src.Pix, b.Dx(), b.Dy())
	return dst
}

// Rotate270 returns img rotated 90 degrees clockwise, i.e. 270 degrees
// counter-clockwise (numpy.rot90 with k=3). A w-by-h image becomes h-by-w.
func Rotate270(img image.Image) *image.RGBA {
	src := ToRGBA(img)
	b := src.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, b.Dy(), b.Dx()))
	kernels.Rotate270(dst.Pix, src.Pix, b.Dx(), b.Dy())
	return dst
}

// Transpose returns img reflected across its main diagonal (top-left to
// bottom-right): pixel (x, y) becomes (y, x). A w-by-h image becomes h-by-w.
// It matches PIL Image.TRANSPOSE and is the geometry of EXIF orientation 5.
func Transpose(img image.Image) *image.RGBA {
	src := ToRGBA(img)
	b := src.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, b.Dy(), b.Dx()))
	kernels.Transpose(dst.Pix, src.Pix, b.Dx(), b.Dy())
	return dst
}

// Transverse returns img reflected across its anti-diagonal (top-right to
// bottom-left): pixel (x, y) becomes (h-1-y, w-1-x). A w-by-h image becomes
// h-by-w. It matches PIL Image.TRANSVERSE and is the geometry of EXIF
// orientation 7.
func Transverse(img image.Image) *image.RGBA {
	src := ToRGBA(img)
	b := src.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, b.Dy(), b.Dx()))
	kernels.Transverse(dst.Pix, src.Pix, b.Dx(), b.Dy())
	return dst
}

// Crop returns the rectangular region r of img as a new image anchored at the
// origin. r is interpreted in the coordinate system of img converted to RGBA
// (origin at the top-left). It returns an error if r is empty or extends outside
// the image bounds.
func Crop(img image.Image, r image.Rectangle) (*image.RGBA, error) {
	src := ToRGBA(img)
	b := src.Bounds()
	if r.Empty() {
		return nil, fmt.Errorf("images: crop: empty rectangle %v", r)
	}
	if r.Min.X < 0 || r.Min.Y < 0 || r.Max.X > b.Dx() || r.Max.Y > b.Dy() {
		return nil, fmt.Errorf("images: crop: rectangle %v outside image bounds %v", r, b)
	}
	cw, ch := r.Dx(), r.Dy()
	dst := image.NewRGBA(image.Rect(0, 0, cw, ch))
	kernels.Crop(dst.Pix, src.Pix, b.Dx(), r.Min.X, r.Min.Y, cw, ch)
	return dst, nil
}
