package images

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

// orientPlace is an independent oracle: it returns the destination coordinate a
// source pixel (x, y) lands on under EXIF orientation o, plus the destination
// dimensions, derived purely from the geometric definition of each orientation
// (flip / rotate / diagonal reflection) — sharing no code with the kernels.
func orientPlace(o Orientation, x, y, w, h int) (dx, dy, dw, dh int) {
	switch o {
	case OrientationFlipHorizontal:
		return w - 1 - x, y, w, h
	case OrientationRotate180:
		return w - 1 - x, h - 1 - y, w, h
	case OrientationFlipVertical:
		return x, h - 1 - y, w, h
	case OrientationTranspose:
		return y, x, h, w
	case OrientationRotate90CW:
		return h - 1 - y, x, h, w
	case OrientationTransverse:
		return h - 1 - y, w - 1 - x, h, w
	case OrientationRotate90CCW:
		return y, w - 1 - x, h, w
	default: // OrientationNormal.
		return x, y, w, h
	}
}

func TestExifTransposeAllOrientations(t *testing.T) {
	const w, h = 3, 2
	src := distinct(w, h)
	for o := Orientation(1); o <= 8; o++ {
		out := ExifTranspose(src, o)
		_, _, dw, dh := orientPlace(o, 0, 0, w, h)
		if b := out.Bounds(); b.Dx() != dw || b.Dy() != dh {
			t.Fatalf("orientation %d: bounds %v, want %dx%d", o, b.Bounds(), dw, dh)
		}
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				dx, dy, _, _ := orientPlace(o, x, y, w, h)
				if px(out, dx, dy) != px(src, x, y) {
					t.Fatalf("orientation %d: source %d,%d landed wrong", o, x, y)
				}
			}
		}
	}
}

func TestExifTransposeInvalidIsNormal(t *testing.T) {
	src := distinct(3, 2)
	for _, o := range []Orientation{0, 9, 200} {
		out := ExifTranspose(src, o)
		if b := out.Bounds(); b.Dx() != 3 || b.Dy() != 2 {
			t.Fatalf("orientation %d: bounds %v, want 3x2", o, b)
		}
		for y := 0; y < 2; y++ {
			for x := 0; x < 3; x++ {
				if px(out, x, y) != px(src, x, y) {
					t.Fatalf("orientation %d: pixel %d,%d changed", o, x, y)
				}
			}
		}
	}
}

// tiffExif builds a minimal TIFF byte block: a header, an IFD0 at offset 8 with
// the given entries, and terminated by a zero next-IFD offset. Each entry is
// (tag, type, count, value) with the value stored inline as a SHORT in its low
// two bytes. When little is true the block is little-endian ("II"), else
// big-endian ("MM").
func tiffExif(little bool, entries [][3]uint16) []byte {
	var bo binary.ByteOrder = binary.BigEndian
	buf := new(bytes.Buffer)
	if little {
		bo = binary.LittleEndian
		buf.WriteString("II")
	} else {
		buf.WriteString("MM")
	}
	put16 := func(v uint16) { b := make([]byte, 2); bo.PutUint16(b, v); buf.Write(b) }
	put32 := func(v uint32) { b := make([]byte, 4); bo.PutUint32(b, v); buf.Write(b) }
	put16(42) // TIFF magic.
	put32(8)  // IFD0 begins right after the 8-byte header.
	put16(uint16(len(entries)))
	for _, e := range entries {
		put16(e[0]) // tag
		put16(e[1]) // type
		put32(1)    // count
		put16(e[2]) // value (SHORT), inline
		put16(0)    // value-field padding
	}
	put32(0) // no next IFD
	return buf.Bytes()
}

// orientationEntries wraps a single orientation tag entry.
func orientationEntries(o uint16) [][3]uint16 {
	return [][3]uint16{{0x0112, 3, o}}
}

