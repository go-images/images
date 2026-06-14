package images

import (
	"bytes"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func sampleImage() *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, 3, 2))
	img.SetRGBA(0, 0, color.RGBA{255, 0, 0, 255})
	img.SetRGBA(1, 0, color.RGBA{0, 255, 0, 255})
	img.SetRGBA(2, 0, color.RGBA{0, 0, 255, 255})
	img.SetRGBA(0, 1, color.RGBA{255, 255, 0, 255})
	img.SetRGBA(1, 1, color.RGBA{0, 255, 255, 255})
	img.SetRGBA(2, 1, color.RGBA{255, 0, 255, 255})
	return img
}

func TestPNGRoundTrip(t *testing.T) {
	src := sampleImage()
	var buf bytes.Buffer
	if err := Encode(&buf, src, PNG); err != nil {
		t.Fatal(err)
	}
	out, err := Decode(&buf)
	if err != nil {
		t.Fatal(err)
	}
	for y := 0; y < 2; y++ {
		for x := 0; x < 3; x++ {
			if out.RGBAAt(x, y) != src.RGBAAt(x, y) {
				t.Fatalf("png round-trip mismatch at %d,%d: %v vs %v", x, y, out.RGBAAt(x, y), src.RGBAAt(x, y))
			}
		}
	}
}

func TestJPEGRoundTrip(t *testing.T) {
	src := sampleImage()
	var buf bytes.Buffer
	if err := Encode(&buf, src, JPEG); err != nil {
		t.Fatal(err)
	}
	out, err := Decode(&buf)
	if err != nil {
		t.Fatal(err)
	}
	// JPEG is lossy; just confirm dimensions survive.
	if out.Bounds().Dx() != 3 || out.Bounds().Dy() != 2 {
		t.Fatalf("jpeg dimensions wrong: %v", out.Bounds())
	}
}

func TestEncodeUnknownFormat(t *testing.T) {
	var buf bytes.Buffer
	if err := Encode(&buf, sampleImage(), Format(99)); err == nil {
		t.Fatal("expected error for unknown format")
	}
}

func TestEncodeWriterError(t *testing.T) {
	if err := Encode(failWriter{}, sampleImage(), PNG); err == nil {
		t.Fatal("expected png encode error on failing writer")
	}
	if err := Encode(failWriter{}, sampleImage(), JPEG); err == nil {
		t.Fatal("expected jpeg encode error on failing writer")
	}
}

type failWriter struct{}

func (failWriter) Write([]byte) (int, error) { return 0, errFail }

var errFail = &fixedError{"write failed"}

type fixedError struct{ msg string }

func (e *fixedError) Error() string { return e.msg }

func TestDecodeBadData(t *testing.T) {
	if _, err := Decode(strings.NewReader("not an image")); err == nil {
		t.Fatal("expected decode error for garbage input")
	}
}

func TestSaveLoadPNG(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.PNG") // upper-case to exercise case folding
	src := sampleImage()
	if err := Save(path, src); err != nil {
		t.Fatal(err)
	}
	out, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if out.RGBAAt(0, 0) != src.RGBAAt(0, 0) {
		t.Fatalf("save/load png mismatch: %v vs %v", out.RGBAAt(0, 0), src.RGBAAt(0, 0))
	}
}

func TestSaveJPEG(t *testing.T) {
	dir := t.TempDir()
	for _, ext := range []string{"a.jpg", "b.jpeg"} {
		path := filepath.Join(dir, ext)
		if err := Save(path, sampleImage()); err != nil {
			t.Fatalf("save %s: %v", ext, err)
		}
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("file %s not written: %v", ext, err)
		}
	}
}

func TestSaveUnsupportedExtension(t *testing.T) {
	dir := t.TempDir()
	if err := Save(filepath.Join(dir, "x.gif"), sampleImage()); err == nil {
		t.Fatal("expected error for unsupported extension")
	}
}

func TestSaveCreateError(t *testing.T) {
	// A path inside a non-existent directory cannot be created.
	bad := filepath.Join(t.TempDir(), "nope", "x.png")
	if err := Save(bad, sampleImage()); err == nil {
		t.Fatal("expected create error")
	}
}

// failWriteCloser fails on Write, recording whether it was closed.
type failWriteCloser struct{ closed bool }

func (w *failWriteCloser) Write([]byte) (int, error) { return 0, errFail }
func (w *failWriteCloser) Close() error              { w.closed = true; return nil }

// failCloser writes successfully but fails on Close.
type failCloser struct{ buf bytes.Buffer }

func (w *failCloser) Write(p []byte) (int, error) { return w.buf.Write(p) }
func (w *failCloser) Close() error                { return errFail }

func TestSaveToEncodeError(t *testing.T) {
	wc := &failWriteCloser{}
	if err := saveTo(wc, sampleImage(), PNG); err == nil {
		t.Fatal("expected encode error")
	}
	if !wc.closed {
		t.Fatal("expected the writer to be closed after an encode error")
	}
}

func TestSaveToCloseError(t *testing.T) {
	if err := saveTo(&failCloser{}, sampleImage(), PNG); err == nil {
		t.Fatal("expected close error")
	}
}

func TestLoadMissingFile(t *testing.T) {
	if _, err := Load(filepath.Join(t.TempDir(), "missing.png")); err == nil {
		t.Fatal("expected error for missing file")
	}
}
