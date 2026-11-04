package fluxcd

import (
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	kyaml "sigs.k8s.io/kustomize/kyaml/yaml"
)

// TestBuildGuard_LoadedKindsFollowsAMergeChainInFull: the apiVersion, kind,
// items and metadata of a mapping come through its merge chain at any depth,
// so what lies behind two hops is read as what lies behind one (go-kure/kure#1253).
func TestBuildGuard_LoadedKindsFollowsAMergeChainInFull(t *testing.T) {
	const deepObject = "a: &a {apiVersion: v1, kind: ConfigMap, metadata: {name: x}}\n" +
		"b: &b {<<: *a}\nc: &c {<<: *b}\n"
	const itemsBehind = "a: &a {apiVersion: v1, kind: List, items: [{apiVersion: v1, kind: Item}]}\n" +
		"b: &b {<<: *a}\nc: &c {<<: *b}\n"
	for _, tc := range []struct {
		name, file string
		refused    bool
		want       []kindKey
	}{
		{"the kind and the metadata behind two hops", "a: &a {apiVersion: v1, kind: ConfigMap, metadata: {name: x}}\nb: &b {<<: *a}\n<<: *b\n",
			false, []kindKey{{"v1", "ConfigMap"}}},
		{"the kind and the metadata behind three hops", deepObject + "<<: *c\n", false, []kindKey{{"v1", "ConfigMap"}}},
		{"an own key beats the chain", deepObject + "kind: Own\n<<: *c\n", false, []kindKey{{"v1", "Own"}}},
		{"a key of a nearer source beats a farther one", "a: &a {kind: Far, apiVersion: v1}\nb: &b {<<: *a, kind: Near}\n<<: *b\n",
			false, []kindKey{{"v1", "Near"}}},
		{"the first source of a sequence that has a key gives it", "a: &a {kind: A}\nb: &b {<<: *a}\nc: &c {kind: C, apiVersion: x/v1}\n<<: [*b, *c]\n",
			false, []kindKey{{"x/v1", "A"}}},
		{"items behind three hops", itemsBehind + "<<: *c\n", false, []kindKey{{"v1", "Item"}, {"v1", "List"}}},
		{"a List with its kind behind three hops and an object among its items, below the depth cap",
			listWith("a: &a {kind: List, apiVersion: v1}\nb: &b {<<: *a}\nc: &c {<<: *b}\n",
				listsAround(maxListDepth, "{<<: *c, items: [{apiVersion: v1, kind: Leaf, metadata: {name: x}}]}")), true, nil},
		{"a List with its kind behind three hops and plain items, below the depth cap",
			listWith("a: &a {kind: List, apiVersion: v1}\nb: &b {<<: *a}\nc: &c {<<: *b}\n",
				listsAround(maxListDepth, "{<<: *c, items: [{apiVersion: v1, kind: Leaf}]}")), false, []kindKey{{"v1", "List"}}},
		{"metadata behind three hops on an object below the depth cap",
			listWith("a: &a {metadata: {name: x}}\nb: &b {<<: *a}\nc: &c {<<: *b}\n",
				listsAround(maxListDepth+1, "{apiVersion: v1, kind: Leaf, <<: *c}")), true, nil},
		{"a data file with a merge chain", "a: &a {apiVersion: v1, kind: ConfigMap}\nb: &b {<<: *a}\nc: &c {<<: *b}\n<<: *c\ndata: {x: y}\n",
			false, []kindKey{{"v1", "ConfigMap"}}},
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

// TestBuildGuard_LoadedKindsMergeCycles: a merge chain that leads back to
// itself ends. It is refused only when an object reaches it, whether by its
// own metadata or by sitting in a List that has some; a data file with one
// builds (go-kure/kure#1253).
func TestBuildGuard_LoadedKindsMergeCycles(t *testing.T) {
	// two is a cycle of two mappings: a merges b, which merges a, and only b
	// has metadata.
	const two = "a: &a {inner: &b {<<: *a, metadata: {name: m}}, <<: *b}\n"
	for _, tc := range []struct {
		name, file string
		refused    bool
		want       []kindKey
	}{
		{"a self merge on data", "a: &a {<<: *a}\napiVersion: v1\nkind: Foo\n<<: *a\n", false, []kindKey{{"v1", "Foo"}}},
		{"a self merge on an object", "a: &a {<<: *a}\napiVersion: v1\nkind: Foo\nmetadata: {name: x}\n<<: *a\n", true, nil},
		{"a self merge with the metadata in the cycle", "a: &a {metadata: {name: m}, <<: *a}\napiVersion: v1\nkind: Foo\n<<: *a\n", true, nil},
		{"a self merge on an item of an object List", "a: &a {<<: *a}\napiVersion: v1\nkind: List\nmetadata: {}\nitems:\n  - {kind: Foo, apiVersion: v1, <<: *a}\n", true, nil},
		{"a self merge on an item of a List with no metadata", "a: &a {<<: *a}\napiVersion: v1\nkind: List\nitems:\n  - {kind: Foo, apiVersion: v1, <<: *a}\n",
			false, []kindKey{{"v1", "Foo"}, {"v1", "List"}}},
		{"a chain that reaches a cycle, on data", "a: &a {<<: *a}\nb: &b {<<: *a}\nc: &c {<<: *b}\napiVersion: v1\nkind: Foo\n<<: *c\n", false, []kindKey{{"v1", "Foo"}}},
		{"a chain that reaches a cycle, on an object", "a: &a {<<: *a}\nb: &b {<<: *a}\nc: &c {<<: *b}\napiVersion: v1\nkind: Foo\nmetadata: {name: x}\n<<: *c\n", true, nil},
		{"two mappings that merge each other, with metadata in one, met from the one without it", two + "apiVersion: v1\nkind: Foo\n<<: *a\n", true, nil},
		{"two mappings that merge each other, with metadata in one, met from it", two + "apiVersion: v1\nkind: Foo\n<<: *b\n", true, nil},
		{"two mappings that merge each other, with no metadata", "a: &a {inner: &b {<<: *a}, <<: *b}\napiVersion: v1\nkind: Foo\n<<: *a\n", false, []kindKey{{"v1", "Foo"}}},
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

// TestBuildGuard_LoadedKindsMergeCycleMetadataIsExact: every member of a cycle
// holds the metadata any member does, whichever member the walk entered it by,
// so a later mapping that merges only a member does not miss it. The first
// member entered here is the one without metadata's neighbour, and the second
// is asked after the component has closed.
func TestBuildGuard_LoadedKindsMergeCycleMetadataIsExact(t *testing.T) {
	var doc kyaml.Node
	const file = "a: &a {inner: &b {<<: *a, metadata: {name: m}}, <<: *b}\n"
	if err := kyaml.NewDecoder(strings.NewReader(file)).Decode(&doc); err != nil {
		t.Fatal(err)
	}
	root := doc.Content[0]
	a := root.Content[1]
	b := a.Content[1]
	for _, order := range [][]*kyaml.Node{{a, b}, {b, a}} {
		g := newBuildGuard(nil, "")
		w := newKindWalk()
		for _, n := range order {
			c := w.mergeChain(g, n)
			if !c.cyclic || !c.meta.hasMetadata {
				t.Errorf("a member of the cycle: cyclic %v, metadata %v, want both", c.cyclic, c.meta.hasMetadata)
			}
		}
		if len(w.gray) != 0 {
			t.Errorf("%d nodes left gray after the walk", len(w.gray))
		}
	}
}

// TestBuildGuard_LoadedKindsLongMergeChain: a chain of 50000 mappings is
// followed with no recursion, and each edge of it is absorbed once.
func TestBuildGuard_LoadedKindsLongMergeChain(t *testing.T) {
	const hops = 50000
	var b strings.Builder
	b.WriteString("n0: &n0 {apiVersion: v1, kind: Deep, metadata: {name: x}}\n")
	for i := 1; i < hops; i++ {
		fmt.Fprintf(&b, "n%d: &n%d {<<: *n%d}\n", i, i, i-1)
	}
	fmt.Fprintf(&b, "<<: *n%d\n", hops-1)
	g := newBuildGuard(nil, "")
	start := time.Now()
	if why := g.loadedKinds([]byte(b.String())); why != "" {
		t.Fatalf("refused: %s", why)
	}
	if want := []kindKey{{"v1", "Deep"}}; !slices.Equal(g.kinds, want) {
		t.Errorf("kinds %v, want %v", g.kinds, want)
	}
	// One absorb for the merge key of the root and one for each of the chain.
	if g.kindAbsorbs != hops {
		t.Errorf("absorbed %d edges, want %d", g.kindAbsorbs, hops)
	}
	t.Logf("%d hops read in %v", hops, time.Since(start))
}

// TestTreeBuilds_AGeneratorDataFileWithAMultiHopMergeChainStillBuilds: the
// same as the one-hop control, with a chain three hops deep. The kinds it
// gives are now recorded and asked about, and the scope of a kind is asked
// about only to compare it with kyaml's built-in one, which holds for a kind
// no schema was added for, so a data file still builds.
func TestTreeBuilds_AGeneratorDataFileWithAMultiHopMergeChainStillBuilds(t *testing.T) {
	files := map[string]string{
		"app/kustomization.yaml": "configMapGenerator:\n  - name: data\n    files:\n      - data.yaml\n",
		"app/data.yaml": "a: &a {apiVersion: example.com/v1, kind: Widget}\nb: &b {<<: *a}\nc: &c {<<: *b}\nd: &d {<<: *c}\n" +
			"e: &e [*e]\ne2: &e2 {<<: *e2}\n<<: *d\ndata: {x: y}\n",
	}
	b, k := deletedWidgetTree(t, files)
	objs, err := b.build(k)
	if err != nil || len(objs) != 1 || objs[0].GetObjectKind().GroupVersionKind().Kind != "ConfigMap" {
		t.Fatalf("got %v, %v, want a read build of one ConfigMap", objs, err)
	}
}
