package fluxcd

import (
	"slices"
	"strings"
	"testing"

	kyaml "sigs.k8s.io/kustomize/kyaml/yaml"
)

// TestBuildGuard_LoadedKindsMergeCycleDoesNotDependOnEntry: a merge cycle has no
// first member, so what a file reads of it is the same whichever member it
// enters the cycle by. A member keeps its own keys; a field it lacks is the one
// value the cycle offers; and a cycle that offers several values of a field a
// member lacks is refused from every entry, data or object (go-kure/kure#1253).
func TestBuildGuard_LoadedKindsMergeCycleDoesNotDependOnEntry(t *testing.T) {
	const (
		// listCycle: a has the kind, b the items; a merges b and b merges a.
		listCycle = "a: &a {kind: List, apiVersion: v1, inner: &b {<<: *a, items: [{apiVersion: v1, kind: Leaf, metadata: {name: x}}]}, <<: *b}\n"
		// ring: a merges b, b merges c, c merges a; the kind, the apiVersion
		// and the items are each held by one member.
		ring = "a: &a {kind: List, inner: &b {apiVersion: v1, inner: &c {items: [{apiVersion: v1, kind: Leaf}], <<: *a}, <<: *c}, <<: *b}\n"
		// conflict: c lacks the kind, and a and b hold different ones.
		conflict = "a: &a {kind: A, apiVersion: v1, inner: &b {kind: B, inner: &c {<<: *a}, <<: *c}, <<: *b}\n"
		// agreeing: the same, with the same kind in a and b.
		agreeing = "a: &a {kind: A, apiVersion: v1, inner: &b {kind: A, inner: &c {<<: *a}, <<: *c}, <<: *b}\n"
		// itemsConflict: c lacks the items, and a and b hold different ones.
		itemsConflict = "a: &a {kind: List, apiVersion: v1, items: [{apiVersion: v1, kind: One}], inner: &b {items: [{apiVersion: v1, kind: Two}], inner: &c {<<: *a}, <<: *c}, <<: *b}\n"
	)
	leaf := []kindKey{{"v1", "Leaf"}, {"v1", "List"}}
	for _, tc := range []struct {
		name, file string
		entries    []string
		refused    bool
		want       []kindKey
	}{
		{"a List split over two members", listCycle, []string{"a", "b"}, false, leaf},
		{"a ring of three, each member entered", ring, []string{"a", "b", "c"}, false, leaf},
		{"a kind two members offer differently, to a third", conflict, []string{"a", "b", "c"}, true, nil},
		{"a kind two members offer alike, to a third", agreeing, []string{"a", "b", "c"}, false, []kindKey{{"v1", "A"}}},
		{"items two members offer differently, to a third", itemsConflict, []string{"a", "b", "c"}, true, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var firstKinds []kindKey
			for i, entry := range tc.entries {
				g := newBuildGuard(nil, "")
				why := g.loadedKinds([]byte(tc.file + "<<: *" + entry + "\n"))
				if (why != "") != tc.refused {
					t.Fatalf("entered by %s: refusal %q, want refused %v", entry, why, tc.refused)
				}
				if tc.refused {
					if !strings.Contains(why, "ambiguous") {
						t.Errorf("entered by %s: refusal %q does not name the ambiguity", entry, why)
					}
					continue
				}
				if i == 0 {
					firstKinds = g.kinds
					if tc.want != nil && !slices.Equal(firstKinds, tc.want) {
						t.Errorf("entered by %s: kinds %v, want %v", entry, firstKinds, tc.want)
					}
					continue
				}
				if !slices.Equal(g.kinds, firstKinds) {
					t.Errorf("entered by %s: kinds %v, entered by %s: %v", entry, g.kinds, tc.entries[0], firstKinds)
				}
			}
		})
	}
}

// TestBuildGuard_LoadedKindsMergeCycleMembersAgree: after a cycle is settled,
// every member gives the same apiVersion, kind and items, whichever member the
// walk entered it by, so a mapping that merges only the other member reads the
// same List (go-kure/kure#1253).
func TestBuildGuard_LoadedKindsMergeCycleMembersAgree(t *testing.T) {
	var doc kyaml.Node
	const file = "a: &a {kind: List, apiVersion: v1, inner: &b {<<: *a, items: [{apiVersion: v1, kind: Leaf}]}, <<: *b}\n"
	if err := kyaml.NewDecoder(strings.NewReader(file)).Decode(&doc); err != nil {
		t.Fatal(err)
	}
	a := doc.Content[0].Content[1]
	b := a.Content[5] // the value of inner
	var items *kyaml.Node
	for i := 0; i+1 < len(b.Content); i += 2 {
		if b.Content[i].Value == "items" {
			items = b.Content[i+1]
		}
	}
	for _, order := range [][]*kyaml.Node{{a, b}, {b, a}} {
		g := newBuildGuard(nil, "")
		w := newKindWalk()
		for _, n := range order {
			c := w.mergeChain(g, n)
			if c.ambiguous || c.meta.kind != "List" || c.meta.apiVersion != "v1" || c.meta.items != items {
				t.Errorf("a member of the cycle, entered by %s first: kind %q, apiVersion %q, items %v (want %v), ambiguous %v",
					map[bool]string{true: "a", false: "b"}[order[0] == a], c.meta.kind, c.meta.apiVersion,
					c.meta.items != nil, items != nil, c.ambiguous)
			}
		}
	}
}

