package images

import (
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strings"
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

// Decode reads an image from r, auto-detecting the format among the registered
// decoders (PNG and JPEG). It returns the decoded image converted to *image.RGBA.
func Decode(r io.Reader) (*image.RGBA, error) {
	img, _, err := image.Decode(r)
	if err != nil {
		return nil, fmt.Errorf("images: decode: %w", err)
	}
	return ToRGBA(img), nil
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
