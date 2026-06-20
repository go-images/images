package images

import (
	"fmt"
	"image"

	"github.com/go-images/images/internal/kernels"
)

// Erode returns the grayscale morphological erosion of img with a square
// structuring element of the given radius: every output channel is the minimum
// of the (2*radius+1) by (2*radius+1) source neighbourhood. Borders use
// clamp-to-edge addressing; alpha is preserved. The operation is separable
// (matching a square footprint in scipy.ndimage.grey_erosion). On a binary
// image (0/255) this is ordinary binary erosion. It returns an error if radius
// is not positive.
func Erode(img image.Image, radius int) (*image.RGBA, error) {
	return morph1(img, radius, kernels.Erode)
}

// Dilate returns the grayscale morphological dilation of img: the per-channel
// local maximum over a square structuring element. See Erode for borders,
// alpha and the radius rule.
func Dilate(img image.Image, radius int) (*image.RGBA, error) {
	return morph1(img, radius, kernels.Dilate)
}

// Open returns the morphological opening of img (erosion followed by dilation
// with the same square structuring element). Opening removes small bright
// features smaller than the element while preserving overall shape. It returns
// an error if radius is not positive.
func Open(img image.Image, radius int) (*image.RGBA, error) {
	eroded, err := Erode(img, radius)
	if err != nil {
		return nil, err
	}
	return Dilate(eroded, radius)
}

// Close returns the morphological closing of img (dilation followed by erosion).
// Closing fills small dark features smaller than the structuring element. It
// returns an error if radius is not positive.
func Close(img image.Image, radius int) (*image.RGBA, error) {
	dilated, err := Dilate(img, radius)
	if err != nil {
		return nil, err
	}
	return Erode(dilated, radius)
}

// morph1 is the shared body of Erode and Dilate: it validates radius and runs
// the given kernel.
func morph1(img image.Image, radius int, op func(dst, src []uint8, w, h, r int)) (*image.RGBA, error) {
	if radius <= 0 {
		return nil, fmt.Errorf("images: morphology: radius must be positive, got %d", radius)
	}
	src := ToRGBA(img)
	b := src.Bounds()
	dst := newLike(src)
	op(dst.Pix, src.Pix, b.Dx(), b.Dy(), radius)
	return dst, nil
}
