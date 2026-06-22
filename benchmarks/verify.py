#!/usr/bin/env python3
"""Correctness gate: compare go-images outputs (written by ./verify) against
scikit-image for each benchmarked op, on a fixed 128x128 image. Prints a PASS/
FAIL table with the max absolute difference per op. Parity timing is only
meaningful once these pass.

Tolerances: separable byte ops should match the reference to within a couple of
levels (independent rounding order); Gaussian differs because go-images truncates
at 3*sigma while skimage truncates at 4*sigma, so a looser bound is used and the
difference is reported rather than gated hard.
"""
import os, sys, struct
import numpy as np
from skimage import filters as skfilters
from skimage import color as skcolor
from scipy import ndimage as ndi
from skimage import morphology as skmorph


def load(d, name):
    return np.fromfile(os.path.join(d, name), dtype=np.uint8)


def main():
    d = sys.argv[1]
    side = struct.unpack("<I", load(d, "side.bin").tobytes())[0]
    inp = load(d, "input.bin").reshape(side, side, 4)
    rgb = inp[:, :, :3]
    rgbf = rgb.astype(np.float64) / 255.0
    gray_u8 = np.round(
        0.299 * rgb[:, :, 0] + 0.587 * rgb[:, :, 1] + 0.114 * rgb[:, :, 2]
    ).clip(0, 255).astype(np.uint8)

    def go(name):
        return load(d, name).reshape(side, side, 4)[:, :, :3].astype(np.int32)

    rows = []

    # BoxBlur r2 (5x5 uniform, nearest border)
    ref = ndi.uniform_filter(rgbf, size=(5, 5, 1), mode="nearest")
    ref = np.round(ref * 255).clip(0, 255).astype(np.int32)
    rows.append(("BoxBlur_r2", int(np.abs(go("boxblur_r2.bin") - ref).max()), 2))

    # Gaussian sigma 2 -- go truncates 3*sigma, skimage 4*sigma: looser bound.
    ref = skfilters.gaussian(rgbf, sigma=2.0, channel_axis=-1)
    ref = np.round(ref * 255).clip(0, 255).astype(np.int32)
    rows.append(("GaussianBlur_s2", int(np.abs(go("gaussian_s2.bin") - ref).max()), 4))

    # Erode / Dilate r3 (7x7 square), per-channel grayscale morphology, nearest.
    fp = np.ones((7, 7), bool)
    for nm, op in (("erode_r3.bin", ndi.grey_erosion), ("dilate_r3.bin", ndi.grey_dilation)):
        ref = np.dstack([op(rgb[:, :, c], footprint=fp, mode="nearest") for c in range(3)]).astype(np.int32)
        rows.append((nm.split(".")[0], int(np.abs(go(nm) - ref).max()), 1))

    # Flip / rotate
    rows.append(("FlipHorizontal", int(np.abs(go("fliph.bin") - np.fliplr(rgb).astype(np.int32)).max()), 0))
    rows.append(("Rotate90", int(np.abs(go("rot90.bin") - np.rot90(rgb).astype(np.int32)).max()), 0))

    # Grayscale (Rec.601) -- go writes luma to R,G,B
    gref = np.round(0.299 * rgb[:, :, 0] + 0.587 * rgb[:, :, 1] + 0.114 * rgb[:, :, 2]).clip(0, 255).astype(np.int32)
    gg = go("gray.bin")[:, :, 0]
    rows.append(("Grayscale", int(np.abs(gg - gref).max()), 1))

    # Otsu threshold level
    go_otsu = int(load(d, "otsu.bin")[0])
    sk_otsu = int(skfilters.threshold_otsu(gray_u8))
    rows.append(("Otsu(level)", abs(go_otsu - sk_otsu), 1))

    print(f"{'op':<18}{'max|diff|':>10}{'tol':>6}  verdict")
    ok = True
    for nm, diff, tol in rows:
        v = "PASS" if diff <= tol else "FAIL"
        if v == "FAIL":
            ok = False
        print(f"{nm:<18}{diff:>10}{tol:>6}  {v}")
    print("\nALL PASS" if ok else "\nSOME FAILED")
    sys.exit(0 if ok else 1)


if __name__ == "__main__":
    main()
