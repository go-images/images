# Performance parity — go-images vs scikit-image / OpenCV

Pure-Go (CGO=0) image processing measured against the C/SIMD reference stack on
the same machine. scikit-image (+ scipy.ndimage) is the **primary** reference;
OpenCV (`cv2`, hand-tuned C++/SIMD) is a second, deliberately *tougher* bar shown
where it has the op.

## Methodology

| | |
|---|---|
| CPU | Apple M4 Max (16 cores), macOS 26.5 (arm64) |
| Go | go1.26.4, `go test -bench -benchmem` |
| Python | 3.14.5 · scikit-image 0.26.0 · scipy 1.18.0 · numpy 2.5.0 · OpenCV 4.13.0 |
| Threading (core comparison) | **single-thread**: Go `-cpu=1` (GOMAXPROCS=1 collapses the parallel tiling to serial); Python pinned via `threadpool_limits(1)`, `OMP/OPENBLAS/MKL/VECLIB_NUM_THREADS=1`, `cv2.setNumThreads(1)` |
| Sizes | 512², 1024², 4096² — grayscale (uint8) and RGB (uint8) as appropriate |
| Iterations | warm-up + best-of over `-count=5` (Go) / ≥5 iters & ≥0.5 s (Python); metric = **best** ns/op and **best** Mpix/s |
| Correctness gate | every op verified against scikit-image/scipy within tolerance **before** timing (see `benchmarks/verify.py`) — BoxBlur exact, Gaussian ≤1 level, morphology exact, Grayscale ≤1 level, Otsu exact |

Mpix/s = (side² pixels) / (best ns/op) × 10³. Higher is better. Ratio = go-images
Mpix/s ÷ scikit-image Mpix/s (> 1 means go-images is faster). Reproduce with
`benchmarks/` (see below).

> **Fairness notes.** A few reference "ops" are not apples-to-apples and are
> flagged in the table: scikit-image **Crop** is a bare numpy slice + copy (≈
> `memcpy`, no per-pixel work), **Rotate90** is `np.rot90().copy()` (stride
> trick + copy), and **Otsu** on the Python side is only a 256-bin histogram
> reduction. go-images does the full RGBA round-trip in each case. They are
> retained for completeness but are memory-bandwidth / allocator races, not
> compute parity.

## Parity table — single thread (core comparison)

