package fluxcd

import (
	"encoding/json"
	"path"
	"slices"
	"strings"

	kustv1 "github.com/fluxcd/kustomize-controller/api/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-kure/kure/internal/built"
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
//
// A member's spec.path is a directory of the tree: the integration generated
// or kept it for that directory, on that directory's source. Another's is
// only when readsLocalBuild says so; else its build applies nothing of the
// tree (go-kure/kure#1062).
type kustomizationTree struct {
	root     *layout.ManifestLayout
	kusts    []*kustv1.Kustomization
	byKey    map[string]*kustv1.Kustomization
	hostOf   map[string]*layout.ManifestLayout
	layoutAt map[string]*layout.ManifestLayout
	// others lists, per layout, the keys of the Flux Kustomizations it hosts
	// that are not members, and otherDeps their spec.dependsOn as keys, read
	// in typed form (typedKustomization) as their health checks are, so that
	// a type embedding one, or another API version, counts the same.
	// otherKeys lists them in depth-first layout order, and otherObjs holds
	// each object. otherErr is the error of the first that cannot be read in
	// typed form: UmbrellasDoNotWait and UmbrellaTerminals, which read
	// otherDeps, refuse the tree with it; the reconcile-order check reads
	// none and ignores it.
	others    map[*layout.ManifestLayout][]string
	otherDeps map[string][]string
	otherKeys []string
	otherObjs map[string]client.Object
	// otherSpecs holds each other Kustomization in typed form, absent for
	// one that cannot be read so (otherErr).
	otherSpecs map[string]*kustv1.Kustomization
	otherErr   error
	reached    map[string]map[*layout.ManifestLayout]bool
	fromRoot   map[*layout.ManifestLayout]bool
	// local and scope cache localOthers and rootScope.
	local []string
	scope []*layout.ManifestLayout
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
		root:       root,
		byKey:      map[string]*kustv1.Kustomization{},
		hostOf:     map[string]*layout.ManifestLayout{},
		layoutAt:   map[string]*layout.ManifestLayout{},
		others:     map[*layout.ManifestLayout][]string{},
		otherDeps:  map[string][]string{},
		otherObjs:  map[string]client.Object{},
		otherSpecs: map[string]*kustv1.Kustomization{},
		reached:    map[string]map[*layout.ManifestLayout]bool{},
		fromRoot:   map[*layout.ManifestLayout]bool{},
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
		objs, err := builtItems(l)
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
			t.otherKeys = append(t.otherKeys, key)
			t.otherObjs[key] = obj
			t.hostOf[key] = l
			spec, err := typedKustomization(obj)
			if err != nil {
				if t.otherErr == nil {
					t.otherErr = err
				}
				continue
			}
			t.otherSpecs[key] = spec
			for _, d := range spec.Spec.DependsOn {
				ns := d.Namespace
				if ns == "" {
					ns = obj.GetNamespace()
				}
				t.otherDeps[key] = append(t.otherDeps[key], crKey(ns, d.Name))
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
// or nil when its spec.path is no layout of the tree, or it is no member and
// its build is not the tree's (readsLocalBuild).
func (t *kustomizationTree) builds(key string) map[*layout.ManifestLayout]bool {
	if r, ok := t.reached[key]; ok {
		return r
	}
	var r map[*layout.ManifestLayout]bool
	k := t.byKey[key]
	if k == nil && t.readsLocalBuild(key) {
		k = t.otherSpecs[key]
	}
	if k != nil {
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

// readsLocalBuild reports whether the Flux Kustomization with key, which is
// no member, builds from this tree, so that its spec.path is the directory of
// the tree it names. A path alone does not say so: the same path in another
// repository, or on another cluster, is another directory. It holds only when
// the Kustomization sets no spec.kubeConfig (with one, it applies to, and
// health-checks on, that cluster), a member writes it (writer), and its
// spec.sourceRef names the Source that member's own does (sourceRefKey, an
// omitted namespace each Kustomization's own; the identity, not the URL it
// fetches). One the bootstrap or nothing writes, or that cannot be read in
// typed form, builds from no directory of the tree: it applies nothing the
// tree holds, and adds no edge. One that transforms its build (spec.patches,
// spec.components, spec.namePrefix, spec.nameSuffix, spec.targetNamespace)
// still reads that directory: the transform is not modelled, and what the
// directory holds is kept as what it applies. A renamed or moved
// Kustomization keeps its dependsOn, which kustomize does not rewrite, so
// dropping the edges would admit a deadlock; keeping them may refuse a
// build that deletes the object (go-kure/kure#1094).
func (t *kustomizationTree) readsLocalBuild(key string) bool {
	k := t.otherSpecs[key]
	if k == nil || k.Spec.KubeConfig != nil {
		return false
	}
	w := t.byKey[t.writer(key)]
	return w != nil && sourceRefKey(k.Spec.SourceRef, k.Namespace) == sourceRefKey(w.Spec.SourceRef, w.Namespace)
}

// applied lists the members the apply of the Kustomization with key writes:
// those its build reaches. With wait it waits for all of them, whoever else
// (the bootstrap) also creates them.
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
	if t.byKey[key] == nil {
		return nil // a leaf: its subtree is its author's
	}
	return t.written(key)
}

// written lists every Flux Kustomization the apply of the Kustomization with
// key writes, as children does for a member, and for another whose build is
// the tree's (readsLocalBuild): what its wait waits for.
func (t *kustomizationTree) written(key string) []string {
	out := t.applied(key)
	if t.builds(key) == nil {
		return out
	}
	k := t.byKey[key]
	if k == nil {
		k = t.otherSpecs[key]
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
// terminals(C) when C is an umbrella, else C itself. Dependencies are compared
// in the form R3 writes them: C, and each Kustomization of u's subtree a
// member's dependsOn names, stand for their terminal leaves when they are
// umbrellas, and C is depended on when another one's dependsOn awaits all of
// C's. So a tree the setting already rewrote has the same sets. A dependsOn
// of a Kustomization that is no member is not rewritten, so its entries on
// umbrellas await nothing. A Flux Kustomization
// that is no member is a leaf: its own subtree is its author's. Entries are
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
	// leaves is what R3 writes a dependsOn on k as: the terminal leaves of an
	// umbrella of u's subtree, else k.
	memo := map[string][]string{}
	leaves := func(k string) ([]string, error) {
		if k == u || !t.isUmbrella(k) || !t.within(u, k) {
			return []string{k}, nil
		}
		if l, ok := memo[k]; ok {
			return l, nil
		}
		l, err := t.terminalsFrom(k, visiting)
		if err != nil {
			return nil, err
		}
		memo[k] = l
		return l, nil
	}
	awaited := map[string][]string{}
	for _, s := range children {
		for _, d := range t.deps(s) {
			if d == s {
				continue
			}
			if t.byKey[s] == nil && t.isUmbrella(d) {
				// Not rewritten: its author's dependsOn on an umbrella,
				// which may not wait, proves no leaf of it Ready.
				continue
			}
			l, err := leaves(d)
			if err != nil {
				return nil, err
			}
			awaited[s] = append(awaited[s], l...)
		}
	}
	var out []string
	for _, c := range children {
		expanded, err := leaves(c)
		if err != nil {
			return nil, err
		}
		if slices.ContainsFunc(children, func(s string) bool { return s != c && dependsOnAll(awaited[s], expanded) }) {
			continue
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

// dependsOnAll reports whether deps holds every key of keys.
func dependsOnAll(deps, keys []string) bool {
	return !slices.ContainsFunc(keys, func(k string) bool { return !slices.Contains(deps, k) })
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
// integration returned, with or without the setting. Unlike the integration's
// own checks, it does not judge a Kustomization's context: every Flux
// Kustomization an application does not emit counts as building the layout
// its spec.path names, whatever its spec.sourceRef or spec.kubeConfig
// (go-kure/kure#1077).
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
		if err == nil {
			err = t.otherErr
		}
		if err != nil {
			return nil, errors.Wrapf(ErrUmbrellaTerminals, "tree %d (%q): %s", i, root.FullRepoPath(), err.Error())
		}
		for key := range t.hostOf {
			if prev, ok := placedIn[key]; ok && prev != i {
				return nil, errors.Wrapf(ErrUmbrellaTerminals, "Flux Kustomization %q is in tree %d and tree %d: a name must identify one Kustomization across the trees", key, prev, i)
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
				return nil, errors.Wrapf(ErrUmbrellaTerminals, "tree %d (%q): %s", i, root.FullRepoPath(), err.Error())
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
// by the object provenance records. A Kustomization a kustomize build drops
// (builtItems) is neither emitted nor a member: an application's dropped
// namesake hides no member of that name.
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
					if _, ok := fluxKustomizationPath(item); !ok {
						continue
					}
					local, err := built.LocalConfig(item)
					if err != nil {
						return errors.Wrapf(err, "layout %q: read an object as written", l.FullRepoPath())
					}
					if !local {
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
		objs, err := builtItems(l)
		if err != nil {
			return nil, err
		}
		var out []*kustv1.Kustomization
		for _, obj := range objs {
			if _, ok := fluxKustomizationPath(obj); !ok || emitted[crKey(obj.GetNamespace(), obj.GetName())] {
				continue
			}
			k, err := typedKustomization(obj)
			if err != nil {
				return nil, errors.Wrapf(err, "layout %q", l.FullRepoPath())
			}
			out = append(out, k)
		}
		return out, nil
	}
}

// typedKustomization returns the Flux Kustomization obj in typed form.
func typedKustomization(obj client.Object) (*kustv1.Kustomization, error) {
	if k, ok := obj.(*kustv1.Kustomization); ok {
		return k, nil
	}
	raw, err := json.Marshal(obj)
	if err == nil {
		k := &kustv1.Kustomization{}
		if err = json.Unmarshal(raw, k); err == nil {
			return k, nil
		}
	}
	return nil, errors.Wrapf(err, "read Flux Kustomization %q in typed form", crKey(obj.GetNamespace(), obj.GetName()))
}

// awaitedKustomizations returns the Flux Kustomizations whose readiness k's
// readiness waits for, as kustomize-controller polls them: with wait, those
// its apply wrote (applied; nil when unknown), and otherwise those its
// spec.healthChecks name (checkedKustomizations). applied says what the
// apply wrote here: nothing, for one on another cluster (readsLocalBuild).
// A spec.healthCheckExprs entry matching Flux Kustomizations changes
// nothing: its expression is arbitrary CEL, which may wait for readiness, so
// it is read as readiness (go-kure/kure#1094).
func awaitedKustomizations(k *kustv1.Kustomization, applied func() []string) []string {
	if !k.Spec.Wait {
		return checkedKustomizations(k)
	}
	if applied == nil {
		return nil
	}
	return applied()
}

// checkedKustomizations returns the Flux Kustomizations k's spec.healthChecks
// name, an entry without namespace in k's, whether or not k sets wait (under
// which Flux ignores them), each waited Ready for (a healthCheckExprs entry
// for them read as readiness, awaitedKustomizations). With spec.kubeConfig
// set Flux polls another cluster: a name there is not this tree's
// Kustomization, and none is returned.
func checkedKustomizations(k *kustv1.Kustomization) []string {
	if k.Spec.KubeConfig != nil {
		return nil
	}
	var targets []string
	for _, hc := range k.Spec.HealthChecks {
		if hc.Kind != "Kustomization" || !strings.HasPrefix(hc.APIVersion, kustv1.GroupVersion.Group+"/") {
			continue
		}
		ns := hc.Namespace
		if ns == "" {
			ns = k.Namespace
		}
		targets = append(targets, crKey(ns, hc.Name))
	}
	return targets
}