// jpegWithExif returns a valid JPEG for img with an APP1 Exif segment carrying
// the given TIFF block spliced in immediately after the SOI marker. Real
// decoders ignore APP1, so the pixels round-trip while OrientationFromExif can
// read the tag.
func jpegWithExif(t *testing.T, img image.Image, tiff []byte) []byte {
	t.Helper()
	var raw bytes.Buffer
	if err := Encode(&raw, img, JPEG); err != nil {
		t.Fatal(err)
	}
	data := raw.Bytes()
	payload := append([]byte("Exif\x00\x00"), tiff...)
	seg := make([]byte, 0, 4+len(payload))
	seg = append(seg, 0xFF, 0xE1)
	segLen := uint16(len(payload) + 2)
	seg = append(seg, byte(segLen>>8), byte(segLen))
	seg = append(seg, payload...)
	// Insert the APP1 segment after the 2-byte SOI.
	out := make([]byte, 0, len(data)+len(seg))
	out = append(out, data[:2]...)
	out = append(out, seg...)
	out = append(out, data[2:]...)
	return out
}

func TestOrientationFromExifJPEG(t *testing.T) {
	img := distinct(4, 2)
	for _, little := range []bool{true, false} {
		for o := uint16(1); o <= 8; o++ {
			data := jpegWithExif(t, img, tiffExif(little, orientationEntries(o)))
			if got := OrientationFromExif(data); got != Orientation(o) {
				t.Fatalf("little=%v o=%d: got %d", little, o, got)
			}
		}
	}
}

func TestOrientationFromExifBareTIFF(t *testing.T) {
	data := tiffExif(false, orientationEntries(6))
	if got := OrientationFromExif(data); got != OrientationRotate90CW {
		t.Fatalf("bare TIFF: got %d, want 6", got)
	}
}

func TestOrientationFromExifSkipsForeignTag(t *testing.T) {
	// A non-orientation tag precedes the orientation tag; the walk must skip it.
	entries := [][3]uint16{{0x011A, 3, 72}, {0x0112, 3, 8}}
	data := tiffExif(true, entries)
	if got := OrientationFromExif(data); got != OrientationRotate90CCW {
		t.Fatalf("got %d, want 8", got)
	}
}

func TestOrientationFromExifOutOfRange(t *testing.T) {
	for _, o := range []uint16{0, 9, 65535} {
		data := tiffExif(true, orientationEntries(o))
		if got := OrientationFromExif(data); got != OrientationNormal {
			t.Fatalf("stored %d: got %d, want Normal", o, got)
		}
	}
}

func TestOrientationFromExifAbsent(t *testing.T) {
	// EXIF present but no orientation tag -> Normal.
	data := tiffExif(true, [][3]uint16{{0x011A, 3, 72}})
	if got := OrientationFromExif(data); got != OrientationNormal {
		t.Fatalf("no orientation tag: got %d", got)
	}
	// A PNG (no EXIF at all) -> Normal.
	var png bytes.Buffer
	if err := Encode(&png, distinct(3, 3), PNG); err != nil {
		t.Fatal(err)
	}
	if got := OrientationFromExif(png.Bytes()); got != OrientationNormal {
		t.Fatalf("PNG: got %d", got)
	}
}

func TestOrientationFromExifMalformed(t *testing.T) {
	good := tiffExif(true, orientationEntries(3))
	cases := map[string][]byte{
		"empty":            {},
		"short":            {0xFF},
		"not-jpeg-or-tiff": {0x01, 0x02, 0x03, 0x04, 0x05},
		"jpeg-desync":      {0xFF, 0xD8, 0x00, 0x00, 0x00, 0x00},
		"jpeg-eoi-first":   {0xFF, 0xD8, 0xFF, 0xD9, 0x00, 0x00},
		"jpeg-sos-first":   {0xFF, 0xD8, 0xFF, 0xDA, 0x00, 0x02},
		"jpeg-bad-seglen":  {0xFF, 0xD8, 0xFF, 0xE1, 0x00, 0x01},
		"jpeg-seg-overrun": {0xFF, 0xD8, 0xFF, 0xE1, 0x7F, 0xFF, 0x00},
		// APP0 (JFIF) then truncation before any APP1: loop runs to the end.
		"jpeg-app0-only": {0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x04, 0x00, 0x00},
		"tiff-truncated": good[:6],
		"tiff-bad-magic": {'I', 'I', 0x00, 0x00, 0x08, 0x00, 0x00, 0x00},
	}
	for name, data := range cases {
		if got := OrientationFromExif(data); got != OrientationNormal {
			t.Fatalf("%s: got %d, want Normal", name, got)
		}
	}
}

