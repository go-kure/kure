package admission

import (
	"fmt"
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"strings"
)

// checkGrammar is grammar. It is a variable so that the package's own test can
// read the verdict of the checks that run before the grammar.
var checkGrammar = grammar

// refused formats the reason a body is outside the grammar: what was found,
// and the rule that refuses it.
func refused(rule, format string, args ...any) string {
	return fmt.Sprintf(format, args...) + " (grammar " + rule + ", purity §4)"
}

// guardKind is what an if statement is to the grammar.
type guardKind int

const (
	// noGuard is any if the grammar has no form for.
	noGuard guardKind = iota
	// receiverGuard is `if P == nil { panic(c) }` on a parameter.
	receiverGuard
	// errorGuard is `if err != nil { panic(M) }` on the error of a local.
	errorGuard
	// nilInitGuard is `if F == nil { F = <empty value> }`.
	nilInitGuard
)

// stepKind is the kind of one step of a path.
type stepKind int

const (
	// fieldStep selects a field, by its index in the struct.
	fieldStep stepKind = iota
	// derefStep follows a pointer, written (*p) or implied by a selector or
	// an index.
	derefStep
	// indexStep indexes a map, slice or array.
	indexStep
)

// step is one step of a path from the object.
type step struct {
	kind stepKind
	// field is the index of the field a fieldStep selects.
	field int
	// index is the key or index expression of an indexStep.
	index ast.Expr
	// over is the type an indexStep indexes: a map, a slice or an array.
	over types.Type
}

// write is one location the body writes, in source order.
type write struct {
	steps []step
	// text is the target as written, for a reason.
	text string
	// nilInit marks the assignment of a nil-init guard.
	nilInit bool
	// appends marks F = append(F, v).
	appends bool
}

// helperBody is what the grammar has read of one helper's body.
type helperBody struct {
	info *types.Info
	// params holds every parameter of the helper.
	params map[types.Object]bool
	// names holds the name of every parameter, in declaration order.
	names []*ast.Ident
	// keys holds the parameters a path read so far is indexed by.
	keys map[types.Object]bool
	// object is the parameter the first path in the body is spelled from, or
	// nil when that path is spelled from anything else.
	object types.Object
	// stable is stableParams: the pointer parameters the body never assigns
	// and never takes the address of.
	stable map[types.Object]bool
	// locals holds the first name of every local read so far.
	locals map[types.Object]bool
	// errs maps the second name of every local read so far to its statement.
	errs map[types.Object]ast.Stmt
	// wrote is set by the first nil-init guard or field write.
	wrote bool
	// writes holds every location written so far.
	writes []write
	// fieldWrites is the number of top-level assignments that declare
	// nothing: the helper's field writes, nil-init guards not counted.
	fieldWrites int
	// ops is the number of field writes read so far that append or insert.
	ops int
	// uses holds every occurrence of an argument in a written value or as the
	// argument of json.Marshal, in source order.
	uses []*ast.Ident
	// guards holds the parameter each receiver guard tests, in source order.
	guards []*ast.Ident
	// derefs holds the parameters a written value dereferences (*P).
	derefs map[types.Object]bool
}

// grammar returns the reason the body of fn is outside the grammar, or "". It
// first reads the whole body for what no statement may hold anywhere, then
// reads the top-level statements in source order and returns at the first
// one a rule refuses, then checks what only the whole body shows.
func grammar(fn *ast.FuncDecl, info *types.Info) string {
	// S9: a function literal has a body of its own that no rule reads, so a
	// closure could hide any write. It is refused wherever it stands, a
	// constant that holds one included.
	if hasFuncLit(fn.Body) {
		return refused("S9", "contains a function literal; its body is not checked, so sugar has none")
	}
	// S11: an append is the whole value of an assignment. One anywhere else,
	// an argument, a slice of its result or inside a constant, is no class a
	// statement, whatever it extends.
	if stray := strayAppends(fn.Body, info); len(stray) > 0 {
		return refused("S11", "uses append on %s other than as the whole value of an assignment", strings.Join(sortedKeys(stray), ", "))
	}
	b := &helperBody{
		info:   info,
		params: map[types.Object]bool{},
		keys:   map[types.Object]bool{},
		stable: stableParams(fn, info),
		locals: map[types.Object]bool{},
		errs:   map[types.Object]ast.Stmt{},
		derefs: map[types.Object]bool{},
	}
	for _, field := range fn.Type.Params.List {
		for _, name := range field.Names {
			b.names = append(b.names, name)
			if obj := info.Defs[name]; obj != nil {
				b.params[obj] = true
			}
		}
	}
	b.object = b.firstRoot(fn.Body.List)
	for _, stmt := range fn.Body.List {
		if s, ok := stmt.(*ast.AssignStmt); ok && s.Tok != token.DEFINE {
			b.fieldWrites++
		}
	}
	for i := range fn.Body.List {
		if reason := b.statement(fn.Body.List, i); reason != "" {
			return reason
		}
	}
	return b.whole()
}

