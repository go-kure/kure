package admission

import (
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"strings"
	"testing"
)

// TestExprText pins what a reason prints: every expression as
// types.ExprString prints it, and a composite literal written out where
// ExprString abbreviates it to T{…}, so two paths that differ only inside a
// literal read apart.
func TestExprText(t *testing.T) {
	cases := []struct{ src, want string }{
		{"d.Plans[len([...]int{1})].Policy.Limit", "d.Plans[len([...]int{1})].Policy.Limit"},
		{"d.Plans[len([...]int{1, 2})].Policy.Limit", "d.Plans[len([...]int{1, 2})].Policy.Limit"},
		{"o.Spec.Groups[string([]byte{'a'})]", "o.Spec.Groups[string([]byte{'a'})]"},
		{"Spec{Ref: nil, Items: []string{\"x\"}}", "Spec{Ref: nil, Items: []string{\"x\"}}"},
		{"Spec{\n\tRef: r,\n\tItems: xs,\n}", "Spec{Ref: r, Items: xs}"},
		{"[][]int{{1}, {2, 3}}", "[][]int{{1}, {2, 3}}"},
		{"map[string][]int{\"a\": {1}}", "map[string][]int{\"a\": {1}}"},
		{"&Ref{Name: n}", "&Ref{Name: n}"},
		{"Policy{}", "Policy{}"},
		{"struct{A [unsafe.Sizeof(d.Any) % 1 + 1]int; Name string; Mode Mode}{Name: n, Mode: m}",
			"struct{A [unsafe.Sizeof(d.Any) % 1 + 1]int; Name string; Mode Mode}{Name: n, Mode: m}"},
		{"len([1]func(){func() {}})", "len([1]func(){(func() literal)})"},
		{"f(xs...)", "f(xs...)"},
		{"xs[i:j:k]", "xs[i:j:k]"},
		{"v.(type)", "v.(type)"},
		{"v.(T)", "v.(T)"},
		{"func(a, b int) (s string, err error)", "func(a, b int) (s string, err error)"},
		{"interface{M(int) string; N()}", "interface{M(int) string; N()}"},
		{"chan<- int", "chan<- int"},
		{"<-chan int", "<-chan int"},
		{"g[int, string](x)", "g[int, string](x)"},
		{"(*o).Spec.Payload", "(*o).Spec.Payload"},
	}
	for _, tc := range cases {
		e, err := parser.ParseExpr(tc.src)
		if err != nil {
			t.Fatalf("parse %q: %v", tc.src, err)
		}
		if got := exprText(e); got != tc.want {
			t.Errorf("exprText(%q) = %q, want %q", tc.src, got, tc.want)
		}
	}
}

// TestExprText_MatchesExprString reads every expression of the two fixture
// sources, fixtureSource and grammarFixtureSource, the helper bodies the
// reasons print: one without a composite literal prints as types.ExprString
// prints it, and one with a literal prints no abbreviation. Skipped: the two
// forms ExprString does not print, a type switch guard, v.(type), and a
// key-value element, which stands in a literal only; and a string that
// spells the abbreviation itself.
func TestExprText_MatchesExprString(t *testing.T) {
	fset := token.NewFileSet()
	plain, literal := 0, 0
	for _, src := range []struct{ name, source string }{
		{"fixture.go", fixtureSource},
		{"grammar.go", grammarFixtureSource},
	} {
		file, err := parser.ParseFile(fset, src.name, src.source, 0)
		if err != nil {
			t.Fatalf("%s: %v", src.name, err)
		}
		for _, decl := range file.Decls {
			ast.Inspect(decl, func(n ast.Node) bool {
				e, ok := n.(ast.Expr)
				if !ok {
					return true
				}
				got := exprText(e)
				switch {
				case holds(e, func(n ast.Node) bool { x, ok := n.(*ast.TypeAssertExpr); return ok && x.Type == nil }):
				case holds(e, func(n ast.Node) bool { x, ok := n.(*ast.BasicLit); return ok && strings.Contains(x.Value, "…") }):
				case holds(e, func(n ast.Node) bool { _, ok := n.(*ast.KeyValueExpr); return ok }) &&
					!holds(e, func(n ast.Node) bool { _, ok := n.(*ast.CompositeLit); return ok }):
				case holds(e, func(n ast.Node) bool { _, ok := n.(*ast.CompositeLit); return ok }):
					literal++
					if strings.Contains(got, "…") || strings.Contains(got, "(ast:") {
						t.Errorf("%s: exprText(%s) = %q abbreviates", fset.Position(e.Pos()), types.ExprString(e), got)
					}
				default:
					plain++
					if want := types.ExprString(e); got != want {
						t.Errorf("%s: exprText = %q, ExprString = %q", fset.Position(e.Pos()), got, want)
					}
				}
				return true
			})
		}
	}
	t.Logf("read %d plain and %d literal expressions", plain, literal)
	if plain == 0 || literal == 0 {
		t.Fatalf("read %d plain and %d literal expressions; both must be read", plain, literal)
	}
}

// holds reports whether any node of e satisfies is.
func holds(e ast.Expr, is func(ast.Node) bool) bool {
	found := false
	ast.Inspect(e, func(n ast.Node) bool {
		found = found || (n != nil && is(n))
		return !found
	})
	return found
}
