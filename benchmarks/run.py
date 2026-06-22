#!/usr/bin/env python3
"""Reference timing harness: scikit-image (primary) and OpenCV (where it has the
op) on the same operations the Go suite benchmarks.

Single-thread is the core comparison: BLAS / OpenMP / OpenCV thread pools are all
pinned to one thread (env vars below + threadpoolctl + cv2.setNumThreads(1)).
Pass --threads to additionally emit a multi-thread row for the parallelisable ops.

Each op is timed best-of and median over many iterations on a deterministic
random image of each size, mirroring the Go harness (512, 1024, 4096; RGB uint8).
Output is JSON on stdout: {op: {size: {skimage:{ns,mpix}, opencv:{...}}}}.
"""
import os

# Pin every BLAS/OpenMP backend to a single thread BEFORE numpy/scipy import.
for _v in ("OMP_NUM_THREADS", "OPENBLAS_NUM_THREADS", "MKL_NUM_THREADS",
           "VECLIB_MAXIMUM_THREADS", "NUMEXPR_NUM_THREADS",
           "SKIMAGE_NUM_THREADS"):
    os.environ.setdefault(_v, "1")

import argparse
import json
import sys
import time

import numpy as np
from threadpoolctl import threadpool_limits

import skimage
from skimage import filters as skfilters
from skimage import morphology as skmorph
from skimage import color as skcolor
from skimage import transform as sktransform
import scipy
from scipy import ndimage as ndi

import cv2

SIZES = [512, 1024, 4096]


def make_img(side):
    """Deterministic uint8 RGB image (matches the Go harness's intent: same
    statistical workload, reproducible)."""
    rng = np.random.default_rng(side)
    return rng.integers(0, 256, size=(side, side, 3), dtype=np.uint8)