// firstRoot returns the parameter the first path in stmts is spelled from:
// the target of the first assignment that declares nothing, at the top level
// or as the only statement of an if. It returns nil when that path is spelled
// from anything but a parameter.
func (b *helperBody) firstRoot(stmts []ast.Stmt) types.Object {
	for _, stmt := range stmts {
		if s, ok := stmt.(*ast.IfStmt); ok && len(s.Body.List) == 1 {
			stmt = s.Body.List[0]
		}
		s, ok := stmt.(*ast.AssignStmt)
		if !ok || s.Tok == token.DEFINE || len(s.Lhs) == 0 {
			continue
		}
		if root := pathRoot(s.Lhs[0], b.info); root != nil && b.params[root] {
			return root
		}
		return nil
	}
	return nil
}

// statement checks the top-level statement stmts[i].
func (b *helperBody) statement(stmts []ast.Stmt, i int) string {
	switch s := stmts[i].(type) {
	case *ast.IfStmt:
		return b.ifStmt(stmts, i, s)
	case *ast.AssignStmt:
		if s.Tok == token.DEFINE {
			return b.local(stmts, i, s)
		}
		return b.assignment(s)
	}
	return refused("S1", "%s is neither an if nor an assignment", stmtKind(stmts[i]))
}

// stmtKind names the kind of a statement the grammar has no form for.
func stmtKind(stmt ast.Stmt) string {
	switch s := stmt.(type) {
	case *ast.ExprStmt:
		return "the call statement " + types.ExprString(s.X)
	case *ast.DeclStmt:
		return "a declaration"
	case *ast.DeferStmt:
		return "a defer"
	case *ast.GoStmt:
		return "a go statement"
	case *ast.SendStmt:
		return "a send"
	case *ast.ForStmt:
		return "a for loop"
	case *ast.RangeStmt:
		return "a range loop"
	case *ast.SwitchStmt, *ast.TypeSwitchStmt:
		return "a switch"
	case *ast.SelectStmt:
		return "a select"
	case *ast.BlockStmt:
		return "a bare block"
	case *ast.LabeledStmt:
		return "a labelled statement"
	case *ast.IncDecStmt:
		return "the increment or decrement of " + types.ExprString(s.X)
	case *ast.EmptyStmt:
		return "an empty statement"
	}
	return "a statement"
}

// guard returns what s is to the grammar and the expression it tests for nil,
// parentheses aside. A receiver guard tests a parameter and an error guard the
// error of a local read so far, both by object.
func (b *helperBody) guard(s *ast.IfStmt) (guardKind, ast.Expr) {
	if s.Init != nil || s.Else != nil || len(s.Body.List) != 1 {
		return noGuard, nil
	}
	cond, ok := ast.Unparen(s.Cond).(*ast.BinaryExpr)
	if !ok || (cond.Op != token.EQL && cond.Op != token.NEQ) {
		return noGuard, nil
	}
	var tested ast.Expr
	switch {
	case isNilIdent(cond.Y, b.info):
		tested = ast.Unparen(cond.X)
	case isNilIdent(cond.X, b.info):
		tested = ast.Unparen(cond.Y)
	default:
		return noGuard, nil
	}
	switch inner := s.Body.List[0].(type) {
	case *ast.ExprStmt:
		id, ok := tested.(*ast.Ident)
		if !ok || !isBuiltinCall(inner.X, b.info, "panic") {
			return noGuard, nil
		}
		obj := b.info.ObjectOf(id)
		switch {
		case obj == nil:
		case cond.Op == token.EQL && b.params[obj]:
			return receiverGuard, tested
		case cond.Op == token.NEQ && b.errs[obj] != nil:
			return errorGuard, tested
		}
	case *ast.AssignStmt:
		if cond.Op == token.EQL && inner.Tok == token.ASSIGN && len(inner.Lhs) == 1 && len(inner.Rhs) == 1 {
			return nilInitGuard, tested
		}
	}
	return noGuard, nil
}

