package main

import (
	"encoding/json"
	"go/parser"
	"go/token"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/go-kure/kure/pkg/kubernetes/internal/kinds"
)

// derivedOnce caches the real derivation: it loads and parses every pinned
// upstream module, which is the most expensive thing in this package's tests.
var derivedOnce struct {
	data tableData
	err  error
	done bool
}

func realTables(t *testing.T) tableData {
	t.Helper()
	if !derivedOnce.done {
		all, err := kinds.Registered()
		if err != nil {
			t.Fatal(err)
		}
		derivedOnce.data, derivedOnce.err = deriveTables(all)
		derivedOnce.done = true
	}
	if derivedOnce.err != nil {
		t.Fatal(derivedOnce.err)
	}
	return derivedOnce.data
}

// Every registered kind must reach the tables. A kind silently missing from an
// artifact is the failure this whole work item exists to prevent, and it is
// invisible in a rendered table that still looks plausible.
func TestDeriveTablesCoversEveryRegisteredKind(t *testing.T) {
	all, err := kinds.Registered()
	if err != nil {
		t.Fatal(err)
	}
	data := realTables(t)
	if len(data.Kinds) != len(all) {
		t.Fatalf("derived %d kinds, the scheme registers %d", len(data.Kinds), len(all))
	}
	seen := map[string]kindRow{}
	for _, k := range data.Kinds {
		key := k.Group + "/" + k.Kind
		if _, dup := seen[key]; dup {
			t.Errorf("%s appears twice", key)
		}
		seen[key] = k
	}
	for _, k := range all {
		row, ok := seen[k.Key()]
		if !ok {
			t.Fatalf("%s is registered but has no row", k.Key())
		}
		if row.Namespaced != k.Namespaced {
			t.Errorf("%s: row says namespaced=%v, the scheme says %v", k.Key(), row.Namespaced, k.Namespaced)
		}
		if row.Module == "" || row.ModuleVersion == "" {
			t.Errorf("%s: row names no module (%q@%q)", k.Key(), row.Module, row.ModuleVersion)
		}
		switch row.ScopeSource {
		case "marker", "builtin", "crd":
		default:
			t.Errorf("%s: unrecognised scope source %q", k.Key(), row.ScopeSource)
		}
	}
}

// Both table rows must be in a defined total order, or the same pins render
// differently between runs and CI's drift check fails with nothing wrong.
// Kinds are sorted here; Fields come out of maturity.Walk already sorted by
// import path, type and field (maturity.go § Walk), and this asserts that
// rather than trusting it — the walk's ordering is not this package's to keep.
func TestDeriveTablesRowsAreInATotalOrder(t *testing.T) {
	data := realTables(t)

	kindKey := func(k kindRow) [3]string { return [3]string{k.Group, k.Kind, k.Version} }
	fieldKey := func(f fieldRow) [3]string { return [3]string{f.ImportPath, f.TypeName, f.Field} }

	for _, tc := range []struct {
		name string
		n    int
		key  func(int) [3]string
	}{
		{"kinds", len(data.Kinds), func(i int) [3]string { return kindKey(data.Kinds[i]) }},
		{"fields", len(data.Fields), func(i int) [3]string { return fieldKey(data.Fields[i]) }},
	} {
		if tc.n == 0 {
			t.Errorf("%s: no rows", tc.name)
			continue
		}
		seen := map[[3]string]bool{}
		for i := 0; i < tc.n; i++ {
			if seen[tc.key(i)] {
				// Two rows with equal keys make the order below undefined: sort
				// is not stable, so either could come first on either run.
				t.Errorf("%s: %v appears twice", tc.name, tc.key(i))
			}
			seen[tc.key(i)] = true
			if i > 0 && !lessKey(tc.key(i-1), tc.key(i)) {
				t.Errorf("%s: row %d (%v) does not follow row %d (%v)", tc.name, i, tc.key(i), i-1, tc.key(i-1))
			}
		}
	}
}

func lessKey(a, b [3]string) bool {
	for i := range a {
		if a[i] != b[i] {
			return a[i] < b[i]
		}
	}
	return false
}

// The derivation is held to the scope kinds.Registered resolved, not only in
// the kinds package's own tests: a disagreement must never reach a committed
// artifact. Flipping one kind's scope proves that guard is wired.
func TestDeriveTablesRejectsAScopeDisagreement(t *testing.T) {
	all, err := kinds.Registered()
	if err != nil {
		t.Fatal(err)
	}
	var i int
	for i = range all {
		if all[i].Key() == "/Namespace" {
			break
		}
	}
	if all[i].Key() != "/Namespace" {
		t.Fatal("/Namespace is not registered; pick another cluster-scoped kind")
	}
	all[i].Namespaced = true
	if _, err := deriveTables(all); err == nil {
		t.Fatal("a kind whose stated scope contradicts the derivation must be an error")
	} else if !strings.Contains(err.Error(), "/Namespace") {
		t.Errorf("the error must name the disagreeing kind, got: %v", err)
	}
}

