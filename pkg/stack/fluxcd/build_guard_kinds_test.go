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
		"an object through a deep merge chain":   {chain + "<<: *b\nmetadata: {name: w}\n", true},
		"a data file through a deep merge chain": {chain + "<<: *b\ndata: {x: y}\n", false},
		"quoted << keys nested in a ConfigMap":   {"apiVersion: v1\nkind: ConfigMap\nmetadata: {name: c}\n\"<<\": {\"<<\": {}}\n", false},
		"an object through a one-hop merge":      {chain + "<<: *a\nmetadata: {name: w}\n", false},
		// The List has a metadata key, so its items are objects (kubectl
		// writes one): an item whose kind comes through a deep chain is
		// refused though it has no metadata key itself.
		"an item of an object List through a deep merge chain": {chain +
			"apiVersion: v1\nkind: List\nmetadata: {resourceVersion: \"\"}\nitems:\n  - {<<: *b}\n", true},
		"an item of a List with no metadata through a deep chain": {chain +
			"apiVersion: v1\nkind: List\nitems:\n  - {<<: *b}\n", false},
		"an item of an object List through a one-hop merge": {chain +
			"apiVersion: v1\nkind: List\nmetadata: {resourceVersion: \"\"}\nitems:\n  - {<<: *a}\n", false},
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

// TestBuildGuard_LoadedKindsAbsorbsAMergeSequenceOnce: what the sources of one
// merge value give together is worked out once for the call, so n mappings
// that merge a sequence of m sources cost m plus n absorbs and not n times m
// (go-kure/kure#1239: a file of 516 KB took 8.9 s under the build lock). The
// kinds recorded are the ones a read of the sources for every mapping gives,
// which the cases that follow pin beside the count.
func TestBuildGuard_LoadedKindsAbsorbsAMergeSequenceOnce(t *testing.T) {
	// sequence is a List of n mappings that each merge one anchored sequence of
	// m+1 references to one source.
	sequence := func(n, m int) string {
		var b strings.Builder
		b.WriteString("c: &c {apiVersion: g/v1, kind: K}\nq: &q [")
		for range m {
			b.WriteString("*c, ")
		}
		b.WriteString("*c]\napiVersion: v1\nkind: List\nitems:\n")
		for i := range n {
			fmt.Fprintf(&b, "  - {<<: *q, a: %d}\n", i)
		}
		return b.String()
	}
	wantKinds := []kindKey{{"g/v1", "K"}, {"v1", "List"}}
	for _, tc := range []struct{ n, m int }{{5, 5}, {2000, 2000}} {
		t.Run(fmt.Sprintf("n=%d m=%d", tc.n, tc.m), func(t *testing.T) {
			g := newBuildGuard(nil, "")
			if why := g.loadedKinds([]byte(sequence(tc.n, tc.m))); why != "" {
				t.Fatalf("refused: %s", why)
			}
			if want := (tc.m + 1) + tc.n; g.kindAbsorbs != want {
				t.Errorf("absorbed %d times, want %d", g.kindAbsorbs, want)
			}
			if !slices.Equal(g.kinds, wantKinds) {
				t.Errorf("kinds %v, want %v", g.kinds, wantKinds)
			}
		})
	}
	// Two merge values, each shared by half of the mappings: the sources of
	// each are worked out once, and every mapping takes one aggregate.
	t.Run("two shared merge values", func(t *testing.T) {
		const n = 50
		var b strings.Builder
		b.WriteString("c: &c {apiVersion: g/v1, kind: K}\nd: &d {apiVersion: h/v1, kind: Late, metadata: {name: m}}\n" +
			"q: &q [*c, *c, *c]\nr: &r [*d, *d]\napiVersion: v1\nkind: List\nitems:\n")
		for i := range n {
			merge := "*q"
			if i%2 == 1 {
				merge = "*r"
			}
			fmt.Fprintf(&b, "  - {<<: %s, a: %d}\n", merge, i)
		}
		g := newBuildGuard(nil, "")
		if why := g.loadedKinds([]byte(b.String())); why != "" {
			t.Fatalf("refused: %s", why)
		}
		if want := 3 + 2 + n; g.kindAbsorbs != want {
			t.Errorf("absorbed %d times, want %d", g.kindAbsorbs, want)
		}
		if want := []kindKey{{"g/v1", "K"}, {"h/v1", "Late"}, {"v1", "List"}}; !slices.Equal(g.kinds, want) {
			t.Errorf("kinds %v, want %v", g.kinds, want)
		}
	})
	// The first source that has a key gives it, and a flag any source has
	// holds: the aggregate of several sources is what absorbing them in order
	// gives.
	t.Run("the aggregate is first-wins for fields and any for flags", func(t *testing.T) {
		const file = "a: &a {kind: First}\nb: &b {kind: Second, apiVersion: x/v1, metadata: {name: m}}\n" +
			"q: &q [*a, *b]\napiVersion: v1\nkind: List\nitems:\n  - {<<: *q}\n  - {<<: *q}\n"
		g := newBuildGuard(nil, "")
		if why := g.loadedKinds([]byte(file)); why != "" {
			t.Fatalf("refused: %s", why)
		}
		if want := []kindKey{{"x/v1", "First"}, {"v1", "List"}}; !slices.Equal(g.kinds, want) {
			t.Errorf("kinds %v, want %v", g.kinds, want)
		}
		// The metadata of the second source makes each item an object, so a
		// deep merge key refuses it.
		const deep = "c: &c {kind: Deep}\nb: &b {<<: *c, metadata: {name: m}}\n" +
			"q: &q [*b]\napiVersion: v1\nkind: List\nitems:\n  - {<<: *q, apiVersion: v1}\n"
		if why := newBuildGuard(nil, "").loadedKinds([]byte(deep)); why == "" {
			t.Error("a deep merge in a source with metadata was not refused")
		}
	})
}

