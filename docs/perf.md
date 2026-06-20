# Performance — `go-images/images` vs scikit-image / SciPy

> **The bar:** honest, like-for-like timings on the *same machine*, at a
> realistic image size, for operations with matched semantics. We show the wins
> **and** the losses — "on n'a pas le droit de se tromper", a fake win is worse
> than a real loss.

## Method

- **Hardware:** a single Tart `debian` VM, `aarch64` (arm64) Linux, kernel
  6.12, 4 vCPUs. Both stacks run on the *same* VM, back to back.
- **Image:** a deterministic `512×512` RGB(A) image (xorshift / NumPy PRNG).
- **go-images:** Go 1.26.4, `GOWORK=off`, `go test -bench`, time = ns/op
  (best of the timed iterations). The colour ops run on RGBA; the comparison
  references run the matched op on each of the three colour channels.
- **scikit-image / SciPy:** the Debian system packages
  (`python3-skimage`, `python3-scipy`, `python3-numpy`), `time.perf_counter`,
  best of 20 runs after a warm-up. NumPy here is single-threaded C with SIMD.
- **Matched semantics** (this is the part that makes the comparison fair):
  - Box blur radius 5 ⇔ `scipy.ndimage.uniform_filter(size=11, mode="nearest")`
    per channel.
  - Gaussian σ=3 ⇔ `scipy.ndimage.gaussian_filter(sigma=3, truncate=3,
    mode="nearest")` per channel (truncate 3 because our radius is `ceil(3σ)`).
  - Sobel ⇔ `skimage.filters.sobel` (gradient magnitude) on the luminance plane.
  - Erode/Dilate radius 2 ⇔ `scipy.ndimage.grey_erosion` /
    `grey_dilation(size=(5,5), mode="nearest")` per channel.

Today these are **scalar, single-threaded pure-Go** kernels: no SIMD, no
goroutine fan-out yet. The numbers below are the *floor* the planned go-asmgen
SIMD kernels and multicore tiling will build on.

## Results (512×512, arm64 Linux, lower is better)

| Operation              | go-images (ns/op) | scikit-image / SciPy (ns/op) | Speed-up |
|------------------------|------------------:|-----------------------------:|---------:|
| **Box blur** (r=5)     |         2,374,701 |                    7,433,789 | **3.13× faster** |
| **Sobel** (magnitude)  |           757,445 |                    2,393,304 | **3.16× faster** |
| **Erode** (r=2)        |        11,628,716 |                   10,287,595 | 0.88× (1.13× slower) |
| **Dilate** (r=2)       |        11,313,066 |                   10,566,680 | 0.93× (1.08× slower) |
| **Gaussian blur** (σ=3)|        12,132,132 |                   11,176,641 | 0.92× (1.08× slower) |

### Where we win

- **Box blur — 3.1× faster.** The running-sum separable filter is O(1) per
  pixel in the radius and stays in a tight float accumulator. SciPy's
  `uniform_filter` is also separable but pays Python-level per-channel dispatch
  and array materialisation.
- **Sobel — 3.2× faster.** The luminance plane is computed once and the two
  gradient passes are fused with the six non-zero taps unrolled.

### Where we lose (honestly)

- **Gaussian (1.08× slower), Erode/Dilate (1.1× slower).** These are the
  arithmetic-heavy kernels where NumPy's compiled C inner loops (and, for
  Gaussian, a tighter 1-D convolution) still edge out our scalar Go. These are
  *exactly* the kernels the next phase targets:
  - **SIMD via go-asmgen** for the Gaussian 1-D pass (a dense multiply-add over
    a regular byte layout — the ideal vector shape) across all six 64-bit
    targets, behind the existing `internal/kernels` seam with the scalar
    version kept as the oracle.
  - **Multicore tiling**: convolution and morphology parallelise trivially
    across rows; a size threshold keeps small images on the scalar path.

## Correctness vs SciPy (not just speed)

Same VM, a `64×48` random image, go-images output compared pixel-for-pixel to
the matched SciPy reference:

| Operation | max \|Δ\| | mean \|Δ\| | verdict |
|-----------|----------:|-----------:|---------|
| Box blur  |         0 |      0.000 | **exact match** |
| Erode     |         0 |      0.000 | **exact match** |
| Dilate    |         0 |      0.000 | **exact match** |
| Gaussian  |         1 |      0.080 | matches within 1 LSB (rounding only) |

Box blur and grayscale morphology reproduce SciPy bit-for-bit; the Gaussian
differs only by the last-bit rounding of the final clamp. Every SIMD kernel
added later must keep matching the scalar oracle, which in turn matches SciPy.

## Reproducing

```sh
# On the arm64 debian VM, with the repo synced to /tmp/goimages:
cd /tmp/goimages
GOWORK=off /usr/local/go/bin/go test -run=^$ \
  -bench 'BoxBlur|GaussianBlur|Sobel$|Erode|Dilate' -benchtime=2s ./...

# scikit-image / SciPy side (see the harness used to produce the table):
python3 skbench.py      # timings
python3 validate.py     # correctness vs scipy
```
