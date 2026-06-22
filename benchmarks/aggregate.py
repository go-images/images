#!/usr/bin/env python3
"""Aggregate Go `go test -bench` output and the Python reference JSON into the
parity table rows. Reads:

  --go      go benchmark text (single-thread)
  --py      run.py JSON (single-thread)

and prints a Markdown table to stdout. For each op/size it takes the best
(max) Mpix/s the Go harness reported across -count runs, and the best ns/op
(min), to mirror the "best of N" the Python side reports.
"""
import argparse, json, re, sys
from collections import defaultdict

# Map Go benchmark base name -> (table op label, python op key)
OPS = {
    "BenchmarkBoxBlur_r2":     ("Box blur r=2 (RGB)",      "BoxBlur_r2"),
    "BenchmarkGaussianBlur_s2":("Gaussian σ=2 (RGB)", "GaussianBlur_s2"),
    "BenchmarkSobel":          ("Sobel edge (gray)",       "Sobel"),
    "BenchmarkErode_r3":       ("Erode r=3 (gray)",        "Erode_r3"),
    "BenchmarkDilate_r3":      ("Dilate r=3 (gray)",       "Dilate_r3"),
    "BenchmarkOpen_r3":        ("Open r=3 (gray)",         "Open_r3"),
    "BenchmarkClose_r3":       ("Close r=3 (gray)",        "Close_r3"),
    "BenchmarkFlipHorizontal": ("Flip horizontal (RGB)",   "FlipHorizontal"),
    "BenchmarkRotate90":       ("Rotate 90 (RGB)",         "Rotate90"),
    "BenchmarkCrop":           ("Crop half (RGB)",         "Crop"),
    "BenchmarkRGBToHSV":       ("RGB→HSV",            "RGBToHSV"),
    "BenchmarkOtsu":           ("Otsu threshold",          "Otsu"),
    "BenchmarkGrayscale":      ("Grayscale",               "Grayscale"),
}
ORDER = list(OPS.keys())

# e.g. "BenchmarkGaussianBlur_s2/1024-1   74  16365039 ns/op  64.07 Mpix/s ..."
LINE = re.compile(
    r"^(Benchmark[A-Za-z0-9_]+)/(\d+)(?:-\d+)?\s+\d+\s+([\d.]+)\s+ns/op.*?([\d.]+)\s+Mpix/s"
)


def parse_go(path):
    best = {}  # (base, size) -> {"ns": min, "mpix": max}
    with open(path) as f:
        for ln in f:
            m = LINE.match(ln.strip())
            if not m:
                continue
            base, size, ns, mpix = m.group(1), int(m.group(2)), float(m.group(3)), float(m.group(4))
            key = (base, size)
            e = best.setdefault(key, {"ns": ns, "mpix": mpix})
            e["ns"] = min(e["ns"], ns)
            e["mpix"] = max(e["mpix"], mpix)
    return best


def fmt_mpix(v):
    return f"{v:,.0f}" if v >= 100 else f"{v:.1f}"


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--go", required=True)
    ap.add_argument("--py", required=True)
    args = ap.parse_args()

    go = parse_go(args.go)
    py = json.load(open(args.py))
    pres = py["results"]
    v = py["versions"]

    print(f"<!-- versions: skimage {v['skimage']}, scipy {v['scipy']}, "
          f"numpy {v['numpy']}, opencv {v['opencv']}, python {v['python']} -->\n")
    print("| op | size | go-images | scikit-image | OpenCV | go/skimage | verdict |")
    print("|----|------|-----------|--------------|--------|-----------|---------|")

    for base in ORDER:
        label, pykey = OPS[base]
        for size in (512, 1024, 4096):
            ge = go.get((base, size))
            if not ge:
                continue
            gm, gns = ge["mpix"], ge["ns"]
            sk = pres.get(pykey, {}).get(str(size), {}).get("skimage")
            cv = pres.get(pykey, {}).get(str(size), {}).get("opencv")
            sk_s = f"{fmt_mpix(sk['mpix'])} Mpix/s" if sk else "—"
            cv_s = f"{fmt_mpix(cv['mpix'])} Mpix/s" if cv else "—"
            if sk:
                ratio = gm / sk["mpix"]
                ratio_s = f"**{ratio:.2f}×**"
                if ratio >= 1.15:
                    verd = "go faster"
                elif ratio <= 0.87:
                    verd = "skimage faster"
                else:
                    verd = "~parity"
            else:
                ratio_s, verd = "—", "—"
            go_s = f"{fmt_mpix(gm)} Mpix/s ({gns:,.0f} ns)"
            print(f"| {label} | {size}² | {go_s} | {sk_s} | {cv_s} | {ratio_s} | {verd} |")


if __name__ == "__main__":
    main()
