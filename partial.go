// Copyright 2026 The go-images authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be found
// in the LICENSE file.

package images

import (
	"bytes"
	"fmt"
	"image"
	"io"
	"os"

	"github.com/go-gfx/gfx/codec"

	partialgif "github.com/go-images/gif"
	partialjpeg "github.com/go-images/jpeg"
	partialpng "github.com/go-images/png"
)

// DecodePartial reads an image from r that may still be arriving, and returns what
// of it is real: the picture at its full declared size, and the number of pixel
// rows that are COMPLETE counting from the top. Rows past the count hold whatever
// the image was allocated with, so a caller draws the first rows and nothing else.
//
// The error that stopped the decode comes back with them, and is nil only when the
// whole image arrived — in which case the count is every row.
//
// ⛔ Every standard decoder is all-or-nothing, which for a file still arriving is
// the same as having nothing. Measured on a real photograph truncated at three
// fractions, image/jpeg, image/png and image/gif each returned a nil image and an
// error EVERY time, with three quarters of the picture on disk. This is why the
// three forks behind this call exist.
//
// Three of the eight formats [Decode] recognises can answer, and each refuses some
// shapes of its own — an interlaced PNG or GIF, a progressive or CMYK JPEG — for
// reasons written where the refusal is made:
//
//   - PNG, by github.com/go-images/png
//   - JPEG, by github.com/go-images/jpeg
//   - GIF, by github.com/go-images/gif (its first frame)
//
// The other five (WebP, TIFF, BMP, ICO, ICNS) report that they cannot. WebP is the
// one that could not simply be forked the same way: its frame is a VP8 keyframe,
// decoded as a whole rather than row by row, so there is no partial state to hand
// back.
//
// ⛔ One divergence from [Decode], stated rather than hidden. On a COMPLETE file
// this returns the FORK's decode, and for a four-component JPEG that differs: the
// fork upsamples chroma the way libjpeg does, where the standard library repeats
// each sample. See github.com/go-images/jpeg. Decode's standard-library path is
// untouched, so anything relying on its exact bytes still gets them.
func DecodePartial(r io.Reader) (*image.RGBA, int, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, 0, fmt.Errorf("images: decode partial: %w", err)
	}
	return decodePartialBytes(data)
}

// DecodePartialFile is [DecodePartial] on a path, for the common case of watching
// a file grow on disk.
func DecodePartialFile(path string) (*image.RGBA, int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, 0, fmt.Errorf("images: decode partial: %w", err)
	}
	return decodePartialBytes(data)
}

func decodePartialBytes(data []byte) (*image.RGBA, int, error) {
	switch f := codec.Sniff(data); f {
	case codec.PNG:
		return partial(partialpng.DecodePartial, data)
	case codec.JPEG:
		return partial(partialjpeg.DecodePartial, data)
	case codec.GIF:
		return partial(partialgif.DecodePartial, data)
	default:
		// ⛔ Named, so a caller is told which format could not answer rather than
		// being left to guess from a nil. A format nobody recognised and a format
		// with no partial decoder are different problems.
		return nil, 0, fmt.Errorf("images: decode partial: no partial decoder for %s", formatName(f))
	}
}

// partial runs one of the forks and brings its result into this package's
// convention, exactly as decodeStd does for a complete image.
//
// ⛔ The row count survives the conversion because ToRGBA is a per-pixel transform
// over the same rectangle: it moves no row and drops none, so a count that named
// the source's rows names the result's.
func partial(dec func(io.Reader) (image.Image, int, error), data []byte) (*image.RGBA, int, error) {
	img, rows, err := dec(bytes.NewReader(data))
	if img == nil {
		return nil, 0, fmt.Errorf("images: decode partial: %w", err)
	}
	return ToRGBA(img), rows, err
}

// formatName is what to call a format in a message. codec.Format has no String
// method that says "unknown" usefully for this purpose.
func formatName(f codec.Format) string {
	switch f {
	case codec.PNG:
		return "PNG"
	case codec.JPEG:
		return "JPEG"
	case codec.GIF:
		return "GIF"
	case codec.WEBP:
		return "WebP (its frame is a VP8 keyframe, decoded whole rather than row by row)"
	case codec.TIFF:
		return "TIFF"
	case codec.BMP:
		return "BMP"
	case codec.ICO:
		return "ICO"
	case codec.ICNS:
		return "ICNS"
	}
	return "an unrecognised format"
}
