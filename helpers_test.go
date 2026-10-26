package images

import (
	"image"
	"image/color"
)

// distinct returns a w*h RGBA where each pixel encodes its own coordinates,
// so any rearrangement can be checked exactly: R=x, G=y, B=x+y, A=255. The
// ops package's tests carry the same helper.
func distinct(w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetRGBA(x, y, color.RGBA{uint8(x), uint8(y), uint8(x + y), 255})
		}
	}
	return img
}

func px(img *image.RGBA, x, y int) color.RGBA {
	return img.RGBAAt(x, y)
}
