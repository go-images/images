// Copyright (c) 2026, the go-images authors
// All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package ops

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
)

// ⛔ A FOREIGN JUDGE, AND THE REASON FOR ONE. Every other test in skew_test.go
// grades the detector against a number this package chose: a planted angle
// recovered to a twentieth of a degree says the arithmetic is self-consistent,
// and says nothing about whether straightening a page helps anybody. Tesseract
// knows nothing of this code, and reads text — the thing a caller deskews a
// scan in order to get.
//
// It also carries its own POSITIVE CONTROL. If tesseract cannot read the
// straight render, "the deskewed page reads better than the skewed one" is a
// comparison of two kinds of failure, so the straight page is measured first
// and the test stops if the judge cannot see.

const ocrSample = "The quick brown fox jumps over the lazy dog. " +
	"Pack my box with five dozen liquor jugs. " +
	"How vexingly quick daft zebras jump. " +
	"Bright vixens jump dozy fowl quack."

// ocrPage renders the sample as real glyphs, large enough for tesseract to
// read: basicfont is 7x13, which no OCR engine handles, so it is drawn and then
// enlarged with nearest-neighbour, which keeps the strokes crisp.
func ocrPage(t *testing.T) *image.RGBA {
	t.Helper()
	const (
		cols  = 46 // characters per line
		scale = 4
		pad   = 8
	)
	words := strings.Fields(ocrSample)
	var lines []string
	cur := ""
	for _, w := range words {
		if cur != "" && len(cur)+1+len(w) > cols {
			lines, cur = append(lines, cur), w
			continue
		}
		if cur == "" {
			cur = w
		} else {
			cur += " " + w
		}
	}
	if cur != "" {
		lines = append(lines, cur)
	}

	face := basicfont.Face7x13
	lineH := face.Height + 6
	small := image.NewRGBA(image.Rect(0, 0, pad*2+cols*face.Advance, pad*2+len(lines)*lineH))
	for i := range small.Pix {
		small.Pix[i] = 0xff
	}
	d := &font.Drawer{Dst: small, Src: image.NewUniform(color.Black), Face: face}
	for i, line := range lines {
		d.Dot = fixed.P(pad, pad+i*lineH+face.Ascent)
		d.DrawString(line)
	}

	big, err := Resize(small, small.Rect.Dx()*scale, small.Rect.Dy()*scale, NearestNeighbor)
	if err != nil {
		t.Fatalf("enlarging the rendered text: %v", err)
	}
	return big
}

// ocrArtefactDir is a durable directory OUTSIDE every repository, so the pages
// this test judges can still be looked at afterwards.
//
// ⛔ t.TempDir() would be the wrong choice twice over: it is removed when the
// test ends, so an artefact is gone before anyone can open it, and $TMPDIR is
// emptied by a reboot. And a directory inside the work tree is one `git add -A`
// from being published, so the choice is verified rather than trusted: the
// walk up to the filesystem root fails the test if it finds a .git.
func ocrArtefactDir(t *testing.T) string {
	t.Helper()
	base := os.Getenv("GO_IMAGES_TEST_ARTEFACTS")
	if base == "" {
		cfg, err := os.UserConfigDir()
		if err != nil {
			t.Skipf("no durable directory to write the pages to: %v", err)
		}
		base = filepath.Join(cfg, "go-images", "ops-skew")
	}
	abs, err := filepath.Abs(base)
	if err != nil {
		t.Fatal(err)
	}
	for dir := abs; ; {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			t.Fatalf("%s is inside the work tree rooted at %s: a test artefact written there can be committed", abs, dir)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	if err := os.MkdirAll(abs, 0o755); err != nil {
		t.Fatal(err)
	}
	return abs
}

func writePNG(t *testing.T, dir, name string, img image.Image) string {
	t.Helper()
	path := filepath.Join(dir, name)
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
	return path
}

// ocrRead runs tesseract over the page and returns what it read.
func ocrRead(t *testing.T, tess, path string) string {
	t.Helper()
	cmd := exec.Command(tess, path, "stdout", "--psm", "6")
	cmd.Stdin = nil
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("tesseract on %s: %v", path, err)
	}
	return string(out)
}

// similarity is 1 minus the Levenshtein distance over the longer string: 1.0
// for a perfect read, 0 for nothing in common. Case and runs of whitespace are
// normalised away, since neither is what deskewing changes.
func similarity(want, got string) float64 {
	norm := func(s string) string { return strings.ToLower(strings.Join(strings.Fields(s), " ")) }
	a, b := []rune(norm(want)), []rune(norm(got))
	if len(a) == 0 && len(b) == 0 {
		return 1
	}
	prev := make([]int, len(b)+1)
	cur := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		cur[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			cur[j] = min(min(prev[j]+1, cur[j-1]+1), prev[j-1]+cost)
		}
		prev, cur = cur, prev
	}
	longest := max(len(a), len(b))
	return 1 - float64(prev[len(b)])/float64(longest)
}

func TestSkewAgainstTesseract(t *testing.T) {
	tess, err := exec.LookPath("tesseract")
	if err != nil {
		t.Skip("tesseract is not installed, so there is no independent judge here")
	}
	dir := ocrArtefactDir(t)
	t.Logf("pages written to %s", dir)

	straight := ocrPage(t)
	straightRead := similarity(ocrSample, ocrRead(t, tess, writePNG(t, dir, "straight.png", straight)))
	t.Logf("straight page: tesseract reads it at %.3f", straightRead)
	if straightRead < 0.80 {
		t.Skipf("the judge cannot read our straight render (%.3f), so it cannot judge the skewed one either", straightRead)
	}

	const planted = 7.0
	skewed := overWhite(Rotate(straight, planted, true))
	skewedRead := similarity(ocrSample, ocrRead(t, tess, writePNG(t, dir, "skewed.png", skewed)))

	straightened, angle, err := Deskew(skewed, 0.1, SkewOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if off := angle - planted; off > 0.2 || off < -0.2 {
		t.Errorf("planted %+.2f°, measured %+.2f°", planted, angle)
	}
	fixedRead := similarity(ocrSample, ocrRead(t, tess, writePNG(t, dir, "deskewed.png", overWhite(straightened))))

	t.Logf("tilted %+.2f°: %.3f → deskewed by %+.2f°: %.3f (straight %.3f)", planted, skewedRead, -angle, fixedRead, straightRead)
	if fixedRead <= skewedRead {
		t.Errorf("deskewing did not help the judge: %.3f tilted, %.3f deskewed", skewedRead, fixedRead)
	}
	// Deskewing resamples, so it cannot reach the straight page; within a
	// tenth of it is the useful claim.
	if fixedRead < straightRead-0.10 {
		t.Errorf("deskewed page reads at %.3f, too far below the straight page's %.3f", fixedRead, straightRead)
	}
}
