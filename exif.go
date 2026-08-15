package images

import (
	"encoding/binary"
	"fmt"
	"image"
	"io"
	"os"
)

// Orientation is the EXIF image-orientation tag (TIFF tag 0x0112). Its eight
// legal values, 1 through 8, each describe how the stored pixels must be
// transformed to be displayed the right way up. [ExifTranspose] applies the
// matching transform. OrientationNormal (1) is the identity.
type Orientation uint8

// The eight EXIF orientation values, named for the transform each one calls for.
// The comment on each is the geometric operation [ExifTranspose] applies.
const (
	// OrientationNormal (1) needs no transform.
	OrientationNormal Orientation = 1
	// OrientationFlipHorizontal (2) is mirrored left-to-right.
	OrientationFlipHorizontal Orientation = 2
	// OrientationRotate180 (3) is rotated 180 degrees.
	OrientationRotate180 Orientation = 3
	// OrientationFlipVertical (4) is mirrored top-to-bottom.
	OrientationFlipVertical Orientation = 4
	// OrientationTranspose (5) is reflected across the main diagonal.
	OrientationTranspose Orientation = 5
	// OrientationRotate90CW (6) needs a 90-degree clockwise rotation.
	OrientationRotate90CW Orientation = 6
	// OrientationTransverse (7) is reflected across the anti-diagonal.
	OrientationTransverse Orientation = 7
	// OrientationRotate90CCW (8) needs a 90-degree counter-clockwise rotation.
	OrientationRotate90CCW Orientation = 8
)

// ExifTranspose returns a copy of img re-oriented so that it displays the right
// way up, applying the geometric transform that the EXIF orientation o calls
// for. It mirrors Pillow's ImageOps.exif_transpose geometry:
//
//	1 identity          2 flip horizontal    3 rotate 180     4 flip vertical
//	5 transpose         6 rotate 90 CW       7 transverse     8 rotate 90 CCW
//
// Orientations 5-8 swap the width and height. Any value outside 1-8 (including a
// zero Orientation) is treated as OrientationNormal and returns an unrotated
// RGBA copy, matching Pillow's lenient handling of an absent or invalid tag.
func ExifTranspose(img image.Image, o Orientation) *image.RGBA {
	switch o {
	case OrientationFlipHorizontal:
		return FlipHorizontal(img)
	case OrientationRotate180:
		return Rotate180(img)
	case OrientationFlipVertical:
		return FlipVertical(img)
	case OrientationTranspose:
		return Transpose(img)
	case OrientationRotate90CW:
		return Rotate270(img) // Rotate270 rotates 90 degrees clockwise.
	case OrientationTransverse:
		return Transverse(img)
	case OrientationRotate90CCW:
		return Rotate90(img) // Rotate90 rotates 90 degrees counter-clockwise.
	default: // OrientationNormal and any out-of-range value.
		return ToRGBA(img)
	}
}

// OrientationFromExif extracts the EXIF orientation tag from an encoded image's
// bytes (a JPEG APP1/Exif segment, or a bare TIFF stream). It is deliberately
// lenient: when no EXIF block, no orientation tag, or a malformed structure is
// found, it returns OrientationNormal so callers can auto-orient unconditionally.
// A value stored outside the legal 1-8 range is also normalised to
// OrientationNormal.
func OrientationFromExif(data []byte) Orientation {
	tiff, ok := exifTIFF(data)
	if !ok {
		return OrientationNormal
	}
	o, ok := orientationFromTIFF(tiff)
	if !ok || o < 1 || o > 8 {
		return OrientationNormal
	}
	return o
}

