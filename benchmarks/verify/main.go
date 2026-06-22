// Command verify emits go-images outputs for a small fixed image as raw .bin
// files (and the input itself) so verify.py can compare them against
// scikit-image / OpenCV within tolerance. This establishes correctness BEFORE
// the timing numbers are trusted: a fast wrong answer is not parity.
//
// Run from the benchmarks module:
//
//	GOWORK=off go run ./verify <outdir>
package main

import (
	"encoding/binary"
	"fmt"
	"image"
	"math/rand"
	"os"
	"path/filepath"

	img "github.com/go-images/images"
)

const side = 128

func mkimg() *image.RGBA {
	rng := rand.New(rand.NewSource(42))
	im := image.NewRGBA(image.Rect(0, 0, side, side))
	for i := 0; i < len(im.Pix); i += 4 {
		im.Pix[i] = uint8(rng.Intn(256))
		im.Pix[i+1] = uint8(rng.Intn(256))
		im.Pix[i+2] = uint8(rng.Intn(256))
		im.Pix[i+3] = 255
	}
	return im
}

func dump(dir, name string, pix []uint8) {
	if err := os.WriteFile(filepath.Join(dir, name), pix, 0o644); err != nil {
		panic(err)
	}
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: verify <outdir>")
		os.Exit(2)
	}
	dir := os.Args[1]
	if err := os.MkdirAll(dir, 0o755); err != nil {
		panic(err)
	}
	src := mkimg()
	dump(dir, "input.bin", src.Pix)
	// scalar header so Python knows the side length
	hdr := make([]byte, 4)
	binary.LittleEndian.PutUint32(hdr, side)
	dump(dir, "side.bin", hdr)

	box, _ := img.BoxBlur(src, 2)
	dump(dir, "boxblur_r2.bin", box.Pix)

	gauss, _ := img.GaussianBlur(src, 2.0)
	dump(dir, "gaussian_s2.bin", gauss.Pix)

	ero, _ := img.Erode(src, 3)
	dump(dir, "erode_r3.bin", ero.Pix)

	dil, _ := img.Dilate(src, 3)
	dump(dir, "dilate_r3.bin", dil.Pix)

	dump(dir, "fliph.bin", img.FlipHorizontal(src).Pix)
	dump(dir, "rot90.bin", img.Rotate90(src).Pix)
	dump(dir, "hsv.bin", img.RGBToHSV(src).Pix)
	dump(dir, "gray.bin", img.Grayscale(src).Pix)

	otsu := img.OtsuThreshold(src)
	dump(dir, "otsu.bin", []byte{otsu})

	fmt.Println("wrote outputs to", dir)
}