// ifStmt checks the if statement stmts[i]: its form (S8), and for a guard
// that panics its place (S2, S4) and its message (S5).
func (b *helperBody) ifStmt(stmts []ast.Stmt, i int, s *ast.IfStmt) string {
	kind, tested := b.guard(s)
	switch kind {
	case noGuard:
		return refused("S8", "if %s is not a nil guard that panics, the guard on the error of a local, or a nil-init guard", types.ExprString(s.Cond))
	case nilInitGuard:
		b.wrote = true
		return b.nilInit(s.Body.List[0].(*ast.AssignStmt), tested)
	case receiverGuard, errorGuard:
		// Both panic: checked below.
	}
	id := tested.(*ast.Ident)
	if b.wrote {
		return refused("S2", "the guard on %s comes after a write", id.Name)
	}
	call := ast.Unparen(s.Body.List[0].(*ast.ExprStmt).X).(*ast.CallExpr)
	if reason := b.message(call, kind, id); reason != "" {
		return reason
	}
	if kind == receiverGuard {
		b.guards = append(b.guards, id)
		return ""
	}
	if i == 0 || stmts[i-1] != b.errs[b.info.ObjectOf(id)] {
		return refused("S4", "the guard on %s does not directly follow the local it belongs to", id.Name)
	}
	return ""
}

// message checks the argument of the panic a guard calls (S5): a string
// constant, or in an error guard fmt.Sprintf of a string constant and that
// error.
func (b *helperBody) message(call *ast.CallExpr, kind guardKind, tested *ast.Ident) string {
	if len(call.Args) != 1 || call.Ellipsis.IsValid() {
		return refused("S5", "the guard on %s panics with other than one argument", tested.Name)
	}
	arg := call.Args[0]
	if b.isStringConst(arg) {
		return ""
	}
	if format, ok := ast.Unparen(arg).(*ast.CallExpr); ok && kind == errorGuard &&
		isFunc(format, b.info, "fmt", "Sprintf") && len(format.Args) == 2 && !format.Ellipsis.IsValid() &&
		b.isStringConst(format.Args[0]) {
		if id, ok := ast.Unparen(format.Args[1]).(*ast.Ident); ok && b.info.ObjectOf(id) == b.info.ObjectOf(tested) {
			return ""
		}
	}
	return refused("S5", "the guard on %s panics with %s, neither a string constant nor fmt.Sprintf of one and the error it guards", tested.Name, types.ExprString(arg))
}

// isStringConst reports whether e is a constant of kind string.
func (b *helperBody) isStringConst(e ast.Expr) bool {
	tv, ok := b.info.Types[ast.Unparen(e)]
	return ok && tv.Value != nil && tv.Value.Kind() == constant.String
}

// isFunc reports whether call calls the package-level function name of the
// package at path, resolved by object: a local or a method of that name is
// not it.
func isFunc(call *ast.CallExpr, info *types.Info, path, name string) bool {
	var id *ast.Ident
	switch fun := ast.Unparen(call.Fun).(type) {
	case *ast.Ident:
		id = fun
	case *ast.SelectorExpr:
		id = fun.Sel
	default:
		return false
	}
	fn, ok := info.Uses[id].(*types.Func)
	if !ok || fn.Pkg() == nil || fn.Pkg().Path() != path || fn.Name() != name {
		return false
	}
	sig, ok := fn.Type().(*types.Signature)
	return ok && sig.Recv() == nil
}

// local checks the short variable declaration stmts[i]: its form (S3), its
// place (S2) and the error guard that must follow it (S4).
func (b *helperBody) local(stmts []ast.Stmt, i int, s *ast.AssignStmt) string {
	value, err, arg := b.marshalled(s)
	if value == nil {
		return refused("S3", "%s is not v, err := json.Marshal(P) with two new names and P a parameter other than the object", stmtText(s))
	}
	if b.wrote {
		return refused("S2", "the local %s is declared after a write", value.Name())
	}
	b.locals[value], b.errs[err] = true, s
	b.uses = append(b.uses, arg)
	if i+1 < len(stmts) {
		if next, ok := stmts[i+1].(*ast.IfStmt); ok {
			if kind, tested := b.guard(next); kind == errorGuard && b.info.ObjectOf(tested.(*ast.Ident)) == err {
				return ""
			}
		}
	}
	return refused("S4", "the local %s is not directly followed by the guard on its error", value.Name())
}

