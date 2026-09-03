// Package admission classifies the exported Set*/Add* sugar helpers under
// pkg/kubernetes by the builder contract's admission classes (ADR-038 §4).
//
// A helper is admissible when its body does one of:
//
//   - class a: append to a slice field or insert into a map field, directly
//     or through a local that the body assigns back to the field;
//   - class b: assign a pointer-typed field (the nil-init of a pointer
//     intermediate before writing through it is itself such an assignment);
//   - class c: write a composite literal with two or more fields, or a nested
//     literal.
//
// Anything else is inadmissible: a bare single-field forwarder, several bare
// forwarders in one body (two field writes without a literal are two
// forwarders, not a composite), a helper with no field write, a helper that
// only delegates. A body that assigns nil to a nillable field is inadmissible
// whatever else it does: that clears a field the caller did not name, which
// the purity rule (§4) forbids. nil is recognised as the literal, a conversion
// of it ((*T)(nil)), a local declared without a value or declared or assigned
// nil anywhere in the body, and any of those as a keyed value inside a struct
// literal being assigned; a nil that arrives through a function call or a
// parameter is not traced. A nil check on its own admits nothing; in
// particular the receiver guard (`if obj == nil { panic(...) }`) does not turn
// a bare forwarder into class b.
// The contract also exempts a fixed set of generic metadata helpers by name
// (§5); callers pass those in.
//
// A field write counts only when it is rooted in a parameter, directly or
// through a local that aliases one (spec := &o.Spec; labels := o.Labels): a
// write into a temporary the helper built itself reaches no caller-visible
// object and admits nothing. An append or map insert into a local counts as
// class a only when the body assigns that local to a rooted field later in
// source order.
//
// The classifier is syntactic with type information (go/packages): it never
// executes code, and it is deliberately conservative, so an unusual but
// legitimate helper shows up as inadmissible rather than slipping through.
// Its dataflow is bounded to what is described above; it does not follow
// control flow (a write-back on one branch only, a loop, a closure), calls,
// or aliases created any other way. A helper written to evade it is caught by
// its own unit test and by review, not by this package.
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
	// Composite is class c: composite literal of two or more fields, or a
	// nested literal.
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
		locals      = nilLocals(fn, info)
		rooted      = rootedNames(fn)        // parameters and locals aliasing them
		localOps    = map[string]token.Pos{} // local -> first append or map insert into it
		writtenBack = map[string]token.Pos{} // local -> last assignment of it to a field
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
			fieldWrite := isFieldWrite(lhs) && rooted[rootIdent(lhs)]
			if isMapIndex(lhs, info) || isAppend(rhs, info) {
				switch {
				case fieldWrite:
					appendOrMap = true
				case !isFieldWrite(lhs):
					if name := rootIdent(lhs); name != "" {
						if _, seen := localOps[name]; !seen {
							localOps[name] = s.Pos()
						}
					}
				}
			}
			if !fieldWrite {
				continue
			}
			writes[types.ExprString(lhs)] = true
			if t := info.TypeOf(lhs); t != nil {
				if _, ok := t.Underlying().(*types.Pointer); ok {
					ptrAssign = true
				}
			}
			if rhs == nil {
				continue
			}
			if id, ok := ast.Unparen(rhs).(*ast.Ident); ok {
				writtenBack[id.Name] = s.Pos()
			}
			if nilClear == "" && isNillable(info.TypeOf(lhs)) && isNilValue(rhs, info, locals) {
				nilClear = types.ExprString(lhs)
			}
			if lit := compositeOf(rhs); lit != nil {
				if len(lit.Elts) >= 2 || hasNestedLiteral(lit) {
					bigLiteral = true
				}
				if key := nilInLiteral(lit, info, locals); key != "" && nilClear == "" {
					nilClear = types.ExprString(lhs) + "." + key
				}
			}
		}
		return true
	})
	for name, opPos := range localOps {
		if back, ok := writtenBack[name]; ok && back > opPos {
			appendOrMap = true
		}
	}

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
		return Inadmissible, fmt.Sprintf("%d bare field writes and no composite literal (a forwarder per field, not class c)", len(writes))
	case len(writes) == 1:
		return Inadmissible, "single bare field assignment (a forwarder for the struct field)"
	}
	return Inadmissible, "no field write (delegation or no-op)"
}

// isFieldWrite reports whether lhs writes through a selector or dereference,
// or indexes into one, rather than into a local variable: o.Labels[k] is a
// field write, labels[k] is not.
func isFieldWrite(lhs ast.Expr) bool {
	switch e := ast.Unparen(lhs).(type) {
	case *ast.SelectorExpr, *ast.StarExpr:
		return true
	case *ast.IndexExpr:
		return isFieldWrite(e.X)
	}
	return false
}