| op | size | go-images | scikit-image | OpenCV | go/skimage | verdict |
|----|------|-----------|--------------|--------|-----------|---------|
| Box blur r=2 (RGB) | 512² | 157 Mpix/s | 59.5 Mpix/s | 2,322 Mpix/s | **2.64×** | go faster |
| Box blur r=2 (RGB) | 1024² | 91.5 Mpix/s | 51.2 Mpix/s | 2,281 Mpix/s | **1.78×** | go faster |
| Box blur r=2 (RGB) | 4096² | 71.6 Mpix/s | 31.2 Mpix/s | 2,128 Mpix/s | **2.29×** | go faster |
| Gaussian σ=2 (RGB) | 512² | 35.8 Mpix/s | 35.1 Mpix/s | 217 Mpix/s | **1.02×** | ~parity |
| Gaussian σ=2 (RGB) | 1024² | 38.6 Mpix/s | 30.7 Mpix/s | 228 Mpix/s | **1.26×** | go faster |
| Gaussian σ=2 (RGB) | 4096² | 30.5 Mpix/s | 23.5 Mpix/s | 236 Mpix/s | **1.30×** | go faster |
| Sobel edge (gray) | 512² | 94.2 Mpix/s | 121 Mpix/s | 901 Mpix/s | **0.78×** | skimage faster |
| Sobel edge (gray) | 1024² | 93.3 Mpix/s | 119 Mpix/s | 937 Mpix/s | **0.78×** | skimage faster |
| Sobel edge (gray) | 4096² | 94.9 Mpix/s | 119 Mpix/s | 914 Mpix/s | **0.80×** | skimage faster |
| Erode r=3 (gray) | 512² | 51.4 Mpix/s | 88.5 Mpix/s | 10,810 Mpix/s | **0.58×** | skimage faster |
| Erode r=3 (gray) | 1024² | 52.9 Mpix/s | 75.9 Mpix/s | 11,782 Mpix/s | **0.70×** | skimage faster |
| Erode r=3 (gray) | 4096² | 45.0 Mpix/s | 58.5 Mpix/s | 11,046 Mpix/s | **0.77×** | skimage faster |
| Dilate r=3 (gray) | 512² | 51.3 Mpix/s | 83.9 Mpix/s | 11,038 Mpix/s | **0.61×** | skimage faster |
| Dilate r=3 (gray) | 1024² | 52.7 Mpix/s | 74.7 Mpix/s | 11,434 Mpix/s | **0.71×** | skimage faster |
| Dilate r=3 (gray) | 4096² | 45.1 Mpix/s | 59.9 Mpix/s | 10,937 Mpix/s | **0.75×** | skimage faster |
| Open r=3 (gray) | 512² | 25.6 Mpix/s | 48.8 Mpix/s | 5,490 Mpix/s | **0.53×** | skimage faster |
| Open r=3 (gray) | 1024² | 26.6 Mpix/s | 43.1 Mpix/s | 5,674 Mpix/s | **0.62×** | skimage faster |
| Open r=3 (gray) | 4096² | 22.6 Mpix/s | 32.9 Mpix/s | 5,304 Mpix/s | **0.69×** | skimage faster |
| Close r=3 (gray) | 512² | 25.9 Mpix/s | 49.6 Mpix/s | 5,424 Mpix/s | **0.52×** | skimage faster |
| Close r=3 (gray) | 1024² | 26.7 Mpix/s | 44.2 Mpix/s | 5,694 Mpix/s | **0.60×** | skimage faster |
| Close r=3 (gray) | 4096² | 22.7 Mpix/s | 33.1 Mpix/s | 5,472 Mpix/s | **0.68×** | skimage faster |
| Flip horizontal (RGB) | 512² | 806 Mpix/s | 401 Mpix/s | 3,421 Mpix/s | **2.01×** | go faster |
| Flip horizontal (RGB) | 1024² | 816 Mpix/s | 400 Mpix/s | 3,270 Mpix/s | **2.04×** | go faster |
| Flip horizontal (RGB) | 4096² | 870 Mpix/s | 381 Mpix/s | 2,889 Mpix/s | **2.28×** | go faster |
| Rotate 90 (RGB) † | 512² | 149 Mpix/s | 398 Mpix/s | 2,081 Mpix/s | **0.37×** | skimage faster |
| Rotate 90 (RGB) † | 1024² | 138 Mpix/s | 398 Mpix/s | 1,960 Mpix/s | **0.35×** | skimage faster |
| Rotate 90 (RGB) † | 4096² | 104 Mpix/s | 195 Mpix/s | 765 Mpix/s | **0.53×** | skimage faster |
| Crop half (RGB) † | 512² | 15,736 Mpix/s | 54,716 Mpix/s | — | **0.29×** | skimage faster |
| Crop half (RGB) † | 1024² | 23,358 Mpix/s | 69,905 Mpix/s | — | **0.33×** | skimage faster |
| Crop half (RGB) † | 4096² | 22,323 Mpix/s | 94,988 Mpix/s | — | **0.24×** | skimage faster |
| RGB→HSV | 512² | 65.6 Mpix/s | 13.7 Mpix/s | 1,554 Mpix/s | **4.81×** | go faster |
| RGB→HSV | 1024² | 64.5 Mpix/s | 13.7 Mpix/s | 1,557 Mpix/s | **4.71×** | go faster |
| RGB→HSV | 4096² | 65.0 Mpix/s | 13.7 Mpix/s | 1,537 Mpix/s | **4.76×** | go faster |
| Otsu threshold † | 512² | 298 Mpix/s | 811 Mpix/s | 4,412 Mpix/s | **0.37×** | skimage faster |
| Otsu threshold † | 1024² | 354 Mpix/s | 830 Mpix/s | 4,552 Mpix/s | **0.43×** | skimage faster |
| Otsu threshold † | 4096² | 358 Mpix/s | 764 Mpix/s | 4,626 Mpix/s | **0.47×** | skimage faster |
| Grayscale | 512² | 653 Mpix/s | 599 Mpix/s | 9,348 Mpix/s | **1.09×** | ~parity |
| Grayscale | 1024² | 502 Mpix/s | 585 Mpix/s | 9,198 Mpix/s | **0.86×** | ~parity |
| Grayscale | 4096² | 687 Mpix/s | 537 Mpix/s | 9,502 Mpix/s | **1.28×** | go faster |