// stmtText prints an assignment for a reason.
func stmtText(s *ast.AssignStmt) string {
	side := func(exprs []ast.Expr) string {
		texts := make([]string, len(exprs))
		for i, e := range exprs {
			texts[i] = types.ExprString(e)
		}
		return strings.Join(texts, ", ")
	}
	return side(s.Lhs) + " " + s.Tok.String() + " " + side(s.Rhs)
}

// marshalled returns the two objects s declares and the parameter it marshals
// when it is v, err := json.Marshal(P): two names, both new and neither
// blank, and P a parameter other than the object, parentheses aside. It
// returns nil for any other declaration.
func (b *helperBody) marshalled(s *ast.AssignStmt) (value, err types.Object, arg *ast.Ident) {
	if len(s.Lhs) != 2 || len(s.Rhs) != 1 {
		return nil, nil, nil
	}
	var declared [2]types.Object
	for i, lhs := range s.Lhs {
		id, ok := lhs.(*ast.Ident)
		if !ok || id.Name == "_" || b.info.Defs[id] == nil {
			return nil, nil, nil
		}
		declared[i] = b.info.Defs[id]
	}
	call, ok := ast.Unparen(s.Rhs[0]).(*ast.CallExpr)
	if !ok || !isFunc(call, b.info, "encoding/json", "Marshal") || len(call.Args) != 1 || call.Ellipsis.IsValid() {
		return nil, nil, nil
	}
	arg, ok = ast.Unparen(call.Args[0]).(*ast.Ident)
	if !ok {
		return nil, nil, nil
	}
	if obj := b.info.ObjectOf(arg); obj == nil || !b.params[obj] || obj == b.object {
		return nil, nil, nil
	}
	return declared[0], declared[1], arg
}

// nilInit checks the assignment of a nil-init guard: its target (P1, P2, P3,
// P5), and that the guard tests the path it assigns, spelled alike, which is
// a map, slice or pointer, and assigns it an empty value with constant sizes
// (N1).
func (b *helperBody) nilInit(s *ast.AssignStmt, tested ast.Expr) string {
	target := s.Lhs[0]
	text := types.ExprString(target)
	steps, reason := b.path(target)
	if reason != "" {
		return reason
	}
	if steps[len(steps)-1].kind != fieldStep {
		return refused("P5", "the nil-init of %s initialises other than a field", text)
	}
	// The guard tests the path it assigns: the same steps and keys, and the
	// same spelling, outer parentheses aside. (*o).Spec.Ref and o.Spec.Ref
	// are one path spelled two ways, and a guard on one is not a guard on
	// the other.
	of, ok := b.steps(tested)
	if !ok || pathRoot(tested, b.info) != b.object || len(of) != len(steps) || !b.leads(of, steps) ||
		types.ExprString(ast.Unparen(target)) != types.ExprString(ast.Unparen(tested)) {
		return refused("N1", "the guard tests %s and initialises %s", types.ExprString(tested), text)
	}
	// What a nil-init initialises is a map, a slice or a pointer
	// (initsField): an empty value in an interface or a channel is a value
	// the caller did not supply.
	if !initsField(b.info.TypeOf(target)) {
		return refused("N1", "the nil-init of %s initialises other than a map, slice or pointer", text)
	}
	if !b.isEmpty(s.Rhs[0]) {
		return refused("N1", "%s is initialised with %s, not T{}, &T{}, new(T) or a make with constant sizes", text, types.ExprString(s.Rhs[0]))
	}
	// A slice make with a length allocates elements the caller did not
	// supply: the value is empty only at length 0 (isEmptyValue).
	if !isEmptyValue(s.Rhs[0], b.info) {
		return refused("N1", "%s is initialised with %s, a make whose length is not 0", text, types.ExprString(s.Rhs[0]))
	}
	b.writes = append(b.writes, write{steps: steps, text: text, nilInit: true})
	return ""
}

