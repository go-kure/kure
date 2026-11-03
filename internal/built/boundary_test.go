package built

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// A caller's object is read in one place. These checks pin the rule that keeps
// a hostile object from crashing kure (go-kure/kure#1201): the package reads a
// List's items by reflection (items.go) and calls a method of a caller's
// object only through the guard (held.go), and the files that read never
// format a value with %v, which has no end for a value that holds itself.

func parseNonTestFiles(t *testing.T, dir string) map[string]*ast.File {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join(dir, "*.go"))
	if err != nil {
		t.Fatal(err)
	}
	files := map[string]*ast.File{}
	for _, p := range paths {
		if strings.HasSuffix(p, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(token.NewFileSet(), p, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", p, err)
		}
		files[filepath.Base(p)] = f
	}
	return files
}

func TestBoundary_NoApimachineryListReader(t *testing.T) {
	for name, f := range parseNonTestFiles(t, ".") {
		for _, imp := range f.Imports {
			if path, _ := strconv.Unquote(imp.Path.Value); path == "k8s.io/apimachinery/pkg/api/meta" {
				t.Errorf("%s imports %s, whose list readers format the object they read", name, path)
			}
		}
	}
}

func TestBoundary_GetObjectKindOnlyInTheGuard(t *testing.T) {
	for name, f := range parseNonTestFiles(t, ".") {
		if name == "held.go" {
			continue
		}
		ast.Inspect(f, func(n ast.Node) bool {
			if sel, ok := n.(*ast.SelectorExpr); ok && sel.Sel.Name == "GetObjectKind" {
				t.Errorf("%s calls GetObjectKind outside held.go", name)
			}
			return true
		})
	}
}

// A caller's object is marshalled only through marshalHeld, which guards the
// methods the encoder calls (MarshalJSON, IsZero). The one other call writes a
// value read back from JSON, which holds plain Go values and no caller's type.
func TestBoundary_MarshalOnlyInTheGuard(t *testing.T) {
	const plainValueCalls = 1 // built.go: the ignored-by-kustomize annotation value
	others := 0
	for name, f := range parseNonTestFiles(t, ".") {
		if name == "held.go" {
			continue
		}
		ast.Inspect(f, func(n ast.Node) bool {
			if call, ok := n.(*ast.CallExpr); ok {
				if sel, ok := call.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "Marshal" {
					if id, ok := sel.X.(*ast.Ident); ok && id.Name == "json" {
						others++
						if name != "built.go" {
							t.Errorf("%s calls json.Marshal outside held.go", name)
						}
					}
				}
			}
			return true
		})
	}
	if others != plainValueCalls {
		t.Errorf("%d json.Marshal calls outside held.go, want %d: route a caller's object through marshalHeld", others, plainValueCalls)
	}
}

func TestBoundary_ReadingFilesDoNotFormatWithV(t *testing.T) {
	files := parseNonTestFiles(t, ".")
	for _, name := range []string{"held.go", "items.go"} {
		ast.Inspect(files[name], func(n ast.Node) bool {
			lit, ok := n.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			s, _ := strconv.Unquote(lit.Value)
			for _, verb := range []string{"%v", "%+v", "%#v"} {
				if strings.Contains(s, verb) {
					t.Errorf("%s has a format string with %s: %s", name, verb, lit.Value)
				}
			}
			return true
		})
	}
}
