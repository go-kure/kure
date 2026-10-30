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
	root   *layout.ManifestLayout
	kusts  []*kustv1.Kustomization
	byKey  map[string]*kustv1.Kustomization
	hostOf map[string]*layout.ManifestLayout
	// layoutAt maps the path of every layout that has a directory
	// (hasDirectory) to that layout.
	layoutAt map[string]*layout.ManifestLayout
	// others lists, per layout, the keys of the Flux Kustomizations it hosts
	// that are not members, and otherDeps their spec.dependsOn, every
	// occurrence of a key's entries together, read in typed form
	// (typedKustomization) as their health checks are, so that a type
	// embedding one, or another API version, counts the same; an entry's exist
	// marks a readyExpr of exactly "true", which deps reads with
	// existenceOnly. otherKeys lists them in depth-first layout order, and
	// otherObjs holds each object. otherErr is the error of the first that
	// cannot be read in typed form: UmbrellasDoNotWait and UmbrellaTerminals,
	// which read otherDeps, refuse the tree with it; the reconcile-order check
	// reads none and ignores it.
	others    map[*layout.ManifestLayout][]string
	otherDeps map[string][]depRef
	otherKeys []string
	otherObjs map[string]client.Object
	// otherSpecs holds each other Kustomization in typed form, absent for
	// one that cannot be read so (otherErr).
	otherSpecs map[string]*kustv1.Kustomization
	otherErr   error
	// copies holds, per key, every copy of a Flux Kustomization the tree
	// holds, member or other, that reads in typed form, and copyErr the keys
	// of which a copy does not. otherSpecs and byKey keep one copy per key;
	// the reconcile-order check reads every copy's spec.path
	// (builtView.applyLoads).
	copies  map[string][]*kustv1.Kustomization
	copyErr map[string]bool
	// existenceOnly is ResourceGenerator.AdditiveCELDependencyCheckOff, set by
	// UmbrellasDoNotWait and ResourceGenerator.UmbrellaTerminals before they
	// read deps (readExistence): deps then leaves out an entry that waits only
	// for its dependency to exist. patched records that a Flux Kustomization
	// the tree holds, at any occurrence, patches what it builds (patchesBuild).
	existenceOnly bool
	patched       bool
	// dropped lists, per layout, the keys of the Flux Kustomizations it holds
	// that a build drops as local config (builtItems), and droppedObjs each
	// object. They are no member, other or writer here; the reconcile-order
	// checks read what a build that keeps one makes (builtView).
	// droppedErrs holds the error of each that cannot be read in typed form,
	// in droppedErrKeys order: one a build that patches what it holds
	// (patchesBuild) may keep (mayBeKept, from mayKeep, built on first use)
	// refuses the tree for UmbrellasDoNotWait and UmbrellaTerminals, which read
	// no build, as otherErr does (keptDroppedErr).
	dropped        map[*layout.ManifestLayout][]string
	droppedObjs    map[string]client.Object
	droppedErrs    map[string]error
	droppedErrKeys []string
	mayKeep        map[string]bool
	reached        map[string]map[*layout.ManifestLayout]bool
	fromRoot       map[*layout.ManifestLayout]bool
}