// isEmpty reports whether e is the value of a nil-init: a composite literal
// with no elements or the address of one, new of a type, or make with
// constant sizes.
func (b *helperBody) isEmpty(e ast.Expr) bool {
	if lit := compositeOf(e); lit != nil {
		return len(lit.Elts) == 0
	}
	call, ok := ast.Unparen(e).(*ast.CallExpr)
	if !ok || len(call.Args) == 0 || call.Ellipsis.IsValid() {
		return false
	}
	switch {
	case isBuiltinCall(call, b.info, "new"):
		tv, ok := b.info.Types[ast.Unparen(call.Args[0])]
		return ok && tv.IsType() && len(call.Args) == 1
	case isBuiltinCall(call, b.info, "make"):
		for _, size := range call.Args[1:] {
			if !b.isConst(size) {
				return false
			}
		}
		return true
	}
	return false
}

// isConst reports whether e is a constant expression.
func (b *helperBody) isConst(e ast.Expr) bool {
	tv, ok := b.info.Types[ast.Unparen(e)]
	return ok && tv.Value != nil
}

// path returns the steps of the target e, or the reason it is not a path of
// the object (P1), names no field (P2) or is indexed by other than a
// parameter or a constant (P3).
func (b *helperBody) path(e ast.Expr) ([]step, string) {
	text := types.ExprString(e)
	root := pathRoot(e, b.info)
	if root == nil || root != b.object || !b.stable[root] {
		return nil, refused("P1", "%s is not spelled from the one pointer parameter the body writes through, never reassigned and its address never taken", text)
	}
	steps, ok := b.steps(e)
	if !ok {
		return nil, refused("P1", "%s is not spelled through fields, indexes and dereferences", text)
	}
	named := false
	for _, s := range steps {
		named = named || s.kind == fieldStep
	}
	if !named {
		return nil, refused("P2", "%s names no field", text)
	}
	for _, s := range steps {
		if s.kind != indexStep {
			continue
		}
		if !b.isKey(s.index) {
			return nil, refused("P3", "%s is indexed by %s, neither a parameter nor a constant", text, types.ExprString(s.index))
		}
		if id, ok := ast.Unparen(s.index).(*ast.Ident); ok {
			if obj := b.info.ObjectOf(id); b.params[obj] {
				b.keys[obj] = true
			}
		}
	}
	return steps, ""
}

// isKey reports whether e is a constant, or a parameter other than the
// object.
func (b *helperBody) isKey(e ast.Expr) bool {
	if b.isConst(e) {
		return true
	}
	id, ok := ast.Unparen(e).(*ast.Ident)
	if !ok {
		return false
	}
	obj := b.info.ObjectOf(id)
	return obj != nil && b.params[obj] && obj != b.object
}

// steps returns the steps e takes from the identifier it is spelled from,
// parentheses aside. A selector gives one step per field on the way to the one
// it names, embedded ones included, so a promoted field and its full spelling
// give the same steps; every pointer followed gives a step of its own, written
// or not, so a write to the struct behind a pointer overlaps a write through
// that pointer. It reports false for an expression that is not a path, or
// whose types are not recorded.
func (b *helperBody) steps(e ast.Expr) ([]step, bool) {
	switch v := ast.Unparen(e).(type) {
	case *ast.Ident:
		return nil, true
	case *ast.StarExpr:
		steps, ok := b.steps(v.X)
		return append(steps, step{kind: derefStep}), ok
	case *ast.IndexExpr:
		steps, ok := b.steps(v.X)
		t := b.info.TypeOf(v.X)
		if !ok || t == nil {
			return nil, false
		}
		if array, isPtr := t.Underlying().(*types.Pointer); isPtr {
			steps = append(steps, step{kind: derefStep})
			t = array.Elem()
		}
		return append(steps, step{kind: indexStep, index: v.Index, over: t}), true
	case *ast.SelectorExpr:
		steps, ok := b.steps(v.X)
		sel := b.info.Selections[v]
		if !ok || sel == nil || sel.Kind() != types.FieldVal {
			return nil, false
		}
		t := sel.Recv()
		for _, i := range sel.Index() {
			if p, isPtr := t.Underlying().(*types.Pointer); isPtr {
				steps = append(steps, step{kind: derefStep})
				t = p.Elem()
			}
			s, isStruct := t.Underlying().(*types.Struct)
			if !isStruct || i >= s.NumFields() {
				return nil, false
			}
			steps = append(steps, step{kind: fieldStep, field: i})
			t = s.Field(i).Type()
		}
		return steps, true
	}
	return nil, false
}

