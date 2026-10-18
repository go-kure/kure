package fluxcd

import (
	"encoding/json"
	"fmt"
	"path"
	"slices"
	"strings"

	kustv1 "github.com/fluxcd/kustomize-controller/api/v1"

	"github.com/go-kure/kure/pkg/errors"
	"github.com/go-kure/kure/pkg/stack/layout"
)

// Sentinels of the "umbrellas do not wait" setting
// (ResourceGenerator.UmbrellasDoNotWait) and of UmbrellaTerminals. Each
// refusal wraps one of them, so a caller can tell them apart with errors.Is
// whatever the message says.
var (
	// ErrUmbrellaWait is wrapped by a refusal of a wait the setting exists to
	// prevent: an explicit Wait true, or health checks, on an umbrella, and
	// the setting on an entry point that cannot apply it.
	ErrUmbrellaWait = errors.New("an umbrella Kustomization does not wait under UmbrellasDoNotWait")
	// ErrUmbrellaDependency is wrapped by a refusal of a dependsOn on an
	// umbrella that cannot be written as one on its terminal leaves.
	ErrUmbrellaDependency = errors.New("a dependsOn on an umbrella Kustomization cannot be written as one on its terminal leaves")
	// ErrUmbrellaTerminals is returned, wrapped by a message only, by
	// UmbrellaTerminals for trees it cannot compute terminal sets for.
	ErrUmbrellaTerminals = errors.New("umbrella terminal sets cannot be computed")
)

// kustomizationTree indexes the Flux Kustomizations of one layout tree by
// where they sit and what their directories build. It is shared by the
// reconcile-order check (checkPlacedReconcileOrder), the "umbrellas do not
// wait" pass (umbrellasDoNotWait) and UmbrellaTerminals, so all three read
// "Kustomization K applies CR C" the same way: K's build applies the
// Kustomizations hosted in the directories its spec.path builds
// (buildDirectories).
//
// The members are the Kustomizations the caller's filter selects (kusts, in
// depth-first layout order and, per layout, in the order kustomize builds its
// resources). Every other Flux Kustomization of the tree, an application's for
// one, is indexed too (others): it is applied like a member, and counts as a
// leaf.
type kustomizationTree struct {
	root     *layout.ManifestLayout
	kusts    []*kustv1.Kustomization
	byKey    map[string]*kustv1.Kustomization
	hostOf   map[string]*layout.ManifestLayout
	layoutAt map[string]*layout.ManifestLayout
	// others lists, per layout, the keys of the Flux Kustomizations it hosts
	// that are not members, and otherDeps their spec.dependsOn as keys.
	others    map[*layout.ManifestLayout][]string
	otherDeps map[string][]string
	reached   map[string]map[*layout.ManifestLayout]bool
	fromRoot  map[*layout.ManifestLayout]bool
}

// memberFilter returns the Flux Kustomizations of l that are members of the
// index, in typed form. Every other Flux Kustomization of l is indexed as a
// leaf (kustomizationTree.others).
type memberFilter func(l *layout.ManifestLayout) ([]*kustv1.Kustomization, error)

// generatedMembers selects the Kustomizations an integration placed or kept
// (generatedKustomizations).
func generatedMembers(generated map[string]bool) memberFilter {
	return func(l *layout.ManifestLayout) ([]*kustv1.Kustomization, error) {
		return generatedKustomizations(l, generated)
	}
}

