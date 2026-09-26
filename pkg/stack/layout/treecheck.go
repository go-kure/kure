package layout

import (
	"fmt"
	"path"
	"path/filepath"
	"slices"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/kustomize/kyaml/resid"

	"github.com/go-kure/kure/pkg/errors"
)

// checkLayoutTree refuses, before anything is written, a tree in which:
//   - two layouts resolve to the same directory, or, for AppFileSingle
//     layouts, the same file (go-kure/kure#771): each writes its own files
//     there, and the later kustomization.yaml silently replaces the earlier
//     one, dropping its resources from the kustomize graph;
//   - an AppFileSingle child has children (see checkSingleChildLeaf) or
//     ConfigMapGenerators (see checkSingleChildGenerators);
//   - an AppFileSingle child's file, which its parent's kustomization.yaml
//     lists, lands outside the parent's directory or in a spelling of it that
//     differs only in case, or the child's Name is rooted or holds a path
//     separator (see checkSingleChildEntry);
//   - a layout holds one object twice, counting the ConfigMaps its
//     kustomization.yaml generates (see checkResourceIdentities);
//   - an extra file takes a path the writer owns (see checkExtraFiles);
//   - an AppFileSingle layout's file or extra file replaces another layout's
//     file, needs a directory where another layout writes a file, or is a
//     file where another layout needs a directory, or a layout's directory is
//     or lies beneath another layout's file (see checkSingleFiles);
//   - a KustomizationRecursive layout contradicts the output (see
//     checkRecursiveLayouts);
//   - one kustomize build takes in two layouts that hold one object (see
//     checkBuildIdentities).
//
// Directories are compared case-insensitively, as on default macOS volumes.
// plan is the writer's own, so the check and the write agree on every path
// and file name.
func checkLayoutTree(root *ManifestLayout, plan writerPlan) error {
	outDir := plan.outDir
	dirs := map[string]*ManifestLayout{}
	files := map[string]*ManifestLayout{}
	var walk func(l *ManifestLayout) error
	walk = func(l *ManifestLayout) error {
		dir, single := outDir(l)
		if single {
			// The whole file path is cleaned, as the writers clean it. The
			// writers already clean the Namespace, and a Name holding a
			// separator is refused, so this is a backstop.
			key := normDir(filepath.Join(dir, l.Name+".yaml"))
			if other, ok := files[key]; ok {
				return errors.NewFileError("write", filepath.Join(dir, l.Name+".yaml"),
					fmt.Sprintf("layouts %q and %q resolve to the same file", other.FullRepoPath(), l.FullRepoPath()), nil)
			}
			files[key] = l
		} else {
			key := normDir(dir)
			if other, ok := dirs[key]; ok {
				return errors.NewFileError("write", dir,
					fmt.Sprintf("layouts %q and %q resolve to the same directory", other.FullRepoPath(), l.FullRepoPath()), nil)
			}
			dirs[key] = l
		}
		if err := checkResourceIdentities(l, plan.writesKustomization(l, l == root)); err != nil {
			return err
		}
		// The writers run it again as they write l; here it refuses before
		// any layout of the tree is written.
		sorted, _ := plan.files(l)
		if err := checkExtraFiles(l, outDir, sorted); err != nil {
			return err
		}
		for _, child := range l.Children {
			if child == nil {
				continue
			}
			if err := checkSingleChildLeaf(child, outDir); err != nil {
				return err
			}
			if err := checkSingleChildGenerators(child, outDir); err != nil {
				return err
			}
			if err := checkSingleChildEntry(l, child, plan, l == root); err != nil {
				return err
			}
			if err := walk(child); err != nil {
				return err
			}
		}
		return nil
	}
	if err := walk(root); err != nil {
		return err
	}
	if err := checkSingleFiles(root, plan); err != nil {
		return err
	}
	if err := checkRecursiveLayouts(root, plan); err != nil {
		return err
	}
	return checkBuildIdentities(root, plan)
}