// overlaps reports whether one of a and c is a prefix of the other, or they
// are equal, with any index matching any index: two keys the body cannot tell
// apart may be the same element.
func overlaps(a, c []step) bool {
	for i := 0; i < len(a) && i < len(c); i++ {
		if a[i].kind != c[i].kind || a[i].field != c[i].field {
			return false
		}
	}
	return true
}

// leads reports whether a is a prefix of c, or equal to it, with every index
// the same one (sameIndex).
func (b *helperBody) leads(a, c []step) bool {
	if len(a) > len(c) {
		return false
	}
	for i := range a {
		if a[i].kind != c[i].kind || a[i].field != c[i].field {
			return false
		}
		if a[i].kind == indexStep && !b.sameIndex(a[i].index, c[i].index) {
			return false
		}
	}
	return true
}

// sameIndex reports whether x and y are the same key or index: the same
// expression (sameExpr), or two constants of one type and one value, however
// each is spelled. The type counts because a key of an interface type is the
// same key only with the same dynamic type.
func (b *helperBody) sameIndex(x, y ast.Expr) bool {
	if sameExpr(x, y, b.info) {
		return true
	}
	tx, okx := b.info.Types[ast.Unparen(x)]
	ty, oky := b.info.Types[ast.Unparen(y)]
	if !okx || !oky || tx.Value == nil || ty.Value == nil || tx.Type == nil || ty.Type == nil {
		return false
	}
	return types.Identical(tx.Type, ty.Type) && constant.Compare(tx.Value, token.EQL, ty.Value)
}

// initialises reports whether the nil-init early is what the write later goes
// through: a strict prefix of it, or the slice it appends to.
func (b *helperBody) initialises(early, later write) bool {
	return early.nilInit && b.leads(early.steps, later.steps) &&
		(len(early.steps) < len(later.steps) || later.appends)
}

// reaches returns the reason the write b.writes[i] cannot run behind the
// nil-init it goes through (P7), or "". A nil-init assigns one pointer, one
// map or one empty slice, and what lies past that holds its zero value. So
// behind the nearest nil-init ahead of it a write may follow that pointer or
// index that map, as its first step there, and past that step it follows no
// pointer and indexes no map. It indexes no slice at all: an append needs
// none. An array has its elements. The map behind &map[K]V{} is there and is
// refused like the one behind new: the rule reads the path, not the value. A
// nil-init is itself such a write behind the nil-init ahead of it.
func (b *helperBody) reaches(i int) string {
	w := b.writes[i]
	from, init := -1, ""
	for _, early := range b.writes[:i] {
		if early.nilInit && len(early.steps) > from && b.leads(early.steps, w.steps) {
			from, init = len(early.steps), early.text
		}
	}
	if from < 0 {
		return ""
	}
	for p := from; p < len(w.steps); p++ {
		s := w.steps[p]
		if s.kind == derefStep && p > from {
			return refused("P7", "%s follows a pointer past what the nil-init of %s assigned", w.text, init)
		}
		if s.kind != indexStep {
			continue
		}
		var over types.Type
		if s.over != nil {
			over = s.over.Underlying()
		}
		switch over.(type) {
		case *types.Array:
		case *types.Map:
			if p > from {
				return refused("P7", "%s indexes a map past what the nil-init of %s assigned", w.text, init)
			}
		default:
			return refused("P7", "%s indexes a slice, which the nil-init of %s leaves empty", w.text, init)
		}
	}
	return ""
}

// assignment checks an assignment that declares nothing: its form (S7), then
// the write it makes.
func (b *helperBody) assignment(s *ast.AssignStmt) string {
	if len(s.Lhs) != 1 || len(s.Rhs) != 1 || s.Tok != token.ASSIGN {
		return refused("S7", "%s is not one target assigned one value with =", stmtText(s))
	}
	b.wrote = true
	return b.fieldWrite(s.Lhs[0], s.Rhs[0])
}

// fieldWrite checks one write: its target (P1, P2, P3), that it is not a
// second append or insert (S10), and the value it writes (V1, V3, V3b, V2).
func (b *helperBody) fieldWrite(target, rhs ast.Expr) string {
	steps, reason := b.path(target)
	if reason != "" {
		return reason
	}
	// S10: class a is one statement. A second append or insert is refused
	// whatever it extends, the same field or another.
	if isMapIndex(target, b.info) || appendCall(rhs, b.info) != nil {
		b.ops++
		if b.ops > 1 {
			return refused("S10", "makes %d appends or inserts; class a is a single one", b.ops)
		}
	}
	w := write{steps: steps, text: types.ExprString(target)}
	// The value written is the element appended or inserted, or the value of
	// the field.
	value, element := rhs, isMapIndex(target, b.info)
	if call := appendCall(rhs, b.info); call != nil {
		if !extendsByOne(call, target, b.info) {
			return refused("V1", "%s is assigned %s, not an append of one value to itself", w.text, types.ExprString(rhs))
		}
		value, element, w.appends = call.Args[1], true, true
	}
	if reason := b.value(value, target, element); reason != "" {
		return reason
	}
	b.writes = append(b.writes, w)
	return ""
}

