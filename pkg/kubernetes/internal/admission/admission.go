// Package admission classifies the exported Set*/Add* sugar helpers under
// pkg/kubernetes by the builder contract's admission classes (ADR-038 §4).
//
// A helper is admissible when its body does one of:
//
//   - class a: append to a slice field or insert into a map field, directly
//     or through a local that came from that field and is assigned back to it
//     (a collection the helper built itself replaces the field's contents,
//     which is not adding to it);
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
// map, slice or pointer field (make, new, an empty literal, followed by a
// write through it) is not such a value. An empty value (an empty literal or
// make, or new(T)) written into any other field, an interface, a struct or a
// channel, is a value the caller did not supply and is refused as a default,
// including when it is written through a pointer the body initialised.
// A make counts as that init only when it provably allocates no elements (a
// map, or a slice of constant length 0); a slice made with any other length
// may hold zero-valued elements the caller never supplied, so it is
// conservatively treated as a default. An increment, decrement or compound
// assignment of anything that reaches the caller (o.Spec.Count++,
// o.Spec.Count += n) writes a value computed from what the target held, so it
// is such a default too, and so is a plain assignment whose value reads its
// own target (o.Labels[k] = o.Labels[k] + v) other than as the slice an append
// extends or as the whole value. That includes an element of a map or slice
// reached through any chain of locals copied, sliced or appended from a
// caller's object or bound to its elements by a range clause (labels :=
// o.Labels; labels[k] += v; items := o.Spec.Items[:]; items, ok :=
// o.Spec.Groups[k]; items := append(o.Spec.Items, s); for _, items := range
// o.Spec.Groups), even when a nil-init guard may have replaced one of them
// since, and the target read through such a local counts as read however it
// is spelled (labels[k] = o.Labels[k] + v).
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
// the body just initialised (o.Spec.Ref.Name or *o.Spec.Replicas), an empty
// make of the map or slice the body then inserts into or appends to, and
// assigning an appended or inserted-into local back to its field are each part of their
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
// A function literal's body is not the helper's own: an uncalled closure that
// appends is a no-op no caller sees, so no walk descends into one.
//
// A counted write (a rooted field write, or a map insert or append into a
// local) that runs only on some paths is inadmissible whatever else the body
// does: `if name != "" { o.Spec.Name = name }` is the conditional no-op §4
// forbids, and so is a write inside a loop, a switch, a type switch or a
// select, or in an if's else branch. A range clause that assigns to a target
// reaching the caller in the read-modify-write sense above (for _, o.Spec.Ref
// = range refs; for _, items[0] = range xs) writes it once per element or not
// at all, so it is conditional too. A goto is refused like an early return,
// because it can jump over the write. The one guard admitted is the nil-init
// that classes a and b are written around: a top-level
// `if P == nil { P = <zero value> }` (nil may be on either side) whose only
// statement zero-initialises the map, slice or pointer path it tests, with no
// Init and no else. Any other guard, including a set-if-unset
// (`if o.Spec.Ref == nil { o.Spec.Ref = ref }`), one that fills a nil
// interface field with an empty value, and a make that may allocate elements
// (`if o.Spec.Items == nil { o.Spec.Items = make([]string, 1) }`), makes the
// writes under it conditional.
//
// The classifier is syntactic with type information (go/packages): it never
// executes code, and it is deliberately conservative, so an unusual but
// legitimate helper shows up as inadmissible rather than slipping through.
// Its dataflow is bounded to what is described above; it does not follow a
// conditionally created alias (a local assigned a parameter inside a branch),
// calls other than append, or aliases created any other way. A helper written
// to evade it is caught by its own unit test and by review, not by this
// package.
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
	if hasGoto(fn.Body) {
		return Inadmissible, "jumps over the write with goto; sugar has no conditional no-op (purity §4)"
	}
	guards := conditionalWrites(fn.Body, info)
	var (
		appendOrMap bool
		ptrAssign   bool
		bigLiteral  bool
		nilClear    string
		conditional []string // counted writes that run only on some paths, with their guard
		writes      = map[string]bool{}
		ptrPaths    = map[string]bool{}         // pointer fields assigned, by expression, for writes through them
		ptrInit     = map[string]bool{}         // of those, the ones assigned a bare new(T)/&T{} with no value
		ptrUsed     = map[string]bool{}         // pointer fields a later write goes through
		bareWrites  = map[string]types.Object{} // bare field writes -> the local assigned, if any
		defaulted   []string                    // admitted operations carrying a value the caller did not supply
		locals      = nilLocals(fn, info)
		rooted      = rootedObjects(fn, info)      // parameters writes reach the caller through, by position
		mayReach    = mayReachCaller(fn, info)     // the same, conservatively, for read-modify-write targets
		supplied    = callerValues(fn, info)       // parameters and locals carrying one, for value provenance
		localOps    = map[types.Object]token.Pos{} // local -> first append or map insert into it
		writtenBack = map[types.Object]token.Pos{} // local -> last assignment of it to a field
	)
	// reachesCaller reports whether a write to x at pos may change something
	// the caller can see: a field, a dereference or an indexed element
	// (labels := o.Labels; labels[k]), parenthesised or not, whose root,
	// seen through slice expressions (o.Spec.Items[:][0]), may have reached a
	// caller's object at some point up to pos through any chain of locals
	// copied or sliced from it (mayReachCaller). A later reassignment does
	// not clear it: after the nil-init
	// `if labels == nil { labels = map[string]string{} }` labels is still the
	// caller's map on the other path. A plain local is not.
	reachesCaller := func(x ast.Expr, pos token.Pos) bool {
		switch ast.Unparen(x).(type) {
		case *ast.SelectorExpr, *ast.StarExpr, *ast.IndexExpr:
			return mayReach.rootedBy(mayReachObj(x, info), pos)
		}
		return false
	}
	// isAlias reports whether e is a local, not a parameter, that may have
	// reached a caller's object by pos: it may hold any caller path of its
	// type, where a parameter is the caller's own distinct argument.
	params := map[types.Object]bool{}
	for _, field := range fn.Type.Params.List {
		for _, name := range field.Names {
			params[info.Defs[name]] = true
		}
	}
	isAlias := func(e ast.Expr, pos token.Pos) bool {
		id, ok := ast.Unparen(e).(*ast.Ident)
		if !ok {
			return false
		}
		obj := info.ObjectOf(id)
		return obj != nil && !params[obj] && mayReach.rootedBy(obj, pos)
	}
	// guardOf returns the guard a statement runs under: "" means it runs on
	// every path, and a statement the pre-pass has no entry for is treated
	// as conditional.
	guardOf := func(s ast.Stmt) string {
		if guard, known := guards[s]; known {
			return guard
		}
		return "a nested statement"
	}
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		var s *ast.AssignStmt
		switch n := n.(type) {
		case *ast.FuncLit:
			// A function literal's body is not the helper's own: an uncalled
			// closure that appends is a no-op the caller never sees.
			return false
		case *ast.RangeStmt:
			// A range clause assigning to a target that reaches the caller
			// writes it once per element or not at all: conditional.
			for _, x := range []ast.Expr{n.Key, n.Value} {
				if x != nil && reachesCaller(x, n.Pos()) {
					conditional = append(conditional, fmt.Sprintf("%s (under %s)", types.ExprString(x), guardOf(n)))
				}
			}
			return true
		case *ast.IncDecStmt:
			// An increment or decrement writes a value computed from what the
			// target held, never one the caller supplied: a default.
			if reachesCaller(n.X, n.Pos()) {
				defaulted = append(defaulted, types.ExprString(n.X))
			}
			return true
		case *ast.AssignStmt:
			s = n
		default:
			return true
		}
		guard := guardOf(s)
		for i, lhs := range s.Lhs {
			var rhs ast.Expr
			if len(s.Rhs) == len(s.Lhs) {
				rhs = s.Rhs[i]
			} else if len(s.Rhs) == 1 {
				rhs = s.Rhs[0]
			}
			fieldWrite := isFieldWrite(lhs) && rooted.at(rootObj(lhs, info), s.Pos())
			localOp := false
			if isMapIndex(lhs, info) || isAppend(rhs, info) {
				switch {
				case fieldWrite:
					appendOrMap = true
				case !isFieldWrite(lhs):
					if obj := rootObj(lhs, info); obj != nil {
						localOp = true
						if _, seen := localOps[obj]; !seen {
							localOps[obj] = s.Pos()
						}
					}
				}
			}
			// A counted write is a rooted field write or a map insert or append
			// into a local. Either one under a guard runs only on some paths,
			// which is the conditional no-op §4 forbids. Uncounted local
			// assignments (labels = map[string]string{}) stay ignored.
			if (fieldWrite || localOp) && guard != "" {
				conditional = append(conditional, fmt.Sprintf("%s (under %s)", types.ExprString(lhs), guard))
			}
			// A compound assignment (+=, |=, ...) writes old op value, which is
			// the same read-modify-write as an increment, not a forward of the
			// caller's value.
			if s.Tok != token.ASSIGN && s.Tok != token.DEFINE && reachesCaller(lhs, s.Pos()) {
				defaulted = append(defaulted, types.ExprString(lhs))
			}
			// So is a plain assignment whose value reads the target it
			// overwrites (o.Labels[k] = o.Labels[k] + v), spelled out.
			if s.Tok == token.ASSIGN && rhs != nil && reachesCaller(lhs, s.Pos()) &&
				readsTarget(lhs, rhs, info, func(e ast.Expr) bool { return isAlias(e, s.Pos()) }) {
				defaulted = append(defaulted, types.ExprString(lhs))
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
			// classes a and b are written around. As the field's own value it
			// is that init only when the field is a map, slice or pointer
			// (initsField): an empty value (isEmptyValue) written into any
			// other field, an interface, a struct or a channel, is a default,
			// or a wipe of what the caller had there. That holds on every
			// path, including a write through a pointer the body initialised,
			// which is otherwise not a bare write. An appended or inserted
			// empty value is not checked against a field type. A make that may
			// allocate elements is not that init, whether it is the field's
			// value or the appended or inserted one: it may fill the slice with
			// zero-valued elements the caller never supplied, whatever its
			// arguments mention, so it is conservatively treated as a default.
			v := admittedValue(lhs, rhs, info)
			fieldValue := !isAppend(rhs, info) && !isMapIndex(lhs, info)
			switch {
			case isFilledMake(rhs, info) || isFilledMake(v, info):
				defaulted = append(defaulted, lhsPath)
			case fieldValue && isEmptyValue(rhs, info) && !initsField(info.TypeOf(lhs)):
				defaulted = append(defaulted, lhsPath)
			case v != nil && !isZeroInit(v, info) && !mentionsSupplied(v, info, supplied):
				defaulted = append(defaulted, lhsPath)
			}
			// A bare write is extra only when its value is not one the caller
			// passed: a parameter (or a local rooted in one) is a forwarder for
			// a caller-named value; a literal, a call or a computed value is a
			// default. Container inits (make) and writes through a pointer the
			// body initialised belong to classes a and b respectively; a make
			// into any other field is refused above as a default.
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
	// A local counts for class a only when it came from the field it is
	// written back to. A collection the helper made itself replaces whatever
	// the field held, which is not adding to it.
	for obj, opPos := range localOps {
		if back, ok := writtenBack[obj]; ok && back > opPos && rooted.everRooted(obj) {
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
	case len(conditional) > 0:
		return Inadmissible, fmt.Sprintf("writes %s only on some paths; sugar has no conditional no-op (purity §4)", strings.Join(conditional, ", "))
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
// one: a make that allocates no elements (isEmptyMake), new(T), or a composite
// literal with no elements. It inspects only e: such a value is the guarded
// nil-init classes a and b are written around only when the field it is
// written into is a map, slice or pointer (initsField), which the callers
// check (isEmptyValue narrows new to a type argument). make and new are
// resolved as builtins, so a function of those names is not one.
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

// isFilledMake reports whether e is a call to the builtin make that may
// allocate elements: a make that isEmptyMake does not accept.
func isFilledMake(e ast.Expr, info *types.Info) bool {
	return isMake(e, info) && !isEmptyMake(e, info)
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

// hasGoto reports whether body contains a goto outside a nested function
// literal. `if v == "" { goto done }` skips the write exactly as an early
// return does.
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

// conditionalWrites maps every assignment and range statement in body outside
// a function literal to "" when it runs on every path through the body, or to
// the text of the outermost statement guarding it (`if <cond>`, `for <cond>`,
// `range <x>`, `switch <tag>`, `type switch`, `select`).
//
// Unconditional is defined positively: an assignment in the top-level
// statement list (descending through bare blocks and labels), the Init of a
// top-level if, for, switch or type switch, and the single assignment of a
// top-level nil-init guard (isNilInitGuard). Everything a top-level if, for,
// range, switch, type switch or select contains maps to that statement's
// guard, including its else branch, post statement, select communication and
// any nested Init. A range statement is never unconditional: its clause
// assigns once per element or not at all, so a top-level range maps to its
// own `range <x>` and a nested one to the outermost guard around it. A
// statement kind this pass does not know leaves its writes without an entry,
// which the caller treats as conditional, so an omission errs toward refusal.
func conditionalWrites(body *ast.BlockStmt, info *types.Info) map[ast.Stmt]string {
	m := map[ast.Stmt]string{}
	guarded := func(stmt, init ast.Stmt, text string) {
		ast.Inspect(stmt, func(n ast.Node) bool {
			if init != nil && n == ast.Node(init) {
				return false
			}
			switch s := n.(type) {
			case *ast.FuncLit:
				return false
			case *ast.AssignStmt:
				m[s] = text
			case *ast.RangeStmt:
				m[s] = text
			}
			return true
		})
	}
	unconditionalInit := func(init ast.Stmt) {
		if s, ok := init.(*ast.AssignStmt); ok {
			m[s] = ""
		}
	}
	var top func(stmt ast.Stmt)
	top = func(stmt ast.Stmt) {
		switch s := stmt.(type) {
		case *ast.AssignStmt:
			m[s] = ""
		case *ast.BlockStmt:
			for _, inner := range s.List {
				top(inner)
			}
		case *ast.LabeledStmt:
			top(s.Stmt)
		case *ast.IfStmt:
			unconditionalInit(s.Init)
			if isNilInitGuard(s, info) {
				m[s.Body.List[0].(*ast.AssignStmt)] = ""
				return
			}
			guarded(s, s.Init, "if "+types.ExprString(s.Cond))
		case *ast.ForStmt:
			unconditionalInit(s.Init)
			text := "for"
			if s.Cond != nil {
				text = "for " + types.ExprString(s.Cond)
			}
			guarded(s, s.Init, text)
		case *ast.RangeStmt:
			guarded(s, nil, "range "+types.ExprString(s.X))
		case *ast.SwitchStmt:
			unconditionalInit(s.Init)
			text := "switch"
			if s.Tag != nil {
				text = "switch " + types.ExprString(s.Tag)
			}
			guarded(s, s.Init, text)
		case *ast.TypeSwitchStmt:
			unconditionalInit(s.Init)
			guarded(s, s.Init, "type switch")
		case *ast.SelectStmt:
			guarded(s, nil, "select")
		}
	}
	top(body)
	return m
}

// isNilInitGuard reports whether s is the one guard the contract admits: it
// tests a path for nil and zero-initialises that same path, with nothing else
// in it (`if o.Labels == nil { o.Labels = map[string]string{} }`). No Init, no
// else, exactly one plain assignment of one value, into a map, slice or pointer
// path (initsField), and an empty value as defined by isEmptyValue: new counts
// only with a type argument, since Go 1.26's new(x) allocates the value of x,
// which makes `if o.Spec.Ref == nil { o.Spec.Ref = new(ref) }` a set-if-unset.
// Filling a nil interface field (`if o.Spec.Payload == nil { o.Spec.Payload =
// &Ref{} }`) is a set-if-unset too: the empty value is the default it sets.
func isNilInitGuard(s *ast.IfStmt, info *types.Info) bool {
	if s.Init != nil || s.Else != nil || len(s.Body.List) != 1 {
		return false
	}
	cond, ok := ast.Unparen(s.Cond).(*ast.BinaryExpr)
	if !ok || cond.Op != token.EQL {
		return false
	}
	var tested ast.Expr
	switch {
	case isNilIdent(cond.Y, info):
		tested = cond.X
	case isNilIdent(cond.X, info):
		tested = cond.Y
	default:
		return false
	}
	assign, ok := s.Body.List[0].(*ast.AssignStmt)
	if !ok || assign.Tok != token.ASSIGN || len(assign.Lhs) != 1 || len(assign.Rhs) != 1 {
		return false
	}
	if types.ExprString(ast.Unparen(assign.Lhs[0])) != types.ExprString(ast.Unparen(tested)) {
		return false
	}
	if !initsField(info.TypeOf(assign.Lhs[0])) {
		return false
	}
	return isEmptyValue(assign.Rhs[0], info)
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
		if _, isLit := n.(*ast.FuncLit); isLit {
			return false
		}
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

// isMake reports whether e is a call to the builtin make: as a field's value,
// the nil-init of a map or slice field before inserting into or appending to
// it, which is not a bare write. A make that may allocate elements
// (isFilledMake), or one into any other field (initsField), is refused as a
// default before that, so only an empty one into a map or slice field is
// admitted there.
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
func rootIdent(e ast.Expr) *ast.Ident { return chainRoot(e, nil) }

// chainRoot is rootIdent, and given info it also steps through a slice
// expression (o in o.Spec.Items[:][0]) and an append to its first argument
// (o in append(o.Spec.Items, s)): either result may share its operand's
// backing array, so a write into it may reach whatever the operand reaches.
func chainRoot(e ast.Expr, info *types.Info) *ast.Ident {
	for {
		switch v := e.(type) {
		case *ast.Ident:
			return v
		case *ast.SelectorExpr:
			e = v.X
		case *ast.IndexExpr:
			e = v.X
		case *ast.SliceExpr:
			if info == nil {
				return nil
			}
			e = v.X
		case *ast.CallExpr:
			if info == nil || !isAppend(v, info) || len(v.Args) == 0 {
				return nil
			}
			e = v.Args[0]
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

// mayReachObj returns the object at the root of e seen through slice
// expressions and appends, or nil.
func mayReachObj(e ast.Expr, info *types.Info) types.Object {
	if id := chainRoot(e, info); id != nil {
		return info.ObjectOf(id)
	}
	return nil
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

// everRooted reports whether obj was a way to reach a caller's object at any
// point in the body. A local that never was is the helper's own value, so
// assigning it to a collection field replaces the collection rather than
// adding to it.
func (r roots) everRooted(obj types.Object) bool {
	if obj == nil {
		return false
	}
	for _, e := range r[obj] {
		if e.rooted {
			return true
		}
	}
	return false
}

// rootedBy reports whether obj reached a caller's object at any event at or
// before pos, whatever it was reassigned to since: the conservative reading
// for a write that must not reach the caller at all.
func (r roots) rootedBy(obj types.Object, pos token.Pos) bool {
	if obj == nil {
		return false
	}
	for _, e := range r[obj] {
		if e.pos > pos {
			break
		}
		if e.rooted {
			return true
		}
	}
	return false
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
// writes into it reach no caller-visible object and admit nothing. A
// comma-ok map read is not followed here: class a asks only whether a local
// was ever rooted, so rooting one read from another field (items, ok :=
// o.Spec.Groups[k]) would admit writing it back to a different field.
func rootedObjects(fn *ast.FuncDecl, info *types.Info) roots {
	return trackRoots(fn, info, rootObj, roots.at, func(e ast.Expr) ast.Expr { return e }, false)
}

// mayReachCaller returns, for the read-modify-write check, the objects that
// may reach a caller's object: rootedObjects' set, except that a local is
// rooted from where it is declared or assigned a chain seen through slice
// expressions (items := o.Spec.Items[:]) whose root reached a caller's object
// at any point up to there. Queried with rootedBy, a local stays rooted
// whatever it is reassigned to since, so a copy of a local a nil-init may
// have replaced (q := p after `if p == nil { p = new(int32) }`) is rooted too,
// and so is a local assigned with its name in parentheses ((q) = p), the
// result of an append (items := append(o.Spec.Items, s)), the first name of a
// comma-ok map read (items, ok := o.Spec.Groups[k]), and a range clause's
// variable that carries writes (for _, items := range o.Spec.Groups).
func mayReachCaller(fn *ast.FuncDecl, info *types.Info) roots {
	return trackRoots(fn, info, mayReachObj, roots.rootedBy, ast.Unparen, true)
}

// trackRoots seeds fn's parameters and records, for each local declared or
// assigned in its body, whether the object root finds in its value reaches a
// caller's object at that statement, as held reads it. dest normalises an
// assignment's destination before it is matched as a local's name; copies
// also roots the first name of a comma-ok map read and a range clause's
// variables.
func trackRoots(fn *ast.FuncDecl, info *types.Info, root func(ast.Expr, *types.Info) types.Object,
	held func(roots, types.Object, token.Pos) bool, dest func(ast.Expr) ast.Expr, copies bool,
) roots {
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
		if _, isLit := n.(*ast.FuncLit); isLit {
			return false
		}
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
					v := assignedValue(vs.Values, len(vs.Names), i, info, copies)
					rooted := v != nil && held(r, root(v, info), s.Pos())
					r.add(info.Defs[id], s.Pos(), rooted)
				}
			}
		case *ast.AssignStmt:
			for i, lhs := range s.Lhs {
				id, ok := dest(lhs).(*ast.Ident)
				if !ok {
					continue
				}
				v := assignedValue(s.Rhs, len(s.Lhs), i, info, copies)
				rooted := v != nil && held(r, root(v, info), s.Pos())
				r.add(info.ObjectOf(id), s.Pos(), rooted)
			}
		case *ast.RangeStmt:
			// Each key and element is a copy, which shares its referent with
			// the ranged collection when it carries writes.
			if !copies {
				return true
			}
			rooted := held(r, root(s.X, info), s.Pos())
			for _, x := range []ast.Expr{s.Key, s.Value} {
				if id, ok := dest(x).(*ast.Ident); ok {
					obj := info.ObjectOf(id)
					r.add(obj, s.Pos(), rooted && obj != nil && carriesWrites(obj.Type()))
				}
			}
		}
		return true
	})
	return r
}

// assignedValue returns the expression the i-th of n names is assigned from
// values: the i-th value, or with commaOk the read itself for the first name
// of a comma-ok map read (items, ok := o.Spec.Groups[k]), which copies the
// element as any other read does; nil when no single expression supplies it.
func assignedValue(values []ast.Expr, n, i int, info *types.Info, commaOk bool) ast.Expr {
	switch {
	case len(values) == n:
		return values[i]
	case commaOk && n == 2 && len(values) == 1 && i == 0 && isMapIndex(ast.Unparen(values[0]), info):
		return values[0]
	}
	return nil
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
// after labels := o.Labels).
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
		return ok && types.ExprString(v.Index) == types.ExprString(t.Index) && sameTarget(v.X, t.X, info, alias)
	case *ast.SelectorExpr:
		v, ok := x.(*ast.SelectorExpr)
		return ok && v.Sel.Name == t.Sel.Name && sameTarget(v.X, t.X, info, alias)
	case *ast.StarExpr:
		v, ok := x.(*ast.StarExpr)
		return ok && sameTarget(v.X, t.X, info, alias)
	}
	return false
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
		if _, isLit := n.(*ast.FuncLit); isLit {
			return false
		}
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