// TestBuildGuard_LoadedKindsMergeCycleMembersKeepTheirOwnKeys: a member that
// sets a field itself keeps it, so two members with different kinds are not
// ambiguous, and each entry reads the kind of the member it enters by.
func TestBuildGuard_LoadedKindsMergeCycleMembersKeepTheirOwnKeys(t *testing.T) {
	const file = "a: &a {kind: A, apiVersion: v1, inner: &b {kind: B, apiVersion: v1, <<: *a}, <<: *b}\n"
	for entry, want := range map[string]kindKey{"a": {"v1", "A"}, "b": {"v1", "B"}} {
		g := newBuildGuard(nil, "")
		if why := g.loadedKinds([]byte(file + "<<: *" + entry + "\n")); why != "" {
			t.Fatalf("entered by %s: refused: %s", entry, why)
		}
		if !slices.Equal(g.kinds, []kindKey{want}) {
			t.Errorf("entered by %s: kinds %v, want %v", entry, g.kinds, want)
		}
	}
}

// TestBuildGuard_LoadedKindsMergeCycleOffers: what the members of a cycle
// offer, and from where, decides whether the cycle is ambiguous
// (go-kure/kure#1253).
func TestBuildGuard_LoadedKindsMergeCycleOffers(t *testing.T) {
	const (
		// A cycle through a sequence of merge values, which sets no key and so
		// lacks all three: the mappings a and b hold different kinds of their own,
		// and what the sequence s gives, and so what the mapping that merges it
		// gives, would depend on the entry.
		throughSequence = "a: &a {apiVersion: v1, kind: Other, items: [], inner: &b {apiVersion: v1, kind: List, items: [{apiVersion: v1, kind: Leaf, metadata: {name: x}}], <<: *a}, <<: &s [*b]}\n"
		// Two nodes outside the cycle offer a kind each, one to each of its
		// members, and neither member sets one.
		blackConflict = "x: &x {kind: X}\ny: &y {kind: Y}\na: &a {apiVersion: v1, inner: &b {<<: [*a, *y]}, <<: [*x, *b]}\n"
		// The same apiVersion lacking in c, offered differently by a and b.
		apiConflict = "a: &a {apiVersion: v1, kind: K, inner: &b {apiVersion: v2, kind: K, inner: &c {<<: *a}, <<: *c}, <<: *b}\n"
		// The same items node, reached through two aliases.
		sameItems = "it: &it [{apiVersion: v1, kind: Leaf}]\na: &a {kind: List, apiVersion: v1, items: *it, inner: &b {items: *it, inner: &c {<<: *a}, <<: *c}, <<: *b}\n"
	)
	for _, tc := range []struct {
		name, file string
		entries    []string
		refused    bool
		want       []kindKey
	}{
		{"a sequence in the cycle", throughSequence + "apiVersion: v1\nkind: List\nitems: [{kind: Probe, <<: *b}, {<<: *s}]\n", nil, true, nil},
		{"a sequence in the cycle, the probe entering by the other member", throughSequence + "apiVersion: v1\nkind: List\nitems: [{kind: Probe, <<: *a}, {<<: *s}]\n", nil, true, nil},
		{"two nodes outside the cycle offering different kinds", blackConflict, []string{"a", "b"}, true, nil},
		{"two members offering different apiVersions", apiConflict, []string{"a", "b", "c"}, true, nil},
		{"one items node through two aliases", sameItems, []string{"a", "b", "c"}, false, []kindKey{{"v1", "Leaf"}, {"v1", "List"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			files := []string{tc.file}
			for _, entry := range tc.entries {
				files = append(files, tc.file+"<<: *"+entry+"\n")
			}
			if tc.entries != nil {
				files = files[1:]
			}
			for _, f := range files {
				g := newBuildGuard(nil, "")
				why := g.loadedKinds([]byte(f))
				if (why != "") != tc.refused {
					t.Fatalf("refusal %q, want refused %v\n%s", why, tc.refused, f)
				}
				if !tc.refused && !slices.Equal(g.kinds, tc.want) {
					t.Errorf("kinds %v, want %v", g.kinds, tc.want)
				}
			}
		})
	}
}

// TestBuildGuard_LoadedKindsMergeCycleOverLongKinds: members of a cycle that
// offer kinds longer than maxKindBytes are compared by a bounded part of each,
// so the cost of an offer does not grow with the length of the string, and
// such a kind names no kind whichever it is: the cycle builds as data and
// records none.
func TestBuildGuard_LoadedKindsMergeCycleOverLongKinds(t *testing.T) {
	long := strings.Repeat("k", maxKindBytes+10)
	for name, other := range map[string]string{"equal": long, "differing at the end": long[:len(long)-1] + "z"} {
		file := "a: &a {apiVersion: v1, kind: " + long + ", inner: &b {kind: " + other + ", inner: &c {<<: *a}, <<: *c}, <<: *b}\n<<: *c\n"
		g := newBuildGuard(nil, "")
		if why := g.loadedKinds([]byte(file)); why != "" {
			t.Errorf("%s: refused: %s", name, why)
		}
		if len(g.kinds) != 0 {
			t.Errorf("%s: kinds %v, want none", name, g.kinds)
		}
	}
}

// TestBuildGuard_LoadedKindsMergeCycleOverLongKindsEndingInList: two over-long
// kinds that share their first part but only one of which ends in List are not
// the same offer, since a kind that ends in List is read as its items, and
// that is asked of a kind of any length.
func TestBuildGuard_LoadedKindsMergeCycleOverLongKindsEndingInList(t *testing.T) {
	p := strings.Repeat("k", maxKindBytes+1)
	const items = "items: [{apiVersion: v1, kind: Leaf, metadata: {name: x}}]"
	file := "a: &a {apiVersion: v1, kind: " + p + "List, " + items + ", inner: &b {kind: " + p + "Data, inner: &c {inner: &s [*b, *a], <<: *s}, <<: *c}, <<: *b}\n"
	for _, entry := range []string{"a", "b", "c", "s"} {
		g := newBuildGuard(nil, "")
		if why := g.loadedKinds([]byte(file + "<<: *" + entry + "\n")); !strings.Contains(why, "ambiguous") {
			t.Errorf("entered by %s: refusal %q, want the ambiguity", entry, why)
		}
	}
}

// TestBuildGuard_LoadedKindsMergeCycleSequenceLacksEachField: a sequence of
// merge values in a cycle lacks apiVersion, kind and items each by itself, so
// the two mappings on either side of it, which set all three, make the cycle
// ambiguous when they differ in any one of them.
func TestBuildGuard_LoadedKindsMergeCycleSequenceLacksEachField(t *testing.T) {
	const prefix = "it: &it [{apiVersion: v1, kind: Leaf}]\nit2: &it2 [{apiVersion: v1, kind: Other}]\n"
	for _, tc := range []struct{ name, a, b string }{
		{"apiVersion", "{apiVersion: v1, kind: K, items: *it", "{apiVersion: v2, kind: K, items: *it"},
		{"kind", "{apiVersion: v1, kind: K1, items: *it", "{apiVersion: v1, kind: K2, items: *it"},
		{"items", "{apiVersion: v1, kind: K, items: *it", "{apiVersion: v1, kind: K, items: *it2"},
	} {
		file := prefix + "a: &a " + tc.a + ", inner: &b " + tc.b + ", <<: *a}, <<: &s [*b]}\n"
		for _, entry := range []string{"a", "b", "s"} {
			g := newBuildGuard(nil, "")
			if why := g.loadedKinds([]byte(file + "<<: *" + entry + "\n")); !strings.Contains(why, "ambiguous") {
				t.Errorf("%s, entered by %s: refusal %q, want the ambiguity", tc.name, entry, why)
			}
		}
	}
}

// TestBuildGuard_LoadedKindsAmbiguousCycleBelowTheDepthCap: a mapping below the
// Lists kure walks that reaches an ambiguous cycle is refused, though it has no
// metadata key and no List above it has one.
func TestBuildGuard_LoadedKindsAmbiguousCycleBelowTheDepthCap(t *testing.T) {
	const cycle = "a: &a {kind: A, inner: &b {kind: B, inner: &c {<<: *a}, <<: *c}, <<: *b}\n"
	g := newBuildGuard(nil, "")
	why := g.loadedKinds([]byte(listWith(cycle, listsAround(maxListDepth+1, "{apiVersion: v1, kind: Leaf, <<: *c}"))))
	if !strings.Contains(why, "Lists nested deeper") {
		t.Errorf("refusal %q, want the depth-cap refusal", why)
	}
}

// TestBuildGuard_LoadedKindsAmbiguousCycleOnData: a data file that reaches a
// merge cycle that leaves a field ambiguous is refused too, but one whose
// cycle offers a single value builds.
func TestBuildGuard_LoadedKindsAmbiguousCycleOnData(t *testing.T) {
	const ambiguous = "a: &a {kind: A, inner: &b {kind: B, inner: &c {<<: *a}, <<: *c}, <<: *b}\ndata: {x: y}\n<<: *c\n"
	const single = "a: &a {kind: A, apiVersion: v1, inner: &b {inner: &c {<<: *a}, <<: *c}, <<: *b}\ndata: {x: y}\n<<: *c\n"
	g := newBuildGuard(nil, "")
	if why := g.loadedKinds([]byte(ambiguous)); !strings.Contains(why, "ambiguous") {
		t.Errorf("ambiguous cycle on data: refusal %q, want one that names the ambiguity", why)
	}
	g = newBuildGuard(nil, "")
	if why := g.loadedKinds([]byte(single)); why != "" {
		t.Fatalf("conflict-free cycle on data: refused: %s", why)
	}
	if want := []kindKey{{"v1", "A"}}; !slices.Equal(g.kinds, want) {
		t.Errorf("kinds %v, want %v", g.kinds, want)
	}
}