// rootIdent returns the name of the identifier at the root of a selector,
// index, dereference or address-of chain (o in o.Spec.Items[i], labels in
// labels[k], spec in *spec), or "" when the chain is not rooted in one.
func rootIdent(e ast.Expr) string {
	for {
		switch v := e.(type) {
		case *ast.Ident:
			return v.Name
		case *ast.SelectorExpr:
			e = v.X
		case *ast.IndexExpr:
			e = v.X
		case *ast.StarExpr:
			e = v.X
		case *ast.ParenExpr:
			e = v.X
		case *ast.UnaryExpr:
			if v.Op != token.AND {
				return ""
			}
			e = v.X
		default:
			return ""
		}
	}
}

// rootedNames returns the names through which fn can reach a caller's
// object: its parameters, and every local whose defining or assigned value
// is a selector, index, dereference or address-of chain rooted in one of
// them (spec := &o.Spec; labels := o.Labels). A local built from anything
// else (tmp := &Obj{}, x := f()) is a temporary: writes into it reach no
// caller-visible object and admit nothing.
func rootedNames(fn *ast.FuncDecl) map[string]bool {
	rooted := map[string]bool{}
	for _, field := range fn.Type.Params.List {
		for _, name := range field.Names {
			rooted[name.Name] = true
		}
	}
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		switch s := n.(type) {
		case *ast.DeclStmt:
			gd, ok := s.Decl.(*ast.GenDecl)
			if !ok || gd.Tok != token.VAR {
				return true
			}
			for _, spec := range gd.Specs {
				vs, ok := spec.(*ast.ValueSpec)
				if !ok || len(vs.Values) != len(vs.Names) {
					continue
				}
				for i, id := range vs.Names {
					if rooted[rootIdent(vs.Values[i])] {
						rooted[id.Name] = true
					}
				}
			}
		case *ast.AssignStmt:
			if len(s.Lhs) != len(s.Rhs) {
				return true
			}
			for i, lhs := range s.Lhs {
				if id, ok := lhs.(*ast.Ident); ok && rooted[rootIdent(s.Rhs[i])] {
					rooted[id.Name] = true
				}
			}
		}
		return true
	})
	return rooted
}

// nilInLiteral returns the key of the first keyed element of lit, or of a
// literal nested in it, whose value is nil; "" when there is none.
func nilInLiteral(lit *ast.CompositeLit, info *types.Info, locals map[string]bool) string {
	for _, elt := range lit.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		if isNilValue(kv.Value, info, locals) {
			return types.ExprString(kv.Key)
		}
		if inner := compositeOf(kv.Value); inner != nil {
			if key := nilInLiteral(inner, info, locals); key != "" {
				return types.ExprString(kv.Key) + "." + key
			}
		}
	}
	return ""
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

// isNilValue reports whether e is the nil identifier, a conversion of a nil
// value to a named type ((*T)(nil)), or a local that nilLocals proved nil.
func isNilValue(e ast.Expr, info *types.Info, locals map[string]bool) bool {
	switch v := ast.Unparen(e).(type) {
	case *ast.Ident:
		return v.Name == "nil" || locals[v.Name]
	case *ast.CallExpr:
		if tv, ok := info.Types[v.Fun]; ok && tv.IsType() && len(v.Args) == 1 {
			return isNilValue(v.Args[0], info, locals)
		}
	}
	return false
}

// isNillable reports whether t can hold nil (pointer, map, slice, interface,
// chan, func), so that assigning a nil value to it is a clear rather than a
// zero-value write to a scalar.
func isNillable(t types.Type) bool {
	if t == nil {
		return false
	}
	switch t.Underlying().(type) {
	case *types.Pointer, *types.Map, *types.Slice, *types.Interface, *types.Chan, *types.Signature:
		return true
	}
	return false
}

// nilLocals returns the names of locals in fn that are declared without a
// value (var x *T) or assigned a nil value anywhere in the body. A local
// reassigned to something else later still counts: the classifier is
// deliberately conservative, and a helper that needs such a local is better
// written without it.
func nilLocals(fn *ast.FuncDecl, info *types.Info) map[string]bool {
	locals := map[string]bool{}
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		switch s := n.(type) {
		case *ast.DeclStmt:
			gd, ok := s.Decl.(*ast.GenDecl)
			if !ok || gd.Tok != token.VAR {
				return true
			}
			for _, spec := range gd.Specs {
				vs, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for i, id := range vs.Names {
					switch {
					case len(vs.Values) == 0:
						if isNillable(info.TypeOf(id)) {
							locals[id.Name] = true
						}
					case len(vs.Values) == len(vs.Names) && isNilValue(vs.Values[i], info, locals):
						locals[id.Name] = true
					}
				}
			}
		case *ast.AssignStmt:
			if len(s.Lhs) != len(s.Rhs) {
				return true
			}
			for i, lhs := range s.Lhs {
				if id, ok := lhs.(*ast.Ident); ok && isNilValue(s.Rhs[i], info, locals) {
					locals[id.Name] = true
				}
			}
		}
		return true
	})
	return locals
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
