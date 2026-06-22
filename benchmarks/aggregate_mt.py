#!/usr/bin/env python3
"""Multi-thread comparison row: go-images (all cores) vs scikit-image / OpenCV
with their thread pools unpinned. Reads the multi-thread Go bench text and the
--threads run.py JSON, prints the Mpix/s for the ops that parallelise."""
import argparse, json, re

OPS = {
    "BenchmarkBoxBlur_r2":     "Box blur r=2",
    "BenchmarkGaussianBlur_s2":"Gaussian σ=2",
    "BenchmarkSobel":          "Sobel",
    "BenchmarkErode_r3":       "Erode r=3",
    "BenchmarkDilate_r3":      "Dilate r=3",
    "BenchmarkOpen_r3":        "Open r=3",
    "BenchmarkClose_r3":       "Close r=3",
}
PYKEY = {
    "BenchmarkBoxBlur_r2": "BoxBlur_r2", "BenchmarkGaussianBlur_s2": "GaussianBlur_s2",
    "BenchmarkSobel": "Sobel", "BenchmarkErode_r3": "Erode_r3",
    "BenchmarkDilate_r3": "Dilate_r3", "BenchmarkOpen_r3": "Open_r3",
    "BenchmarkClose_r3": "Close_r3",
}
LINE = re.compile(r"^(Benchmark[A-Za-z0-9_]+)/(\d+)(?:-\d+)?\s+\d+\s+[\d.]+\s+ns/op.*?([\d.]+)\s+Mpix/s")


def parse_go(path):
    best = {}
    for ln in open(path):
        m = LINE.match(ln.strip())
        if not m:
            continue
        base, size, mpix = m.group(1), int(m.group(2)), float(m.group(3))
        k = (base, size)
        best[k] = max(best.get(k, 0), mpix)
    return best


def fmt(v):
    return f"{v:,.0f}" if v >= 100 else f"{v:.1f}"


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--go", required=True)
    ap.add_argument("--py", required=True)
    a = ap.parse_args()
    go = parse_go(a.go)
    py = json.load(open(a.py))["results"]
    print("| op | size | go-images (all cores) | scikit-image | OpenCV | go/skimage |")
    print("|----|------|----------------------|--------------|--------|-----------|")
    for base, label in OPS.items():
        for size in (1024, 4096):
            gm = go.get((base, size))
            if gm is None:
                continue
            sk = py.get(PYKEY[base], {}).get(str(size), {}).get("skimage")
            cv = py.get(PYKEY[base], {}).get(str(size), {}).get("opencv")
            sk_s = f"{fmt(sk['mpix'])} Mpix/s" if sk else "—"
            cv_s = f"{fmt(cv['mpix'])} Mpix/s" if cv else "—"
            r = f"**{gm/sk['mpix']:.2f}×**" if sk else "—"
            print(f"| {label} | {size}² | {fmt(gm)} Mpix/s | {sk_s} | {cv_s} | {r} |")


if __name__ == "__main__":
    main()