// checkBuildIdentities refuses two layouts holding objects with one identity
// (kustomize's own, see buildIdentity) that one kustomize build the writer
// produces takes in together: kustomize refuses to add the second object ("may
// not add resource with an already registered id"), so the kustomization.yaml
// kure writes, or the Flux build of its Recursive directory, would not build
// (go-kure/kure#880). A layout's objects include the ConfigMap each of its
// ConfigMapGenerators generates when the writer writes it a kustomization.yaml
// (see eachGeneratedIdentity): kustomize refuses a generator whose ConfigMap
// the build already holds, and a ConfigMap the build adds after a generator's
// (go-kure/kure#894). The builds, each checked on its own, are:
//   - every layout the writer writes a kustomization.yaml for: its own
//     resource files, the file of each AppFileSingle child it lists (by its
//     path below the directory, see childFileEntry), and the build of each
//     directory it lists, each entry resolved against its directory as
//     kustomize resolves it;
//   - every KustomizationRecursive layout marked with SetFluxBuild: the
//     objects of every layout whose files land in its directory or below it,
//     where a directory with a kustomization.yaml the writer writes is taken in
//     as that directory's build and not descended into, as Flux generates it.
//
// A child its parent does not list (see childEntry) is outside the parent's
// build, so it may hold an object the parent holds. It runs after
// checkRecursiveLayouts, which refuses a listed Recursive directory and what a
// marked Recursive build would take in beyond the Explicit mode's. Builds are
// checked children first, so the innermost build holding both is named.
func checkBuildIdentities(root *ManifestLayout, plan writerPlan) error {
	type laid struct {
		l       *ManifestLayout
		dir     string // normDir'ed directory its files land in
		single  bool
		writesK bool
	}
	var all []laid                            // in pre-order
	info := map[*ManifestLayout]laid{}        // per layout
	dirOwner := map[string]*ManifestLayout{}  // normDir'ed directory -> directory layout
	fileOwner := map[string]*ManifestLayout{} // normDir'ed file -> AppFileSingle layout writing it
	var index func(l *ManifestLayout)
	index = func(l *ManifestLayout) {
		dir, single := plan.outDir(l)
		li := laid{l, normDir(dir), single, plan.writesKustomization(l, l == root)}
		all = append(all, li)
		info[l] = li
		switch {
		case !single:
			dirOwner[li.dir] = l
		case l.writesSingleFile():
			fileOwner[normDir(path.Join(filepath.ToSlash(dir), l.Name+".yaml"))] = l
		}
		for _, c := range l.Children {
			if c != nil {
				index(c)
			}
		}
	}
	index(root)

	// explicit adds to in the layouts whose objects the kustomization.yaml
	// of l takes in.
	var explicit func(l *ManifestLayout, in map[*ManifestLayout]bool)
	explicit = func(l *ManifestLayout, in map[*ManifestLayout]bool) {
		if in[l] {
			return
		}
		in[l] = true
		for _, c := range l.Children {
			e := plan.childEntry(l, c)
			if e == "" {
				continue
			}
			key := normDir(path.Join(info[l].dir, e))
			if info[c].single {
				if s := fileOwner[key]; s != nil {
					in[s] = true
				}
				continue
			}
			// A listed directory without a kustomization.yaml fails the
			// build before any object is added; checkRecursiveLayouts
			// refuses a Recursive one.
			if d := dirOwner[key]; d != nil && info[d].writesK {
				explicit(d, in)
			}
		}
	}
	var kdirs []string
	for _, m := range all {
		if !m.single && m.writesK {
			kdirs = append(kdirs, m.dir)
		}
	}
	// recursive adds to in the layouts whose objects the Flux build of the
	// Recursive layout d takes in. A directory layout's own kustomization.yaml
	// does not shield it (Flux adds its directory); an AppFileSingle file is
	// shielded by one in its own directory or above it, below d, which lists
	// it or not. A listed file lies at or below its lister's directory, so a
	// lister below d shields it and its build takes it in; one that no
	// kustomization.yaml shields, listed from d's directory or above,
	// checkRecursiveLayouts refuses.
	recursive := func(d laid, in map[*ManifestLayout]bool) {
		for _, t := range all {
			if (t.dir != d.dir && !below(d.dir, t.dir)) || shieldedIn(kdirs, d.dir, t.dir, !t.single) {
				continue
			}
			if !t.single && t.l != d.l && t.writesK {
				explicit(t.l, in)
			} else {
				in[t.l] = true
			}
		}
	}

	type held struct {
		id        string
		generated bool // by one of the layout's ConfigMapGenerators
	}
	ids := map[*ManifestLayout][]held{}
	for _, li := range all {
		if err := eachIdentity(li.l, buildIdentity, func(id string) error {
			ids[li.l] = append(ids[li.l], held{id, false})
			return nil
		}); err != nil {
			return err
		}
		// A layout's generators are objects of its own kustomization.yaml's
		// build, and so of every build that takes that one in.
		if li.writesK {
			if err := eachGeneratedIdentity(li.l, buildIdentity, func(id string) error {
				ids[li.l] = append(ids[li.l], held{id, true})
				return nil
			}); err != nil {
				return err
			}
		}
	}
	for i := len(all) - 1; i >= 0; i-- {
		b := all[i]
		in := map[*ManifestLayout]bool{}
		var what string
		switch {
		case b.writesK:
			explicit(b.l, in)
			what = fmt.Sprintf("the kustomization.yaml of layout %q builds both", b.l.FullRepoPath())
		case !b.single && b.l.fluxBuild && plan.kustomizationMode(b.l) == KustomizationRecursive:
			recursive(b, in)
			what = fmt.Sprintf("the Flux build of KustomizationRecursive layout %q includes both", b.l.FullRepoPath())
		default:
			continue
		}
		type holder struct {
			l         *ManifestLayout
			generated bool
		}
		seen := map[string]holder{}
		for _, li := range all {
			if !in[li.l] {
				continue
			}
			for _, h := range ids[li.l] {
				// One layout counts too: checkResourceIdentities keeps the
				// namespace field of a cluster-scoped kind, which kustomize
				// ignores.
				if other, dup := seen[h.id]; dup {
					both := fmt.Sprintf("layouts %q and %q both hold the object %s", other.l.FullRepoPath(), li.l.FullRepoPath(), h.id)
					if other.l == li.l {
						both = fmt.Sprintf("layout %q holds the object %s twice", li.l.FullRepoPath(), h.id)
					}
					return errors.NewFileError("write", b.l.FullRepoPath(), fmt.Sprintf(
						"%s%s, and %s: kustomize refuses one object twice", both, generatedNote(other.generated, h.generated), what), nil)
				}
				seen[h.id] = holder{li.l, h.generated}
			}
		}
	}
	return nil
}