// indexKustomizationTree indexes the tree under root with the members filter
// selects.
func indexKustomizationTree(root *layout.ManifestLayout, members memberFilter) (*kustomizationTree, error) {
	t := &kustomizationTree{
		root:      root,
		byKey:     map[string]*kustv1.Kustomization{},
		hostOf:    map[string]*layout.ManifestLayout{},
		layoutAt:  map[string]*layout.ManifestLayout{},
		others:    map[*layout.ManifestLayout][]string{},
		otherDeps: map[string][]string{},
		reached:   map[string]map[*layout.ManifestLayout]bool{},
		fromRoot:  map[*layout.ManifestLayout]bool{},
	}
	var index func(l *layout.ManifestLayout) error
	index = func(l *layout.ManifestLayout) error {
		t.layoutAt[path.Clean(l.FullRepoPath())] = l
		own, err := members(l)
		if err != nil {
			return err
		}
		for _, k := range own {
			key := crKey(k.Namespace, k.Name)
			t.kusts = append(t.kusts, k)
			t.byKey[key] = k
			t.hostOf[key] = l
		}
		objs, err := resourceItems(l)
		if err != nil {
			return err
		}
		for _, obj := range objs {
			if _, ok := fluxKustomizationPath(obj); !ok {
				continue
			}
			key := crKey(obj.GetNamespace(), obj.GetName())
			if t.byKey[key] != nil {
				continue
			}
			t.others[l] = append(t.others[l], key)
			t.hostOf[key] = l
			for _, dep := range fluxKustomizationDependsOn(obj) {
				t.otherDeps[key] = append(t.otherDeps[key], dependencyKey(obj.GetNamespace(), dep))
			}
		}
		for _, child := range l.Children {
			if child != nil {
				if err := index(child); err != nil {
					return err
				}
			}
		}
		return nil
	}
	if err := index(root); err != nil {
		return nil, err
	}
	for _, b := range buildDirectories(root) {
		t.fromRoot[b] = true
	}
	return t, nil
}

// dependencyKey is the key of a dependsOn entry as fluxKustomizationDependsOn
// writes it, on a Kustomization in namespace: a bare name is in namespace, a
// "<namespace>/<name>" entry in its own.
func dependencyKey(namespace, entry string) string {
	if ns, name, ok := strings.Cut(entry, "/"); ok {
		return crKey(ns, name)
	}
	return crKey(namespace, entry)
}

// builds returns the layouts the build of the Kustomization with key reaches,
// or nil when its spec.path is no layout of the tree.
func (t *kustomizationTree) builds(key string) map[*layout.ManifestLayout]bool {
	if r, ok := t.reached[key]; ok {
		return r
	}
	var r map[*layout.ManifestLayout]bool
	if k := t.byKey[key]; k != nil {
		if l := t.layoutAt[path.Clean(k.Spec.Path)]; l != nil {
			r = map[*layout.ManifestLayout]bool{}
			for _, b := range buildDirectories(l) {
				r[b] = true
			}
		}
	}
	t.reached[key] = r
	return r
}

// creator names the member whose apply creates the CR with key, or "" when
// the Flux bootstrap does (the root's build reaches its host) or no member
// does. The innermost wins: the member with the longest spec.path.
func (t *kustomizationTree) creator(key string) string {
	host := t.hostOf[key]
	if t.fromRoot[host] {
		return ""
	}
	best, bestLen := "", -1
	for _, k := range t.kusts {
		other := crKey(k.Namespace, k.Name)
		if other == key {
			continue
		}
		if t.builds(other)[host] && len(k.Spec.Path) > bestLen {
			best, bestLen = other, len(k.Spec.Path)
		}
	}
	return best
}

// applied lists the members the apply of the member with key writes: those
// its build reaches. With wait it waits for all of them, whoever else (the
// bootstrap) also creates them.
func (t *kustomizationTree) applied(key string) []string {
	r := t.builds(key)
	if r == nil {
		return nil
	}
	var out []string
	for _, other := range t.kusts {
		ok := crKey(other.Namespace, other.Name)
		if ok != key && r[t.hostOf[ok]] {
			out = append(out, ok)
		}
	}
	return out
}

// children lists every Flux Kustomization the apply of the member with key
// writes: the members its build reaches (applied), then the other Flux
// Kustomizations its build holds (buildScope, so an AppFileSingle
// application's file written into one of its directories counts), in build
// order.
func (t *kustomizationTree) children(key string) []string {
	out := t.applied(key)
	r := t.builds(key)
	k := t.byKey[key]
	if r == nil || k == nil {
		return out
	}
	for _, b := range buildScope(t.layoutAt[path.Clean(k.Spec.Path)]) {
		for _, other := range t.others[b] {
			if other != key {
				out = append(out, other)
			}
		}
	}
	return out
}

