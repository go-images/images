// Copyright 2026 The go-images authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be found
// in the LICENSE file.

package images

import (
	"bytes"
	"errors"
	"image"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-gfx/gfx/codec"
)

// firstDifference is the first row on which two images differ, or -1.
func firstDifference(a, b image.Image, rows int) int {
	for y := 0; y < rows; y++ {
		for x := a.Bounds().Min.X; x < a.Bounds().Max.X; x++ {
			ar, ag, ab, aa := a.At(x, y).RGBA()
			br, bg, bb, ba := b.At(x, y).RGBA()
			if ar != br || ag != bg || ab != bb || aa != ba {
				return y
			}
		}
	}
	return -1
}

// TestEachFormatThatCanAnswerDoes, and the rows survive the conversion.
//
// ⛔ The second half is the claim this file makes that neither fork can: the forks
// hand back a YCbCr, a Gray, a Paletted or an NRGBA, and this package converts every
// one of them to premultiplied RGBA. A count that named the source's rows has to
// name the result's, and the only thing that makes that true is that ToRGBA moves no
// row and drops none. Asserted here against a COMPLETE decode of the same file.
func TestEachFormatThatCanAnswerDoes(t *testing.T) {
	for _, name := range []string{"video-001.png", "video-001.jpeg", "video-001.gif"} {
		t.Run(name, func(t *testing.T) {
			full, err := os.ReadFile("testdata/" + name)
			if err != nil {
				t.Skipf("no such fixture: %v", err)
			}
			whole, err := Decode(bytes.NewReader(full))
			if err != nil {
				t.Fatalf("the complete fixture does not decode, so nothing below "+
					"means anything: %v", err)
			}

			seen := 0
			for _, pct := range []int{30, 60, 90} {
				img, rows, err := DecodePartial(bytes.NewReader(full[:len(full)*pct/100]))
				if err == nil {
					t.Fatalf("%d%% decoded completely, so this cut exercises nothing", pct)
				}
				if img == nil {
					continue
				}
				if rows <= 0 {
					t.Errorf("%d%%: an image came back with %d rows", pct, rows)
					continue
				}
				if img.Bounds() != whole.Bounds() {
					t.Errorf("%d%%: bounds %v, want the full size %v",
						pct, img.Bounds(), whole.Bounds())
				}
				// The conversion claim: these rows, as RGBA, are the rows a
				// complete decode gives.
				if fd := firstDifference(img, whole, rows); fd >= 0 {
					t.Errorf("%d%%: %d rows were offered and row %d already differs "+
						"from the complete decode — the conversion did not carry them",
						pct, rows, fd)
				}
				if rows < seen {
					t.Errorf("%d%%: %d rows, fewer than the %d a smaller cut gave", pct, rows, seen)
				}
				seen = rows
			}
			if seen == 0 {
				t.Errorf("no cut of this file produced a row, so the fixture proves nothing")
			}
		})
	}
}

// TestACompleteFileGivesEveryRow: the count is what distinguishes a whole image
// from a partial one, so it has to be right in the ordinary case too.
func TestACompleteFileGivesEveryRow(t *testing.T) {
	for _, name := range []string{"video-001.png", "video-001.jpeg", "video-001.gif"} {
		t.Run(name, func(t *testing.T) {
			full, err := os.ReadFile("testdata/" + name)
			if err != nil {
				t.Skipf("no such fixture: %v", err)
			}
			img, rows, err := DecodePartial(bytes.NewReader(full))
			if err != nil {
				t.Fatalf("DecodePartial on a whole file: %v", err)
			}
			if rows != img.Bounds().Dy() {
				t.Errorf("rows = %d, want every one of %d", rows, img.Bounds().Dy())
			}
		})
	}
}

