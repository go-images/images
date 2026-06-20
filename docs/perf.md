# Performance — `go-images/images` vs scikit-image / SciPy

> **The bar:** honest, like-for-like timings on the *same machine*, at a
> realistic image size, for operations with matched semantics. We show the wins
> **and** the losses — "on n'a pas le droit de se tromper", a fake win is worse
> than a real loss.

## Method

- **Hardware:** a single Tart `debian` VM, `aarch64` (arm64) Linux, kernel
  6.12, 4 vCPUs. Both stacks run on the *same* VM, back to back.
- **Image:** a deterministic `512×512` (and `1024×1024`) RGB(A) image
  (xorshift / NumPy PRNG).
- **go-images:** Go 1.26.4, `GOWORK=off`, `go test -bench`, time = ns/op
  (best of the timed iterations). The colour ops run on RGBA; the comparison
  references run the matched op on each of the three colour channels.
- **scikit-image / SciPy:** the Debian system packages
  (`python3-skimage` 0.25, `python3-scipy` 1.15, `python3-numpy` 2.2),
  `time.perf_counter`, best of 20 runs after a warm-up. NumPy here is
  single-threaded C with SIMD.
- **Matched semantics** (this is the part that makes the comparison fair):
  - Box blur radius 5 ⇔ `scipy.ndimage.uniform_filter(size=11, mode="nearest")`
    per channel.
  - Gaussian σ=3 ⇔ `scipy.ndimage.gaussian_filter(sigma=3, truncate=3,
    mode="nearest")` per channel (truncate 3 because our radius is `ceil(3σ)`).
  - Sobel ⇔ `skimage.filters.sobel` (gradient magnitude) on the luminance plane.
  - Erode/Dilate radius 2 ⇔ `scipy.ndimage.grey_erosion` /
    `grey_dilation(size=(5,5), mode="nearest")` per channel.

The hot separable kernels now run **SIMD inner loops + multicore tiling**:

- A **multiply-accumulate (axpy) SIMD kernel** — `dst[i] += a*src[i]` over a
  contiguous float64 plane — drives the Gaussian separable convolution. The
  channels are deinterleaved into contiguous planes so each kernel tap is one
  flat axpy; the byte-rounded intermediate between the two passes is unchanged,
  so the numeric output is identical to the former scalar code.
- An **elementwise min/max SIMD kernel** — `dst[i] = min/max(dst[i], src[i])` —
  drives the separable grayscale morphology, likewise over contiguous planes.
- **Multicore tiling** fans the independent rows (horizontal pass) and columns
  (vertical pass) across `GOMAXPROCS` goroutines above a pixel-count threshold;
  below it the serial path runs. The result is independent of the worker count.

