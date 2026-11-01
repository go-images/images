package images

import (
	"bytes"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"runtime"
	"testing"

	"github.com/go-gfx/gfx/codec"
)

func encPNG(t testing.TB, w, h int) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := png.Encode(&b, image.NewRGBA(image.Rect(0, 0, w, h))); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func encJPEG(t testing.TB, w, h int) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := jpeg.Encode(&b, image.NewRGBA(image.Rect(0, 0, w, h)), nil); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func encGIF(t testing.TB, w, h int) []byte {
	t.Helper()
	p := image.NewPaletted(image.Rect(0, 0, w, h), color.Palette{color.Black, color.White})
	var b bytes.Buffer
	if err := gif.Encode(&b, p, nil); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

// pngBomb is a real 1x1 PNG whose IHDR claims w x h, with the IHDR CRC
// recomputed (crc32 over the chunk type and data, bytes 12..29, stored at 29).
func pngBomb(t testing.TB, w, h uint32) []byte {
	t.Helper()
	b := encPNG(t, 1, 1)
	binary.BigEndian.PutUint32(b[16:], w)
	binary.BigEndian.PutUint32(b[20:], h)
	binary.BigEndian.PutUint32(b[29:], crc32.ChecksumIEEE(b[12:29]))
	return b
}

// jpegBomb is a real 8x8 JPEG whose SOF0 claims w x h.
func jpegBomb(t testing.TB, w, h uint16) []byte {
	t.Helper()
	b := encJPEG(t, 8, 8)
	i := bytes.Index(b, []byte{0xFF, 0xC0})
	if i < 0 {
		t.Fatal("no SOF0")
	}
	binary.BigEndian.PutUint16(b[i+5:], h)
	binary.BigEndian.PutUint16(b[i+7:], w)
	return b
}

func TestDecodeWithCeiling(t *testing.T) {
	cases := []struct {
		name string
		data []byte
		px   int // pixels in the image
	}{
		{"png", encPNG(t, 4, 3), 12},
		{"jpeg", encJPEG(t, 8, 8), 64},
		{"gif (codec path)", encGIF(t, 4, 3), 12},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := DecodeWith(bytes.NewReader(c.data), DecodeOptions{MaxPixels: c.px}); err != nil {
				t.Fatalf("at the ceiling: %v", err)
			}
			_, err := DecodeWith(bytes.NewReader(c.data), DecodeOptions{MaxPixels: c.px - 1})
			if !errors.Is(err, codec.ErrTooLarge) {
				t.Fatalf("one over: err = %v, want ErrTooLarge", err)
			}
			// Zero is the default ceiling: a tiny image passes.
			if _, err := DecodeWith(bytes.NewReader(c.data), DecodeOptions{}); err != nil {
				t.Fatalf("default: %v", err)
			}
		})
	}
}

func TestDecodeDefaultRefusesAboveCeiling(t *testing.T) {
	// 7000x6000 = 42 Mpx, above codec.DefaultMaxPixels; the entry points that
	// take no options apply the default.
	for name, data := range map[string][]byte{
		"png":  pngBomb(t, 7000, 6000),
		"jpeg": jpegBomb(t, 7000, 6000),
	} {
		if _, err := Decode(bytes.NewReader(data)); !errors.Is(err, codec.ErrTooLarge) {
			t.Errorf("%s Decode: err = %v, want ErrTooLarge", name, err)
		}
		if _, err := DecodeBest(bytes.NewReader(data), 0); !errors.Is(err, codec.ErrTooLarge) {
			t.Errorf("%s DecodeBest: err = %v, want ErrTooLarge", name, err)
		}
	}
}

func TestDecodeTruncatedHeader(t *testing.T) {
	data := encPNG(t, 4, 3)[:20] // signature and half an IHDR
	if _, err := Decode(bytes.NewReader(data)); err == nil || errors.Is(err, codec.ErrTooLarge) {
		t.Fatalf("truncated header: err = %v", err)
	}
	if _, _, err := DecodePartial(bytes.NewReader(data)); err == nil || errors.Is(err, codec.ErrTooLarge) {
		t.Fatalf("partial, truncated header: err = %v", err)
	}
	// A whole IHDR and no pixels: the header check passes, the decode fails.
	body := encPNG(t, 4, 3)[:41]
	if _, err := Decode(bytes.NewReader(body)); err == nil || errors.Is(err, codec.ErrTooLarge) {
		t.Fatalf("truncated body: err = %v", err)
	}
}

func TestDecodePartialWithCeiling(t *testing.T) {
	data := encPNG(t, 4, 3)
	if _, _, err := DecodePartialWith(bytes.NewReader(data), DecodeOptions{MaxPixels: 12}); err != nil {
		t.Fatalf("at the ceiling: %v", err)
	}
	_, _, err := DecodePartialWith(bytes.NewReader(data), DecodeOptions{MaxPixels: 11})
	if !errors.Is(err, codec.ErrTooLarge) {
		t.Fatalf("one over: err = %v", err)
	}
	if _, _, err := DecodePartialWith(boomReader{},DecodeOptions{}); err == nil {
		t.Fatal("read error not reported")
	}
	if _, err := DecodeWith(boomReader{},DecodeOptions{}); err == nil {
		t.Fatal("read error not reported")
	}
}

type boomReader struct{}

func (boomReader) Read([]byte) (int, error) { return 0, errors.New("boom") }

// FuzzDecodeWith: whatever the header claims, a small ceiling is never passed.
func FuzzDecodeWith(f *testing.F) {
	f.Add(encPNG(f, 4, 3))
	f.Add(encJPEG(f, 8, 8))
	f.Add(encGIF(f, 4, 3))
	f.Add(pngBomb(f, 16000, 16000))
	f.Fuzz(func(t *testing.T, data []byte) {
		img, err := DecodeWith(bytes.NewReader(data), DecodeOptions{MaxPixels: 1 << 16})
		if err == nil && img.Rect.Dx()*img.Rect.Dy() > 1<<16 {
			t.Fatalf("decoded %v past the ceiling", img.Rect)
		}
	})
}

// allocated runs f and returns the bytes the process allocated meanwhile.
func allocated(f func()) uint64 {
	var a, b runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&a)
	f()
	runtime.ReadMemStats(&b)
	return b.TotalAlloc - a.TotalAlloc
}

// A 1x1 PNG claiming 16000x16000 made the standard decoder allocate about a
// gigabyte before reading a pixel. The header check refuses it first.
func TestHeaderBombAllocatesNothing(t *testing.T) {
	bombs := map[string]func(r *bytes.Reader) error{
		"png Decode": func(r *bytes.Reader) error { _, err := Decode(r); return err },
		"png DecodePartial": func(r *bytes.Reader) error {
			_, _, err := DecodePartial(r)
			return err
		},
	}
	png := pngBomb(t, 16000, 16000)
	for name, f := range bombs {
		var err error
		n := allocated(func() { err = f(bytes.NewReader(png)) })
		if !errors.Is(err, codec.ErrTooLarge) {
			t.Fatalf("%s: err = %v, want ErrTooLarge", name, err)
		}
		t.Logf("%s: allocated %d bytes", name, n)
		if n >= 1<<20 {
			t.Errorf("%s: allocated %d bytes, want < 1 MiB", name, n)
		}
	}
	jp := jpegBomb(t, 16000, 16000)
	var err error
	n := allocated(func() { _, err = Decode(bytes.NewReader(jp)) })
	if !errors.Is(err, codec.ErrTooLarge) || n >= 1<<20 {
		t.Errorf("jpeg bomb: err = %v, allocated %d", err, n)
	}
}