// checkSingleChildLeaf refuses an AppFileSingle child (as outDir treats it)
// that has children of its own. Such a child writes one file into its
// Namespace and no kustomization.yaml, so nothing would list the
// layouts below it and they would silently drop out of the build
// (go-kure/kure#860). The root is not checked: it writes a kustomization.yaml
// of its own, into its Namespace, which lists its children there (so they
// take the root's Namespace, not its FullRepoPath()); and the synthetic
// cluster wrapper a walker builds (no origin, one child) takes Config's
// AppFileSingle in WriteManifest.
func checkSingleChildLeaf(child *ManifestLayout, outDir outDirFunc) error {
	dir, single := outDir(child)
	if !single || !slices.ContainsFunc(child.Children, func(c *ManifestLayout) bool { return c != nil }) {
		return nil
	}
	return errors.NewFileError("write", filepath.Join(dir, child.Name+".yaml"),
		fmt.Sprintf("layout %q is AppFileSingle and has child layouts: it writes one file into its Namespace, normally its parent's directory, and no kustomization.yaml, so nothing would list them", child.FullRepoPath()), nil)
}

// checkSingleChildGenerators refuses an AppFileSingle child (as outDir treats
// it) that carries ConfigMapGenerators, with or without resources. A
// configMapGenerator exists only inside a kustomization.yaml, and such a child
// writes none (go-kure/kure#860), so its generators would be silently dropped
// (go-kure/kure#891). They are not moved into the parent's kustomization.yaml,
// which the caller did not put them in.
func checkSingleChildGenerators(child *ManifestLayout, outDir outDirFunc) error {
	dir, single := outDir(child)
	if !single || len(child.ConfigMapGenerators) == 0 {
		return nil
	}
	return errors.NewFileError("write", dir, fmt.Sprintf(
		"layout %q is AppFileSingle, which writes no kustomization.yaml, so its ConfigMapGenerators have nowhere to go", child.FullRepoPath()), nil)
}

// checkResourceIdentities refuses a layout that holds two resources with one
// identity as layoutIdentity defines it (group, kind, namespace and name).
// Resources that share a file name are legitimately written into one
// multi-document file (every FilePerKind file does this), but two objects with
// one kustomize identity in one directory make kustomize fail to build it. The
// version is not compared here, so two versions of one object, which
// kustomize builds, are refused as well. A grouping axis set to flat merges
// several applications' or nodes' resources into one directory, which is
// where this happens.
//
// When the writer writes l a kustomization.yaml (writesK), the ConfigMap each
// of its ConfigMapGenerators generates counts as well (go-kure/kure#894):
// kustomize refuses a generator whose ConfigMap is already in the build
// ("behavior must be merge or replace"), and kure writes no behavior.
func checkResourceIdentities(l *ManifestLayout, writesK bool) error {
	seen := make(map[string]bool, len(l.Resources)) // identity -> generated
	claim := func(generated bool) func(id string) error {
		return func(id string) error {
			if other, dup := seen[id]; dup {
				return errors.NewFileError("write", l.FullRepoPath(),
					fmt.Sprintf("layout %q holds the same object %s twice%s", l.FullRepoPath(), id, generatedNote(other, generated)), nil)
			}
			seen[id] = generated
			return nil
		}
	}
	if err := eachIdentity(l, layoutIdentity, claim(false)); err != nil {
		return err
	}
	if !writesK {
		return nil
	}
	return eachGeneratedIdentity(l, layoutIdentity, claim(true))
}