// listWith is a file whose root List holds item, and prefix is put before it
// for anchors.
func listWith(prefix, item string) string {
	return prefix + "apiVersion: v1\nkind: List\nitems:\n  - " + item + "\n"
}

// listsAround is leaf inside Lists so that the leaf is read at depth depth,
// the root List being depth 0: as flow YAML, the item of the root List.
func listsAround(depth int, leaf string) string {
	doc := leaf
	for range depth - 1 {
		doc = "{apiVersion: v1, kind: List, items: [" + doc + "]}"
	}
	return doc
}

// TestBuildGuard_LoadedKindsRefusesAnObjectBelowTheListDepthCap: Lists nested
// more than maxListDepth deep are not read, and the build is not read when what
// they hold may be an object, though no List above it has a metadata key: an
// object of the Lists is one with a metadata key of its own (go-kure/kure#1239:
// a ConfigMap nine Lists deep was dropped without a refusal).
func TestBuildGuard_LoadedKindsRefusesAnObjectBelowTheListDepthCap(t *testing.T) {
	const (
		object = "{apiVersion: v1, kind: Leaf, metadata: {name: x}}"
		plain  = "{apiVersion: v1, kind: Leaf}"
	)
	for _, tc := range []struct {
		name, file string
		refused    bool
		read       bool // the leaf kind is recorded
	}{
		{"an object at the cap is read", listWith("", listsAround(maxListDepth, object)), false, true},
		{"an object below the cap", listWith("", listsAround(maxListDepth+1, object)), true, false},
		{"an object far below the cap", listWith("", listsAround(maxListDepth+4, object)), true, false},
		{"no object below the cap", listWith("", listsAround(maxListDepth+1, plain)), false, false},
		{"no object far below the cap", listWith("", listsAround(maxListDepth+4, plain)), false, false},
		{"an object with its metadata through a merge key",
			listWith("m: &m {metadata: {name: x}}\n", listsAround(maxListDepth+1, "{apiVersion: v1, kind: Leaf, <<: *m}")), true, false},
		{"an object with its metadata behind a second merge hop",
			listWith("a: &a {metadata: {name: x}}\nb: &b {<<: *a}\n", listsAround(maxListDepth+1, "{apiVersion: v1, kind: Leaf, <<: *b}")), true, false},
		{"a merge chain that leads back to itself, with no metadata",
			listWith("a: &a {<<: *a}\n", listsAround(maxListDepth+1, "{apiVersion: v1, kind: Leaf, <<: *a}")), false, false},
		{"Lists without end that hold data", listWith("", "&a {apiVersion: v1, kind: List, items: [*a]}"), false, false},
		{"Lists without end that hold an object", listWith("", "&a {apiVersion: v1, kind: List, items: [*a, "+object+"]}"), true, false},
		{"an object among plain items of a List below the cap",
			listWith("", listsAround(maxListDepth, "{apiVersion: v1, kind: List, items: ["+plain+", "+object+"]}")), true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g := newBuildGuard(nil, "")
			why := g.loadedKinds([]byte(tc.file))
			if (why != "") != tc.refused {
				t.Fatalf("refusal %q, want refused %v", why, tc.refused)
			}
			if read := slices.Contains(g.kinds, kindKey{"v1", "Leaf"}); !tc.refused && read != tc.read {
				t.Errorf("leaf recorded %v, want %v (kinds %v)", read, tc.read, g.kinds)
			}
		})
	}
}

// TestBuildGuard_LoadedKindsScansBelowTheCapApart: the read of what lies below
// the depth cap keeps its own memo, so a List first met below the cap is still
// walked when an alias brings it to a shallower depth, and the reverse. With
// one memo for both, the second visit would skip the List and its kinds would
// be lost.
func TestBuildGuard_LoadedKindsScansBelowTheCapApart(t *testing.T) {
	// d is a List with one plain item, anchored where it is first met.
	const anchored = "&d {apiVersion: v1, kind: List, items: [{apiVersion: v1, kind: Leaf}]}"
	for _, tc := range []struct{ name, items string }{
		{"met below the cap first", listsAround(maxListDepth, anchored) + "\n  - *d"},
		{"met shallow first", anchored + "\n  - " + listsAround(maxListDepth, "*d")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g := newBuildGuard(nil, "")
			if why := g.loadedKinds([]byte(listWith("", tc.items))); why != "" {
				t.Fatalf("refused: %s", why)
			}
			if !slices.Contains(g.kinds, kindKey{"v1", "Leaf"}) {
				t.Errorf("the leaf the shallow visit reads was not recorded: %v", g.kinds)
			}
		})
	}
}

