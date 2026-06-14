package images

import "image"

// ToRGBA returns img as an *image.RGBA. If img is already an *image.RGBA whose
// bounds start at the origin, it is returned unchanged; otherwise the pixels
// are copied (and, when necessary, colour-converted) into a freshly allocated
// origin-anchored *image.RGBA of the same dimensions.
func ToRGBA(img image.Image) *image.RGBA {
	if rgba, ok := img.(*image.RGBA); ok && rgba.Rect.Min.X == 0 && rgba.Rect.Min.Y == 0 {
		return rgba
	}
	b := img.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw(dst, img, b)
	return dst
}

// draw copies img (over the region b) into dst, anchoring the result at the
// destination origin. It uses image/draw semantics without importing the
// package, to keep the dependency surface minimal.
func draw(dst *image.RGBA, src image.Image, b image.Rectangle) {
	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			r, g, bl, a := src.At(b.Min.X+x, b.Min.Y+y).RGBA()
			i := dst.PixOffset(x, y)
			dst.Pix[i] = uint8(r >> 8)
			dst.Pix[i+1] = uint8(g >> 8)
			dst.Pix[i+2] = uint8(bl >> 8)
			dst.Pix[i+3] = uint8(a >> 8)
		}
	}
}

// newLike returns a freshly allocated *image.RGBA with the same dimensions as
// img, anchored at the origin.
func newLike(img *image.RGBA) *image.RGBA {
	b := img.Bounds()
	return image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
}