def time_op(fn, *, min_iters=5, min_seconds=0.5):
    """Warm up, then run until both min_iters and min_seconds are reached;
    return (best_ns, median_ns, iters)."""
    fn()  # warm up
    samples = []
    t_end = time.perf_counter() + min_seconds
    while len(samples) < min_iters or time.perf_counter() < t_end:
        t0 = time.perf_counter_ns()
        fn()
        samples.append(time.perf_counter_ns() - t0)
        if len(samples) >= 200:
            break
    samples.sort()
    best = samples[0]
    median = samples[len(samples) // 2]
    return best, median, len(samples)


def entry(side, best_ns):
    mpix = (side * side) / best_ns * 1e3  # pixels/ns * 1e3 = Mpix/s
    return {"ns": best_ns, "mpix": round(mpix, 2)}


def bench(threads):
    results = {}

    def record(op, side, lib, fn):
        best, _med, _n = time_op(fn)
        results.setdefault(op, {}).setdefault(str(side), {})[lib] = entry(side, best)

    for side in SIZES:
        rgb = make_img(side)
        gray = skcolor.rgb2gray(rgb)            # float64 [0,1]
        gray_u8 = cv2.cvtColor(rgb, cv2.COLOR_RGB2GRAY)
        rgbf = skimage.img_as_float(rgb)
        se3 = skmorph.footprint_rectangle((7, 7))  # radius-3 square SE
        cv_k7 = cv2.getStructuringElement(cv2.MORPH_RECT, (7, 7))

        # --- BoxBlur radius 2 (7x7 mean) ---
        record("BoxBlur_r2", side, "skimage",
               lambda r=rgbf: ndi.uniform_filter(r, size=(5, 5, 1), mode="nearest"))
        record("BoxBlur_r2", side, "opencv",
               lambda r=rgb: cv2.blur(r, (5, 5)))

        # --- Gaussian blur sigma 2 ---
        record("GaussianBlur_s2", side, "skimage",
               lambda r=rgbf: skfilters.gaussian(r, sigma=2.0, channel_axis=-1))
        record("GaussianBlur_s2", side, "opencv",
               lambda r=rgb: cv2.GaussianBlur(r, (0, 0), 2.0))

        # --- Sobel (edge magnitude on luminance) ---
        record("Sobel", side, "skimage",
               lambda g=gray: skfilters.sobel(g))
        record("Sobel", side, "opencv",
               lambda g=gray_u8: cv2.magnitude(
                   cv2.Sobel(g, cv2.CV_32F, 1, 0, ksize=3),
                   cv2.Sobel(g, cv2.CV_32F, 0, 1, ksize=3)))

        # --- Morphology, radius 3 (7x7 square SE), grayscale ---
        record("Erode_r3", side, "skimage",
               lambda g=gray_u8, s=se3: skmorph.erosion(g, s))
        record("Erode_r3", side, "opencv",
               lambda g=gray_u8, k=cv_k7: cv2.erode(g, k))
        record("Dilate_r3", side, "skimage",
               lambda g=gray_u8, s=se3: skmorph.dilation(g, s))
        record("Dilate_r3", side, "opencv",
               lambda g=gray_u8, k=cv_k7: cv2.dilate(g, k))
        record("Open_r3", side, "skimage",
               lambda g=gray_u8, s=se3: skmorph.opening(g, s))
        record("Open_r3", side, "opencv",
               lambda g=gray_u8, k=cv_k7: cv2.morphologyEx(g, cv2.MORPH_OPEN, k))
        record("Close_r3", side, "skimage",
               lambda g=gray_u8, s=se3: skmorph.closing(g, s))
        record("Close_r3", side, "opencv",
               lambda g=gray_u8, k=cv_k7: cv2.morphologyEx(g, cv2.MORPH_CLOSE, k))

        # --- Flip / rotate / crop ---
        record("FlipHorizontal", side, "skimage", lambda r=rgb: np.fliplr(r).copy())
        record("FlipHorizontal", side, "opencv", lambda r=rgb: cv2.flip(r, 1))
        record("Rotate90", side, "skimage", lambda r=rgb: np.rot90(r).copy())
        record("Rotate90", side, "opencv",
               lambda r=rgb: cv2.rotate(r, cv2.ROTATE_90_COUNTERCLOCKWISE))
        record("Crop", side, "skimage",
               lambda r=rgb, s=side: r[: s // 2, : s // 2].copy())

        # --- RGB -> HSV ---
        record("RGBToHSV", side, "skimage", lambda r=rgbf: skcolor.rgb2hsv(r))
        record("RGBToHSV", side, "opencv", lambda r=rgb: cv2.cvtColor(r, cv2.COLOR_RGB2HSV))

        # --- Otsu threshold ---
        record("Otsu", side, "skimage", lambda g=gray_u8: skfilters.threshold_otsu(g))
        record("Otsu", side, "opencv",
               lambda g=gray_u8: cv2.threshold(g, 0, 255, cv2.THRESH_BINARY + cv2.THRESH_OTSU))

        # --- Grayscale (RGB -> luminance) ---
        record("Grayscale", side, "skimage", lambda r=rgbf: skcolor.rgb2gray(r))
        record("Grayscale", side, "opencv", lambda r=rgb: cv2.cvtColor(r, cv2.COLOR_RGB2GRAY))

    return results


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--threads", action="store_true",
                    help="allow library thread pools (multi-thread row)")
    args = ap.parse_args()

    n = 0 if args.threads else 1
    cv2.setNumThreads(n)
    versions = {
        "skimage": skimage.__version__, "scipy": scipy.__version__,
        "numpy": np.__version__, "opencv": cv2.__version__,
        "python": sys.version.split()[0],
        "threads": "multi" if args.threads else "single",
    }
    if args.threads:
        res = bench(threads=True)
    else:
        with threadpool_limits(limits=1):
            res = bench(threads=False)
    print(json.dumps({"versions": versions, "results": res}))


if __name__ == "__main__":
    main()