// TestBuildGuard_LoadedKindsScansBelowTheCapOnce: the read of what lies below
// the cap keeps to the nodes of the file, however many Lists name the same
// items: k Lists at the cap that hold one sequence of s items read the s items
// once, not k times.
func TestBuildGuard_LoadedKindsScansBelowTheCapOnce(t *testing.T) {
	const k, s = 200, 200
	var b strings.Builder
	b.WriteString("s: &s [")
	for range s {
		b.WriteString("{apiVersion: v1, kind: Leaf}, ")
	}
	b.WriteString("{apiVersion: v1, kind: Leaf}]\napiVersion: v1\nkind: List\nitems:\n")
	for range k {
		b.WriteString("  - " + listsAround(maxListDepth, "{apiVersion: v1, kind: List, items: *s}") + "\n")
	}
	g := newBuildGuard(nil, "")
	if why := g.loadedKinds([]byte(b.String())); why != "" {
		t.Fatalf("refused: %s", why)
	}
	// Each of the k chains reads its 8 Lists of 3 keys each, and the s+1 items
	// of the sequence have 2 keys each: well under the k times s items read
	// again for each List.
	if limit := k*8*3 + 2*(s+1) + 100; g.kindKeys > limit {
		t.Errorf("read %d keys, want at most %d", g.kindKeys, limit)
	}
	// The scan itself reads the sequence and its s+1 items once, for all k
	// Lists at the cap, and it did read them.
	if want := 1 + s + 1; g.kindScans != want {
		t.Errorf("scan read %d nodes, want %d", g.kindScans, want)
	}
	if want := []kindKey{{"v1", "List"}}; !slices.Equal(g.kinds, want) {
		t.Errorf("kinds %v, want %v", g.kinds, want)
	}
}

// TestBuildGuard_LoadedKindsScansAMergeChainOnce: n mappings below the depth
// cap that name one sequence of m mappings, each with a merge key of its own,
// walk the sequence once, not n times, and a chain that leads back to itself
// ends.
func TestBuildGuard_LoadedKindsScansAMergeChainOnce(t *testing.T) {
	const n, m = 2000, 2000
	var b strings.Builder
	b.WriteString("z: &z {}\nc: &c {<<: *z}\nq: &q [")
	for range m {
		b.WriteString("*c, ")
	}
	b.WriteString("*c]\n")
	var items strings.Builder
	for range n {
		items.WriteString("{<<: *q}, ")
	}
	file := listWith(b.String(), listsAround(maxListDepth, "{apiVersion: v1, kind: List, items: ["+items.String()+"]}"))
	g := newBuildGuard(nil, "")
	if why := g.loadedKinds([]byte(file)); why != "" {
		t.Fatalf("refused: %s", why)
	}
	// The n mappings and their sequence, the m+1 references once, and c, z.
	if limit := n + m + 20; g.kindScans < n || g.kindScans > limit {
		t.Errorf("scan read %d nodes, want %d to %d", g.kindScans, n, limit)
	}
	if want := []kindKey{{"v1", "List"}}; !slices.Equal(g.kinds, want) {
		t.Errorf("kinds %v, want %v", g.kinds, want)
	}
}

// TestBuildGuard_ReadsTheKindsOfInlinePatches: the kinds of the inline patches
// of a kustomization are asked about too, with no resource file naming them
// (go-kure/kure#1235). The Widget occurs only in the patches here, so a guard
// that does not read the inline patches records no Widget.
func TestBuildGuard_ReadsTheKindsOfInlinePatches(t *testing.T) {
	const widget = "apiVersion: example.com/v1\nkind: Widget\nmetadata:\n  name: w\n"
	indent := func(s string, n int) string {
		pad := strings.Repeat(" ", n)
		return pad + strings.ReplaceAll(strings.TrimSuffix(s, "\n"), "\n", "\n"+pad)
	}
	deletion := widget + "$patch: delete\n"
	for name, file := range map[string]string{
		"patches":               "patches:\n  - patch: |-\n" + indent(deletion, 6) + "\n",
		"patchesStrategicMerge": "patchesStrategicMerge:\n  - |-\n" + indent(deletion, 4) + "\n",
	} {
		t.Run(name, func(t *testing.T) {
			g := newBuildGuard(nil, "")
			if why := g.kustomization("app/kustomization.yaml", []byte(file)); why != "" {
				t.Fatalf("refused: %s", why)
			}
			if want := []kindKey{{"example.com/v1", "Widget"}}; !slices.Equal(g.kinds, want) {
				t.Errorf("kinds %v, want %v", g.kinds, want)
			}
		})
	}
}