† reference op is a numpy view/copy or histogram, not per-pixel compute — see fairness note.

## Parity table — multi-thread (go-images all 16 cores)

go-images fans the separable passes across cores above a 16 k-pixel threshold;
scikit-image/scipy stay single-threaded for these ops, OpenCV uses its own pool.
This row shows the durable advantage of a parallel pure-Go library on large
images.

| op | size | go-images (all cores) | scikit-image | OpenCV | go/skimage |
|----|------|----------------------|--------------|--------|-----------|
| Box blur r=2 | 1024² | 390 Mpix/s | 50.8 Mpix/s | 2,248 Mpix/s | **7.67×** |
| Box blur r=2 | 4096² | 302 Mpix/s | 29.3 Mpix/s | 2,096 Mpix/s | **10.33×** |
| Gaussian σ=2 | 1024² | 140 Mpix/s | 31.9 Mpix/s | 231 Mpix/s | **4.41×** |
| Gaussian σ=2 | 4096² | 140 Mpix/s | 23.3 Mpix/s | 235 Mpix/s | **6.03×** |
| Sobel | 1024² | 295 Mpix/s | 123 Mpix/s | 978 Mpix/s | **2.41×** |
| Sobel | 4096² | 338 Mpix/s | 118 Mpix/s | 919 Mpix/s | **2.87×** |
| Erode r=3 | 1024² | 216 Mpix/s | 75.5 Mpix/s | 12,081 Mpix/s | **2.86×** |
| Erode r=3 | 4096² | 268 Mpix/s | 58.1 Mpix/s | 10,844 Mpix/s | **4.62×** |
| Dilate r=3 | 1024² | 243 Mpix/s | 76.9 Mpix/s | 11,465 Mpix/s | **3.16×** |
| Dilate r=3 | 4096² | 273 Mpix/s | 56.6 Mpix/s | 10,898 Mpix/s | **4.82×** |
| Open r=3 | 1024² | 131 Mpix/s | 45.6 Mpix/s | 6,265 Mpix/s | **2.86×** |
| Open r=3 | 4096² | 126 Mpix/s | 34.3 Mpix/s | 5,176 Mpix/s | **3.66×** |
| Close r=3 | 1024² | 97.2 Mpix/s | 44.7 Mpix/s | 5,764 Mpix/s | **2.17×** |
| Close r=3 | 4096² | 80.6 Mpix/s | 33.8 Mpix/s | 4,572 Mpix/s | **2.39×** |

With all 16 cores go-images is faster than single-threaded scikit-image on **every**
op (morphology included, 2.2–4.8×). But OpenCV's morphology — single-threaded
O(1) van-Herk min/max + SIMD — is still **40–160× ahead** even of go-images on all
cores, which is the algorithmic evidence for action item **A** below: throwing
cores at an O(radius) kernel cannot substitute for the O(1) algorithm.

## Summary

**Where go-images already wins (single thread):**

- **Box blur** — 1.8–2.6× faster than scikit-image. Root cause: an **O(1)-in-radius
  separable moving-window sum** (`BoxBlur` in `internal/kernels/kernels.go`); scipy's
  `uniform_filter` is also separable but its per-pixel cost is heavier in float and
  it doesn't beat a tight running sum.
- **Gaussian σ=2 — now at parity → 1.3× faster** (was the known ~1.1× *slower* gap).
  The separable convolution was rewritten to deinterleave R/G/B into contiguous
  float planes and run each tap as a SIMD `axpy` (`ConvolveSeparable` + `simd_*.s`),
  and go-images truncates at 3σ vs scipy's 4σ (fewer taps). The gap is closed.
- **RGB→HSV — 4.7–4.8× faster.** scikit-image's `rgb2hsv` is a chain of numpy
  temporaries over the whole array; go-images does it in one fused per-pixel pass.
- **Flip horizontal — ~2×.** Tight byte-reversal vs `np.fliplr().copy()`.
- **Grayscale — ~parity** (0.86–1.28×), single fused luma pass.

**Where go-images lags (the real compute gaps):**

