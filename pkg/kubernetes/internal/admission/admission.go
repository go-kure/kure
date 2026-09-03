// Package admission classifies the exported Set*/Add* sugar helpers under
// pkg/kubernetes by the builder contract's admission classes (ADR-038 §4).
//
// A helper is admissible when its body does one of:
//
//   - class a: append to a slice field or insert into a map field, directly
//     or through a local that the body assigns back to the field;
//   - class b: assign a pointer-typed field (the nil-init of a pointer
//     intermediate before writing through it is itself such an assignment);
//   - class c: write an upstream struct literal with two or more fields, or a
//     nested literal. A slice or map literal is not class c: it replaces a
//     collection rather than composing a value.
//
// Every admitted operation must carry a value the caller supplied: an append
// or map insert of a constant, a struct literal built only from constants, and
// a pointer allocated with new(T) that nothing is written through, each set a
// value the caller never named (§4). The zero-value init that guards a nil
// field (make, new, an empty literal, followed by a write through it) is not
// such a value.
//
// A helper that returns before writing is inadmissible whatever its body
// does: `if obj == nil { return }` swallows the nil receiver §4 says must
// panic, and `if v == "" { return }` is the optional-argument idiom §4
// forbids. Anything else is inadmissible: a bare single-field forwarder, several bare
// forwarders in one body (two field writes without a literal are two
// forwarders, not a composite), a helper with no field write, a helper that
// only delegates. A helper that returns anything is inadmissible whatever its
// body does: the purity rule (§4) allows no error return, and a nil receiver
// panics rather than being reported, so a result slot has nothing to carry.
// Validation that does not surface as a result is not detected. An admitted
// operation does not cover the rest of the body: a bare write next to an
// append, a pointer assignment or a literal is inadmissible when its value is
// not a parameter (a literal, a call, a computed value, a local of the body's
// own), because that sets a field the caller did not name (§4). A bare write
// of a parameter next to an admitted operation forwards a value the caller
// supplied and leaves the class alone. Writing through a pointer intermediate
// the body just initialised (o.Spec.Ref.Name or *o.Spec.Replicas), a make of
// the map or slice the body then inserts into or appends to, and assigning an
// appended or inserted-into local back to its field are each part of their
// class; none is bare. A body that assigns nil to a nillable field is inadmissible
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
// A field write counts only when, at the position of the write, it is rooted
// in a parameter, directly or through a local that aliases one (spec :=
// &o.Spec; labels := o.Labels): a write into a temporary the helper built
// itself reaches no caller-visible object and admits nothing. An append or
// map insert into a local counts as class a only when the body assigns that
// same local to a rooted field later in source order. Locals are tracked as
// type-checker objects, so a shadowing declaration is a different local and
// a name shared by two blocks conflates nothing.
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
	if fn.Type.Results != nil && len(fn.Type.Results.List) > 0 {
		return Inadmissible, "returns a value; sugar returns nothing and panics on a nil receiver instead of reporting it (purity §4)"
	}
	if hasEarlyReturn(fn.Body) {
		return Inadmissible, "returns early instead of writing; a nil receiver panics and sugar has no conditional no-op (purity §4)"
	}
	var (
		appendOrMap bool
		ptrAssign   bool
		bigLiteral  bool
		nilClear    string
		writes      = map[string]bool{}
		ptrPaths    = map[string]bool{}         // pointer fields assigned, by expression, for writes through them
		ptrInit     = map[string]bool{}         // of those, the ones assigned a bare new(T)/&T{} with no value
		ptrUsed     = map[string]bool{}         // pointer fields a later write goes through
		bareWrites  = map[string]types.Object{} // bare field writes -> the local assigned, if any
		defaulted   []string                    // admitted operations carrying a value the caller did not supply
		locals      = nilLocals(fn, info)
		rooted      = rootedObjects(fn, info)      // parameters writes reach the caller through, by position
		supplied    = callerValues(fn, info)       // parameters and locals carrying one, for value provenance
		localOps    = map[types.Object]token.Pos{} // local -> first append or map insert into it
		writtenBack = map[types.Object]token.Pos{} // local -> last assignment of it to a field
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
			fieldWrite := isFieldWrite(lhs) && rooted.at(rootObj(lhs, info), s.Pos())
			if isMapIndex(lhs, info) || isAppend(rhs, info) {
				switch {
				case fieldWrite:
					appendOrMap = true
				case !isFieldWrite(lhs):
					if obj := rootObj(lhs, info); obj != nil {
						if _, seen := localOps[obj]; !seen {
							localOps[obj] = s.Pos()
						}
					}
				}
			}
			if !fieldWrite {
				continue
			}
			lhsPath := types.ExprString(lhs)
			writes[lhsPath] = true
			if p, through := throughPointer(lhsPath, ptrPaths); through {
				ptrUsed[p] = true
			}
			isPtr := false
			if t := info.TypeOf(lhs); t != nil {
				if _, ok := t.Underlying().(*types.Pointer); ok {
					ptrAssign, isPtr = true, true
					ptrPaths[lhsPath] = true
					if isZeroInit(rhs, info) {
						ptrInit[lhsPath] = true
					}
				}
			}
			var rhsObj types.Object
			if rhs != nil {
				if id, ok := ast.Unparen(rhs).(*ast.Ident); ok {
					rhsObj = info.ObjectOf(id)
				}
			}
			// An admitted operation must carry a value the caller supplied. An
			// append of a constant, a map insert of a constant, a pointer or
			// literal built from nothing the caller passed is a default, which
			// the purity rule (§4) forbids just as it forbids defaulting a
			// second field. The zero-value init of a container or of a pointer
			// intermediate is not a value: it is the guarded nil-init that
			// classes a and b are written around.
			if v := admittedValue(lhs, rhs, info); v != nil && !isZeroInit(v, info) && !mentionsSupplied(v, info, supplied) {
				defaulted = append(defaulted, lhsPath)
			}
			// A bare write is extra only when its value is not one the caller
			// passed: a parameter (or a local rooted in one) is a forwarder for
			// a caller-named value; a literal, a call or a computed value is a
			// default. Container inits (make) and writes through a pointer the
			// body initialised belong to classes a and b respectively.
			callerValue := rhsObj != nil && supplied[rhsObj]
			_, through := throughPointer(lhsPath, ptrPaths)
			if !isPtr && !callerValue && !isMapIndex(lhs, info) && !isAppend(rhs, info) && !isMake(rhs, info) &&
				compositeOf(rhs) == nil && !through {
				bareWrites[lhsPath] = rhsObj
			}
			if rhs == nil {
				continue
			}
			if rhsObj != nil {
				writtenBack[rhsObj] = s.Pos()
			}
			if nilClear == "" && isNillable(info.TypeOf(lhs)) && isNilValue(rhs, info, locals) {
				nilClear = types.ExprString(lhs)
			}
			if lit := compositeOf(rhs); lit != nil {
				// Class c is an upstream struct literal. A slice or map
				// literal replaces a collection rather than composing a
				// value, so it is not class c whatever its length.
				if isStructLit(lit, info) && (len(lit.Elts) >= 2 || hasNestedLiteral(lit)) {
					bigLiteral = true
				}
				if key := nilInLiteral(lit, info, locals); key != "" && nilClear == "" {
					nilClear = types.ExprString(lhs) + "." + key
				}
			}
		}
		return true
	})
	for obj, opPos := range localOps {
		if back, ok := writtenBack[obj]; ok && back > opPos {
			appendOrMap = true
		}
	}
	// A bare write is one the admitted operation does not cover: neither the
	// write-back of a local the body appended to or inserted into (class a),
	// nor a write through a pointer the body initialised (class b).
	var extra []string
	for path, obj := range bareWrites {
		if obj != nil {
			if _, op := localOps[obj]; op {
				continue
			}
		}
		extra = append(extra, path)
	}
	sort.Strings(extra)
	// A pointer intermediate the body allocates but never writes through
	// leaves the caller a zero value it did not ask for.
	for path := range ptrInit {
		if !ptrUsed[path] {
			defaulted = append(defaulted, path)
		}
	}
	sort.Strings(defaulted)

	switch {
	case nilClear != "":
		return Inadmissible, fmt.Sprintf("assigns nil to %s, a field the caller did not name (purity §4)", nilClear)
	case len(defaulted) > 0:
		return Inadmissible, fmt.Sprintf("writes a value the caller did not supply to %s (purity §4)", strings.Join(defaulted, ", "))
	case len(extra) > 0 && (appendOrMap || ptrAssign || bigLiteral):
		return Inadmissible, fmt.Sprintf("bare write to %s alongside the admitted operation, a field the caller did not name (purity §4)", strings.Join(extra, ", "))
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

// throughPointer reports whether path writes through a pointer field the body
// assigned earlier in source order (o.Spec.Ref.Name or *o.Spec.Ref after
// o.Spec.Ref), and returns that pointer field's path.
func throughPointer(path string, ptrPaths map[string]bool) (string, bool) {
	for p := range ptrPaths {
		if strings.HasPrefix(path, p+".") || path == "*"+p {
			return p, true
		}
	}
	return "", false
}

// admittedValue returns the expression an admitted operation writes: the
// element appended, the value inserted into a map, or the value assigned to a
// field. It returns nil when the statement is not one of those, and for a
// multi-element append (a splice, not a single caller value).
func admittedValue(lhs, rhs ast.Expr, info *types.Info) ast.Expr {
	if isAppend(rhs, info) {
		call := ast.Unparen(rhs).(*ast.CallExpr)
		if len(call.Args) != 2 {
			return nil
		}
		return call.Args[1]
	}
	if rhs == nil {
		return nil
	}
	if isMapIndex(lhs, info) || compositeOf(rhs) != nil {
		return rhs
	}
	if t := info.TypeOf(lhs); t != nil {
		if _, ok := t.Underlying().(*types.Pointer); ok {
			return rhs
		}
	}
	return nil
}

// isZeroInit reports whether e allocates an empty value rather than carrying
// one: make(T), new(T), or a composite literal with no elements. These are the
// guarded nil-inits classes a and b are written around, not values. make and
// new are resolved as builtins, so a function of those names is not one.
func isZeroInit(e ast.Expr, info *types.Info) bool {
	if e == nil {
		return false
	}
	if lit := compositeOf(e); lit != nil {
		return len(lit.Elts) == 0
	}
	return isBuiltinCall(e, info, "make") || isBuiltinCall(e, info, "new")
}

// hasEarlyReturn reports whether body returns anywhere other than by falling
// off its end. In a helper with no results every return is a guard that skips
// the write: `if obj == nil { return }` swallows the nil receiver the contract
// says must panic, and `if v == "" { return }` is the optional-argument idiom
// §4 forbids. Returns inside a nested function literal belong to that
// literal, not to the helper.
func hasEarlyReturn(body *ast.BlockStmt) bool {
	found := false
	ast.Inspect(body, func(n ast.Node) bool {
		if found {
			return false
		}
		switch n.(type) {
		case *ast.FuncLit:
			return false
		case *ast.ReturnStmt:
			found = true
			return false
		}
		return true
	})
	return found
}

// isBuiltinCall reports whether e calls the named builtin.
func isBuiltinCall(e ast.Expr, info *types.Info, name string) bool {
	call, ok := ast.Unparen(e).(*ast.CallExpr)
	if !ok {
		return false
	}
	id, ok := call.Fun.(*ast.Ident)
	if !ok {
		return false
	}
	b, ok := info.ObjectOf(id).(*types.Builtin)
	return ok && b.Name() == name
}

// mentionsSupplied reports whether e names any parameter, or a local carrying
// one: the value it computes came at least in part from the caller.
func mentionsSupplied(e ast.Expr, info *types.Info, supplied map[types.Object]bool) bool {
	found := false
	ast.Inspect(e, func(n ast.Node) bool {
		if found {
			return false
		}
		if id, ok := n.(*ast.Ident); ok && supplied[info.ObjectOf(id)] {
			found = true
		}
		return !found
	})
	return found
}

// callerValues returns every object in fn that carries a value the caller
// supplied: its parameters, and each local declared or assigned from an
// expression naming one. Unlike the rooted set this ignores types, because a
// scalar parameter is a caller-named value even though writes cannot reach
// the caller through it.
func callerValues(fn *ast.FuncDecl, info *types.Info) map[types.Object]bool {
	supplied := map[types.Object]bool{}
	for _, field := range fn.Type.Params.List {
		for _, name := range field.Names {
			if obj := info.Defs[name]; obj != nil {
				supplied[obj] = true
			}
		}
	}
	// Locals are visited in source order, so a local's initialiser is seen
	// before any use of it.
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		switch s := n.(type) {
		case *ast.ValueSpec:
			for i, id := range s.Names {
				if i < len(s.Values) && mentionsSupplied(s.Values[i], info, supplied) {
					if obj := info.Defs[id]; obj != nil {
						supplied[obj] = true
					}
				}
			}
		case *ast.AssignStmt:
			for i, lhs := range s.Lhs {
				id, ok := lhs.(*ast.Ident)
				if !ok || i >= len(s.Rhs) {
					continue
				}
				if mentionsSupplied(s.Rhs[i], info, supplied) {
					if obj := info.ObjectOf(id); obj != nil {
						supplied[obj] = true
					}
				}
			}
		}
		return true
	})
	return supplied
}