// TestAFormatThatCannotAnswerSaysWhichItWas.
//
// ⛔ "No partial decoder" and "not an image at all" are different problems, and a
// caller told only that something failed cannot tell them apart. The message names
// the format, and for WebP it names the reason too — its frame is a VP8 keyframe,
// decoded whole rather than row by row, which is why it could not be forked the way
// the other three were.
func TestAFormatThatCannotAnswerSaysWhichItWas(t *testing.T) {
	// A WebP header, enough for the sniffer and no more: this is about the
	// dispatch, so the body would be doing nothing.
	webp := append([]byte("RIFF"), 0, 0, 0, 0)
	webp = append(webp, []byte("WEBPVP8 ")...)

	_, rows, err := DecodePartial(bytes.NewReader(webp))
	if err == nil {
		t.Fatal("a WebP header decoded")
	}
	if rows != 0 {
		t.Errorf("rows = %d, want 0", rows)
	}
	if !strings.Contains(err.Error(), "WebP") {
		t.Errorf("the error does not name the format: %v", err)
	}
	if !strings.Contains(err.Error(), "VP8") {
		t.Errorf("the error does not say why this format cannot answer: %v", err)
	}

	// And something that is not an image says THAT instead, so the two cases stay
	// distinguishable.
	_, _, err = DecodePartial(strings.NewReader("this is not a picture at all"))
	if err == nil {
		t.Fatal("a sentence decoded as an image")
	}
	if strings.Contains(err.Error(), "no partial decoder for WebP") {
		t.Errorf("a non-image was reported as a known format: %v", err)
	}
	if !strings.Contains(err.Error(), "unrecognised") {
		t.Errorf("the error does not say the format was not recognised: %v", err)
	}
}

// TestNothingIsOfferedBeforeTheFirstRow: a header and no pixels is a size, not a
// picture, and a blank rectangle must not be handed over as though it were content.
func TestNothingIsOfferedBeforeTheFirstRow(t *testing.T) {
	for _, name := range []string{"video-001.png", "video-001.jpeg", "video-001.gif"} {
		full, err := os.ReadFile("testdata/" + name)
		if err != nil {
			continue
		}
		for _, n := range []int{8, 20, 40} {
			img, rows, err := DecodePartial(bytes.NewReader(full[:n]))
			if err == nil {
				t.Errorf("%s: %d bytes decoded completely", name, n)
			}
			if img != nil || rows != 0 {
				t.Errorf("%s: %d bytes offered an image with %d rows", name, n, rows)
			}
		}
	}
}

// errReader fails on the first read, which is what a source that goes away mid
// transfer looks like from here.
type errReader struct{ err error }

func (e errReader) Read([]byte) (int, error) { return 0, e.err }

// TestAReaderThatFailsIsReportedAsSuch: a read error is not a truncated image, and
// answering "no partial decoder" for it would name the wrong problem.
func TestAReaderThatFailsIsReportedAsSuch(t *testing.T) {
	want := errors.New("the pipe went away")
	img, rows, err := DecodePartial(errReader{want})
	if !errors.Is(err, want) {
		t.Errorf("err = %v, want the read error carried through", err)
	}
	if img != nil || rows != 0 {
		t.Errorf("a failed read offered an image with %d rows", rows)
	}
}

// TestDecodePartialFileReadsFromDisk covers the path a caller watching a file grow
// actually takes, and the missing-file case beside it.
func TestDecodePartialFileReadsFromDisk(t *testing.T) {
	full, err := os.ReadFile("testdata/video-001.png")
	if err != nil {
		t.Skipf("no such fixture: %v", err)
	}
	part := filepath.Join(t.TempDir(), "growing.png")
	if err := os.WriteFile(part, full[:len(full)*60/100], 0o644); err != nil {
		t.Fatal(err)
	}
	img, rows, err := DecodePartialFile(part)
	if err == nil {
		t.Fatal("a truncated file decoded completely")
	}
	if img == nil || rows <= 0 {
		t.Fatalf("nothing came back from a file holding 60%% of a picture (rows=%d, err=%v)", rows, err)
	}

	if _, _, err := DecodePartialFile(filepath.Join(t.TempDir(), "absent.png")); err == nil {
		t.Error("a file that is not there decoded")
	}
}

// TestEveryFormatHasAName.
//
// ⛔ Driven against formatName directly rather than through crafted headers for all
// eight. One format is proven end to end — the WebP case above goes through the
// sniffer — and that is what shows the wiring; this covers the vocabulary, where a
// missing case would print a number to somebody trying to understand a refusal.
func TestEveryFormatHasAName(t *testing.T) {
	for _, c := range []struct {
		f    codec.Format
		want string
	}{
		{codec.PNG, "PNG"},
		{codec.JPEG, "JPEG"},
		{codec.GIF, "GIF"},
		{codec.WEBP, "WebP"},
		{codec.TIFF, "TIFF"},
		{codec.BMP, "BMP"},
		{codec.ICO, "ICO"},
		{codec.ICNS, "ICNS"},
		{codec.Format(-1), "unrecognised"},
	} {
		if got := formatName(c.f); !strings.Contains(got, c.want) {
			t.Errorf("formatName(%v) = %q, want it to mention %q", c.f, got, c.want)
		}
	}
}
