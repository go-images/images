package images

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"strings"
	"testing"
)

// exported returns the exported top-level names (types, constants, functions;
// methods excluded) declared by the non-test files of dir.
func exported(t *testing.T, dir, pkg string) map[string]bool {
	t.Helper()
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, dir, func(fi os.FileInfo) bool {
		return !strings.HasSuffix(fi.Name(), "_test.go")
	}, 0)
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{}
	for _, f := range pkgs[pkg].Files {
		for _, d := range f.Decls {
			switch d := d.(type) {
			case *ast.FuncDecl:
				if d.Recv == nil && d.Name.IsExported() {
					names[d.Name.Name] = true
				}
			case *ast.GenDecl:
				for _, s := range d.Specs {
					switch s := s.(type) {
					case *ast.TypeSpec:
						if s.Name.IsExported() {
							names[s.Name.Name] = true
						}
					case *ast.ValueSpec:
						for _, n := range s.Names {
							if n.IsExported() {
								names[n.Name] = true
							}
						}
					}
				}
			}
		}
	}
	return names
}

// TestRootForwardsAllOfOps keeps the root package a superset of ./ops: every
// exported ops name must exist here, so moving the implementation broke no
// caller of github.com/go-images/images.
func TestRootForwardsAllOfOps(t *testing.T) {
	ops := exported(t, "ops", "ops")
	root := exported(t, ".", "images")
	if len(ops) < 50 {
		t.Fatalf("parsed only %d exported names from ./ops; the parse is wrong", len(ops))
	}
	for n := range ops {
		if !root[n] {
			t.Errorf("ops.%s has no forwarder in package images (regenerate ops_forward.go)", n)
		}
	}
}

func deps(t *testing.T, pkg string) map[string]bool {
	t.Helper()
	out, err := exec.Command("go", "list", "-deps", pkg).Output()
	if err != nil {
		t.Skipf("go list unavailable: %v", err)
	}
	m := map[string]bool{}
	for _, l := range strings.Fields(string(out)) {
		m[l] = true
	}
	return m
}

// TestOpsLinksNoCodec is the reason ./ops exists: importing it must not import
// a decoder, because every decoder registers itself with image.RegisterFormat
// in an init function, and the linker keeps whatever an init reaches. The
// root package is the planted positive: it must show the codecs, or the
// check could not see them.
func TestOpsLinksNoCodec(t *testing.T) {
	codecs := []string{"github.com/go-gfx/gfx/codec", "image/jpeg", "image/png", "image/gif"}
	root := deps(t, ".")
	for _, c := range codecs {
		if !root[c] {
			t.Fatalf("control failed: package images does not import %s, so this check sees nothing", c)
		}
	}
	ops := deps(t, "./ops")
	for _, c := range codecs {
		if ops[c] {
			t.Errorf("package ops imports %s: every importer would link its decoder", c)
		}
	}
}
