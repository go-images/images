package images

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-gfx/gfx/codec"
)

// Format identifies an encodable image format.
type Format int

const (
	// PNG is the lossless PNG format.
	PNG Format = iota
	// JPEG is the lossy JPEG format, encoded at the package's default quality.
	JPEG
)

// jpegQuality is the quality used when encoding JPEG output.
const jpegQuality = 90

// Decode reads an image from r, auto-detecting the container format from its
// magic bytes, and returns it converted to *image.RGBA.
//
// Eight formats are recognised. PNG and JPEG are decoded by the standard library
// exactly as before — this path is byte-for-byte unchanged. The six additional
// formats (GIF, WebP, TIFF, BMP, ICO, ICNS) are delegated to the shared reference
// registry github.com/go-gfx/gfx/codec, which reimplements no decoder and hands
// each container to a battle-tested pure-Go (CGO-free) library. For the
// multi-representation containers (ICO, ICNS) the largest representation is
// returned; use [DecodeBest] to target a pixel size.
//
// Alpha convention: every source is brought into this package under the same
// premultiplied-at-input convention that [ToRGBA] already applies to any
// non-*image.RGBA source. codec returns straight (non-premultiplied) alpha, so
// its output is routed back through [ToRGBA] to premultiply, keeping the whole
// library's decoded bytes consistent regardless of container format.
func Decode(r io.Reader) (*image.RGBA, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("images: decode: %w", err)
	}
	return decodeBytes(data, 0)
}

// DecodeBest is [Decode] with a target pixel size for the multi-representation
// containers (ICO, ICNS): among their stored representations it selects the one
// whose longer side is the smallest that is still at least targetSize, falling
// back to the largest when none reaches it. A targetSize <= 0 selects the
// largest, making it identical to [Decode]. targetSize is ignored for
// single-image formats. The result is a premultiplied *image.RGBA, exactly as
// from [Decode].
func DecodeBest(r io.Reader, targetSize int) (*image.RGBA, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("images: decode: %w", err)
	}
	return decodeBytes(data, targetSize)
}

// decodeBytes dispatches an already-buffered image on its sniffed format. PNG and
// JPEG keep the historical standard-library path unchanged (byte-identical); all
// other recognised formats go through the go-gfx codec registry and are
// premultiplied to match this package's input convention.
func decodeBytes(data []byte, targetSize int) (*image.RGBA, error) {
	switch codec.Sniff(data) {
	case codec.PNG:
		return decodeStd(png.Decode, data)
	case codec.JPEG:
		return decodeStd(jpeg.Decode, data)
	default:
		return decodeViaCodec(data, targetSize)
	}
}

// decodeStd runs a standard-library single-image decoder over data and converts
// the result with [ToRGBA]. This reproduces the pre-existing
// image.Decode+ToRGBA behaviour for PNG and JPEG exactly: image.Decode dispatches
// a matched PNG/JPEG signature to precisely png.Decode / jpeg.Decode, and ToRGBA
// is applied identically — so the returned pixels are byte-for-byte unchanged.
func decodeStd(dec func(io.Reader) (image.Image, error), data []byte) (*image.RGBA, error) {
	img, err := dec(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("images: decode: %w", err)
	}
	return ToRGBA(img), nil
}

// decodeViaCodec decodes the six non-PNG/JPEG formats through the go-gfx codec
// registry. codec yields a straight-alpha raster whose byte layout is identical
// to image.NRGBA (dense, origin-anchored, straight R,G,B,A). Viewing it as an
// NRGBA and running it through [ToRGBA] premultiplies it exactly as ToRGBA
// premultiplies any NRGBA source, so a decoded transparent pixel obeys the same
// convention as a transparent PNG decoded by the standard-library path.
func decodeViaCodec(data []byte, targetSize int) (*image.RGBA, error) {
	rimg, err := codec.DecodeBest(data, targetSize)
	if err != nil {
		return nil, fmt.Errorf("images: decode: %w", err)
	}
	nrgba := &image.NRGBA{
		Pix:    rimg.Pix,
		Stride: 4 * rimg.W,
		Rect:   image.Rect(0, 0, rimg.W, rimg.H),
	}
	return ToRGBA(nrgba), nil
}

// Encode writes img to w in the given format.
func Encode(w io.Writer, img image.Image, format Format) error {
	switch format {
	case PNG:
		if err := png.Encode(w, img); err != nil {
			return fmt.Errorf("images: encode png: %w", err)
		}
	case JPEG:
		if err := jpeg.Encode(w, img, &jpeg.Options{Quality: jpegQuality}); err != nil {
			return fmt.Errorf("images: encode jpeg: %w", err)
		}
	default:
		return fmt.Errorf("images: encode: unknown format %d", format)
	}
	return nil
}

// Load reads and decodes the image at path, returning it as *image.RGBA. The
// format is auto-detected from the file contents.
func Load(path string) (*image.RGBA, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("images: load: %w", err)
	}
	defer f.Close()
	return Decode(f)
}

// Save encodes img and writes it to path, choosing the format from the file
// extension: ".png" for PNG and ".jpg" or ".jpeg" for JPEG (case-insensitive).
// It returns an error for any other extension.
func Save(path string, img image.Image) error {
	format, err := formatFromExt(path)
	if err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("images: save: %w", err)
	}
	return saveTo(f, img, format)
}

// saveTo encodes img to wc in the given format and closes wc, returning the
// first error encountered. It is factored out of Save so the encode-failure and
// close-failure paths can be exercised independently of the filesystem.
func saveTo(wc io.WriteCloser, img image.Image, format Format) error {
	if err := Encode(wc, img, format); err != nil {
		wc.Close()
		return err
	}
	if err := wc.Close(); err != nil {
		return fmt.Errorf("images: save: %w", err)
	}
	return nil
}

// formatFromExt maps a file path's extension to a Format.
func formatFromExt(path string) (Format, error) {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".png":
		return PNG, nil
	case ".jpg", ".jpeg":
		return JPEG, nil
	default:
		return 0, fmt.Errorf("images: save: unsupported file extension %q", filepath.Ext(path))
	}
}