// value checks the value v written to target: its form (V1), the constants
// in it (V3), that every literal in it carries an argument (V3b) and that a
// literal is written where one may be (V2): as the element appended or
// inserted, as &T{...} into a pointer-typed field, or as the value of the
// helper's only field write. It then counts the arguments v carries.
func (b *helperBody) value(v, target ast.Expr, element bool) string {
	text, to := types.ExprString(v), types.ExprString(target)
	if bad := b.malformed(v); bad != nil {
		return refused("V1", "%s is written to %s, and %s is neither an argument passed whole nor a struct literal of arguments and constants", text, to, types.ExprString(bad))
	}
	if bad := b.unnamed(v); bad != nil {
		return refused("V3", "%s is written to %s, and %s is not a named constant of a defined type", text, to, types.ExprString(bad))
	}
	if bad := b.argless(v); bad != nil {
		return refused("V3b", "%s is written to %s, and %s carries no argument", text, to, types.ExprString(bad))
	}
	if compositeOf(v) != nil && !element && b.fieldWrites != 1 {
		_, addr := ast.Unparen(v).(*ast.UnaryExpr)
		toPointer := false
		if t := b.info.TypeOf(target); t != nil {
			_, toPointer = t.Underlying().(*types.Pointer)
		}
		if !addr || !toPointer {
			return refused("V2", "%s replaces %s beside another write; a struct literal is an appended or inserted element, &T{...} into a pointer-typed field, or the only write", text, to)
		}
	}
	b.count(v)
	return ""
}

// arg returns the identifier of the argument e passes whole, and whether e
// dereferences it: a parameter other than the object, written P, &P or *P, or
// the first name of a local, parentheses aside. It returns nil for anything
// else, the error of a local included.
func (b *helperBody) arg(e ast.Expr) (id *ast.Ident, deref bool) {
	e = ast.Unparen(e)
	operand := e
	switch v := e.(type) {
	case *ast.UnaryExpr:
		if v.Op != token.AND {
			return nil, false
		}
		operand = ast.Unparen(v.X)
	case *ast.StarExpr:
		operand, deref = ast.Unparen(v.X), true
	}
	id, ok := operand.(*ast.Ident)
	if !ok {
		return nil, false
	}
	obj := b.info.ObjectOf(id)
	switch {
	case obj == nil:
	case b.params[obj] && obj != b.object:
		return id, deref
	case b.locals[obj] && operand == e:
		return id, false
	}
	return nil, false
}

// elements returns the values of the elements of lit, keyed or positional.
func elements(lit *ast.CompositeLit) []ast.Expr {
	values := make([]ast.Expr, len(lit.Elts))
	for i, elt := range lit.Elts {
		values[i] = elt
		if kv, ok := elt.(*ast.KeyValueExpr); ok {
			values[i] = kv.Value
		}
	}
	return values
}

// malformed returns the first expression in e that has no form as a value:
// e is an argument, or a struct literal or the address of one whose elements
// are arguments, constants and such literals. It returns nil when e is
// well-formed.
func (b *helperBody) malformed(e ast.Expr) ast.Expr {
	if id, _ := b.arg(e); id != nil {
		return nil
	}
	lit := compositeOf(e)
	if lit == nil || !isStructLit(lit, b.info) {
		return e
	}
	for _, v := range elements(lit) {
		if id, _ := b.arg(v); id != nil || b.isConst(v) {
			continue
		}
		if bad := b.malformed(v); bad != nil {
			return bad
		}
	}
	return nil
}

// unnamed returns the first constant in the literals of e that is not a named
// constant of a defined type, or nil.
func (b *helperBody) unnamed(e ast.Expr) ast.Expr {
	lit := compositeOf(e)
	if lit == nil {
		return nil
	}
	for _, v := range elements(lit) {
		if id, _ := b.arg(v); id != nil {
			continue
		}
		if b.isConst(v) {
			if !b.isNamedConst(v) {
				return v
			}
			continue
		}
		if bad := b.unnamed(v); bad != nil {
			return bad
		}
	}
	return nil
}

