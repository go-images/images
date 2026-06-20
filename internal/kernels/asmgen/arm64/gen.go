//go:build ignore

// Command gen produces simd_arm64.s — the NEON float64 multiply-accumulate
// (axpy) and elementwise min/max kernels — via go-asmgen. Run with
// `go run gen.go` or `go generate` from the kernels package.
//
// NEON (ASIMD) is part of the arm64 baseline the Go toolchain requires, so every
// kernel is always callable with no CPU-feature branch.
//
//	axpyNEON(dst, src *float64, a float64, n int)
//	  dst[i] += a*src[i] for i in [0,n). The Gaussian separable pass applies one
//	  kernel tap weight a to a whole shifted channel line. arm64's scalar oracle
//	  fuses dst[i] = dst[i] + a*src[i] into FMADDD (verified by disassembly), so
//	  the kernel uses the packed FMA VFMLA (vd += vn*vm) and is bit-identical to
//	  the oracle: FMA(a, src, dst) rounds exactly as the fused scalar does.
//
//	vminNEON / vmaxNEON(dst, src *float64, n int)
//	  dst[i] = min/max(dst[i], src[i]) for i in [0,n), the morphology per-window
//	  reduction. Go's arm64 assembler exposes NO vector double FMIN/FMAX
//	  mnemonic, so the 2-lane reduction is emitted as the raw FMIN.2D / FMAX.2D
//	  instruction word (encoding verified empirically on arm64 hardware). On the
//	  finite, uint8-derived values morphology runs on, FMIN/FMAX match the scalar
//	  `if` oracle bit-for-bit (no NaN / signed-zero cases).
package main

import (
	"fmt"
	"os"

	"github.com/go-asmgen/asmgen/arm64"
	"github.com/go-asmgen/asmgen/emit"
)

func main() {
	f := emit.NewFile("arm64")
	f.Add(axpyKernel())
	// FMIN.2D Vd=V0,Vn=V0,Vm=V1 -> 0x4EE0F400 | Rm<<16 | Rn<<5 | Rd.
	f.Add(minmaxKernel("vminNEON", 0x4EE0F400, "FMIN"))
	// FMAX.2D base 0x4E60F400.
	f.Add(minmaxKernel("vmaxNEON", 0x4E60F400, "FMAX"))
	if err := os.WriteFile("simd_arm64.s", []byte(f.String()), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("wrote simd_arm64.s")
}

// axpyKernel builds axpyNEON(dst, src *float64, a float64, n int): dst[i] +=
// a*src[i]. a is broadcast into both lanes of V16; the loop processes 4 doubles
// (two D2 vectors) per iteration via VFMLA, with a scalar FMADDD tail.
func axpyKernel() *emit.Function {
	sig := arm64.Layout(
		[]string{"dst", "src", "a", "n"},
		[]arm64.Type{arm64.Ptr, arm64.Ptr, arm64.Float64, arm64.Int64},
		nil, nil,
	)
	b := arm64.NewFunc("axpyNEON", sig, 0)
	b.LoadArg("dst", "R0").
		LoadArg("src", "R1").
		LoadArg("a", "F8"). // a kept in F8 — high enough not to alias the V0..V3
		LoadArg("n", "R2"). //   vector scratch used by the body (F0 == V0.lane0).
		// Move a's bit pattern through a GP register and broadcast to V16.D2.
		Raw("FMOVD F8, R3").
		Raw("VDUP R3, V16.D2").
		// Main loop: 4 doubles (32 bytes) per iteration.
		Raw("block:").
		Raw("CMP $4, R2").
		Raw("BLT tail").
		Raw("VLD1 (R1), [V0.D2, V1.D2]").
		Raw("VLD1 (R0), [V2.D2, V3.D2]").
		Raw("VFMLA V0.D2, V16.D2, V2.D2"). // V2 += V0 * a
		Raw("VFMLA V1.D2, V16.D2, V3.D2").
		Raw("VST1 [V2.D2, V3.D2], (R0)").
		Raw("ADD $32, R0, R0").
		Raw("ADD $32, R1, R1").
		Raw("SUB $4, R2").
		Raw("B block").
		// Scalar tail: remaining (n mod 4) elements via fused FMADDD.
		Raw("tail:").
		Raw("CBZ R2, done").
		Raw("FMOVD.P 8(R1), F1").     // src[i]
		Raw("FMOVD (R0), F2").        // dst[i]
		Raw("FMADDD F8, F2, F1, F2"). // F2 = F1*F8 + F2 = a*src + dst
		Raw("FMOVD.P F2, 8(R0)").
		Raw("SUB $1, R2").
		Raw("B tail").
		Raw("done:").
		Ret()
	return b.Func()
}

// minmaxKernel builds vminNEON / vmaxNEON(dst, src *float64, n int): dst[i] =
// op(dst[i], src[i]). word is the raw FMIN/FMAX.2D base encoding; scalarMn is
// the scalar (single-double) mnemonic FMIND/FMAXD for the tail.
func minmaxKernel(name string, word uint32, op string) *emit.Function {
	sig := arm64.Layout(
		[]string{"dst", "src", "n"},
		[]arm64.Type{arm64.Ptr, arm64.Ptr, arm64.Int64},
		nil, nil,
	)
	// dst pair loads into V0,V1 (contiguous); src pair into V2,V3 (contiguous).
	// V0 = op(V0, V2): Rm=2,Rn=0,Rd=0. V1 = op(V1, V3): Rm=3,Rn=1,Rd=1.
	vec0 := fmt.Sprintf("WORD $0x%08X", word|(2<<16)|(0<<5)|0)
	vec1 := fmt.Sprintf("WORD $0x%08X", word|(3<<16)|(1<<5)|1)
	b := arm64.NewFunc(name, sig, 0)
	b.LoadArg("dst", "R0").
		LoadArg("src", "R1").
		LoadArg("n", "R2").
		Raw("block:").
		Raw("CMP $4, R2").
		Raw("BLT tail").
		Raw("VLD1 (R0), [V0.D2, V1.D2]").
		Raw("VLD1 (R1), [V2.D2, V3.D2]").
		Raw(vec0). // V0 = op(V0, V2)
		Raw(vec1). // V1 = op(V1, V3)
		Raw("VST1 [V0.D2, V1.D2], (R0)").
		Raw("ADD $32, R0, R0").
		Raw("ADD $32, R1, R1").
		Raw("SUB $4, R2").
		Raw("B block").
		Raw("tail:").
		Raw("CBZ R2, done").
		Raw("FMOVD (R0), F0").
		Raw("FMOVD.P 8(R1), F1").
		Raw(op + "D F1, F0, F0"). // FMIND/FMAXD F0 = op(F0,F1)
		Raw("FMOVD.P F0, 8(R0)").
		Raw("SUB $1, R2").
		Raw("B tail").
		Raw("done:").
		Ret()
	return b.Func()
}