// isStructLit reports whether lit constructs a struct rather than a slice or
// a map.
func isStructLit(lit *ast.CompositeLit, info *types.Info) bool {
	t := info.TypeOf(lit)
	if t == nil {
		return false
	}
	if p, ok := t.Underlying().(*types.Pointer); ok {
		t = p.Elem()
	}
	_, ok := t.Underlying().(*types.Struct)
	return ok
}

// isMake reports whether e is a call to the builtin make: the nil-init of a
// map or slice field before inserting into or appending to it.
func isMake(e ast.Expr, info *types.Info) bool { return isBuiltinCall(e, info, "make") }

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

// rootIdent returns the identifier at the root of a selector, index,
// dereference or address-of chain (o in o.Spec.Items[i], labels in
// labels[k], spec in *spec), or nil when the chain is not rooted in one.
func rootIdent(e ast.Expr) *ast.Ident {
	for {
		switch v := e.(type) {
		case *ast.Ident:
			return v
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
				return nil
			}
			e = v.X
		default:
			return nil
		}
	}
}

// rootObj returns the object the root identifier of e denotes, or nil.
// Objects, not names, identify locals: a shadowing declaration is a
// different object.
func rootObj(e ast.Expr, info *types.Info) types.Object {
	if id := rootIdent(e); id != nil {
		return info.ObjectOf(id)
	}
	return nil
}

