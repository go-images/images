# Implementation plan — pure-Go image processing (`go-images/images`)

> Goal: a **pure-Go (no cgo)** image-processing library in the style of
> scikit-image, built on the Go standard library. It is **embeddable** (no
> native dependency) and **cross-compilable** to every Go target, and is the
> image backend intended for
> [go-embedded-ruby](https://github.com/go-embedded-ruby/ruby).

## 1. Why pure Go?

The mainstream image stacks all require a native C library:

- **ruby-vips** → libvips, **RMagick** → ImageMagick.
- The cgo-backed Go wrappers (`govips`, `go-imagick`) link the same shared
  objects.

That makes static builds, cross-compilation and embedding painful. This library
is **CGO=0**, so a single `go build` produces a self-contained binary for any of
the supported targets with no system packages.

### Honest comparison

Pure-Go image libraries already exist, and we are not pretending otherwise:

- **[bild](https://github.com/anthonynsimon/bild)** — a broad pure-Go toolkit
  (blur, edge detection, morphology, histograms). Mature and feature-rich.
- **[disintegration/imaging](https://github.com/disintegration/imaging)** — a
  focused, well-loved resize/transform/adjust library.
- **libvips / ImageMagick** — far more features and **hand-tuned SIMD**; far
  faster on large images today, but cgo-bound.

Where this project aims to differentiate:

1. **Clean, correct, 100%-covered core** with pure functions and a small,
   stable surface.
2. **Kernels isolated** in `internal/kernels` behind narrow signatures so they
   can be replaced wholesale by **SIMD generated with
   [go-asmgen](https://github.com/go-asmgen)** — the only Go SIMD generator
   covering all six 64-bit targets (amd64, arm64, riscv64, loong64, ppc64le
   VSX, s390x vector) — without touching the public API. This is the path to
   closing the speed gap with libvips/ImageMagick while staying cgo-free.
3. **Embeddability** as the image backend for go-embedded-ruby.

We do **not** claim to be faster than libvips/ImageMagick today, nor more
featureful than bild. The bet is: cgo-free + embeddable + a SIMD story across
six arches.

## 2. Architecture

```
image.Image
   │  ToRGBA  → *image.RGBA (origin-anchored, tightly packed R,G,B,A)
   ▼
public ops (ops.go, io.go)        ← pure funcs, return a new *image.RGBA
   │  delegate hot loops to
   ▼
internal/kernels                  ← scalar pure-Go pixel loops (today)
   │  same signatures
   ▼
internal/kernels (SIMD, later)    ← go-asmgen kernels per target (Phase 2)
```

The RGBA layout (row-major, 4 bytes/pixel) is deliberate: it is exactly the
shape that maps onto vector registers.

## 3. Roadmap

### Phase 0 — core (DONE)

- `ToRGBA` conversion helper.
- I/O: `Load`, `Save` (PNG/JPEG by extension), `Decode`, `Encode`.
- Point ops: `Grayscale` (Rec. 601 luma), `Invert`, `AdjustBrightness`,
  `AdjustContrast` (both clamped to `[0,255]`).
- `Resize` with nearest-neighbour and bilinear modes.
- `Convolve` with arbitrary odd-sized float kernels and clamp-to-edge borders.
- `GaussianBlur` as a separable convolution.
- 100% statement coverage; CI on three OSes + six 64-bit arches (native
  amd64/arm64, qemu riscv64/loong64/ppc64le/s390x).

### Phase 1 — more filters & transforms (in progress)

- **Sobel edge detection (DONE)** — `Sobel` (gradient magnitude),
  `SobelX`/`SobelY` (directional responses), all on Rec. 601 luminance with
  clamp-to-edge borders. The luminance plane is computed once and the
  gradient/magnitude pass is fused with unrolled kernel taps; a benchmark
  compares it to an unfused two-pass baseline and a differential test pins it to
  an independent naive reference.
- **Box blur (DONE)** — `BoxBlur`, separable running-sum (O(1) per pixel in the
  radius), float intermediate so the two passes compose to the exact 2-D mean;
  matches `scipy.ndimage.uniform_filter` (mode=nearest) bit-for-bit. Validated
  against an independent naive oracle and against SciPy on the VM.
- **Geometric transforms (DONE)** — `FlipHorizontal`/`FlipVertical`
  (numpy.fliplr/flipud), `Rotate90`/`Rotate180`/`Rotate270` (numpy.rot90) and
  `Crop`. Exact pixel rearrangements, no interpolation.
- **Colour & threshold (DONE)** — `RGBToHSV`/`HSVToRGB` (byte-encoded,
  round-trip stable), `OtsuThreshold` (matches `skimage.filters.threshold_otsu`)
  and `Threshold`/`Otsu`.
- Sharpen/unsharp-mask, Prewitt/Scharr/Laplacian, Canny, emboss.
- Median and bilateral filters.
- Arbitrary-angle rotate, affine/warp; bicubic resize.
- Colour adjustments: gamma, channel mixing, RGB↔Lab, histogram equalisation.
- Configurable edge handling (clamp / wrap / reflect / constant).

### Phase 2 — morphology & analysis

- **Grayscale morphology (DONE)** — `Erode`/`Dilate` (separable per-channel
  local min/max over a square element, matching `scipy.ndimage.grey_erosion`/
  `grey_dilation` bit-for-bit) and the derived `Open`/`Close`. Binary on 0/255.
- Connected components, labelling, region properties.
- Histograms (`kernels.Histogram` exists) and histogram equalisation.

### Performance

`docs/perf.md` tracks honest go-images-vs-scikit-image/SciPy benchmarks on the
arm64 Linux VM. The scalar pure-Go kernels already beat SciPy on the separable
hot ops (box blur 3.1×, Sobel 3.2×); Gaussian and morphology are ~1.1× behind
NumPy's compiled C and are the first targets for the Phase 3 SIMD work.

### Phase 3 — SIMD kernels via go-asmgen

- Replace `internal/kernels` scalar loops with SIMD across all six 64-bit Go
  targets, keeping the scalar versions as the fallback and the correctness
  oracle (differential tests: SIMD output must equal scalar output).
- Benchmark suite to measure the speed-up and track against libvips/ImageMagick.

### Phase 4 — Ruby binding

- Expose the library through
  [go-embedded-ruby](https://github.com/go-embedded-ruby/ruby) as a cgo-free
  alternative to ruby-vips/RMagick.

## 4. Quality bar (org rules)

- Pure Go, **CGO=0**, no vendoring — build from source on every target.
- **100% statement coverage**, enforced in CI; the test runner exit code must
  be 0 (not just the coverage number).
- English-only repository content.