func TestOrientationFromExifBadIFDOffset(t *testing.T) {
	// Hand-build a TIFF whose IFD offset points past the end.
	data := []byte{'M', 'M', 0x00, 0x2A, 0x00, 0x00, 0xFF, 0xFF}
	if got := OrientationFromExif(data); got != OrientationNormal {
		t.Fatalf("bad IFD offset: got %d", got)
	}
	// IFD offset < 8 (into the header) is rejected too.
	data2 := []byte{'M', 'M', 0x00, 0x2A, 0x00, 0x00, 0x00, 0x00}
	if got := OrientationFromExif(data2); got != OrientationNormal {
		t.Fatalf("IFD offset in header: got %d", got)
	}
}

func TestOrientationFromExifTruncatedEntry(t *testing.T) {
	// IFD claims two entries but the block is cut off inside the second.
	full := tiffExif(false, [][3]uint16{{0x011A, 3, 72}, {0x0112, 3, 6}})
	// Header(8) + count(2) + one full entry(12) = 22 bytes; cut mid second entry.
	if got := OrientationFromExif(full[:26]); got != OrientationNormal {
		t.Fatalf("truncated entry: got %d, want Normal", got)
	}
}

func TestOrientationFromExifNonExifAPP1(t *testing.T) {
	// An APP1 that is not Exif (e.g. an XMP packet) must be skipped, and a later
	// Exif APP1 still found.
	img := distinct(4, 2)
	data := jpegWithExif(t, img, tiffExif(true, orientationEntries(2)))
	// Splice a non-Exif APP1 right after SOI, before the real one.
	xmp := []byte{0xFF, 0xE1, 0x00, 0x08, 'h', 't', 't', 'p', 0x00, 0x00}
	spliced := append(append(append([]byte{}, data[:2]...), xmp...), data[2:]...)
	if got := OrientationFromExif(spliced); got != OrientationFlipHorizontal {
		t.Fatalf("non-Exif APP1: got %d, want 2", got)
	}
}

func TestDecodeExifTransposeNoExif(t *testing.T) {
	// A PNG has no orientation: DecodeExifTranspose returns it byte-identical.
	src := distinct(4, 3)
	var buf bytes.Buffer
	if err := png.Encode(&buf, src); err != nil {
		t.Fatal(err)
	}
	out, err := DecodeExifTranspose(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if b := out.Bounds(); b.Dx() != 4 || b.Dy() != 3 {
		t.Fatalf("bounds %v, want 4x3", b)
	}
	if !bytes.Equal(out.Pix, src.Pix) {
		t.Fatal("PNG round-trip changed pixels")
	}
}

func TestDecodeExifTransposeAppliesOrientation(t *testing.T) {
	// A JPEG tagged orientation 6 (rotate 90 CW) must come back with its width
	// and height swapped. JPEG is lossy, so only the geometry is asserted here;
	// exact pixels are proven by TestExifTransposeAllOrientations.
	src := distinct(8, 4)
	data := jpegWithExif(t, src, tiffExif(true, orientationEntries(6)))
	out, err := DecodeExifTranspose(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if b := out.Bounds(); b.Dx() != 4 || b.Dy() != 8 {
		t.Fatalf("bounds %v, want 4x8 (swapped)", b)
	}
}

func TestDecodeExifTransposeErrors(t *testing.T) {
	if _, err := DecodeExifTranspose(failReader{}); err == nil {
		t.Fatal("expected read error")
	}
	if _, err := DecodeExifTranspose(bytes.NewReader([]byte("not an image"))); err == nil {
		t.Fatal("expected decode error")
	}
}

func TestLoadExifTranspose(t *testing.T) {
	dir := t.TempDir()
	src := distinct(6, 4)
	data := jpegWithExif(t, src, tiffExif(true, orientationEntries(8)))
	path := filepath.Join(dir, "photo.jpg")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := LoadExifTranspose(path)
	if err != nil {
		t.Fatal(err)
	}
	if b := out.Bounds(); b.Dx() != 4 || b.Dy() != 6 {
		t.Fatalf("bounds %v, want 4x6 (swapped)", b)
	}
	if _, err := LoadExifTranspose(filepath.Join(dir, "missing.jpg")); err == nil {
		t.Fatal("expected error for missing file")
	}
}