// isUmbrella reports whether the Kustomization with key is an umbrella: a
// member whose build applies at least one Flux Kustomization.
func (t *kustomizationTree) isUmbrella(key string) bool {
	return t.byKey[key] != nil && len(t.children(key)) > 0
}

// deps returns the spec.dependsOn of the Kustomization with key as keys.
func (t *kustomizationTree) deps(key string) []string {
	if k := t.byKey[key]; k != nil {
		var out []string
		for _, d := range k.Spec.DependsOn {
			ns := d.Namespace
			if ns == "" {
				ns = k.Namespace
			}
			out = append(out, crKey(ns, d.Name))
		}
		return out
	}
	return t.otherDeps[key]
}

// terminals returns the terminal leaves of umbrella u: for each Kustomization
// C that u applies and that no other Kustomization u applies depends on,
// terminals(C) when C is an umbrella, else C itself. A Flux Kustomization that
// is no member is a leaf: its own subtree is its author's. Entries are
// deduplicated, in the order of u's children. An empty set is an error that
// wraps ErrUmbrellaDependency.
func (t *kustomizationTree) terminals(u string) ([]string, error) {
	return t.terminalsFrom(u, map[string]bool{})
}

func (t *kustomizationTree) terminalsFrom(u string, visiting map[string]bool) ([]string, error) {
	if visiting[u] {
		return nil, errors.Wrapf(ErrUmbrellaDependency, "Flux Kustomization %q applies itself through the Kustomizations it applies", u)
	}
	visiting[u] = true
	defer delete(visiting, u)
	children := t.children(u)
	dependedOn := map[string]bool{}
	for _, c := range children {
		for _, d := range t.deps(c) {
			if d != c && slices.Contains(children, d) {
				dependedOn[d] = true
			}
		}
	}
	var out []string
	for _, c := range children {
		if dependedOn[c] {
			continue
		}
		expanded := []string{c}
		if t.isUmbrella(c) {
			var err error
			if expanded, err = t.terminalsFrom(c, visiting); err != nil {
				return nil, err
			}
		}
		for _, e := range expanded {
			if !slices.Contains(out, e) {
				out = append(out, e)
			}
		}
	}
	if len(out) == 0 {
		return nil, errors.Wrapf(ErrUmbrellaDependency, "Flux Kustomization %q has no terminal leaf: every Kustomization it applies is depended on by another one it applies", u)
	}
	return out, nil
}

// within reports whether the Kustomization with key d is umbrella u itself or
// is applied, at any depth, through u.
func (t *kustomizationTree) within(u, d string) bool {
	seen := map[string]bool{}
	var walk func(k string) bool
	walk = func(k string) bool {
		if k == d {
			return true
		}
		if seen[k] {
			return false
		}
		seen[k] = true
		for _, c := range t.children(k) {
			if walk(c) {
				return true
			}
		}
		return false
	}
	return walk(u)
}