// generatedNote says which of two objects with one identity a
// configMapGenerator generates, if any.
func generatedNote(a, b bool) string {
	switch {
	case a && b:
		return ", both generated by configMapGenerators"
	case a || b:
		return ", one generated by a configMapGenerator"
	}
	return ""
}

// eachGeneratedIdentity calls fn, in order, with the identity key renders for
// the ConfigMap each of l's ConfigMapGenerators generates: v1 ConfigMap, the
// generator's name and no namespace. kustomize compares the name before the
// content-hash suffix, and reads the missing namespace as "default"; kure
// writes neither a namespace nor a behavior for a generator.
func eachGeneratedIdentity(l *ManifestLayout, key identityKey, fn func(id string) error) error {
	gvk := schema.GroupVersionKind{Version: "v1", Kind: "ConfigMap"}
	for _, gen := range l.ConfigMapGenerators {
		if err := fn(key(gvk, "", gen.Name)); err != nil {
			return err
		}
	}
	return nil
}

// identityKey renders an object's identity from its group, version and kind,
// its namespace as set (possibly empty) and its name.
type identityKey func(gvk schema.GroupVersionKind, namespace, name string) string

// layoutIdentity is checkResourceIdentities' key,
// "<group>/<kind> <namespace>/<name>". kustomize reads an omitted namespace as
// "default", so an object without one and the same object in "default" are
// one identity.
func layoutIdentity(gvk schema.GroupVersionKind, namespace, name string) string {
	if namespace == "" {
		namespace = "default"
	}
	return fmt.Sprintf("%s/%s %s/%s", gvk.Group, gvk.Kind, namespace, name)
}

// buildIdentity is kustomize's own identity (resid.ResId.Equals), the one it
// refuses twice in a build: group, version, kind, name and the effective
// namespace. A kind kustomize knows is cluster-scoped has no namespace, even
// when the object sets one; any other kind with no namespace is in "default".
// It renders as "<apiVersion> <kind> [<namespace>/]<name>".
func buildIdentity(gvk schema.GroupVersionKind, namespace, name string) string {
	if resid.NewGvk(gvk.Group, gvk.Version, gvk.Kind).IsClusterScoped() {
		return fmt.Sprintf("%s %s %s", gvk.GroupVersion(), gvk.Kind, name)
	}
	if namespace == "" {
		namespace = "default"
	}
	return fmt.Sprintf("%s %s %s/%s", gvk.GroupVersion(), gvk.Kind, namespace, name)
}

// eachIdentity calls fn, in order, with the identity key renders for every
// object l holds, a List's items standing in for the List. An object without
// a kind has none and is skipped.
func eachIdentity(l *ManifestLayout, key identityKey, fn func(id string) error) error {
	claim := func(obj runtime.Object) error {
		acc, err := meta.Accessor(obj)
		if err != nil {
			return errors.Wrapf(err, "layout %q: read object metadata", l.FullRepoPath())
		}
		gvk := obj.GetObjectKind().GroupVersionKind()
		if gvk.Kind == "" {
			// A typed object with an unset TypeMeta has no kind to
			// identify it by; two of different Go types would share an
			// empty key. It is not this check's to judge.
			return nil
		}
		return fn(key(gvk, acc.GetNamespace(), acc.GetName()))
	}
	for _, obj := range l.Resources {
		if obj == nil {
			continue
		}
		// A List is an envelope: kustomize builds its items, so the
		// items are what must be unique, not the (usually unnamed) List.
		if u, ok := obj.(*unstructured.Unstructured); ok && u.IsList() {
			list, err := u.ToList()
			if err != nil {
				return errors.Wrapf(err, "layout %q: read list items", l.FullRepoPath())
			}
			for i := range list.Items {
				if err := claim(&list.Items[i]); err != nil {
					return err
				}
			}
			continue
		}
		if meta.IsListType(obj) {
			items, err := meta.ExtractList(obj)
			if err != nil {
				return errors.Wrapf(err, "layout %q: read list items", l.FullRepoPath())
			}
			for _, item := range items {
				if err := claim(item); err != nil {
					return err
				}
			}
			continue
		}
		if err := claim(obj); err != nil {
			return err
		}
	}
	return nil
}