1. **Morphology (erode/dilate/open/close) — 0.5–0.77× of scikit-image, ~200× behind
   OpenCV.** *This is the priority gap.* Root cause: `morph()` /
   `morphLine` / `morphColumn` use a **naïve O(radius) fold** — each of the
   `2·radius+1` structuring-element offsets is a full `vminInto`/`vmaxInto` pass over
   the line. Work scales with the kernel width; scipy's C inner loop is tighter and
   OpenCV uses an O(1) separable min/max with SIMD. Open/Close are 2× worse again
   because they run erosion then dilation (two full O(radius) operators).
2. **Sobel — 0.78–0.80×.** go-images recomputes Rec.601 luminance and the gradient
   magnitude (with a `sqrt`) per pixel through the RGBA buffer; scikit-image's
   `sobel` runs two correlate1d passes on a single pre-extracted float plane.
3. **Rotate90 / Crop / Otsu (†).** Reference side is a numpy view/copy or a histogram;
   these are memory-bandwidth and allocator races, not algorithmic gaps. go-images
   allocates a fresh origin-anchored RGBA each call (2 allocs); the only realistic
   win is an in-place / caller-supplied-buffer API.

## Action items to reach parity

**A. Morphology — O(1) van Herk / Gil-Werman (highest impact).**
Replace the per-offset fold in `morphLine`/`morphColumn` with the **van Herk /
Gil-Werman** running min/max: split each line into windows of size `2r+1`, build a
forward prefix-min/max and a backward suffix-min/max over each window, then every
output is `min(suffix[left], prefix[right])` — **3 comparisons per pixel
independent of radius** instead of `2r+1`. Apply it to both separable passes. This
alone makes erode/dilate cost flat in radius and should move the family from ~0.6×
to ≥ scikit-image; combined with SIMD it approaches OpenCV. Open/Close inherit it
for free.

**B. Sobel — operate on a cached luminance plane.**
`lumaPlane` already exists; extend the gradient operators to consume it directly
(as Canny does via `GaussianPlane`) so luminance is computed once, not per tap, and
hoist the magnitude `sqrt` to a vectorised pass. Target: close the 0.78× to ≥1×.

**C. SIMD the morphology/Sobel reductions via go-asmgen (all 6 arches).**
The min/max and axpy kernels already have `simd_amd64.s` / `simd_arm64.s` /
`simd_s390x.s` with a generic fallback. Once the algorithm is O(1) (A), regenerate
packed-min/max and the magnitude kernel through **go-asmgen** for amd64/arm64/
riscv64/loong64/ppc64le/s390x so the constant-factor inner loop is vectorised on
every 64-bit target, matching the project's SIMD-on-6-arches standard.

**D. Multicore is already in place — keep it as the large-image lever.**
The parallel tiling (`forLines`, `ParThreshold`) is what carries the multi-thread
row. After (A)–(C) the per-core kernel is O(1)+SIMD, so the multi-thread numbers
scale the single-thread parity by core count — the durable way a pure-Go library
stays ahead of single-threaded scikit-image on 1024²–4096² images.

**E. Allocation-light transform API (Crop/Rotate/Flip).**
Offer in-place or destination-buffer variants so the trivially memory-bound ops
stop paying for a fresh `image.NewRGBA` per call.

## Reproducing

```sh
cd benchmarks                      # separate Go module: excluded from the
                                   # parent module's 100%-coverage gate

# 1. correctness gate (writes go-images outputs, diffs vs scikit-image/scipy)
GOWORK=off go run ./verify /tmp/gi-out
python verify.py /tmp/gi-out

# 2. Go single-thread + multi-thread
GOWORK=off go test -run xxx -bench=. -benchmem -cpu=1 -count=5  . | tee go-1t.txt
GOWORK=off go test -run xxx -bench=. -benchmem -cpu=16 -count=3 . | tee go-mt.txt

# 3. Python reference (single + multi thread)
python run.py            > py-1t.json
python run.py --threads  > py-mt.json

# 4. parity tables
python aggregate.py    --go go-1t.txt --py py-1t.json   # single-thread
python aggregate_mt.py --go go-mt.txt --py py-mt.json   # multi-thread
```

Python deps: `pip install scikit-image scipy numpy opencv-python-headless threadpoolctl`.
