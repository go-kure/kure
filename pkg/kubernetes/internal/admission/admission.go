// Package admission classifies the exported Set*/Add* sugar helpers under
// pkg/kubernetes by the builder contract's admission classes (ADR-038 §4).
//
// A helper is admissible when its body does one of:
//
//   - class a: append to a slice or insert into a map;
//   - class b: assign a pointer-typed field (the nil-init of a pointer
//     intermediate before writing through it is itself such an assignment);
//   - class c: write a composite literal with two or more fields, or a nested
//     literal, or write two or more distinct fields.
//
// Anything else is inadmissible: a bare single-field forwarder, a helper with
// no field write, a helper that only delegates. A body that assigns the
// literal nil to a field is inadmissible whatever else it does: that clears a
// field the caller did not name, which the purity rule (§4) forbids. A nil
// check on its own admits nothing; in particular the receiver guard
// (`if obj == nil { panic(...) }`) does not turn a bare forwarder into class b.
// The contract also exempts a fixed set of generic metadata helpers by name
// (§5); callers pass those in.
//
// The classifier is syntactic with type information (go/packages): it never
// executes code, and it is deliberately conservative, so an unusual but
// legitimate helper shows up as inadmissible rather than slipping through.
package admission

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"golang.org/x/tools/go/packages"

	"github.com/go-kure/kure/pkg/errors"
)

// Class is the admission class of one helper.
type Class int

const (
	// Inadmissible is a helper that matches no admissible class.
	Inadmissible Class = iota
	// Append is class a: slice append or map insert.
	Append
	// Pointer is class b: pointer-typed field assignment or nil-init.
	Pointer
	// Composite is class c: composite literal of two or more fields, a nested
	// literal, or two or more distinct field writes.
	Composite
	// Exempt is a helper admitted by name (the generic metadata helpers).
	Exempt
)

func (c Class) String() string {
	switch c {
	case Inadmissible:
		return "inadmissible"
	case Append:
		return "append"
	case Pointer:
		return "pointer"
	case Composite:
		return "composite"
	case Exempt:
		return "exempt"
	}
	return fmt.Sprintf("Class(%d)", int(c))
}

// Finding is the classification of one exported Set*/Add* helper.
type Finding struct {
	// Package is the import path of the helper's package.
	Package string
	// Name is the helper's function name.
	Name string
	// Pos is the position of the declaration.
	Pos token.Position
	// Class is the admission class.
	Class Class
	// Reason explains the class in one clause.
	Reason string
}

// Key returns "<package>.<Name>", the form used in exclusion lists.
func (f Finding) Key() string { return f.Package + "." + f.Name }

// Options configures Classify.
type Options struct {
	// Dir is the directory go/packages resolves Patterns from.
	Dir string
	// Patterns are go/packages patterns, e.g. "./...".
	Patterns []string
	// Exempt holds Finding keys admitted by name regardless of body.
	Exempt map[string]bool
	// Env overrides the environment for go/packages; nil inherits the process
	// environment.
	Env []string
}

// Classify loads the packages matching opts.Patterns and classifies every
// exported top-level function named Set<Upper>... or Add<Upper>... in their
// non-test, non-generated files. Findings are sorted by package then name.
func Classify(opts Options) ([]Finding, error) {
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedSyntax |
			packages.NeedTypes | packages.NeedTypesInfo | packages.NeedImports,
		Dir:   opts.Dir,
		Env:   opts.Env,
		Tests: false,
	}
	pkgs, err := packages.Load(cfg, opts.Patterns...)
	if err != nil {
		return nil, errors.Wrap(err, "load packages")
	}
	var loadErrs []string
	packages.Visit(pkgs, nil, func(p *packages.Package) {
		for _, e := range p.Errors {
			loadErrs = append(loadErrs, e.Error())
		}
	})
	if len(loadErrs) > 0 {
		return nil, errors.Errorf("load packages: %s", strings.Join(loadErrs, "; "))
	}

	var findings []Finding
	for _, p := range pkgs {
		for _, file := range p.Syntax {
			name := filepath.Base(p.Fset.Position(file.Pos()).Filename)
			if strings.HasPrefix(name, "zz_generated") {
				continue
			}
			for _, decl := range file.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || !isSugarHelper(fn) {
					continue
				}
				f := Finding{
					Package: p.PkgPath,
					Name:    fn.Name.Name,
					Pos:     p.Fset.Position(fn.Pos()),
				}
				if opts.Exempt[f.Key()] {
					f.Class, f.Reason = Exempt, "admitted by name (contract §5)"
				} else {
					f.Class, f.Reason = classify(fn, p.TypesInfo)
				}
				findings = append(findings, f)
			}
		}
	}
	sort.Slice(findings, func(i, j int) bool {
		if findings[i].Package != findings[j].Package {
			return findings[i].Package < findings[j].Package
		}
		return findings[i].Name < findings[j].Name
	})
	return findings, nil
}