// The Go artifact is compiled by everything that imports the package, so a
// syntax error is caught anyway. What is not caught is a row that renders as
// valid Go while naming a constant nothing declares, so the maturity constants
// are checked by name here.
func TestRenderTablesGoParsesAndNamesTheMaturityConstants(t *testing.T) {
	src, err := renderTablesGo(realTables(t))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "zz_generated_tables.go", src, parser.AllErrors); err != nil {
		t.Fatalf("generated Go does not parse: %v", err)
	}
	text := string(src)
	if !strings.HasPrefix(text, tablesHeader) {
		t.Error("the generated Go lacks its header")
	}
	if !strings.Contains(text, `{Group: "", Version: "v1", Kind: "Namespace"`) {
		t.Error("no row for the core Namespace kind")
	}
	if !strings.Contains(text, `Namespaced: false, ScopeSource: "builtin"`) {
		t.Error("no cluster-scoped built-in row")
	}
	for _, want := range []string{"MaturityAlpha", "MaturityBeta", "MaturityDeprecated"} {
		if !strings.Contains(text, want) {
			t.Errorf("the pinned sources declare %s fields, but the artifact names no %s", want, want)
		}
	}
	if strings.Contains(text, `Stability: "`) {
		t.Error("stability must be rendered as the exported constant, not a bare string")
	}
}

func TestRenderTablesJSONRoundTripsAndCarriesProvenance(t *testing.T) {
	data := realTables(t)
	raw, err := renderTablesJSON(data)
	if err != nil {
		t.Fatal(err)
	}
	if raw[len(raw)-1] != '\n' {
		t.Error("the JSON artifact must end in a newline; it is committed and diffed")
	}
	var back tableJSON
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatalf("the JSON artifact does not parse: %v", err)
	}
	if back.Generated != jsonProvenance {
		t.Errorf("provenance = %q, want %q", back.Generated, jsonProvenance)
	}
	if len(back.Kinds) != len(data.Kinds) || len(back.Fields) != len(data.Fields) {
		t.Errorf("round trip lost rows: %d/%d kinds, %d/%d fields",
			len(back.Kinds), len(data.Kinds), len(back.Fields), len(data.Fields))
	}
	if back.Kinds[0].ScopeSource == "" {
		t.Error("the JSON must carry the scope provenance per row")
	}
}

func TestRenderTablesDocStatesItsCountsAndSources(t *testing.T) {
	data := realTables(t)
	page := string(renderTablesDoc(data))
	if !strings.HasPrefix(page, "<!-- Code generated by") {
		t.Error("the page lacks its generated header")
	}
	for _, want := range []string{
		"## Kinds (" + strconv.Itoa(len(data.Kinds)) + ")",
		"## Field maturity (" + strconv.Itoa(len(data.Fields)) + ")",
		"| `v1` | `Namespace` | Cluster | `builtin` |",
	} {
		if !strings.Contains(page, want) {
			t.Errorf("the page does not contain %q", want)
		}
	}
	// Every kind and every field is a row, plus the two column headers. The
	// separator rows start "|-" and are not counted. A table that silently
	// renders a subset still reads as a table, so the row count is asserted
	// rather than eyeballed.
	if got, want := strings.Count(page, "\n| "), len(data.Kinds)+len(data.Fields)+2; got != want {
		t.Errorf("%d table rows, want %d", got, want)
	}
}

func TestGoStrings(t *testing.T) {
	cases := []struct {
		in   []string
		want string
	}{
		{nil, "nil"},
		{[]string{}, "nil"},
		{[]string{"A"}, `[]string{"A"}`},
		{[]string{"A", "B"}, `[]string{"A", "B"}`},
	}
	for _, c := range cases {
		if got := goStrings(c.in); got != c.want {
			t.Errorf("goStrings(%q) = %s, want %s", c.in, got, c.want)
		}
	}
}

func TestMaturityConst(t *testing.T) {
	cases := map[string]string{
		"alpha":      "MaturityAlpha",
		"beta":       "MaturityBeta",
		"deprecated": "MaturityDeprecated",
		"":           "MaturityStable",
	}
	for in, want := range cases {
		if got := maturityConst(in); got != want {
			t.Errorf("maturityConst(%q) = %s, want %s", in, got, want)
		}
	}
}

func TestAPIVersion(t *testing.T) {
	if got := apiVersion("", "v1"); got != "v1" {
		t.Errorf("core group = %q, want v1", got)
	}
	if got := apiVersion("apps", "v1"); got != "apps/v1" {
		t.Errorf("apps = %q, want apps/v1", got)
	}
}

func TestShortImport(t *testing.T) {
	cases := map[string]string{
		"k8s.io/api/core/v1": "core/v1",
		"k8s.io/api":         "k8s.io/api",
		"single":             "single",
	}
	for in, want := range cases {
		if got := shortImport(in); got != want {
			t.Errorf("shortImport(%q) = %q, want %q", in, got, want)
		}
	}
}

// The table artifacts must never be written outside the tree the generator was
// pointed at: a test running against a temporary root would otherwise rewrite
// the repository's own docs directory as a side effect.
// Every spelling of the default root must resolve to the repo's own docs
// directory. A spelling treated as "somewhere else" writes the tables under
// <root>/docs and leaves the committed ones stale, with generate reporting
// success — the drift check would then be the only thing that notices, one
// commit later.
func TestDocsRoot(t *testing.T) {
	absDefault, err := filepath.Abs(defaultRoot)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		root, docs, want string
	}{
		{defaultRoot, "", defaultDocs},
		{"./" + defaultRoot, "", defaultDocs},
		{defaultRoot + "/", "", defaultDocs},
		{defaultRoot + "/../../" + defaultRoot, "", defaultDocs},
		{absDefault, "", defaultDocs},
		{defaultRoot, "elsewhere", "elsewhere"},
		{"/tmp/x", "", "/tmp/x/docs"},
		{"/tmp/x", "elsewhere", "elsewhere"},
	}
	for _, c := range cases {
		if got := docsRoot(c.root, c.docs); got != c.want {
			t.Errorf("docsRoot(%q, %q) = %q, want %q", c.root, c.docs, got, c.want)
		}
	}
}
