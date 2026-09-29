package admission

import (
	"bufio"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// This file uses the exported API of the package only (Classify, Options,
// Finding, Class), so the same file runs against an earlier classifier whose
// source files are overlaid with go test -overlay, the other test files
// overlaid empty.

// goldenEnv, when set, rewrites the golden files under testdata from the
// current verdicts instead of checking against them. The diff of a rewrite is
// what changed.
const goldenEnv = "KURE_ADMISSION_GOLDEN"

// generatedEnv, when set to a path, writes the source of the generated package
// there, so a body named in a failure or in a golden file can be read.
const generatedEnv = "KURE_ADMISSION_GENERATED"

// differentialGolden holds the class of every generated body the classifier
// admits. Every other well-typed generated body is inadmissible.
const differentialGolden = "testdata/differential.golden"

// differentialRuns is how often a rewrite classifies the generated bodies. A
// body whose class differs between runs is recorded as unstable, with every
// class seen. The iteration of a small map starts at one of eight slots, so
// an order of two entries can turn up once in eight runs: at 128 runs the
// chance that it never does is below one in ten million.
const differentialRuns = 128

// genHeader is the generated package without its helpers: the types they
// write and the three imports some of them use.
const genHeader = `package gen

import (
	"encoding/json"
	"fmt"
	"unsafe"
)

var (
	_ = json.Marshal
	_ = fmt.Sprintf
	_ = unsafe.Sizeof(0)
)

type Kind string

const KindA Kind = "a"

type Ref struct {
	Name string
	Kind Kind
}

type Emb struct{ Tag string }

type Inner struct {
	Ref  *Ref
	Name string
	Kind Kind
}

type Spec struct {
	*Emb
	Ref      *Ref
	Ref2     *Ref
	Nested   *Inner
	Arr      *[3]string
	Slice    *[]string
	Items    []string
	Extra    []string
	Labels   map[string]string
	ExtraMap map[string]string
	Refs     []Ref
	RefMap   map[string]Ref
	Anys     []any
	Name     string
	Count    int
	Payload  any
	Inner    Inner
	Other    Inner
	Raw      []byte
}

type Obj struct{ Spec Spec }
`

// genWrite is one spelling of the write a body makes: the statement, with {F}
// the field as written, {O} the field spelled from (*o), {K} the key or index
// and {V} the value, and the type of the value.
type genWrite struct {
	stmt, vtype string
}

// genBase is one helper shape the generator varies: the field it writes or
// writes through, and every spelling of that write.
type genBase struct {
	// name is the helper's name after Set or Add.
	name string
	// add names the helper Add rather than Set.
	add bool
	// field is the field written, spelled from o.
	field string
	// typ is the type of field, and elem what a nil-init allocates for it:
	// the pointee of a pointer, the type itself otherwise.
	typ, elem string
	// init is the nil-init of the default body, a label of genNilInits.
	init string
	// key is the default key or index {K}, "" when the write has none.
	key string
	// value is the default value, a label of the value list.
	value string
	// nest is the field that nests this one behind a second pointer, "" when
	// the nesting list does not apply.
	nest string
	// writes maps a label of the write list to the write.
	writes map[string]genWrite
}

// through returns the spellings of a write through the pointer field {F} to
// what sel selects of its pointee: vtype is the type written, elem the pointee.
func through(sel, vtype, elem string) map[string]genWrite {
	return map[string]genWrite{
		"Plain":      {"{F}" + sel + " = {V}", vtype},
		"Paren":      {"({F})" + sel + " = {V}", vtype},
		"DerefObj":   {"{O}" + sel + " = {V}", vtype},
		"Star":       {"(*{F})" + sel + " = {V}", vtype},
		"StarParen":  {"(*({F}))" + sel + " = {V}", vtype},
		"Whole":      {"*{F} = {V}", elem},
		"WholeParen": {"*({F}) = {V}", elem},
	}
}

// direct returns the spellings of a write of the field itself.
func direct(vtype string) map[string]genWrite {
	return map[string]genWrite{
		"Plain":    {"{F} = {V}", vtype},
		"Paren":    {"({F}) = {V}", vtype},
		"DerefObj": {"{O} = {V}", vtype},
	}
}

// appendTo returns the spellings of an append to the field, and of a write
// of one of its elements.
func appendTo(vtype string) map[string]genWrite {
	return map[string]genWrite{
		"Plain":    {"{F} = append({F}, {V})", vtype},
		"Paren":    {"({F}) = append(({F}), {V})", vtype},
		"DerefObj": {"{O} = append({O}, {V})", vtype},
		"Index":    {"{F}[0] = {V}", vtype},
	}
}

// insert returns the spellings of an insert into the field.
func insert(vtype string) map[string]genWrite {
	return map[string]genWrite{
		"Plain":    {"{F}[{K}] = {V}", vtype},
		"Paren":    {"({F})[{K}] = {V}", vtype},
		"DerefObj": {"{O}[{K}] = {V}", vtype},
	}
}

// genBases lists the helper shapes. Each is varied by every list below.
var genBases = func() []genBase {
	emb := through(".Tag", "string", "Emb")
	emb["Promoted"] = genWrite{"o.Spec.Tag = {V}", "string"}
	emb["PromotedDerefObj"] = genWrite{"(*o).Spec.Tag = {V}", "string"}
	return []genBase{
		{name: "Ref", field: "o.Spec.Ref", typ: "*Ref", elem: "Ref", init: "Addr", nest: "o.Spec.Nested.Ref", writes: through(".Name", "string", "Ref")},
		{name: "Nested", field: "o.Spec.Nested", typ: "*Inner", elem: "Inner", init: "Addr", writes: through(".Name", "string", "Inner")},
		{name: "Arr", field: "o.Spec.Arr", typ: "*[3]string", elem: "[3]string", init: "Addr", key: "0", writes: through("[{K}]", "string", "[3]string")},
		{name: "SliceP", add: true, field: "o.Spec.Slice", typ: "*[]string", elem: "[]string", init: "Addr", writes: map[string]genWrite{
			"Plain":    {"*{F} = append(*{F}, {V})", "string"},
			"Paren":    {"*({F}) = append(*({F}), {V})", "string"},
			"DerefObj": {"*{O} = append(*{O}, {V})", "string"},
			"Star":     {"(*{F}) = append((*{F}), {V})", "string"},
			"Index":    {"(*{F})[0] = {V}", "string"},
			"Whole":    {"*{F} = {V}", "[]string"},
		}},
		{name: "Items", add: true, field: "o.Spec.Items", typ: "[]string", elem: "[]string", init: "Lit", writes: appendTo("string")},
		{name: "Labels", add: true, field: "o.Spec.Labels", typ: "map[string]string", elem: "map[string]string", init: "Lit", key: "key", writes: insert("string")},
		{name: "Emb", field: "o.Spec.Emb", typ: "*Emb", elem: "Emb", init: "Addr", writes: emb},
		{name: "Ptr", field: "o.Spec.Ref", typ: "*Ref", elem: "Ref", writes: direct("*Ref")},
		{name: "Comp", field: "o.Spec.Inner", typ: "Inner", elem: "Inner", value: "Keyed", writes: direct("Inner")},
		{name: "Refs", add: true, field: "o.Spec.Refs", typ: "[]Ref", elem: "[]Ref", writes: appendTo("Ref")},
		{name: "RefMap", add: true, field: "o.Spec.RefMap", typ: "map[string]Ref", elem: "map[string]Ref", key: "key", writes: insert("Ref")},
		{name: "Name", field: "o.Spec.Name", typ: "string", elem: "string", writes: direct("string")},
		{name: "Payload", field: "o.Spec.Payload", typ: "any", elem: "any", writes: direct("any")},
		{name: "Raw", field: "o.Spec.Raw", typ: "[]byte", elem: "[]byte", writes: direct("[]byte")},
		{name: "Anys", add: true, field: "o.Spec.Anys", typ: "[]any", elem: "[]any", writes: appendTo("any")},
	}
}()

// genNilInits is the list of nil-init values: {T} is the field's type, {E}
// what a nil-init allocates for it. "None" is a body without the nil-init.
var genNilInits = map[string]string{
	"None":       "",
	"Lit":        "{T}{}",
	"Addr":       "&{E}{}",
	"New":        "new({E})",
	"NewParen":   "new(({E}))",
	"NewValue":   "new({E}{})",
	"Make":       "make({T})",
	"Make0":      "make({T}, 0)",
	"MakeParen0": "make({T}, (0))",
	"Make0Cap4":  "make({T}, 0, 4)",
	"Make3":      "make({T}, 3)",
	"MakeParam":  "make({T}, n)",
	"Param":      "iv",
}

// genSpellings is the list of spellings of a nil-init's guard and target, of
// the field F.
var genSpellings = map[string]string{
	"Plain":    "{F}",
	"Paren":    "({F})",
	"DerefObj": "{O}",
}

// genGuards is genSpellings for the guard, with the operands of the
// comparison swapped.
var genGuards = map[string]string{
	"Plain":    "{F} == nil",
	"Paren":    "({F}) == nil",
	"DerefObj": "{O} == nil",
	"Reversed": "nil == {F}",
}

// genConsts is the list of constant expressions that hold what no constant
// of a helper may: a function literal, an append, or a read of what the
// write it sits in writes. Each is placed as a key or index, a make size, an
// array length or in the type of a literal. {R} is the location read.
var genConsts = map[string]string{
	"Func":   "len([1]func(){func() {}})",
	"Append": "unsafe.Sizeof(append([]int{}, 0))",
	"Read":   "unsafe.Sizeof({R})",
}

// genSeconds is the list of second statements, written after the write, with
// the parameters each needs.
var genSeconds = map[string]struct {
	stmt   string
	params []string
}{
	"Fwd":        {"o.Spec.Name = s", []string{"s string"}},
	"AmpFwd":     {"o.Spec.Payload = &s", []string{"s string"}},
	"StarFwd":    {"o.Spec.Count = *c", []string{"c *int"}},
	"Append":     {"o.Spec.Extra = append(o.Spec.Extra, s)", []string{"s string"}},
	"Insert":     {"o.Spec.ExtraMap[x] = s", []string{"x string", "s string"}},
	"Literal":    {"o.Spec.Other = Inner{Name: s, Kind: sk}", []string{"s string", "sk Kind"}},
	"PtrLiteral": {"o.Spec.Ref2 = &Ref{Name: s}", []string{"s string"}},
}

// genValue returns the value label writes into a location of type vtype, the
// parameters it needs and the statements it needs ahead of the body, or false
// when the value has no such form.
func genValue(label, vtype string) (expr string, params, pre []string, ok bool) {
	switch label {
	case "P":
		return "p", []string{"p " + vtype}, nil, true
	case "Paren":
		return "(p)", []string{"p " + vtype}, nil, true
	case "Amp":
		switch {
		case strings.HasPrefix(vtype, "*"):
			return "&p", []string{"p " + vtype[1:]}, nil, true
		case vtype == "any":
			return "&p", []string{"p string"}, nil, true
		}
	case "Star":
		if vtype == "any" {
			return "*p", []string{"p *string"}, nil, true
		}
		return "*p", []string{"p *" + vtype}, nil, true
	case "Nil":
		return "nil", []string{"nil " + vtype}, nil, true
	case "Keyed", "Positional", "Nested", "KeyedNil":
		lits := map[string]map[string]struct {
			lit    string
			params []string
		}{
			"Keyed": {
				"Ref":   {"Ref{Name: p, Kind: k}", []string{"p string", "k Kind"}},
				"Inner": {"Inner{Name: p, Kind: k}", []string{"p string", "k Kind"}},
				"Emb":   {"Emb{Tag: p}", []string{"p string"}},
			},
			"Positional": {
				"Ref":   {"Ref{p, k}", []string{"p string", "k Kind"}},
				"Inner": {"Inner{r, p, k}", []string{"r *Ref", "p string", "k Kind"}},
				"Emb":   {"Emb{p}", []string{"p string"}},
			},
			"Nested": {
				"Inner": {"Inner{Ref: &Ref{Name: p}, Name: q}", []string{"p string", "q string"}},
			},
			"KeyedNil": {
				"Inner": {"Inner{Ref: nil, Name: p}", []string{"nil *Ref", "p string"}},
			},
		}[label]
		switch t := vtype; {
		case strings.HasPrefix(t, "*"):
			if l, has := lits[t[1:]]; has {
				return "&" + l.lit, l.params, nil, true
			}
		case t == "any":
			for _, name := range []string{"Inner", "Ref"} {
				if l, has := lits[name]; has {
					return l.lit, l.params, nil, true
				}
			}
		default:
			if l, has := lits[t]; has {
				return l.lit, l.params, nil, true
			}
		}
	case "Marshal":
		if vtype == "[]byte" || vtype == "any" {
			return "b", []string{"p map[string]any"}, []string{
				"b, err := json.Marshal(p)",
				"if err != nil {\n\t\tpanic(fmt.Sprintf(\"marshal: %v\", err))\n\t}",
			}, true
		}
	}
	return "", nil, nil, false
}

// genValues is the list of values.
var genValues = []string{"P", "Paren", "Amp", "Star", "Nil", "Keyed", "Positional", "Nested", "KeyedNil", "Marshal"}

// genDims names the lists a body is varied by, in the order its name gives
// them, with each list's labels.
var genDims = []struct {
	code   string
	labels []string
}{
	{"Niv", sortedLabels(genNilInits)},
	{"Gs", sortedLabels(genGuards)},
	{"Ts", sortedLabels(genSpellings)},
	{"Ws", []string{"Plain", "Paren", "DerefObj", "Star", "StarParen", "Whole", "WholeParen", "Index", "Promoted", "PromotedDerefObj"}},
	{"Val", genValues},
	{"Cx", genConstPlaces()},
	{"Sec", append([]string{"None"}, sortedLabels(genSeconds)...)},
	{"Nest", []string{"Off", "On"}},
}

// genConstPlaces returns the labels of the constant list: every constant in
// every place, and "None".
func genConstPlaces() []string {
	labels := []string{"None"}
	for _, c := range sortedLabels(genConsts) {
		for _, place := range []string{"Key", "Size", "Length", "Type"} {
			labels = append(labels, c+place)
		}
	}
	return labels
}

// sortedLabels returns the keys of m in order.
func sortedLabels[V any](m map[string]V) []string {
	labels := make([]string, 0, len(m))
	for label := range m {
		labels = append(labels, label)
	}
	sort.Strings(labels)
	return labels
}

// defaults returns the label of each list in the default body of g.
func (g genBase) defaults() []string {
	init, value := g.init, g.value
	if init == "" {
		init = "None"
	}
	if value == "" {
		value = "P"
	}
	return []string{init, "Plain", "Plain", "Plain", value, "None", "None", "Off"}
}

// render returns the helper g with the labels choice, or false when the
// choice has no well-formed body for g. The body is written as: the statements
// a value needs, the outer nil-init of a nested field, the nil-init, the write,
// the second statement.
func (g genBase) render(choice []string) (name, body string, ok bool) {
	def := g.defaults()
	niv, gs, ts, ws, val, cx, sec, nest := choice[0], choice[1], choice[2], choice[3], choice[4], choice[5], choice[6], choice[7]
	var params, stmts []string
	paramType := map[string]string{}
	need := func(ps ...string) bool {
		for _, p := range ps {
			n, t, _ := strings.Cut(p, " ")
			if have, seen := paramType[n]; seen {
				if have != t {
					return false
				}
				continue
			}
			paramType[n] = t
			params = append(params, p)
		}
		return true
	}
	need("o *Obj")

	field := g.field
	if nest == "On" {
		if g.nest == "" {
			return "", "", false
		}
		field = g.nest
		stmts = append(stmts, "if o.Spec.Nested == nil {\n\t\to.Spec.Nested = &Inner{}\n\t}")
	}
	spell := func(tmpl string) string {
		return strings.NewReplacer("{F}", field, "{O}", "(*o)"+strings.TrimPrefix(field, "o")).Replace(tmpl)
	}

	// The constant, its place and the location it reads.
	cxConst, cxPlace := "", ""
	if cx != "None" {
		for _, c := range sortedLabels(genConsts) {
			if place, found := strings.CutPrefix(cx, c); found {
				cxConst, cxPlace = c, place
			}
		}
	}
	constant := func(read string) string {
		return strings.ReplaceAll(genConsts[cxConst], "{R}", read)
	}

	// The nil-init.
	target := spell(genSpellings[ts])
	nivText := genNilInits[niv]
	switch cxPlace {
	case "Size":
		if niv != def[0] {
			return "", "", false
		}
		switch {
		case strings.HasPrefix(g.typ, "[]"):
			nivText = "make({T}, 0, " + constant(target) + ")"
		case strings.HasPrefix(g.typ, "map["):
			nivText = "make({T}, " + constant(target) + ")"
		default:
			return "", "", false
		}
	case "Length":
		if niv != def[0] || !strings.HasPrefix(g.elem, "[3]") {
			return "", "", false
		}
		nivText = "&[(" + constant(target) + ")%1+3]" + strings.TrimPrefix(g.elem, "[3]") + "{}"
	}
	if nivText != "" {
		switch niv {
		case "Param":
			if !need("iv " + g.typ) {
				return "", "", false
			}
		case "MakeParam":
			if !need("n int") {
				return "", "", false
			}
		}
		nivText = strings.NewReplacer("{T}", g.typ, "{E}", g.elem).Replace(nivText)
		stmts = append(stmts, "if "+spell(genGuards[gs])+" {\n\t\t"+target+" = "+nivText+"\n\t}")
	} else if gs != def[1] || ts != def[2] {
		return "", "", false
	}

	// The write, its key and its value.
	w, has := g.writes[ws]
	if !has {
		return "", "", false
	}
	key := g.key
	switch {
	case cxPlace == "Key" && key == "":
		return "", "", false
	case cxPlace == "Key" && key == "0":
		key = "(" + constant(field) + ")%3"
	case cxPlace == "Key":
		key = "string(rune(" + constant(field) + "))"
	case key == "key":
		need("key string")
	}
	value, vparams, vpre, vok := genValue(val, w.vtype)
	if cxPlace == "Type" {
		if val != def[4] || w.vtype != "any" {
			return "", "", false
		}
		value = "struct {\n\t\tA    [(" + constant(field) + ")%1+1]int\n\t\tName string\n\t\tKind Kind\n\t}{Name: p, Kind: k}"
		vparams, vpre, vok = []string{"p string", "k Kind"}, nil, true
	}
	if !vok || !need(vparams...) {
		return "", "", false
	}
	stmts = append(stmts, strings.NewReplacer("{K}", key, "{V}", value).Replace(spell(w.stmt)))

	// The second statement.
	if sec != "None" {
		s := genSeconds[sec]
		if !need(s.params...) {
			return "", "", false
		}
		stmts = append(stmts, s.stmt)
	}

	prefix := "Set"
	if g.add {
		prefix = "Add"
	}
	name = prefix + g.name
	for i, label := range choice {
		if label != def[i] {
			name += "_" + genDims[i].code + label
		}
	}
	body = "(" + strings.Join(params, ", ") + ") {\n\t" + strings.Join(append(vpre, stmts...), "\n\t") + "\n}\n"
	return name, body, true
}

// genBody is one generated helper: its name, and its signature and body.
type genBody struct {
	name, body string
}

// generate returns every helper the generator writes: for each shape, its
// default body, the default with one label of one list changed, and the
// default with one label of each of two lists changed. A body written alike
// by two choices is kept once, under the first name.
func generate() []genBody {
	var bodies []genBody
	seen := map[string]bool{}
	add := func(g genBase, choice []string) {
		name, body, ok := g.render(choice)
		if !ok || seen[body] {
			return
		}
		seen[body] = true
		bodies = append(bodies, genBody{name, body})
	}
	with := func(choice []string, i int, label string) []string {
		c := append([]string(nil), choice...)
		c[i] = label
		return c
	}
	for _, g := range genBases {
		def := g.defaults()
		add(g, def)
		for i, di := range genDims {
			for _, a := range di.labels {
				if a == def[i] {
					continue
				}
				one := with(def, i, a)
				add(g, one)
				for j := i + 1; j < len(genDims); j++ {
					for _, b := range genDims[j].labels {
						if b != def[j] {
							add(g, with(one, j, b))
						}
					}
				}
			}
		}
	}
	return bodies
}

// genSource returns the generated package with the helpers bodies.
func genSource(bodies []genBody) string {
	var b strings.Builder
	b.WriteString(genHeader)
	for _, body := range bodies {
		b.WriteString("\nfunc " + body.name + body.body)
	}
	return b.String()
}

// genImporter gives the generated package the two functions it calls from
// the standard library, with their signatures, and unsafe: the type check
// that drops ill-typed bodies needs no export data.
type genImporter struct{}

func (genImporter) Import(path string) (*types.Package, error) {
	anyType := types.Universe.Lookup("any").Type()
	var fn *types.Func
	switch path {
	case "unsafe":
		return types.Unsafe, nil
	case "encoding/json":
		pkg := types.NewPackage(path, "json")
		fn = types.NewFunc(token.NoPos, pkg, "Marshal", types.NewSignatureType(nil, nil, nil,
			types.NewTuple(types.NewParam(token.NoPos, pkg, "v", anyType)),
			types.NewTuple(
				types.NewParam(token.NoPos, pkg, "", types.NewSlice(types.Typ[types.Byte])),
				types.NewParam(token.NoPos, pkg, "", types.Universe.Lookup("error").Type())),
			false))
	case "fmt":
		pkg := types.NewPackage(path, "fmt")
		fn = types.NewFunc(token.NoPos, pkg, "Sprintf", types.NewSignatureType(nil, nil, nil,
			types.NewTuple(
				types.NewParam(token.NoPos, pkg, "format", types.Typ[types.String]),
				types.NewParam(token.NoPos, pkg, "a", types.NewSlice(anyType))),
			types.NewTuple(types.NewParam(token.NoPos, pkg, "", types.Typ[types.String])),
			true))
	default:
		return nil, fmt.Errorf("the generated package imports %s", path)
	}
	fn.Pkg().Scope().Insert(fn)
	fn.Pkg().MarkComplete()
	return fn.Pkg(), nil
}

// wellTyped returns the bodies that parse and type-check. A package that
// fails to load fails as a whole (Classify reports every load error), so an
// ill-typed body is dropped here rather than classified.
func wellTyped(t *testing.T, bodies []genBody) []genBody {
	t.Helper()
	var parsed []genBody
	for _, b := range bodies {
		if _, err := parser.ParseFile(token.NewFileSet(), "", "package p\nfunc "+b.name+b.body, 0); err == nil {
			parsed = append(parsed, b)
		}
	}
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "gen.go", genSource(parsed), 0)
	if err != nil {
		t.Fatal(err)
	}
	ill := map[string]bool{}
	conf := types.Config{GoVersion: "go1.26", Importer: genImporter{}, Error: func(err error) {
		var terr types.Error
		if !errors.As(err, &terr) {
			t.Fatalf("type check: %v", err)
		}
		for _, decl := range file.Decls {
			if fn, isFunc := decl.(*ast.FuncDecl); isFunc && fn.Pos() <= terr.Pos && terr.Pos < fn.End() {
				ill[fn.Name.Name] = true
				return
			}
		}
		t.Fatalf("type check outside every helper: %v", err)
	}}
	_, _ = conf.Check("gen", fset, []*ast.File{file}, nil)
	var typed []genBody
	for _, b := range parsed {
		if !ill[b.name] {
			typed = append(typed, b)
		}
	}
	return typed
}