// isSugarHelper reports whether fn is an exported top-level function whose
// name is Set or Add followed by an upper-case letter (so Settle and Address
// are not helpers).
func isSugarHelper(fn *ast.FuncDecl) bool {
	if fn.Recv != nil || fn.Body == nil || !fn.Name.IsExported() {
		return false
	}
	name := fn.Name.Name
	for _, prefix := range []string{"Set", "Add"} {
		if strings.HasPrefix(name, prefix) && len(name) > len(prefix) {
			return unicode.IsUpper(rune(name[len(prefix)]))
		}
	}
	return false
}

// classify inspects fn's body and returns the first matching class in the
// order append, pointer, composite; otherwise Inadmissible with the reason. A
// literal-nil field assignment is checked first and makes the whole body
// inadmissible, whatever else it does.
func classify(fn *ast.FuncDecl, info *types.Info) (Class, string) {
	var (
		appendOrMap bool
		ptrAssign   bool
		bigLiteral  bool
		nilClear    string
		writes      = map[string]bool{}
	)
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		s, ok := n.(*ast.AssignStmt)
		if !ok {
			return true
		}
		for i, lhs := range s.Lhs {
			var rhs ast.Expr
			if len(s.Rhs) == len(s.Lhs) {
				rhs = s.Rhs[i]
			} else if len(s.Rhs) == 1 {
				rhs = s.Rhs[0]
			}
			if isMapIndex(lhs, info) || isAppend(rhs, info) {
				appendOrMap = true
			}
			if !isFieldWrite(lhs) {
				continue
			}
			writes[types.ExprString(lhs)] = true
			if rhs != nil && isNilIdent(rhs) && nilClear == "" {
				nilClear = types.ExprString(lhs)
			}
			if t := info.TypeOf(lhs); t != nil {
				if _, ok := t.Underlying().(*types.Pointer); ok {
					ptrAssign = true
				}
			}
			if lit := compositeOf(rhs); lit != nil && (len(lit.Elts) >= 2 || hasNestedLiteral(lit)) {
				bigLiteral = true
			}
		}
		return true
	})

	switch {
	case nilClear != "":
		return Inadmissible, fmt.Sprintf("assigns nil to %s, a field the caller did not name (purity §4)", nilClear)
	case appendOrMap:
		return Append, "slice append or map insert (class a)"
	case ptrAssign:
		return Pointer, "pointer-typed field assignment (class b)"
	case bigLiteral:
		return Composite, "composite literal with two or more fields or nested (class c)"
	case len(writes) >= 2:
		return Composite, fmt.Sprintf("%d distinct field writes (class c)", len(writes))
	case len(writes) == 1:
		return Inadmissible, "single bare field assignment (a forwarder for the struct field)"
	}
	return Inadmissible, "no field write (delegation or no-op)"
}

// isFieldWrite reports whether lhs writes through a selector, index or
// dereference rather than to a local variable.
func isFieldWrite(lhs ast.Expr) bool {
	switch lhs.(type) {
	case *ast.SelectorExpr, *ast.IndexExpr, *ast.StarExpr:
		return true
	}
	return false
}

func isMapIndex(lhs ast.Expr, info *types.Info) bool {
	idx, ok := lhs.(*ast.IndexExpr)
	if !ok {
		return false
	}
	t := info.TypeOf(idx.X)
	if t == nil {
		return false
	}
	_, isMap := t.Underlying().(*types.Map)
	return isMap
}

func isAppend(rhs ast.Expr, info *types.Info) bool {
	call, ok := ast.Unparen(rhs).(*ast.CallExpr)
	if !ok {
		return false
	}
	id, ok := call.Fun.(*ast.Ident)
	if !ok {
		return false
	}
	b, ok := info.Uses[id].(*types.Builtin)
	return ok && b.Name() == "append"
}

func isNilIdent(e ast.Expr) bool {
	id, ok := ast.Unparen(e).(*ast.Ident)
	return ok && id.Name == "nil"
}

// compositeOf returns the composite literal in rhs, looking through & and
// parentheses, or nil.
func compositeOf(rhs ast.Expr) *ast.CompositeLit {
	if rhs == nil {
		return nil
	}
	e := ast.Unparen(rhs)
	if u, ok := e.(*ast.UnaryExpr); ok && u.Op == token.AND {
		e = ast.Unparen(u.X)
	}
	lit, _ := e.(*ast.CompositeLit)
	return lit
}

func hasNestedLiteral(lit *ast.CompositeLit) bool {
	for _, elt := range lit.Elts {
		v := elt
		if kv, ok := elt.(*ast.KeyValueExpr); ok {
			v = kv.Value
		}
		if compositeOf(v) != nil {
			return true
		}
	}
	return false
}

// ReadExclusions parses an exclusion list: one Finding key per line, blank
// lines and lines starting with # ignored. Keys are returned in file order.
func ReadExclusions(path string) ([]string, error) {
	data, err := os.ReadFile(path) //nolint:gosec // path comes from the test, not from input
	if err != nil {
		return nil, errors.Wrapf(err, "read exclusions %s", path)
	}
	var keys []string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		keys = append(keys, line)
	}
	return keys, nil
}
