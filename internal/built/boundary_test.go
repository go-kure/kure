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