// isNamedConst reports whether e names a declared constant, in this package
// or qualified by another, whose type is a defined type: the choice of one of
// a type's own values (corev1.ProtocolTCP), which an inline literal or a
// constant of a basic type is not.
func (b *helperBody) isNamedConst(e ast.Expr) bool {
	var id *ast.Ident
	switch v := ast.Unparen(e).(type) {
	case *ast.Ident:
		id = v
	case *ast.SelectorExpr:
		pkg, ok := ast.Unparen(v.X).(*ast.Ident)
		if !ok {
			return false
		}
		if _, isPkg := b.info.Uses[pkg].(*types.PkgName); !isPkg {
			return false
		}
		id = v.Sel
	default:
		return false
	}
	c, ok := b.info.Uses[id].(*types.Const)
	if !ok {
		return false
	}
	_, named := types.Unalias(c.Type()).(*types.Named)
	return named
}

// argless returns the first literal in e, the innermost first, that carries
// no argument at any depth, or nil.
func (b *helperBody) argless(e ast.Expr) *ast.CompositeLit {
	lit := compositeOf(e)
	if lit == nil {
		return nil
	}
	carries := false
	for _, v := range elements(lit) {
		if id, _ := b.arg(v); id != nil {
			carries = true
			continue
		}
		if compositeOf(v) == nil {
			continue
		}
		if bad := b.argless(v); bad != nil {
			return bad
		}
		carries = true
	}
	if !carries {
		return lit
	}
	return nil
}

// count records every argument the well-formed value e carries, and the
// parameters it dereferences.
func (b *helperBody) count(e ast.Expr) {
	if id, deref := b.arg(e); id != nil {
		b.uses = append(b.uses, id)
		if deref {
			b.derefs[b.info.ObjectOf(id)] = true
		}
		return
	}
	if lit := compositeOf(e); lit != nil {
		for _, v := range elements(lit) {
			b.count(v)
		}
	}
}

// whole checks what only the whole body shows: every receiver guard tests the
// object or a parameter a written value dereferences (S6), no location is
// written after a write to it, to a prefix of it or to anything under it,
// other than through a nil-init ahead of it (P4), every nil-init is written
// through (P6), a write behind a nil-init meets only what that nil-init
// assigned (P7), every argument is written once (V4), and every parameter is
// the object, a key or an argument (V5).
func (b *helperBody) whole() string {
	for _, id := range b.guards {
		if obj := b.info.ObjectOf(id); obj != b.object && !b.derefs[obj] {
			return refused("S6", "the guard on %s makes nil unexpressible for a parameter the body does not write as *%s", id.Name, id.Name)
		}
	}
	for i, later := range b.writes {
		for _, early := range b.writes[:i] {
			if overlaps(early.steps, later.steps) && !b.initialises(early, later) {
				return refused("P4", "%s is written after the write to %s, which it overlaps", later.text, early.text)
			}
		}
	}
	// A nil-init is used by a later write it initialises, and a later nil-init
	// counts only when it is used itself: read the writes last to first.
	used := make([]bool, len(b.writes))
	for i := len(b.writes) - 1; i >= 0; i-- {
		used[i] = !b.writes[i].nilInit
		for j := i + 1; j < len(b.writes) && !used[i]; j++ {
			used[i] = used[j] && b.initialises(b.writes[i], b.writes[j])
		}
	}
	for i, w := range b.writes {
		if !used[i] {
			return refused("P6", "the nil-init of %s is not written through", w.text)
		}
	}
	for i := range b.writes {
		if reason := b.reaches(i); reason != "" {
			return reason
		}
	}
	seen := map[types.Object]bool{}
	for _, id := range b.uses {
		obj := b.info.ObjectOf(id)
		if seen[obj] {
			return refused("V4", "the argument %s is written more than once", id.Name)
		}
		seen[obj] = true
	}
	for _, name := range b.names {
		obj := b.info.Defs[name]
		if obj == nil || obj != b.object && !seen[obj] && !b.keys[obj] {
			return refused("V5", "the parameter %s is not the object, not a key and not an argument written or marshalled", name.Name)
		}
	}
	return ""
}
