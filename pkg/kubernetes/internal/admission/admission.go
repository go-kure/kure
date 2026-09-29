// Package admission classifies the exported Set*/Add* sugar helpers under
// pkg/kubernetes by the builder contract's admission classes (ADR-038 §4).
//
// A helper is admissible when its body does one of:
//
//   - class a: append one value to a slice field or insert one key into a map
//     field, as the one statement F = append(F, v) or F[k] = v;
//   - class b: assign a pointer-typed field (the nil-init of a pointer
//     intermediate before writing through it is itself such an assignment);
//   - class c: write an upstream struct literal with two or more fields, or a
//     nested literal. A slice or map literal is not class c: it replaces a
//     collection rather than composing a value.
//
// Anything else is inadmissible: a bare single-field forwarder, several bare
// forwarders in one body (two field writes without a literal are two
// forwarders, not a composite), a helper with no field write, a helper that
// only delegates. A helper that returns anything is inadmissible whatever its
// body does: the purity rule (§4) allows no error return, and a nil receiver
// panics rather than being reported, so a result slot has nothing to carry.
// So is a helper declaring type parameters: the classifier reads concrete
// types only. The contract also exempts a fixed set of generic metadata
// helpers by name (§5); callers pass those in.
//
// The classifier is syntactic with type information (go/packages): it never
// executes code, and it is deliberately conservative, so an unusual but
// legitimate helper shows up as inadmissible rather than slipping through.
// What no rule sees is what the caller did before the call (an argument that
// points into the object, two pointer fields of the object sharing one
// struct) and what the caller's own code does when json.Marshal or %v runs
// it. Those, and whether a helper's name matches the field it writes, stay
// with the helper's unit test and with review.
//
// # Grammar
//
// The signature is read first: a helper that returns a value or declares
// type parameters is refused unread. Every other body is read through a
// closed grammar (grammar.go) and nothing else: every top-level statement,
// every path written and every value written has one of the forms below, and
// the first rule a body breaks is the verdict, with a reason ending in
// "(grammar <ID>, purity §4)". What was found is printed as written, a
// composite literal's elements included (exprText), so two paths that differ
// only inside a literal read apart. A body the grammar admits is classified by its
// writes: class a when one appends or inserts, else class b when one is
// pointer-typed, a nil-init included, else class c when one writes a struct
// literal of two or more fields or a nested one. A body with none of these
// forwards its arguments bare, and one with no field write delegates or does
// nothing; both are inadmissible. The README of pkg/kubernetes states the
// rules in full, under the same IDs.
//
// A body is nil guards and marshalled locals, then nil-init guards and field
// writes. The object is the parameter the first write is spelled from.
// Parentheses change nothing, and panic, append, make, new, json.Marshal and
// fmt.Sprintf are resolved by object, not by name.
//
//   - S1: a statement is an if or an assignment.
//   - S2: guards that panic and locals come before the first nil-init guard
//     or field write.
//   - S3: a := is exactly v, err := json.Marshal(P), both names new and
//     neither blank, P a parameter other than the object.
//   - S4: the statement directly after a local is the guard on its error, and
//     that guard appears nowhere else.
//   - S5: a panic takes one argument: a string constant, or in the guard on
//     an error fmt.Sprintf(c, err).
//   - S6: a nil guard that panics tests the object, or a parameter the body
//     writes as *P.
//   - S7: any other assignment has one target, one value and the token =.
//   - S8: an if has no init, no else and a one-statement body, and is a nil
//     guard that panics, the guard on an error or a nil-init guard.
//   - S9: the body holds no function literal, a constant that holds one
//     included.
//   - S10: at most one statement appends or inserts.
//   - S11: every call of append is the whole value of an assignment.
//   - P1: a path is spelled from the object through field selectors, indexes
//     and dereferences, and the object is a pointer parameter the body never
//     reassigns or takes the address of.
//   - P2: a path names a field.
//   - P3: a key or index is a parameter other than the object, or a constant.
//   - P4: no location is written after a write to it, or to a location above
//     or below it on the same path, other than through a nil-init guard ahead
//     of the write. Every pointer followed is a step of the path, a promoted
//     field is its full spelling, and any two keys may be the same element.
//     A guard is ahead of a write on the same path with the same keys: the
//     same parameter, or constants of one type and one value.
//   - P5: a nil-init guard initialises a field, not an element.
//   - P6: every nil-init is written through, a pointer nil-init by a later
//     write spelled <its text>.… or *<its text> as written.
//   - P7: behind the nearest nil-init guard ahead of it a write meets only
//     what that guard assigned. Its first step there may follow that pointer
//     or index that map; past that step it follows no pointer and indexes no
//     map, and it indexes no slice at all. An append needs no slice and an
//     array has its elements. A nil-init guard is itself such a write.
//   - N1: a nil-init guard tests the path it assigns, spelled alike outer
//     parentheses aside and with the same keys as under P4; the path is a
//     map, slice or pointer; and the guard assigns T{}, &T{}, new(T) or a
//     make with constant sizes, a slice make of length 0.
//   - V1: a value is an argument passed whole (a parameter other than the
//     object as P, &P or *P, or the first name of a local), a struct literal
//     of arguments, constants and such literals, or the address of one. The
//     name nil, as spelled, is not the value of a nillable target, nor a
//     keyed element of a literal written as a field or inserted value.
//   - V2: a struct literal is an appended or inserted element, &T{...} into a
//     pointer-typed field, or the helper's only field write.
//   - V3: a constant in a struct literal is a named constant of a defined
//     type.
//   - V3b: every struct literal, at every depth, carries an argument.
//   - V4: each argument occurs once over all written values and marshalled
//     locals.
//   - V5: every parameter, a blank one included, is the object, a key or
//     index of a path, or an argument written or marshalled.
//   - V6: &P and *P are an appended or inserted element, the value of a
//     pointer-typed target, or written to a target spelled <its text>.… or
//     *<its text>, as written, of a pointer-typed target written earlier.
//   - V7: no written value reads its target, the value of a nil-init
//     included, in a constant, a make size or the type of a literal; the
//     target is read as written, parentheses aside.
//
// S9, then S11, are read off the whole body first. Then the statements are
// read in source order and the first rule one breaks names the refusal: S1;
// for an if S8, S2, S5, S4, and for a nil-init guard P1, P2, P3, P5, N1, V7;
// for a := S3, S2, S4; for any other assignment S7, P1, P2, P3, S10, V1, V3,
// V3b, V2, V6, V7. A body read to its end with no field write is refused as
// a delegation or no-op. S6, P4, P6, P7, V4 and V5 are then read off the
// whole body, in that order, and the class off its writes.
package admission