// exifTIFF locates the TIFF byte block that carries the EXIF IFD. For a JPEG it
// is the payload of the first APP1 marker whose data begins with "Exif\0\0"; for
// a stream that itself starts with a TIFF header the whole slice is returned.
// The bool is false when no such block is present.
func exifTIFF(data []byte) ([]byte, bool) {
	if isTIFFHeader(data) {
		return data, true
	}
	if len(data) < 2 || data[0] != 0xFF || data[1] != 0xD8 {
		return nil, false // not a JPEG, no EXIF to find.
	}
	// Walk the JPEG marker segments looking for APP1 (0xFFE1).
	i := 2
	for i+4 <= len(data) {
		if data[i] != 0xFF {
			return nil, false // desynchronised marker stream.
		}
		marker := data[i+1]
		if marker == 0xD9 || marker == 0xDA {
			return nil, false // end of image / start of scan: no EXIF.
		}
		segLen := int(binary.BigEndian.Uint16(data[i+2 : i+4]))
		if segLen < 2 || i+2+segLen > len(data) {
			return nil, false // truncated or malformed segment length.
		}
		payload := data[i+4 : i+2+segLen]
		if marker == 0xE1 && len(payload) >= 6 &&
			payload[0] == 'E' && payload[1] == 'x' && payload[2] == 'i' &&
			payload[3] == 'f' && payload[4] == 0 && payload[5] == 0 {
			return payload[6:], true
		}
		i += 2 + segLen
	}
	return nil, false
}

// isTIFFHeader reports whether data begins with a little- or big-endian TIFF
// header ("II"+42 or "MM"+42).
func isTIFFHeader(data []byte) bool {
	if len(data) < 4 {
		return false
	}
	if data[0] == 'I' && data[1] == 'I' && data[2] == 0x2A && data[3] == 0x00 {
		return true
	}
	if data[0] == 'M' && data[1] == 'M' && data[2] == 0x00 && data[3] == 0x2A {
		return true
	}
	return false
}

// orientationFromTIFF parses a TIFF byte block and returns the SHORT value of
// tag 0x0112 in IFD0, if present. The bool is false when the header is invalid,
// the offsets run off the end, or the tag is absent.
func orientationFromTIFF(t []byte) (Orientation, bool) {
	if len(t) < 8 || !isTIFFHeader(t) {
		return 0, false
	}
	var bo binary.ByteOrder = binary.BigEndian
	if t[0] == 'I' {
		bo = binary.LittleEndian
	}
	ifdOff := int(bo.Uint32(t[4:8]))
	if ifdOff < 8 || ifdOff+2 > len(t) {
		return 0, false
	}
	count := int(bo.Uint16(t[ifdOff : ifdOff+2]))
	entry := ifdOff + 2
	for e := 0; e < count; e++ {
		if entry+12 > len(t) {
			return 0, false
		}
		tag := bo.Uint16(t[entry : entry+2])
		if tag == 0x0112 {
			// Orientation is a SHORT; its value sits in the first two bytes of
			// the entry's value field (bytes 8-9 of the 12-byte entry).
			return Orientation(bo.Uint16(t[entry+8 : entry+10])), true
		}
		entry += 12
	}
	return 0, false
}

// DecodeExifTranspose decodes an image from r and returns it already re-oriented
// according to its embedded EXIF orientation tag, as a premultiplied
// *image.RGBA. It is the auto-orienting counterpart of [Decode]: an image with
// no EXIF orientation (or a non-JPEG/TIFF source) is returned unrotated. This is
// the equivalent of Pillow's ImageOps.exif_transpose applied at decode time.
func DecodeExifTranspose(r io.Reader) (*image.RGBA, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("images: decode: %w", err)
	}
	im, err := decodeBytes(data, 0)
	if err != nil {
		return nil, err
	}
	return ExifTranspose(im, OrientationFromExif(data)), nil
}

// LoadExifTranspose reads the image at path and returns it re-oriented per its
// EXIF orientation tag, as a premultiplied *image.RGBA. It is the auto-orienting
// counterpart of [Load].
func LoadExifTranspose(path string) (*image.RGBA, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("images: load: %w", err)
	}
	defer f.Close()
	return DecodeExifTranspose(f)
}