// addCopy records a copy of the Flux Kustomization key the tree holds: spec
// when it reads in typed form, else that one of key's copies does not.
func (t *kustomizationTree) addCopy(key string, spec *kustv1.Kustomization, err error) {
	if err != nil {
		t.copyErr[key] = true
		return
	}
	t.copies[key] = append(t.copies[key], spec)
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
		root:        root,
		byKey:       map[string]*kustv1.Kustomization{},
		copies:      map[string][]*kustv1.Kustomization{},
		copyErr:     map[string]bool{},
		hostOf:      map[string]*layout.ManifestLayout{},
		layoutAt:    map[string]*layout.ManifestLayout{},
		others:      map[*layout.ManifestLayout][]string{},
		otherDeps:   map[string][]depRef{},
		otherObjs:   map[string]client.Object{},
		otherSpecs:  map[string]*kustv1.Kustomization{},
		dropped:     map[*layout.ManifestLayout][]string{},
		droppedObjs: map[string]client.Object{},
		droppedErrs: map[string]error{},
		reached:     map[string]map[*layout.ManifestLayout]bool{},
		fromRoot:    map[*layout.ManifestLayout]bool{},
	}
	var index func(l *layout.ManifestLayout) error
	index = func(l *layout.ManifestLayout) error {
		if hasDirectory(l, root) {
			t.layoutAt[path.Clean(l.FullRepoPath())] = l
		}
		own, err := members(l)
		if err != nil {
			return err
		}
		for _, k := range own {
			key := crKey(k.Namespace, k.Name)
			t.kusts = append(t.kusts, k)
			t.byKey[key] = k
			t.copies[key] = append(t.copies[key], k)
			t.hostOf[key] = l
			t.patched = t.patched || patchesBuild(k)
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
				// Another copy of a member is no other, but a patch it
				// carries may rewrite a readyExpr all the same; one that
				// cannot be read in typed form may carry one.
				spec, err := typedKustomization(obj)
				t.patched = t.patched || err != nil || patchesBuild(spec)
				t.addCopy(key, spec, err)
				continue
			}
			t.others[l] = append(t.others[l], key)
			t.otherKeys = append(t.otherKeys, key)
			t.otherObjs[key] = obj
			t.hostOf[key] = l
			spec, err := typedKustomization(obj)
			t.addCopy(key, spec, err)
			if err != nil {
				if t.otherErr == nil {
					t.otherErr = err
				}
				continue
			}
			t.otherSpecs[key] = spec
			t.otherDeps[key] = append(t.otherDeps[key], specDeps(spec, true)...)
			t.patched = t.patched || patchesBuild(spec)
		}
		items, err := resourceItems(l)
		if err != nil {
			return err
		}
		for _, obj := range items {
			if _, ok := fluxKustomizationPath(obj); !ok {
				continue
			}
			// builtItems read the annotation of every item without error.
			if local, _ := built.LocalConfig(obj); !local {
				continue
			}
			key := crKey(obj.GetNamespace(), obj.GetName())
			t.dropped[l] = append(t.dropped[l], key)
			t.droppedObjs[key] = obj
			if _, err := typedKustomization(obj); err != nil && t.droppedErrs[key] == nil {
				t.droppedErrKeys = append(t.droppedErrKeys, key)
				t.droppedErrs[key] = err
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
// or nil when its spec.path is no directory of the tree (directoryAt: an
// AppFileSingle layout other than the root is none, go-kure/kure#1197), or it
// is no member and its build is not the tree's (readsLocalBuild).
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
		if l := t.directoryAt(k.Spec.Path); l != nil {
			r = map[*layout.ManifestLayout]bool{}
			for _, b := range buildDirectories(l) {
				r[b] = true
			}
		}
	}
	t.reached[key] = r
	return r
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
// directory holds is kept as what it applies, with a patch also what it holds
// as local config (patchesBuild). A renamed or moved
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
// key writes, as the tree holds them, as children does for a member, and for
// another whose build is the tree's (readsLocalBuild). The reconcile-order
// checks read what a build makes instead (builtView.written).
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

// patchesBuild reports whether k, a Flux Kustomization that is no member,
// patches what its build holds: spec.patches, or a component, which may carry
// patches. kustomize transforms before it drops the objects local config
// marks (builtItems), so a patch setting that annotation to "false" keeps one,
// and its wait waits for it: what the build holds as local config counts as
// written too. A name prefix or suffix, or a target namespace, changes no
// annotation.
func patchesBuild(k *kustv1.Kustomization) bool {
	return len(k.Spec.Patches) > 0 || len(k.Spec.Components) > 0
}

// isUmbrella reports whether the Kustomization with key is an umbrella: a
// member whose build applies at least one Flux Kustomization.
func (t *kustomizationTree) isUmbrella(key string) bool {
	return t.byKey[key] != nil && len(t.children(key)) > 0
}

// deps returns, as keys, the entries of the spec.dependsOn of the
// Kustomization with key that wait for the dependency's Ready. With
// existenceOnly an entry whose readyExpr is exactly "true" waits only for the
// dependency to exist (specDeps) and is left out: it proves nothing Ready.
// A Kustomization that is no member is read from every occurrence of its key
// (otherDeps).
func (t *kustomizationTree) deps(key string) []string {
	refs := t.otherDeps[key]
	if k := t.byKey[key]; k != nil {
		refs = specDeps(k, true)
	}
	var out []string
	for _, d := range refs {
		if !(t.existenceOnly && d.exist) {
			out = append(out, d.key)
		}
	}
	return out
}

// readExistence sets existenceOnly to on, unless the tree holds a Flux
// Kustomization that patches what it builds (patched). The tree reads every
// spec as it holds it, and a patch may rewrite a readyExpr held as "true"
// into one that waits for Ready: read as an existence wait, that entry would
// be admitted where its apply waits for Ready. With such a patch every entry
// is read as waiting for Ready, as without the setting, which may refuse an
// entry the patch makes an existence wait, and admits none the default
// refuses.
func (t *kustomizationTree) readExistence(on bool) {
	t.existenceOnly = on && !t.patched
}

// mayBeKept reports whether a build that is the tree's and patches what it
// holds holds the local-config Kustomization with key, whatever else has the
// key.
func (t *kustomizationTree) mayBeKept(key string) bool {
	if t.mayKeep == nil {
		t.mayKeep = map[string]bool{}
		for _, o := range t.otherKeys {
			k := t.otherSpecs[o]
			if k == nil || !patchesBuild(k) || t.builds(o) == nil {
				continue
			}
			for _, b := range buildScope(t.layoutAt[path.Clean(k.Spec.Path)]) {
				for _, d := range t.dropped[b] {
					t.mayKeep[d] = true
				}
			}
		}
	}
	return t.mayKeep[key]
}

// keptDroppedErr returns the error of the first local-config Kustomization a
// build may keep (mayBeKept) that cannot be read in typed form, so that its
// dependsOn is never read as empty; nil when there is none.
func (t *kustomizationTree) keptDroppedErr() error {
	for _, key := range t.droppedErrKeys {
		if t.mayBeKept(key) {
			return t.droppedErrs[key]
		}
	}
	return nil
}

// terminals returns the terminal leaves of umbrella u: for each Kustomization
// C that u applies and that no other Kustomization u applies depends on,
// terminals(C) when C is an umbrella, else C itself. Dependencies are compared
// in the form R3 writes them: C, and each Kustomization of u's subtree a
// member's dependsOn names, stand for their terminal leaves when they are
// umbrellas, and C is depended on when another one's dependsOn awaits all of
// C's; an entry that waits only for its dependency to exist awaits nothing
// (deps). So a tree the setting already rewrote has the same sets. A dependsOn
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
// An umbrella is a Flux Kustomization the integration placed or kept whose
// build applies at least one Flux Kustomization: its spec.path is a layout of
// the tree, and the directories a kustomize build of that layout includes host
// another Flux Kustomization. The terminal leaves are those the integrator
// writes (see UmbrellasDoNotWait). Each root is indexed on its own, because a
// spec.path is relative to its own tree; the trees are those an integration
// returned, with or without the setting.
//
// Which Kustomizations the integration placed or kept is what it recorded in
// each layout (layout.ManifestLayout FluxKustomizations): one counts only while
// it still reads what was recorded, the same spec.path, spec.sourceRef and
// presence of spec.kubeConfig, in whatever form it is held (typed,
// unstructured or a List item). Every other Flux Kustomization is a leaf,
// whatever its spec.path names: one an application emits and the integration
// did not keep, one a caller added, one whose Source, cluster or path was
// changed after the integration (it may no longer build this tree's
// directory), and every one of a tree built by hand or read back from disk,
// which carries no record (go-kure/kure#1077). A caller placing Flux
// Kustomizations itself records them with SetFluxKustomizations to have them
// read as umbrellas; a record is compared in normal form (normalRecord).
//
// It returns an error that wraps ErrUmbrellaTerminals, and no map, for a
// Kustomization identity found in two of the trees, for an umbrella without a
// terminal leaf and for a tree whose objects cannot be read. The error is a
// plain wrapped sentinel, not a typed resource error. Nil roots are skipped;
// no umbrella gives an empty map.
//
// It reads every dependsOn as the integrator does by default: for trees
// integrated with ResourceGenerator.AdditiveCELDependencyCheckOff, call
// ResourceGenerator.UmbrellaTerminals on that generator instead.
func UmbrellaTerminals(roots ...*layout.ManifestLayout) (map[string][]string, error) {
	return umbrellaTerminals(false, roots)
}

// UmbrellaTerminals is the package function UmbrellaTerminals, with the
// dependsOn entries read as g's integration reads them: with
// AdditiveCELDependencyCheckOff, an entry whose readyExpr is exactly "true"
// waits only for its dependency to exist, so it makes no Kustomization it
// names depended on, and the terminal leaves are those g's integration
// writes. As there, a tree holding a Flux Kustomization that sets
// spec.patches or spec.components is read as without the setting: a patch
// may rewrite a readyExpr, and the tree is read as held.
func (g *ResourceGenerator) UmbrellaTerminals(roots ...*layout.ManifestLayout) (map[string][]string, error) {
	return umbrellaTerminals(g.AdditiveCELDependencyCheckOff, roots)
}

// umbrellaTerminals is UmbrellaTerminals, with existenceOnly as
// kustomizationTree.existenceOnly.
func umbrellaTerminals(existenceOnly bool, roots []*layout.ManifestLayout) (map[string][]string, error) {
	out := map[string][]string{}
	placedIn := map[string]int{}
	for i, root := range roots {
		if root == nil {
			continue
		}
		t, err := indexKustomizationTree(root, recordedMembers)
		if err == nil {
			t.readExistence(existenceOnly)
			err = t.otherErr
		}
		if err == nil {
			err = t.keptDroppedErr()
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

// recordedMembers selects, in typed form, the Flux Kustomizations of l that the
// integration recorded there (layout.ManifestLayout FluxKustomizations,
// recordFluxKustomizations) and that still read what was recorded: identity,
// cleaned spec.path, Source and presence of spec.kubeConfig, both sides read
// as kure reads identities (normalRecord). It reads l as kustomize builds it
// (builtItems), so a recorded one counts in whatever form it is held, and one
// a build drops is none. A recorded identity that cannot be read in typed form
// is an error naming the layout.
func recordedMembers(l *layout.ManifestLayout) ([]*kustv1.Kustomization, error) {
	recs := l.FluxKustomizations()
	if len(recs) == 0 {
		return nil, nil
	}
	objs, err := builtItems(l)
	if err != nil {
		return nil, err
	}
	var out []*kustv1.Kustomization
	for _, obj := range objs {
		if _, ok := fluxKustomizationPath(obj); !ok {
			continue
		}
		if !slices.ContainsFunc(recs, func(r layout.FluxKustomizationRecord) bool {
			return crKey(r.Namespace, r.Name) == crKey(obj.GetNamespace(), obj.GetName())
		}) {
			continue
		}
		k, err := typedKustomization(obj)
		if err != nil {
			return nil, errors.Wrapf(err, "layout %q", l.FullRepoPath())
		}
		cur := normalRecord(fluxKustomizationRecord(k))
		if slices.ContainsFunc(recs, func(r layout.FluxKustomizationRecord) bool { return normalRecord(r) == cur }) {
			out = append(out, k)
		}
	}
	return out, nil
}

// normalRecord is r read as kure reads identities (crKey): an omitted
// namespace is "default", an omitted Source namespace the Kustomization's own,
// and the path cleaned. Two records that differ only in those forms name the
// same Kustomization, path and Source.
func normalRecord(r layout.FluxKustomizationRecord) layout.FluxKustomizationRecord {
	if r.Namespace == "" {
		r.Namespace = "default"
	}
	if r.SourceNamespace == "" {
		r.SourceNamespace = r.Namespace
	}
	r.Path = path.Clean(r.Path)
	return r
}

// typedKustomization returns the Flux Kustomization obj in typed form, as it
// is applied: kustomize-controller drops its status and creation timestamp
// before applying it (kustomization_controller.go:912; fluxcd/pkg/ssa v0.83.0
// normalize/normalize.go:155-162), so neither can keep it from being read.
func typedKustomization(obj client.Object) (*kustv1.Kustomization, error) {
	if k, ok := obj.(*kustv1.Kustomization); ok {
		return k, nil
	}
	content, err := writtenContent(obj)
	if err == nil {
		delete(content, "status")
		if metadata, ok := content["metadata"].(map[string]any); ok {
			delete(metadata, "creationTimestamp")
		}
		var raw []byte
		if raw, err = json.Marshal(content); err == nil {
			k := &kustv1.Kustomization{}
			if err = json.Unmarshal(raw, k); err == nil {
				return k, nil
			}
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
// Kustomization, and none is returned. A check that writes a namespace and
// names k itself is not returned: Flux drops it, comparing it with k as it
// exists on the cluster, where an omitted namespace is "default" (crKey)
// (kustomize-controller v1.10.1
// internal/controller/kustomization_controller.go:1118-1126). One with no
// namespace is returned, as k's own key: Flux does not default its namespace
// before that guard, and its poller then rejects the check, so k never
// becomes Ready (checkReconcileOrder names that case).
func checkedKustomizations(k *kustv1.Kustomization) []string {
	if k.Spec.KubeConfig != nil {
		return nil
	}
	var targets []string
	for _, hc := range k.Spec.HealthChecks {
		if hc.Kind != "Kustomization" || !strings.HasPrefix(hc.APIVersion, kustv1.GroupVersion.Group+"/") {
			continue
		}
		if hc.Namespace != "" && crKey(hc.Namespace, hc.Name) == crKey(k.Namespace, k.Name) {
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