// UmbrellaTerminals returns the terminal leaves of every umbrella
// Kustomization in one or more layout trees, keyed by the umbrella. Keys and
// values are "<namespace>/<name>", with an omitted namespace read as
// "default".
//
// It is for a caller that generated several trees separately (one
// LayoutIntegrator.CreateLayoutWithResources per tree) and wires dependencies
// between them by name (stack.Bundle.NamedDependsOn): the integrator rewrites
// a dependsOn on an umbrella to its terminal leaves only inside the tree it
// places (ResourceGenerator.UmbrellasDoNotWait), and keeps a name it does not
// find as given. The caller applies the same rewrite to such a name: a
// dependsOn on a key of the map becomes one on each of its values.
//
// An umbrella is a Flux Kustomization of a tree whose build applies at least
// one Flux Kustomization: its spec.path is a layout of the tree, and the
// directories a kustomize build of that layout includes host another Flux
// Kustomization. The terminal leaves are those the integrator writes (see
// UmbrellasDoNotWait). A Flux Kustomization an application emits, or one whose
// spec.path is no layout of its tree, is a leaf. Each root is indexed on its
// own, because a spec.path is relative to its own tree; the trees are those an
// integration returned, with or without the setting.
//
// It returns an error that wraps ErrUmbrellaTerminals, and no map, for a
// Kustomization identity found in two of the trees, for an umbrella without a
// terminal leaf and for a tree whose objects cannot be read. The error is a
// plain wrapped sentinel, not a typed resource error. Nil roots are skipped;
// no umbrella gives an empty map.
func UmbrellaTerminals(roots ...*layout.ManifestLayout) (map[string][]string, error) {
	out := map[string][]string{}
	placedIn := map[string]int{}
	for i, root := range roots {
		if root == nil {
			continue
		}
		t, err := indexKustomizationTree(root, nonApplicationMembers(root))
		if err != nil {
			return nil, fmt.Errorf("%w: tree %d (%q): %s", ErrUmbrellaTerminals, i, root.FullRepoPath(), err.Error())
		}
		for key := range t.hostOf {
			if prev, ok := placedIn[key]; ok && prev != i {
				return nil, fmt.Errorf("%w: Flux Kustomization %q is in tree %d and tree %d: a name must identify one Kustomization across the trees", ErrUmbrellaTerminals, key, prev, i)
			}
			placedIn[key] = i
		}
		for _, k := range t.kusts {
			key := crKey(k.Namespace, k.Name)
			if !t.isUmbrella(key) {
				continue
			}
			terms, err := t.terminals(key)
			if err != nil {
				return nil, fmt.Errorf("%w: tree %d (%q): %s", ErrUmbrellaTerminals, i, root.FullRepoPath(), err.Error())
			}
			out[key] = terms
		}
	}
	return out, nil
}

// nonApplicationMembers selects every Flux Kustomization of the tree under
// root but those an application emits (layout.ManifestLayout
// OriginBundleObjects), in typed form. An application object that is a List
// emits its items (objectItems), so they are matched by identity (crKey), not
// by the object provenance records.
func nonApplicationMembers(root *layout.ManifestLayout) memberFilter {
	emitted := map[string]bool{}
	var collect func(l *layout.ManifestLayout) error
	collect = func(l *layout.ManifestLayout) error {
		for _, b := range l.OriginBundles() {
			for _, o := range l.OriginBundleObjects(b) {
				if o == nil {
					continue
				}
				items, err := objectItems(o)
				if err != nil {
					return errors.Wrapf(err, "layout %q: read list items", l.FullRepoPath())
				}
				for _, item := range items {
					if _, ok := fluxKustomizationPath(item); ok {
						emitted[crKey(item.GetNamespace(), item.GetName())] = true
					}
				}
			}
		}
		for _, c := range l.Children {
			if c != nil {
				if err := collect(c); err != nil {
					return err
				}
			}
		}
		return nil
	}
	collectErr := collect(root)
	return func(l *layout.ManifestLayout) ([]*kustv1.Kustomization, error) {
		if collectErr != nil {
			return nil, collectErr
		}
		objs, err := resourceItems(l)
		if err != nil {
			return nil, err
		}
		var out []*kustv1.Kustomization
		for _, obj := range objs {
			if _, ok := fluxKustomizationPath(obj); !ok || emitted[crKey(obj.GetNamespace(), obj.GetName())] {
				continue
			}
			k, ok := obj.(*kustv1.Kustomization)
			if !ok {
				raw, err := json.Marshal(obj)
				if err == nil {
					k = &kustv1.Kustomization{}
					err = json.Unmarshal(raw, k)
				}
				if err != nil {
					return nil, errors.Wrapf(err, "layout %q: read Flux Kustomization %q in typed form", l.FullRepoPath(), crKey(obj.GetNamespace(), obj.GetName()))
				}
			}
			out = append(out, k)
		}
		return out, nil
	}
}
