//go:build ignore

// Command gen produces simd_s390x.s — the z/Architecture vector-facility
// float64 multiply-accumulate (axpy) and elementwise min/max kernels — via
// go-asmgen. Run with `go run gen.go` or `go generate` from the kernels package.
//
// s390x is the only big-endian go-asmgen target, but these are elementwise
// kernels (no cross-lane interleave), so lane order is irrelevant: every lane
// does the identical scalar computation. The vector facility (V0–V31, 128-bit,
// two float64 lanes) exposes 2-wide double FMA (VFMADB) and min/max (VFMINDB /
// VFMAXDB), so all three kernels ship here, validated against their scalar
// oracles on the per-arch qemu CI job (and the operand order / mask were
// verified empirically under qemu-s390x).
//
//	axpyVX(dst, src *float64, a float64, n int)  dst[i] += a*src[i]
//	  s390x's scalar oracle fuses dst[i] = dst[i] + a*src[i] to FMADD (verified
//	  by disassembly), and VFMADB reproduces that fusion two-lane, so the kernel
//	  is bit-identical to the oracle.
//	vminVX / vmaxVX(dst, src *float64, n int)    dst[i] = min/max(dst[i], src[i])
//	  VFMINDB/VFMAXDB match the scalar `if` oracle on the finite, uint8-derived
//	  values morphology runs on.
package main

import (
	"fmt"
	"os"

	"github.com/go-asmgen/asmgen/emit"
	"github.com/go-asmgen/asmgen/s390x"
)

func main() {
	f := emit.NewFile("s390x")
	f.Add(axpyKernel())
	f.Add(minmaxKernel("vminVX", "VFMINDB", "WFMINDB"))
	f.Add(minmaxKernel("vmaxVX", "VFMAXDB", "WFMAXDB"))
	if err := os.WriteFile("simd_s390x.s", []byte(f.String()), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("wrote simd_s390x.s")
}

// axpyKernel builds axpyVX(dst, src *float64, a float64, n int): dst[i] +=
// a*src[i]. a is broadcast into both lanes of V3; the loop processes 2 doubles
// per iteration via VFMADB, with a scalar FMADD tail.
func axpyKernel() *emit.Function {
	sig := s390x.Layout(
		[]string{"dst", "src", "a", "n"},
		[]s390x.Type{s390x.Ptr, s390x.Ptr, s390x.Float64, s390x.Int64},
		nil, nil,
	)
	b := s390x.NewFunc("axpyVX", sig, 0)
	b.LoadArg("dst", "R1").
		LoadArg("src", "R2").
		LoadArg("a", "F0").
		LoadArg("n", "R3").
		Raw("VLREPG a+16(FP), V3"). // V3 = [a, a]
		Raw("block:").
		Raw("CMPBLT R3, $2, tail").
		Raw("VL (R2), V1").           // src[0:2]
		Raw("VL (R1), V2").           // dst[0:2]
		Raw("VFMADB V1, V3, V2, V2"). // V2 = src*a + dst
		Raw("VST V2, (R1)").
		Raw("ADD $16, R1").
		Raw("ADD $16, R2").
		Raw("SUB $2, R3").
		Raw("BR block").
		Raw("tail:").
		Raw("CMPBEQ R3, $0, done").
		Raw("FMOVD (R2), F1").   // src[i]
		Raw("FMOVD (R1), F2").   // dst[i]
		Raw("FMADD F0, F1, F2"). // F2 = F0*F1 + F2 = a*src + dst
		Raw("FMOVD F2, (R1)").
		Raw("done:").
		Ret()
	return b.Func()
}

// minmaxKernel builds vminVX / vmaxVX(dst, src *float64, n int): dst[i] =
// op(dst[i], src[i]). vecOp is the 2-lane VFMINDB/VFMAXDB (mask $1, IEEE
// behaviour); wOp is the element (lane-0) form WFMINDB/WFMAXDB used for the
// single-element tail — s390x has no scalar FP min/max mnemonic, but F0/F1
// alias V0/V1 lane 0, so the W-form reduces the tail element in place.
func minmaxKernel(name, vecOp, wOp string) *emit.Function {
	sig := s390x.Layout(
		[]string{"dst", "src", "n"},
		[]s390x.Type{s390x.Ptr, s390x.Ptr, s390x.Int64},
		nil, nil,
	)
	b := s390x.NewFunc(name, sig, 0)
	b.LoadArg("dst", "R1").
		LoadArg("src", "R2").
		LoadArg("n", "R3").
		Raw("block:").
		Raw("CMPBLT R3, $2, tail").
		Raw("VL (R1), V0").
		Raw("VL (R2), V1").
		Raw(vecOp + " $1, V1, V0, V0"). // V0 = op(V0, V1)
		Raw("VST V0, (R1)").
		Raw("ADD $16, R1").
		Raw("ADD $16, R2").
		Raw("SUB $2, R3").
		Raw("BR block").
		Raw("tail:").
		Raw("CMPBEQ R3, $0, done").
		Raw("FMOVD (R1), F0").        // dst[i] -> V0 lane 0
		Raw("FMOVD (R2), F1").        // src[i] -> V1 lane 0
		Raw(wOp + " $1, V1, V0, V0"). // V0 lane 0 = op(V0, V1)
		Raw("FMOVD F0, (R1)").
		Raw("done:").
		Ret()
	return b.Func()
}
