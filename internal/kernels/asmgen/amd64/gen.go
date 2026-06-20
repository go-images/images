//go:build ignore

// Command gen produces simd_amd64.s — the SSE2 float64 multiply-accumulate
// (axpy) and elementwise min/max kernels — via go-asmgen. Run with
// `go run gen.go` or `go generate` from the kernels package.
//
// SSE2 is part of the amd64 baseline (GOAMD64=v1), so every kernel is always
// callable with no CPU-feature branch.
//
//	axpySSE2(dst, src *float64, a float64, n int)
//	  dst[i] += a*src[i] for i in [0,n). The Gaussian separable pass applies one
//	  kernel tap weight a to a whole shifted channel line. The packed form is
//	  MULPD then ADDPD — exactly the two instructions the gc compiler emits for
//	  the scalar oracle at the GOAMD64=v1 baseline (it does NOT fuse there), so
//	  at v1 the kernel is bit-identical; at v3 the scalar oracle itself fuses to
//	  VFMADD, a <=0.5 ULP regrouping, so axpy is validated to a tight relative
//	  tolerance rather than held bit-identical across GOAMD64 levels.
//
//	vminSSE2 / vmaxSSE2(dst, src *float64, n int)
//	  dst[i] = min/max(dst[i], src[i]) for i in [0,n), the morphology per-window
//	  reduction. MINPD/MAXPD match the scalar `if` oracle bit-for-bit on the
//	  finite, uint8-derived values morphology runs on (no NaN / signed-zero
//	  cases, where x86 MIN/MAX semantics would otherwise differ).
package main

import (
	"fmt"
	"os"

	"github.com/go-asmgen/asmgen/amd64"
	"github.com/go-asmgen/asmgen/emit"
)

func main() {
	f := emit.NewFile("amd64")
	f.Add(axpyKernel())
	f.Add(minmaxKernel("vminSSE2", "MINPD", "MINSD"))
	f.Add(minmaxKernel("vmaxSSE2", "MAXPD", "MAXSD"))
	if err := os.WriteFile("simd_amd64.s", []byte(f.String()), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("wrote simd_amd64.s")
}

// axpyKernel builds axpySSE2(dst, src *float64, a float64, n int): dst[i] +=
// a*src[i]. The scalar a is broadcast into both lanes of X0 (a-vector); the loop
// processes 4 doubles (two XMM pairs) per iteration, with a scalar tail.
func axpyKernel() *emit.Function {
	sig := amd64.Layout(
		[]string{"dst", "src", "a", "n"},
		[]amd64.Type{amd64.Ptr, amd64.Ptr, amd64.Float64, amd64.Int64},
		nil, nil,
	)
	b := amd64.NewFunc("axpySSE2", sig, 0)
	b.LoadArg("dst", "AX").
		LoadArg("src", "BX").
		LoadArg("a", "X0").
		LoadArg("n", "CX").
		// Broadcast a into both lanes of X0: [a, a].
		Raw("MOVDDUP X0, X0").
		// Main loop: 4 doubles (32 bytes) per iteration.
		Raw("block:").
		Raw("CMPQ CX, $4").
		Raw("JL tail").
		Raw("MOVUPD (BX), X1").   // src[0:2]
		Raw("MOVUPD 16(BX), X2"). // src[2:4]
		Raw("MULPD X0, X1").      // a*src
		Raw("MULPD X0, X2").
		Raw("MOVUPD (AX), X3").   // dst[0:2]
		Raw("MOVUPD 16(AX), X4"). // dst[2:4]
		Raw("ADDPD X1, X3").      // dst + a*src
		Raw("ADDPD X2, X4").
		Raw("MOVUPD X3, (AX)").
		Raw("MOVUPD X4, 16(AX)").
		Raw("ADDQ $32, AX").
		Raw("ADDQ $32, BX").
		Raw("SUBQ $4, CX").
		Raw("JMP block").
		// Scalar tail: remaining (n mod 4) elements.
		Raw("tail:").
		Raw("TESTQ CX, CX").
		Raw("JZ done").
		Raw("MOVSD (BX), X1").
		Raw("MULSD X0, X1").
		Raw("MOVSD (AX), X2").
		Raw("ADDSD X1, X2").
		Raw("MOVSD X2, (AX)").
		Raw("ADDQ $8, AX").
		Raw("ADDQ $8, BX").
		Raw("DECQ CX").
		Raw("JMP tail").
		Raw("done:").
		Ret()
	return b.Func()
}

// minmaxKernel builds vminSSE2 / vmaxSSE2(dst, src *float64, n int): dst[i] =
// op(dst[i], src[i]). packed is the 2-lane op (MINPD/MAXPD), scalar the 1-lane
// tail op (MINSD/MAXSD).
func minmaxKernel(name, packed, scalar string) *emit.Function {
	sig := amd64.Layout(
		[]string{"dst", "src", "n"},
		[]amd64.Type{amd64.Ptr, amd64.Ptr, amd64.Int64},
		nil, nil,
	)
	b := amd64.NewFunc(name, sig, 0)
	b.LoadArg("dst", "AX").
		LoadArg("src", "BX").
		LoadArg("n", "CX").
		Raw("block:").
		Raw("CMPQ CX, $4").
		Raw("JL tail").
		Raw("MOVUPD (AX), X0").
		Raw("MOVUPD 16(AX), X1").
		Raw("MOVUPD (BX), X2").
		Raw("MOVUPD 16(BX), X3").
		Raw(packed + " X2, X0"). // X0 = op(X0, src)
		Raw(packed + " X3, X1").
		Raw("MOVUPD X0, (AX)").
		Raw("MOVUPD X1, 16(AX)").
		Raw("ADDQ $32, AX").
		Raw("ADDQ $32, BX").
		Raw("SUBQ $4, CX").
		Raw("JMP block").
		Raw("tail:").
		Raw("TESTQ CX, CX").
		Raw("JZ done").
		Raw("MOVSD (AX), X0").
		Raw("MOVSD (BX), X1").
		Raw(scalar + " X1, X0").
		Raw("MOVSD X0, (AX)").
		Raw("ADDQ $8, AX").
		Raw("ADDQ $8, BX").
		Raw("DECQ CX").
		Raw("JMP tail").
		Raw("done:").
		Ret()
	return b.Func()
}