// roots records, per object, the positions at which it starts or stops
// being a way to reach a caller's object, in source order.
type roots map[types.Object][]rootEvent

type rootEvent struct {
	pos    token.Pos
	rooted bool
}

func (r roots) add(obj types.Object, pos token.Pos, rooted bool) {
	if obj != nil {
		r[obj] = append(r[obj], rootEvent{pos: pos, rooted: rooted})
	}
}

// at reports whether obj reaches a caller's object at position pos: the
// state set by the last event at or before pos.
func (r roots) at(obj types.Object, pos token.Pos) bool {
	if obj == nil {
		return false
	}
	rooted := false
	for _, e := range r[obj] {
		if e.pos > pos {
			break
		}
		rooted = e.rooted
	}
	return rooted
}

// rootedObjects returns the objects through which fn can reach a caller's
// object, with the position from which each does: its parameters from the
// start, and every local from the point where it is declared or assigned a
// selector, index, dereference or address-of chain rooted in one of them
// (spec := &o.Spec; labels := o.Labels). A local declared or assigned from
// anything else (tmp := &Obj{}, x := f()) is a temporary from that point:
// writes into it reach no caller-visible object and admit nothing.
func rootedObjects(fn *ast.FuncDecl, info *types.Info) roots {
	r := roots{}
	for _, field := range fn.Type.Params.List {
		for _, name := range field.Names {
			obj := info.Defs[name]
			// A parameter roots writes only when they can reach the caller
			// through it. A struct taken by value is a copy: assigning its
			// fields changes nothing the caller can observe.
			r.add(obj, fn.Pos(), obj != nil && carriesWrites(obj.Type()))
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
				if !ok {
					continue
				}
				for i, id := range vs.Names {
					rooted := len(vs.Values) == len(vs.Names) && r.at(rootObj(vs.Values[i], info), s.Pos())
					r.add(info.Defs[id], s.Pos(), rooted)
				}
			}
		case *ast.AssignStmt:
			for i, lhs := range s.Lhs {
				id, ok := lhs.(*ast.Ident)
				if !ok {
					continue
				}
				rooted := len(s.Lhs) == len(s.Rhs) && r.at(rootObj(s.Rhs[i], info), s.Pos())
				r.add(info.ObjectOf(id), s.Pos(), rooted)
			}
		}
		return true
	})
	return r
}

