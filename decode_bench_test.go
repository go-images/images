package images

import (
	"bytes"
	"image/png"
	"testing"
)

// BenchmarkDecodePNG anchors the (unchanged) standard-library PNG decode path so
// a future regression on the hot decode path is visible. The Wave-2 dispatch adds
// only an io.ReadAll and a magic-byte sniff ahead of png.Decode; measured against
// the pre-Wave-2 image.Decode+ToRGBA path the difference is within run-to-run
// noise (~0.3% on a 256x256 gradient).
func BenchmarkDecodePNG(b *testing.B) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, straightGradient(256, 256)); err != nil {
		b.Fatal(err)
	}
	data := buf.Bytes()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := Decode(bytes.NewReader(data)); err != nil {
			b.Fatal(err)
		}
	}
}
