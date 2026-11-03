package fluxcd

import (
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"
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

// aliasedList is a List whose items are the aliases of one anchor: n of them
// name the List itself when self is set, else a Widget.
func aliasedList(n int, metadata bool) string {
	var b strings.Builder
	b.WriteString("&a {apiVersion: v1, kind: List, ")
	if metadata {
		b.WriteString("metadata: {name: l}, ")
	}
	b.WriteString("items: [")
	for i := range n {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString("*a")
	}
	return b.String() + "]}\n"
}

// TestBuildGuard_LoadedKindsWalksAShareOnce: a node met again with the same
// depth and object flag gives the same result, so it is walked once. A List
// that holds itself n times would otherwise be walked n to the power of the
// depth bound times, under the build lock (go-kure/kure#1240).
func TestBuildGuard_LoadedKindsWalksAShareOnce(t *testing.T) {
	t.Run("a List holding itself, pinned exactly", func(t *testing.T) {
		g := newBuildGuard(nil, "")
		if why := g.loadedKinds([]byte(aliasedList(4, false))); why != "" {
			t.Fatalf("refused: %s", why)
		}
		// One walk per depth, from the root to the bound.
		if want := maxListDepth + 1; g.kindVisits != want {
			t.Errorf("walked %d mappings, want %d", g.kindVisits, want)
		}
		if len(g.kinds) != 1 || g.kinds[0] != (kindKey{"v1", "List"}) {
			t.Errorf("kinds = %v, want the List", g.kinds)
		}
	})
	t.Run("a hundred aliases of a List that holds itself", func(t *testing.T) {
		for _, metadata := range []bool{false, true} {
			done := make(chan string, 1)
			go func() {
				g := newBuildGuard(nil, "")
				done <- g.loadedKinds([]byte(aliasedList(100, metadata)))
			}()
			select {
			case why := <-done:
				if refused := why != ""; refused != metadata {
					t.Errorf("metadata %v: refusal = %q", metadata, why)
				}
			case <-time.After(10 * time.Second):
				t.Fatalf("metadata %v: the walk did not end in 10 s", metadata)
			}
		}
	})
	t.Run("a shared tree reads as its expansion", func(t *testing.T) {
		shared := "w: &w {apiVersion: example.com/v1, kind: Widget}\n" +
			"l: &l {apiVersion: v1, kind: List, items: [*w, *w, *w]}\n" +
			"apiVersion: v1\nkind: List\nitems: [*l, *l]\n"
		const l = "{apiVersion: v1, kind: List, items: [" +
			"{apiVersion: example.com/v1, kind: Widget}, " +
			"{apiVersion: example.com/v1, kind: Widget}, " +
			"{apiVersion: example.com/v1, kind: Widget}]}"
		expanded := "apiVersion: v1\nkind: List\nitems: [" + l + ", " + l + "]\n"
		gs, ge := newBuildGuard(nil, ""), newBuildGuard(nil, "")
		if why := gs.loadedKinds([]byte(shared)); why != "" {
			t.Fatalf("shared refused: %s", why)
		}
		if why := ge.loadedKinds([]byte(expanded)); why != "" {
			t.Fatalf("expanded refused: %s", why)
		}
		if !slices.Equal(gs.kinds, ge.kinds) {
			t.Errorf("shared kinds %v, expanded %v", gs.kinds, ge.kinds)
		}
		// The shared tree: the root, one List, one Widget. The expansion
		// walks the List twice and the Widget six times.
		if gs.kindVisits != 3 || ge.kindVisits != 9 {
			t.Errorf("walked shared %d, expanded %d, want 3 and 9", gs.kindVisits, ge.kindVisits)
		}
	})
	t.Run("the object flag is part of a visit", func(t *testing.T) {
		// The anchored List is met first outside an object, where the depth
		// bound is no refusal, then inside a List with a metadata key, where
		// it is: the second visit must not be read as the first.
		file := "apiVersion: v1\nkind: List\nitems:\n" +
			"  - &a {apiVersion: v1, kind: List, items: [*a]}\n" +
			"  - {apiVersion: v1, kind: List, metadata: {name: m}, items: [*a]}\n"
		g := newBuildGuard(nil, "")
		if why := g.loadedKinds([]byte(file)); why == "" {
			t.Error("a List nested past the bound inside an object was not refused")
		}
	})
	t.Run("a legitimate List of fifty items is read whole", func(t *testing.T) {
		var b strings.Builder
		b.WriteString("apiVersion: v1\nkind: List\nmetadata: {name: l}\nitems:\n")
		for i := range 50 {
			b.WriteString("  - {apiVersion: example.com/v1, kind: Widget" + strings.Repeat("k", i%5) + "}\n")
		}
		g := newBuildGuard(nil, "")
		if why := g.loadedKinds([]byte(b.String())); why != "" {
			t.Fatalf("refused: %s", why)
		}
		if len(g.kinds) != 6 || g.kindVisits != 51 {
			t.Errorf("kinds %d, walked %d, want 6 and 51", len(g.kinds), g.kindVisits)
		}
	})
}

// sharedSequence is a file with one anchored sequence of n aliases of a
// Widget, and a List of m Lists that all hold that sequence as their items.
func sharedSequence(n, m int) string {
	var b strings.Builder
	b.WriteString("w: &w {apiVersion: example.com/v1, kind: Widget}\nseq: &s [")
	for range n {
		b.WriteString("*w, ")
	}
	b.WriteString("*w]\napiVersion: v1\nkind: List\nitems:\n")
	for range m {
		b.WriteString("  - {apiVersion: v1, kind: List, items: *s}\n")
	}
	return b.String()
}

// TestBuildGuard_LoadedKindsStepsASharedSequenceOnce: the items of a List are
// stepped over once for each sequence node, depth and object flag, so m Lists
// that hold one sequence of n items cost m plus n steps and not m times n
// (go-kure/kure#1240: about a minute at 1 MiB, under the build lock).
func TestBuildGuard_LoadedKindsStepsASharedSequenceOnce(t *testing.T) {
	t.Run("steps are pinned exactly", func(t *testing.T) {
		g := newBuildGuard(nil, "")
		if why := g.loadedKinds([]byte(sharedSequence(4, 3))); why != "" {
			t.Fatalf("refused: %s", why)
		}
		// The root steps over its 3 Lists, the first List over the 5 items of
		// the sequence, and the other two Lists find it stepped over.
		if g.kindItems != 3+5 || g.kindVisits != 1+3+1 {
			t.Errorf("stepped %d items, walked %d mappings, want 8 and 5", g.kindItems, g.kindVisits)
		}
		if len(g.kinds) != 2 {
			t.Errorf("kinds = %v, want the Widget and the List", g.kinds)
		}
	})
	t.Run("a file of about a megabyte", func(t *testing.T) {
		const n, m = 131000, 11400
		file := sharedSequence(n, m)
		if len(file) < 1<<20-64<<10 {
			t.Fatalf("the file is %d bytes, not megabyte-sized", len(file))
		}
		g := newBuildGuard(nil, "")
		if why := g.loadedKinds([]byte(file)); why != "" {
			t.Fatalf("refused: %s", why)
		}
		if g.kindItems != m+n+1 || g.kindVisits != m+2 {
			t.Errorf("stepped %d items, walked %d mappings, want %d and %d", g.kindItems, g.kindVisits, m+n+1, m+2)
		}
	})
	t.Run("Lists at different depths read as the expansion", func(t *testing.T) {
		shared := "w: &w {apiVersion: example.com/v1, kind: Widget}\n" +
			"s: &s [*w, {apiVersion: example.com/v1, kind: Other}]\n" +
			"apiVersion: v1\nkind: List\nitems:\n" +
			"  - {apiVersion: v1, kind: List, items: *s}\n" +
			"  - {apiVersion: v1, kind: List, items: [{apiVersion: v1, kind: List, items: *s}]}\n"
		const seq = "[{apiVersion: example.com/v1, kind: Widget}, {apiVersion: example.com/v1, kind: Other}]"
		expanded := "apiVersion: v1\nkind: List\nitems:\n" +
			"  - {apiVersion: v1, kind: List, items: " + seq + "}\n" +
			"  - {apiVersion: v1, kind: List, items: [{apiVersion: v1, kind: List, items: " + seq + "}]}\n"
		gs, ge := newBuildGuard(nil, ""), newBuildGuard(nil, "")
		if why := gs.loadedKinds([]byte(shared)); why != "" {
			t.Fatalf("shared refused: %s", why)
		}
		if why := ge.loadedKinds([]byte(expanded)); why != "" {
			t.Fatalf("expanded refused: %s", why)
		}
		if !slices.Equal(gs.kinds, ge.kinds) {
			t.Errorf("shared kinds %v, expanded %v", gs.kinds, ge.kinds)
		}
		// The root steps over 2 Lists, the first holds the sequence at depth 1
		// (2 items), the second one List (1 item) that holds it at depth 2
		// (2 items): the sequence is met at two depths, so stepped over twice.
		if gs.kindItems != 2+2+1+2 {
			t.Errorf("stepped %d items in the shared file, want 7", gs.kindItems)
		}
	})
	t.Run("the object flag is part of a step", func(t *testing.T) {
		// A sequence that holds a List that holds the sequence again is met
		// first outside an object, where the depth bound is no refusal, then
		// inside a List with a metadata key, where it is.
		file := "apiVersion: v1\nkind: List\nitems:\n" +
			"  - {apiVersion: v1, kind: List, items: &q [&a {apiVersion: v1, kind: List, items: *q}]}\n" +
			"  - {apiVersion: v1, kind: List, metadata: {name: m}, items: *q}\n"
		g := newBuildGuard(nil, "")
		if why := g.loadedKinds([]byte(file)); why == "" {
			t.Error("a List nested past the bound inside an object was not refused")
		}
	})
}

// TestBuildGuard_LoadedKindsReadsAMergeSourceOnce: what a mapping merges in
// through a merge key is read once for each source node and call, so n
// mappings that merge one source of m keys cost n plus m key reads and not n
// times m (go-kure/kure#1240: 44 s for a file of 1.25 MB, under the build
// lock). kindKeys counts the keys loadedKinds reads.
func TestBuildGuard_LoadedKindsReadsAMergeSourceOnce(t *testing.T) {
	// mergedList is a file with one anchored mapping of m keys and a List of n
	// mappings, each holding that mapping through its merge key.
	mergedList := func(n, m int, merge string) string {
		var b strings.Builder
		b.WriteString("b: &b {")
		for i := range m {
			fmt.Fprintf(&b, "k%d: 1, ", i)
		}
		b.WriteString("z: 1}\nc: &c {y: 1}\nq: &q [*b, *c]\napiVersion: v1\nkind: List\nitems:\n")
		for range n {
			b.WriteString("  - {" + merge + ", a: 1}\n")
		}
		return b.String()
	}
	t.Run("one source, pinned exactly", func(t *testing.T) {
		g := newBuildGuard(nil, "")
		if why := g.loadedKinds([]byte(mergedList(5, 7, "<<: *b"))); why != "" {
			t.Fatalf("refused: %s", why)
		}
		// The root reads its 6 keys, each item its 2, and the source its 8,
		// once.
		if want := 6 + 5*2 + 8; g.kindKeys != want {
			t.Errorf("read %d keys, want %d", g.kindKeys, want)
		}
	})
	t.Run("a shared sequence of sources", func(t *testing.T) {
		g := newBuildGuard(nil, "")
		if why := g.loadedKinds([]byte(mergedList(5, 7, "<<: *q"))); why != "" {
			t.Fatalf("refused: %s", why)
		}
		// The two sources of the sequence, 8 keys and 1, are read once.
		if want := 6 + 5*2 + 8 + 1; g.kindKeys != want {
			t.Errorf("read %d keys, want %d", g.kindKeys, want)
		}
	})
	t.Run("a file of about a megabyte", func(t *testing.T) {
		const n, m = 29000, 65000
		file := mergedList(n, m, "<<: *b")
		if len(file) < 1<<20 {
			t.Fatalf("the file is %d bytes, under a megabyte", len(file))
		}
		g := newBuildGuard(nil, "")
		if why := g.loadedKinds([]byte(file)); why != "" {
			t.Fatalf("refused: %s", why)
		}
		if want := 6 + n*2 + m + 1; g.kindKeys != want {
			t.Errorf("read %d keys, want %d", g.kindKeys, want)
		}
	})
	// The cases below hold the result of reading a merge source: they pass
	// with a read of the source for every mapping, and must go on passing.
	for _, tc := range []struct {
		name, file string
		want       []kindKey
		refused    bool
	}{
		{"an own key beats a merged one", "b: &b {apiVersion: v1, kind: Base}\n" +
			"apiVersion: v1\nkind: List\nitems:\n  - {kind: Own, <<: *b}\n",
			[]kindKey{{"v1", "Own"}, {"v1", "List"}}, false},
		{"the first source that has a key gives it", "b1: &b1 {kind: A}\nb2: &b2 {kind: B, apiVersion: x/v1}\n" +
			"apiVersion: v1\nkind: List\nitems:\n  - {<<: [*b1, *b2]}\n",
			[]kindKey{{"x/v1", "A"}, {"v1", "List"}}, false},
		{"an empty own value hides the merged one", "b: &b {apiVersion: v1, kind: Base}\n" +
			"apiVersion: v1\nkind: List\nitems:\n  - {kind: \"\", <<: *b}\n",
			[]kindKey{{"v1", "List"}}, false},
		{"a source that merges another is not followed", "c: &c {kind: Deep}\nb: &b {<<: *c, apiVersion: v1}\n" +
			"apiVersion: v1\nkind: List\nitems:\n  - {<<: *b}\n",
			[]kindKey{{"v1", "List"}}, false},
		{"a source that merges another, in an object, is refused", "c: &c {kind: Deep}\nb: &b {<<: *c, apiVersion: v1}\n" +
			"apiVersion: v1\nkind: List\nitems:\n  - {metadata: {name: m}, <<: *b}\n",
			nil, true},
		{"a source met directly, by alias and inline", "b: &b {apiVersion: v1, kind: Base}\n" +
			"apiVersion: v1\nkind: List\nitems:\n" +
			"  - {<<: *b}\n  - {<<: [*b, *b]}\n  - {<<: {apiVersion: v1, kind: Inline}}\n",
			[]kindKey{{"v1", "Base"}, {"v1", "Inline"}, {"v1", "List"}}, false},
		{"a source that holds items", "b: &b {kind: WidgetList, apiVersion: v1, items: [{apiVersion: x/v1, kind: Item}]}\n" +
			"apiVersion: v1\nkind: List\nitems:\n  - {<<: *b}\n",
			[]kindKey{{"x/v1", "Item"}, {"v1", "WidgetList"}, {"v1", "List"}}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g := newBuildGuard(nil, "")
			why := g.loadedKinds([]byte(tc.file))
			if (why != "") != tc.refused {
				t.Fatalf("refusal %q, want refused %v", why, tc.refused)
			}
			if !tc.refused && !slices.Equal(g.kinds, tc.want) {
				t.Errorf("kinds %v, want %v", g.kinds, tc.want)
			}
		})
	}
}
