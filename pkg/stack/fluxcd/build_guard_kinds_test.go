package fluxcd

import (
	"slices"
	"strings"
	"testing"
	"unsafe"
)

// TestBuildGuard_LoadedKindsReadsNodesNotObjects: the kinds of a file are read
// from its YAML nodes, so a cyclic anchor in data kustomize only copies into a
// ConfigMap is not expanded (kyaml's de-anchoring recurses without end on it),
// a document that is no mapping holds no kind and does not end the read, a
// value or a mapping given as an alias is read through one hop, a merge key
// brings the fields the mapping lacks, and a kind that ends in List holds the
// kinds of its items (go-kure/kure#1235).
func TestBuildGuard_LoadedKindsReadsNodesNotObjects(t *testing.T) {
	const widget = "apiVersion: example.com/v1\nkind: Widget\nmetadata:\n  name: w\n"
	const w = "example.com/v1 Widget"
	indent := func(s string) string { return strings.ReplaceAll(strings.TrimSuffix(s, "\n"), "\n", "\n    ") }
	cases := map[string]struct {
		file string
		want []string
	}{
		"a cyclic anchor":                                    {"a: &a [*a]\n", nil},
		"a cyclic anchor and an object":                      {"a: &a [*a]\n---\n" + widget, []string{w}},
		"a list of patch operations":                         {"- op: remove\n  path: /spec\n", nil},
		"a scalar":                                           {"just text\n", nil},
		"empty":                                              {"", nil},
		"a separator with nothing after":                     {"---\n", nil},
		"empty documents around an object":                   {"---\n---\n" + widget + "---\n", []string{w}},
		"a list then an object":                              {"- a\n---\n" + widget, []string{w}},
		"one object twice":                                   {widget + "---\n" + widget + "$patch: delete\n", []string{w}},
		"no apiVersion":                                      {"kind: Widget\n", nil},
		"a non-scalar kind":                                  {"apiVersion: v1\nkind: [Widget]\n", nil},
		"a kind given as an alias":                           {"k: &k Widget\napiVersion: example.com/v1\nkind: *k\n", []string{w}},
		"an apiVersion given as an alias":                    {"v: &v example.com/v1\napiVersion: *v\nkind: Widget\n", []string{w}},
		"an alias of a mapping":                              {"m: &m {a: b}\napiVersion: example.com/v1\nkind: *m\n", nil},
		"a key given as an alias":                            {"k: &k kind\nv: &v apiVersion\n*v : example.com/v1\n*k : Widget\n", []string{w}},
		"a merge key":                                        {"base: &b {apiVersion: example.com/v1, kind: Widget}\n<<: *b\nmetadata: {name: w}\n", []string{w}},
		"a merge key of a sequence":                          {"a: &a {kind: Widget}\nb: &b {apiVersion: example.com/v1, kind: Other}\n<<: [*a, *b]\n", []string{w}},
		"a field the object sets wins":                       {"base: &b {apiVersion: example.com/v1, kind: Other}\n<<: *b\nkind: Widget\napiVersion: example.com/v1\n", []string{w}},
		"an empty kind hides the merged":                     {"base: &b {apiVersion: example.com/v1, kind: Widget}\nkind: \"\"\n<<: *b\n", nil},
		"a non-scalar kind hides the merged":                 {"base: &b {apiVersion: example.com/v1, kind: Widget}\nkind: [x]\n<<: *b\n", nil},
		"an empty kind in the first source hides the second": {"a: &a {kind: \"\"}\nb: &b {apiVersion: example.com/v1, kind: Widget}\n<<: [*a, *b]\n", nil},
		"a quoted << is no merge key":                        {"base: &b {apiVersion: example.com/v1, kind: Widget}\n\"<<\": *b\nmetadata: {name: w}\n", nil},
		"a List":                                             {"apiVersion: v1\nkind: List\nitems:\n  - " + indent(widget) + "\n", []string{w, "v1 List"}},
		"a WidgetList":                                       {"apiVersion: example.com/v1\nkind: WidgetList\nitems:\n  - " + indent(widget) + "\n", []string{w, "example.com/v1 WidgetList"}},
		"aliased items":                                      {"i: &i [{apiVersion: example.com/v1, kind: Widget}]\napiVersion: v1\nkind: List\nitems: *i\n", []string{w, "v1 List"}},
		"an aliased item":                                    {"i: &i {apiVersion: example.com/v1, kind: Widget}\napiVersion: v1\nkind: List\nitems: [*i]\n", []string{w, "v1 List"}},
		"a value over the bound":                             {"apiVersion: " + strings.Repeat("a", maxKindBytes+1) + "\nkind: Widget\n", nil},
		"a malformed second document":                        {widget + "---\n: : [\n", []string{w}},
		// kustomize splits a file at the --- lines before it parses, and parses
		// the first YAML document of each part: text after a document end marker
		// is no document of its own, and a malformed part hides no later one.
		"text after a document end marker":     {"apiVersion: v1\nkind: ConfigMap\nmetadata: {name: c}\n...\nignored\n---\n" + widget, []string{"v1 ConfigMap", w}},
		"a malformed middle document":          {widget + "---\n: : [\n---\napiVersion: v1\nkind: Other\n", []string{w, "v1 Other"}},
		"a separator with a trailing comment":  {widget + "--- # next\napiVersion: v1\nkind: Other\n", []string{w, "v1 Other"}},
		"a separator with content ends":        {widget + "--- x\napiVersion: v1\nkind: Other\n", []string{w}},
		"windows line endings":                 {"apiVersion: example.com/v1\r\nkind: Widget\r\n---\r\napiVersion: v1\r\nkind: Other\r\n", []string{w, "v1 Other"}},
		"a separator in the first line":        {"---\n" + widget, []string{w}},
		"a document-start marker with content": {"--- !!map\napiVersion: v1\nkind: Other\n", []string{"v1 Other"}},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			g := newBuildGuard(nil, "")
			if why := g.loadedKinds([]byte(tc.file)); why != "" {
				t.Fatalf("refused: %s", why)
			}
			var got []string
			for _, k := range g.kinds {
				got = append(got, k.apiVersion+" "+k.kind)
			}
			if !slices.Equal(got, tc.want) {
				t.Errorf("kinds = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestBuildGuard_LoadedKindsRefusesWhatItCannotFollow: a build is not read
// when an object's kind may come through a merge chain deeper than one hop, or
// through Lists nested deeper than maxListDepth, never silently skipped. A file
// with no metadata key, which is no object, is not refused for the merge chain.
func TestBuildGuard_LoadedKindsRefusesWhatItCannotFollow(t *testing.T) {
	chain := "a: &a {apiVersion: example.com/v1, kind: Widget}\nb: &b {<<: *a}\n"
	// lists is levels Lists, each inside the items of the one before; the
	// List at depth metaAt (0 is the outermost, -1 none) has a metadata key.
	// loadedKinds refuses the List at depth maxListDepth, the ninth level.
	lists := func(levels, metaAt int) string {
		var b strings.Builder
		for i := range levels {
			b.WriteString("{apiVersion: v1, kind: List, ")
			if i == metaAt {
				b.WriteString("metadata: {name: l}, ")
			}
			b.WriteString("items: [")
		}
		b.WriteString(strings.Repeat("]}", levels))
		return b.String() + "\n"
	}
	cases := map[string]struct {
		file    string
		refused bool
	}{
		"an object through a deep merge chain":             {chain + "<<: *b\nmetadata: {name: w}\n", true},
		"a data file through a deep merge chain":           {chain + "<<: *b\ndata: {x: y}\n", false},
		"quoted << keys nested in a ConfigMap":             {"apiVersion: v1\nkind: ConfigMap\nmetadata: {name: c}\n\"<<\": {\"<<\": {}}\n", false},
		"an object through a one-hop merge":                {chain + "<<: *a\nmetadata: {name: w}\n", false},
		"Lists nested past the bound, no metadata":         {lists(maxListDepth+1, -1), false},
		"Lists nested past the bound, outer metadata":      {lists(maxListDepth+1, 0), true},
		"Lists nested past the bound, inner metadata":      {lists(maxListDepth+1, maxListDepth-1), true},
		"Lists nested past the bound, last-level metadata": {lists(maxListDepth+1, maxListDepth), true},
		"Lists nested to the bound, with metadata":         {lists(maxListDepth, 0), false},
		"Lists nested to the bound, inner metadata":        {lists(maxListDepth, maxListDepth-1), false},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			g := newBuildGuard(nil, "")
			if why := g.loadedKinds([]byte(tc.file)); (why != "") != tc.refused {
				t.Errorf("refusal = %q, want refused %v", why, tc.refused)
			}
		})
	}
}

// TestBuildGuard_LoadedKindsSharesTheStringsOfAnAlias: a long value many kinds
// repeat through an alias is held once, as a pair of strings, not joined into a
// key for each (a 64 KB anchor and 10,000 items held 655 MB that way).
func TestBuildGuard_LoadedKindsSharesTheStringsOfAnAlias(t *testing.T) {
	var b strings.Builder
	b.WriteString("v: &v " + strings.Repeat("a", maxKindBytes) + "\napiVersion: v1\nkind: List\nitems:\n")
	const items = 2000
	for i := range items {
		b.WriteString("  - {apiVersion: *v, kind: K" + strings.Repeat("k", i%7) + "}\n")
	}
	g := newBuildGuard(nil, "")
	if why := g.loadedKinds([]byte(b.String())); why != "" {
		t.Fatalf("refused: %s", why)
	}
	if len(g.kinds) != 8 { // seven distinct kinds and the List
		t.Errorf("recorded %d kinds, want 8", len(g.kinds))
	}
	// The kinds that name the anchor's value hold the one string the node
	// holds, not a copy or a joined key of their own.
	var shared *byte
	for _, k := range g.kinds {
		if k.apiVersion != strings.Repeat("a", maxKindBytes) {
			continue
		}
		if p := unsafe.StringData(k.apiVersion); shared == nil {
			shared = p
		} else if p != shared {
			t.Fatalf("kind %q holds a copy of the apiVersion", k.kind)
		}
	}
	if shared == nil {
		t.Fatal("no kind names the anchor's apiVersion")
	}
}

// TestTreeBuilds_AGeneratorDataFileWithAMergeChainStillBuilds: a data file a
// configMapGenerator copies is no object, so a merge chain or a cyclic anchor
// in it is neither followed nor refused.
func TestTreeBuilds_AGeneratorDataFileWithAMergeChainStillBuilds(t *testing.T) {
	files := map[string]string{
		"app/kustomization.yaml": "configMapGenerator:\n  - name: data\n    files:\n      - data.yaml\n",
		"app/data.yaml":          "a: &a {apiVersion: example.com/v1, kind: Widget}\nb: &b {<<: *a}\nc: &c [*c]\n<<: *b\ndata: {x: y}\n",
	}
	b, k := deletedWidgetTree(t, files)
	objs, err := b.build(k)
	if err != nil || len(objs) != 1 || objs[0].GetObjectKind().GroupVersionKind().Kind != "ConfigMap" {
		t.Fatalf("got %v, %v, want a read build of one ConfigMap", objs, err)
	}
}