SIMD coverage by architecture (the multi-arch story `go-fft` / `go-ndarray`
document): **amd64 SSE2**, **arm64 NEON** (the axpy uses VFMLA; the vector
double FMIN/FMAX, which the Go assembler has no mnemonic for, are emitted as the
raw instruction word, encoding verified on hardware), and **s390x z/vector**
(VFMADB + VFMINDB/VFMAXDB — big-endian, but these elementwise kernels are
lane-order-independent; operand order verified under qemu-s390x). On **loong64 /
ppc64le / riscv64** the Go assembler exposes no usable vector-double arithmetic
(or, for riscv64, the V extension faults under the default qemu CPU), so those
keep the **scalar inner loop plus multicore tiling** — still a multi-core win.
Every SIMD kernel is validated bit-for-bit against its scalar oracle (axpy to a
tight tolerance, since a GOAMD64=v3 build's oracle fuses to VFMADD), and the
whole op is validated against SciPy.

## Results — 512×512 (arm64 Linux, lower is better)

| Operation               | go-images **before** | go-images **after** | scikit-image / SciPy | After vs SciPy |
|-------------------------|---------------------:|--------------------:|---------------------:|---------------:|
| **Box blur** (r=5)      |            2,812,064 |             870,237 |           10,999,974 | **12.6× faster** |
| **Sobel** (magnitude)   |              882,498 |             351,701 |            2,748,223 | **7.8× faster** |
| **Gaussian blur** (σ=3) |           12,900,987 |           4,259,656 |           16,029,709 | **3.8× faster** |
| **Erode** (r=2)         |           13,865,054 |           1,581,921 |           10,559,638 | **6.7× faster** |
| **Dilate** (r=2)        |           12,898,691 |           1,482,165 |           11,025,974 | **7.4× faster** |

### Phase 1 edge / filter additions — 512×512 (arm64 Linux, lower is better)

These are the newer multicore (scalar-inner-loop) ops, benchmarked on the same
VM against the matched scikit-image / SciPy reference. They have no SIMD inner
loop yet — the win is the multicore fan-out plus a cheaper algorithm (counting
selection for the median) — and they already beat scikit-image:

| Operation               | go-images (ns/op) | scikit-image / SciPy (ns/op) | Speed-up |
|-------------------------|------------------:|-----------------------------:|---------:|
| **Scharr** (magnitude)  |         1,421,773 |                    3,833,147 | **2.7× faster** |
| **Canny** (σ=1)         |        10,086,768 |                   27,031,572 | **2.7× faster** |
| **Median** (r=2, 5×5)   |        37,565,475 |                  214,597,360 | **5.7× faster** |

Matched semantics: Scharr ⇔ `skimage.filters.scharr` on the luminance plane;
Canny ⇔ `skimage.feature.canny(sigma=1, low=20, high=40)`; Median ⇔
`scipy.ndimage.median_filter(size=5, mode="nearest")` per channel. Prewitt and
SobelMag share Scharr's kernel; UnsharpMask/Sharpen are GaussianBlur plus a
fused per-pixel add (Gaussian's numbers above apply). The median's large lead is
the O(window+256) counting selection vs SciPy's general n-D median, widened by
the multicore tiling. Adding a SIMD Sobel-gradient kernel (shared by the edge
family and Canny) is the next vectorisation target.

The "before" column is the scalar single-threaded kernel; "after" is SIMD +
4-core. **Both former losses are now wins:** Gaussian goes from 0.92× (1.08×
slower) to **3.8× faster**, and Erode/Dilate from ~0.9× to **6.7×/7.4×
faster**. The already-winning Box blur and Sobel widen their lead (the multicore
tiling alone roughly triples them on 4 cores).

## Results — 1024×1024 (arm64 Linux, lower is better)

| Operation               | go-images (ns/op) | scikit-image / SciPy (ns/op) | Speed-up |
|-------------------------|------------------:|-----------------------------:|---------:|
| **Box blur** (r=5)      |         7,548,773 |                   29,963,031 | **4.0× faster** |
| **Sobel** (magnitude)   |         6,003,498 |                   11,327,476 | **1.9× faster** |
| **Gaussian blur** (σ=3) |        11,451,846 |                   46,418,949 | **4.1× faster** |
| **Erode** (r=2)         |         7,896,837 |                   59,335,849 | **7.5× faster** |
| **Dilate** (r=2)        |         7,893,893 |                   61,166,651 | **7.7× faster** |

At 1024² the lead grows on the ops scipy materialises large intermediate arrays
for (morphology, Gaussian); Sobel narrows to 1.9× because its arithmetic per
pixel is small and the win is mostly the multicore fan-out (single SciPy edge
filter, no per-channel Python overhead at this size).

### Per-component speed-up (512², before → after)

- **Gaussian — 3.0× faster than the scalar floor** (and now ahead of SciPy):
  the axpy SIMD kernel vectorises the dense multiply-add and the two passes fan
  out across cores. The vertical pass is a pure contiguous whole-row axpy; the
  horizontal pass vectorises the interior columns and handles the few
  border-clamped columns scalar.
- **Erode/Dilate — ~8.7× faster than the scalar floor:** the per-window running
  min/max became contiguous SIMD MIN/MAX over planes plus multicore tiling, and
  the per-row scratch was hoisted out of the inner loop (allocations dropped
  from ~6,200/op to 71/op).

## Correctness vs SciPy (not just speed)

Same VM, a `64×48` deterministic image, go-images output compared
pixel-for-pixel to the matched SciPy reference, **after** the SIMD/multicore
rewrite:

| Operation | max \|Δ\| | mean \|Δ\| | verdict |
|-----------|----------:|-----------:|---------|
| Box blur  |         0 |      0.000 | **exact match** |
| Erode     |         0 |      0.000 | **exact match** |
| Dilate    |         0 |      0.000 | **exact match** |
| Gaussian  |         1 |      0.079 | matches within 1 LSB (rounding only) |

The SIMD/multicore rewrite changed **nothing** numerically: box blur and
grayscale morphology still reproduce SciPy bit-for-bit, and the Gaussian still
differs only by the last-bit rounding of the final clamp — the same figures as
the scalar implementation. Each SIMD kernel is also held against its scalar
oracle (`internal/kernels/simd_test.go`) across every length residue, and the
whole separable op against an independent naive reference at both the serial and
parallel thresholds (`separable_test.go`).

### Residual notes (honest)

- **Memory:** the plane-based Gaussian allocates more transient float64 scratch
  than the old in-place byte path (≈14.8 MB/op at 512² vs 2.1 MB), the cost of
  the contiguous SIMD layout. It is transient (freed each call) and pays for a
  3.8× speed-up; reducing it with a reusable scratch pool is a follow-up.
- **loong64 / ppc64le / riscv64** run the scalar inner loop (no usable
  vector-double in the Go assembler / faulting qemu V), relying on multicore
  tiling alone for the speed-up. They are validated on the per-arch qemu CI job.

## Reproducing

```sh
# On the arm64 debian VM, with the repo synced to /tmp/goimages:
cd /tmp/goimages
GOWORK=off /usr/local/go/bin/go test -run=^$ \
  -bench 'BoxBlur|GaussianBlur|Sobel$|Erode|Dilate|Scharr|Canny|Median' -benchtime=2s ./...

# scikit-image / SciPy side (see the harness used to produce the table):
python3 skbench.py      # timings
python3 validate.py     # correctness vs scipy
```
