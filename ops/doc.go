// Package ops is the codec-free core of github.com/go-images/images: every
// pixel operation (resize, convolution, blurs, edges, morphology, colour,
// geometric transforms, warps, the raster facades) and nothing that decodes
// or encodes a file.
//
// It exists for the linker. A decoder registers itself with
// image.RegisterFormat in an init function, and the Go linker keeps whatever a
// reachable init reaches (cmd/link/internal/ld/deadcode.go starts its flood
// fill at main.main and the init tasks of every imported package). Importing
// package images therefore links every format it can decode — JPEG 2000,
// JBIG2, TIFF, WebP, BMP, ICO, GIF, PNG, JPEG — whether or not the program
// ever reads a file: about 1.7 MB of WebAssembly code for a program that only
// blurs. Importing ops links only the operations the program calls.
//
// Package images forwards every name declared here, under the same name, and
// adds the I/O (Load, Save, Decode, Encode, DecodePartial, the EXIF helpers).
// Use images when you read or write files, ops when you only process pixels.
package ops