import (
	"fmt"
	"go/ast"
	"go/constant"
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

// classify reads fn and returns its class, or Inadmissible with the reason.
// The signature is read first: a helper that returns anything, or declares
// type parameters, is refused unread. The body is then read through the
// grammar (grammar.go) and nothing else: the reason of the first rule it
// breaks is the verdict, a body that writes no field is a delegation or a
// no-op, and the class of an admitted body is the class of its writes.
func classify(fn *ast.FuncDecl, info *types.Info) (Class, string) {
	if fn.Type.Results != nil && len(fn.Type.Results.List) > 0 {
		return Inadmissible, "returns a value; sugar returns nothing and panics on a nil receiver instead of reporting it (purity §4)"
	}
	if fn.Type.TypeParams.NumFields() > 0 {
		return Inadmissible, "declares type parameters; the classifier reads concrete types only"
	}
	b, reason := read(fn, info)
	if reason != "" {
		return Inadmissible, reason
	}
	if len(b.writes) == 0 {
		return Inadmissible, "no field write (delegation or no-op)"
	}
	if reason := b.whole(); reason != "" {
		return Inadmissible, reason
	}
	return b.class()
}

// sortedKeys returns the keys of set in sorted order.
func sortedKeys(set map[string]bool) []string {
	keys := make([]string, 0, len(set))
	for k := range set {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// stableParams returns fn's parameters of pointer type that the body never
// assigns (the target of an assignment of any token, or a range clause's key
// or value, parentheses aside) and never takes the address of: a field path
// spelled from one of them denotes the caller's object on every path.
func stableParams(fn *ast.FuncDecl, info *types.Info) map[types.Object]bool {
	stable := map[types.Object]bool{}
	for _, field := range fn.Type.Params.List {
		for _, name := range field.Names {
			if obj := info.Defs[name]; obj != nil {
				if _, ok := obj.Type().Underlying().(*types.Pointer); ok {
					stable[obj] = true
				}
			}
		}
	}
	unstable := func(e ast.Expr) {
		if id, ok := ast.Unparen(e).(*ast.Ident); ok {
			delete(stable, info.ObjectOf(id))
		}
	}
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		switch s := n.(type) {
		case *ast.AssignStmt:
			for _, lhs := range s.Lhs {
				unstable(lhs)
			}
		case *ast.RangeStmt:
			unstable(s.Key)
			unstable(s.Value)
		case *ast.UnaryExpr:
			if s.Op == token.AND {
				unstable(s.X)
			}
		}
		return true
	})
	return stable
}

// pathRoot returns the object of the identifier e is spelled from through
// selectors, index operands, dereferences and parentheses only (o in
// (*o).Spec.Groups[k]), or nil for any other spelling, such as an address-of
// or a call anywhere in the path ((*&o.Labels)[k]).
func pathRoot(e ast.Expr, info *types.Info) types.Object {
	for {
		switch v := e.(type) {
		case *ast.Ident:
			return info.ObjectOf(v)
		case *ast.SelectorExpr:
			e = v.X
		case *ast.IndexExpr:
			e = v.X
		case *ast.StarExpr:
			e = v.X
		case *ast.ParenExpr:
			e = v.X
		default:
			return nil
		}
	}
}

// extendsByOne reports whether call appends exactly one value, not spread, to
// the same expression it is assigned to (sameExpr): F = append(F, v).
func extendsByOne(call *ast.CallExpr, lhs ast.Expr, info *types.Info) bool {
	return len(call.Args) == 2 && !call.Ellipsis.IsValid() && sameExpr(call.Args[0], lhs, info)
}

// sameExpr reports whether a and b are the same expression, parentheses
// aside: the same object for an identifier, the same field of the same
// operand for a selector, the same index of the same operand, the same
// dereference, or the same literal. Anything else is not the same, however it
// prints: types.ExprString abbreviates composite literals, so
// string([]byte{'a'}) and string([]byte{'b'}) print alike.
func sameExpr(a, b ast.Expr, info *types.Info) bool {
	a, b = ast.Unparen(a), ast.Unparen(b)
	switch x := a.(type) {
	case *ast.Ident:
		y, ok := b.(*ast.Ident)
		if !ok {
			return false
		}
		obj := info.ObjectOf(x)
		return obj != nil && obj == info.ObjectOf(y)
	case *ast.SelectorExpr:
		y, ok := b.(*ast.SelectorExpr)
		return ok && x.Sel.Name == y.Sel.Name && sameExpr(x.X, y.X, info)
	case *ast.IndexExpr:
		y, ok := b.(*ast.IndexExpr)
		return ok && sameExpr(x.X, y.X, info) && sameExpr(x.Index, y.Index, info)
	case *ast.StarExpr:
		y, ok := b.(*ast.StarExpr)
		return ok && sameExpr(x.X, y.X, info)
	case *ast.BasicLit:
		y, ok := b.(*ast.BasicLit)
		return ok && x.Kind == y.Kind && x.Value == y.Value
	}
	return false
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

// isZeroInit reports whether e allocates an empty value rather than carrying
// one: a make that allocates no elements (isEmptyMake), new(T), or a composite
// literal with no elements. It inspects only e: such a value is the nil-init
// classes a and b are written around only when the field it is written into
// is a map, slice or pointer (initsField), which the grammar checks under N1
// (isEmptyValue narrows new to a type argument). make and new are resolved as
// builtins, so a function of those names is not one.
func isZeroInit(e ast.Expr, info *types.Info) bool {
	if e == nil {
		return false
	}
	if lit := compositeOf(e); lit != nil {
		return len(lit.Elts) == 0
	}
	return isEmptyMake(e, info) || isBuiltinCall(e, info, "new")
}

// isEmptyValue reports whether e is a zero-init (isZeroInit) that carries no
// value at all: new counts only with a type argument, because Go 1.26's new(x)
// allocates the value of x, which is a value rather than an empty one.
func isEmptyValue(e ast.Expr, info *types.Info) bool {
	if !isZeroInit(e, info) {
		return false
	}
	if isBuiltinCall(e, info, "new") {
		call := ast.Unparen(e).(*ast.CallExpr)
		if len(call.Args) != 1 {
			return false
		}
		tv, ok := info.Types[call.Args[0]]
		return ok && tv.IsType()
	}
	return true
}

// initsField reports whether a field of type t is one the zero-value init
// initialises: a map, a slice or a pointer, the fields classes a and b insert
// into, append to or write through. An interface or struct field holds a value
// rather than a nil collection or referent, so an empty value written into one
// is a value the caller did not supply. A type parameter (whose underlying type
// is its constraint interface) and a channel are conservatively not counted:
// an empty make of a channel is a default like any other empty value there.
func initsField(t types.Type) bool {
	if t == nil {
		return false
	}
	switch t.Underlying().(type) {
	case *types.Map, *types.Slice, *types.Pointer:
		return true
	}
	return false
}

// isEmptyMake reports whether e is a call to the builtin make that allocates
// no elements: a map or channel whatever its size hint, or a slice whose
// length is the constant 0 (a capacity allocates none). A length that is not
// a constant, and a type parameter, may allocate elements and are not empty.
func isEmptyMake(e ast.Expr, info *types.Info) bool {
	if !isMake(e, info) {
		return false
	}
	call := ast.Unparen(e).(*ast.CallExpr)
	if len(call.Args) == 0 {
		return false
	}
	t := info.TypeOf(call.Args[0])
	if t == nil {
		return false
	}
	switch t.Underlying().(type) {
	case *types.Map, *types.Chan:
		return true
	case *types.Slice:
		if len(call.Args) < 2 {
			return false
		}
		tv, ok := info.Types[call.Args[1]]
		return ok && tv.Value != nil && constant.Sign(tv.Value) == 0
	}
	return false
}

// hasEarlyReturn reports whether body returns anywhere other than by falling
// off its end. In a helper with no results every return is a guard that skips
// the write: `if obj == nil { return }` swallows the nil receiver the contract
// says must panic, and `if v == "" { return }` is the optional-argument idiom
// §4 forbids. The grammar names it as the cause of an S1 or S8 refusal.
// Returns inside a nested function literal belong to that literal, not to the
// helper.
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

// strayAppends returns the bases of the calls of append in body that are not
// the whole value of an assignment: var items = append(...), an argument, a
// slice of its result, an append inside a constant. Such an append is not the
// class a statement, whatever it extends.
func strayAppends(body *ast.BlockStmt, info *types.Info) map[string]bool {
	assigned := map[*ast.CallExpr]bool{}
	ast.Inspect(body, func(n ast.Node) bool {
		if s, ok := n.(*ast.AssignStmt); ok {
			for _, rhs := range s.Rhs {
				if call := appendCall(rhs, info); call != nil {
					assigned[call] = true
				}
			}
		}
		return true
	})
	stray := map[string]bool{}
	ast.Inspect(body, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok && appendCall(call, info) == call && !assigned[call] && len(call.Args) > 0 {
			stray[exprText(call.Args[0])] = true
		}
		return true
	})
	return stray
}

// hasFuncLit reports whether body contains a function literal. Its body is
// not the helper's own and no rule descends into it, so a closure could
// hide any write; sugar needs none (S9).
func hasFuncLit(body *ast.BlockStmt) bool {
	found := false
	ast.Inspect(body, func(n ast.Node) bool {
		if _, ok := n.(*ast.FuncLit); ok {
			found = true
		}
		return !found
	})
	return found
}

// hasGoto reports whether body contains a goto outside a nested function
// literal. `if v == "" { goto done }` skips the write exactly as an early
// return does; the grammar names it as the cause of an S1 or S8 refusal.
func hasGoto(body *ast.BlockStmt) bool {
	found := false
	ast.Inspect(body, func(n ast.Node) bool {
		if found {
			return false
		}
		switch s := n.(type) {
		case *ast.FuncLit:
			return false
		case *ast.BranchStmt:
			if s.Tok == token.GOTO {
				found = true
				return false
			}
		}
		return true
	})
	return found
}

// isNilIdent reports whether e is the predeclared nil, resolved through type
// information rather than by spelling: a parameter or local named nil is a
// caller value, and a guard comparing against it is a set-if-equal, not a
// nil-init.
func isNilIdent(e ast.Expr, info *types.Info) bool {
	id, ok := ast.Unparen(e).(*ast.Ident)
	if !ok {
		return false
	}
	_, isNil := info.ObjectOf(id).(*types.Nil)
	return isNil
}

// isBuiltinCall reports whether e calls the named builtin, its name
// parenthesised or not ((make)(T, n)).
func isBuiltinCall(e ast.Expr, info *types.Info, name string) bool {
	call, ok := ast.Unparen(e).(*ast.CallExpr)
	if !ok {
		return false
	}
	id, ok := ast.Unparen(call.Fun).(*ast.Ident)
	if !ok {
		return false
	}
	b, ok := info.ObjectOf(id).(*types.Builtin)
	return ok && b.Name() == name
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
// map or slice path, which N1 admits only with constant sizes and, for a
// slice, a length of 0 (isEmptyMake).
func isMake(e ast.Expr, info *types.Info) bool { return isBuiltinCall(e, info, "make") }

// nilInLiteral returns the key of the first keyed element of lit whose value
// is nil, or of a literal reached from lit through keyed elements only, as
// key.key; "" when there is none. A positional element is not read, whatever
// it holds.
func nilInLiteral(lit *ast.CompositeLit, info *types.Info, locals map[types.Object]bool) string {
	for _, elt := range lit.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		if isNilValue(kv.Value, info, locals) {
			return exprText(kv.Key)
		}
		if inner := compositeOf(kv.Value); inner != nil {
			if key := nilInLiteral(inner, info, locals); key != "" {
				return exprText(kv.Key) + "." + key
			}
		}
	}
	return ""
}

func isMapIndex(lhs ast.Expr, info *types.Info) bool {
	idx, ok := ast.Unparen(lhs).(*ast.IndexExpr)
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

// readsTarget reports whether rhs reads the target lhs it is assigned to,
// other than as the slice an append extends
// (o.Spec.Items = append(o.Spec.Items, s)), which is class a, or as the
// whole value (o.Labels = labels), which writes back what the target already
// held. alias reports whether an expression is a local that may reach the
// caller (sameTarget).
func readsTarget(lhs, rhs ast.Expr, info *types.Info, alias func(ast.Expr) bool) bool {
	if sameTarget(rhs, lhs, info, alias) {
		return false
	}
	reads := func(e ast.Expr) bool {
		found := false
		ast.Inspect(e, func(n ast.Node) bool {
			if x, ok := n.(ast.Expr); ok && !found && sameTarget(x, lhs, info, alias) {
				found = true
			}
			return !found
		})
		return found
	}
	if isAppend(rhs, info) {
		for i, arg := range ast.Unparen(rhs).(*ast.CallExpr).Args {
			if i > 0 && reads(arg) {
				return true
			}
		}
		return false
	}
	return reads(rhs)
}

// sameTarget reports whether x may denote the location target does: the same
// expression as written, outer parentheses aside, or the same element, field
// or pointee of operands that may be the same, where a local that may reach
// the caller may be any caller path of its type (labels[k] and o.Labels[k]
// after labels := o.Labels). Two indexes are the same when they are the same
// expression however parenthesised (sameExpr: o.Labels[(k)] is o.Labels[k])
// or print alike.
func sameTarget(x, target ast.Expr, info *types.Info, alias func(ast.Expr) bool) bool {
	x, target = ast.Unparen(x), ast.Unparen(target)
	if types.ExprString(x) == types.ExprString(target) {
		return true
	}
	if tx, tt := info.TypeOf(x), info.TypeOf(target); tx != nil && tt != nil &&
		(alias(x) || alias(target)) && types.Identical(tx, tt) {
		return true
	}
	switch t := target.(type) {
	case *ast.IndexExpr:
		v, ok := x.(*ast.IndexExpr)
		return ok && (sameExpr(v.Index, t.Index, info) || types.ExprString(v.Index) == types.ExprString(t.Index)) &&
			sameTarget(v.X, t.X, info, alias)
	case *ast.SelectorExpr:
		v, ok := x.(*ast.SelectorExpr)
		return ok && v.Sel.Name == t.Sel.Name && sameTarget(v.X, t.X, info, alias)
	case *ast.StarExpr:
		v, ok := x.(*ast.StarExpr)
		return ok && sameTarget(v.X, t.X, info, alias)
	}
	return false
}

func isAppend(rhs ast.Expr, info *types.Info) bool { return appendCall(rhs, info) != nil }

// appendCall returns e, parentheses aside, as a call of the builtin append,
// its name parenthesised or not ((append)(F, v)), or nil for anything else.
func appendCall(e ast.Expr, info *types.Info) *ast.CallExpr {
	call, ok := ast.Unparen(e).(*ast.CallExpr)
	if !ok {
		return nil
	}
	id, ok := ast.Unparen(call.Fun).(*ast.Ident)
	if !ok {
		return nil
	}
	if b, ok := info.Uses[id].(*types.Builtin); ok && b.Name() == "append" {
		return call
	}
	return nil
}

// isNilValue reports whether e is the nil identifier, a conversion of a nil
// value to a named type ((*T)(nil)), or a local in locals. The grammar
// passes no locals: the one local it admits is a marshalled value (S3), and
// the name nil is read as spelled (V1).
func isNilValue(e ast.Expr, info *types.Info, locals map[types.Object]bool) bool {
	switch v := ast.Unparen(e).(type) {
	case *ast.Ident:
		return v.Name == "nil" || (info.ObjectOf(v) != nil && locals[info.ObjectOf(v)])
	case *ast.CallExpr:
		if op := conversionOperand(v, info); op != nil {
			return isNilValue(op, info, locals)
		}
	}
	return false
}

// conversionOperand returns the operand of call when call is a type
// conversion (T(x), (*T)(x)), and nil for any other call.
func conversionOperand(call *ast.CallExpr, info *types.Info) ast.Expr {
	if tv, ok := info.Types[call.Fun]; ok && tv.IsType() && len(call.Args) == 1 {
		return call.Args[0]
	}
	return nil
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