// writeGenerated writes the generated package src as a module of its own and
// returns its directory.
func writeGenerated(t *testing.T, src string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module gen\n\ngo 1.26\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "gen.go"), []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}
	return dir
}

// classifyGenerated classifies the generated package src, by helper name.
func classifyGenerated(t *testing.T, src string) map[string]Finding {
	t.Helper()
	return classifyDir(t, writeGenerated(t, src))
}

// classifyDir classifies the package in dir, by helper name.
func classifyDir(t *testing.T, dir string) map[string]Finding {
	t.Helper()
	findings, err := Classify(Options{
		Dir:      dir,
		Patterns: []string{"."},
		Env:      append(os.Environ(), "GOWORK=off", "GOFLAGS=-mod=mod"),
	})
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]Finding{}
	for _, f := range findings {
		got[f.Name] = f
	}
	return got
}

// classNamed returns the class whose String is s.
func classNamed(s string) (Class, bool) {
	for c := Inadmissible; c <= Exempt; c++ {
		if c.String() == s {
			return c, true
		}
	}
	return Inadmissible, false
}

// TestClassify_Differential classifies every body the generator writes and
// compares each verdict with differentialGolden: an admitted body with its
// class, an unstable one with any class it was seen with, every other body
// inadmissible. The lists the generator varies are what the classifier is
// known to decide by spelling or by the place of an expression; the golden
// freezes the verdict on each, so a change of any shows up as a failure.
func TestClassify_Differential(t *testing.T) {
	if testing.Short() {
		t.Skip("classifies a generated package of some thousand helpers")
	}
	generated := generate()
	bodies := wellTyped(t, generated)
	src := genSource(bodies)
	if path := os.Getenv(generatedEnv); path != "" {
		if err := os.WriteFile(path, []byte(src), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	count := fmt.Sprintf("bodies %d generated, %d well-typed", len(generated), len(bodies))

	if os.Getenv(goldenEnv) != "" {
		seen := map[string]map[Class]bool{}
		for range differentialRuns {
			for name, f := range classifyGenerated(t, src) {
				if seen[name] == nil {
					seen[name] = map[Class]bool{}
				}
				seen[name][f.Class] = true
			}
		}
		var b strings.Builder
		b.WriteString("# The class of every generated body the classifier admits; every other\n")
		b.WriteString("# well-typed generated body is inadmissible. A body whose class differed\n")
		b.WriteString("# between runs is unstable, with every class seen. Written by\n")
		b.WriteString("# TestClassify_Differential with " + goldenEnv + "=1; review the diff.\n")
		b.WriteString(count + "\n")
		for _, body := range bodies {
			classes := sortedLabels(func() map[string]bool {
				m := map[string]bool{}
				for c := range seen[body.name] {
					m[c.String()] = true
				}
				return m
			}())
			switch {
			case len(classes) > 1:
				b.WriteString(body.name + " unstable " + strings.Join(classes, " ") + "\n")
			case len(classes) == 1 && classes[0] != Inadmissible.String():
				b.WriteString(body.name + " " + classes[0] + "\n")
			}
		}
		writeGolden(t, differentialGolden, b.String())
		t.Logf("wrote %s: %s", differentialGolden, count)
		return
	}

	want, header := readDifferentialGolden(t)
	if header != count {
		t.Fatalf("%s says %q, the generator gives %q; rewrite it with %s=1", differentialGolden, header, count, goldenEnv)
	}
	got := classifyGenerated(t, src)
	names := map[string]bool{}
	mismatches := 0
	for _, body := range bodies {
		names[body.name] = true
		f, ok := got[body.name]
		if !ok {
			t.Errorf("%s: not classified", body.name)
			continue
		}
		accept := want[body.name]
		if accept == nil {
			accept = map[Class]bool{Inadmissible: true}
		}
		if !accept[f.Class] {
			mismatches++
			if mismatches <= 40 {
				t.Errorf("%s: %s (%s), want %s", body.name, f.Class, f.Reason, sortedLabels(func() map[string]bool {
					m := map[string]bool{}
					for c := range accept {
						m[c.String()] = true
					}
					return m
				}()))
			}
		}
	}
	if mismatches > 40 {
		t.Errorf("%d mismatches in all", mismatches)
	}
	for name := range want {
		if !names[name] {
			t.Errorf("%s: in %s but not generated", name, differentialGolden)
		}
	}
}

// writeGolden writes content to the golden file path under testdata.
func writeGolden(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

// readDifferentialGolden returns the classes differentialGolden accepts for
// each body it lists, and its count line.
func readDifferentialGolden(t *testing.T) (map[string]map[Class]bool, string) {
	t.Helper()
	f, err := os.Open(differentialGolden)
	if err != nil {
		t.Fatalf("%v; write it with %s=1", err, goldenEnv)
	}
	defer func() { _ = f.Close() }()
	want := map[string]map[Class]bool{}
	header := ""
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)
		switch {
		case len(fields) == 0 || strings.HasPrefix(line, "#"):
			continue
		case fields[0] == "bodies":
			header = line
			continue
		}
		names := fields[1:]
		if names[0] == "unstable" {
			names = names[1:]
		}
		want[fields[0]] = map[Class]bool{}
		for _, s := range names {
			c, ok := classNamed(s)
			if !ok {
				t.Fatalf("%s: unknown class %q in %s", fields[0], s, differentialGolden)
			}
			want[fields[0]][c] = true
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	return want, header
}