// nilInLiteral returns the key of the first keyed element of lit, or of a
// literal nested in it, whose value is nil; "" when there is none.
func nilInLiteral(lit *ast.CompositeLit, info *types.Info, locals map[types.Object]bool) string {
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
func isNilValue(e ast.Expr, info *types.Info, locals map[types.Object]bool) bool {
	switch v := ast.Unparen(e).(type) {
	case *ast.Ident:
		return v.Name == "nil" || (info.ObjectOf(v) != nil && locals[info.ObjectOf(v)])
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
// carriesWrites reports whether a write through a value of type t is visible
// to the caller: a pointer, map, slice, interface or channel shares its
// referent, a struct or scalar taken by value does not. A map or slice field
// reached through a by-value struct does share, but the classifier does not
// trace that: an unusual helper reads as inadmissible rather than slipping
// through.
func carriesWrites(t types.Type) bool {
	if t == nil {
		return false
	}
	switch t.Underlying().(type) {
	case *types.Pointer, *types.Map, *types.Slice, *types.Interface, *types.Chan:
		return true
	}
	return false
}

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
func nilLocals(fn *ast.FuncDecl, info *types.Info) map[types.Object]bool {
	locals := map[types.Object]bool{}
	mark := func(id *ast.Ident) {
		if obj := info.ObjectOf(id); obj != nil {
			locals[obj] = true
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
				if !ok {
					continue
				}
				for i, id := range vs.Names {
					switch {
					case len(vs.Values) == 0:
						if isNillable(info.TypeOf(id)) {
							mark(id)
						}
					case len(vs.Values) == len(vs.Names) && isNilValue(vs.Values[i], info, locals):
						mark(id)
					}
				}
			}
		case *ast.AssignStmt:
			if len(s.Lhs) != len(s.Rhs) {
				return true
			}
			for i, lhs := range s.Lhs {
				if id, ok := lhs.(*ast.Ident); ok && isNilValue(s.Rhs[i], info, locals) {
					mark(id)
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
