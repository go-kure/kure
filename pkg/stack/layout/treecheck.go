package layout

import (
	"encoding/json"
	"fmt"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/kustomize/kyaml/resid"

	"github.com/go-kure/kure/pkg/errors"
	"github.com/go-kure/kure/pkg/stack"
)

// checkLayoutTree refuses, before anything is written, a tree in which:
//   - two layouts resolve to the same directory, or, for AppFileSingle
//     layouts, the same file (go-kure/kure#771): each writes its own files
//     there, and the later kustomization.yaml silently replaces the earlier
//     one, dropping its resources from the kustomize graph. Two bundles that
//     take one directory name under one parent are refused here, named by
//     their paths (see sameDirectory);
//   - an AppFileSingle child has children (see checkSingleChildLeaf) or
//     ConfigMapGenerators (see checkSingleChildGenerators);
//   - any other layout the writer writes no kustomization.yaml for has
//     ConfigMapGenerators (see checkUnwrittenGenerators);
//   - an AppFileSingle child's file, which its parent's kustomization.yaml
//     lists, lands outside the parent's directory or in a spelling of it that
//     differs only in case, or the child's Name is rooted or holds a path
//     separator (see checkSingleChildEntry);
//   - a directory child, which its parent's kustomization.yaml lists by its
//     Name, has its own path as Namespace and so is written below the
//     directory listed (see checkDirectoryChildEntry);
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
//     checkBuildIdentities);
//   - a layout's Namespace or Name has a ".." path segment (see
//     checkLayoutIdentity);
//   - two Flux Kustomizations share a namespace and name (see
//     checkKustomizationNames);
//   - in a tree Flux delivers, a layout its parent does not list is not marked
//     as built by a Flux Kustomization (see checkUnappliedLayouts).
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
		// An AppFileSingle child that climbs out of its parent's directory
		// was refused, in the words of that case, before the walk reached it
		// (see checkSingleChildEntry).
		if err := checkLayoutIdentity(l); err != nil {
			return err
		}
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
				return errors.NewFileError("write", dir, sameDirectory(other, l), nil)
			}
			dirs[key] = l
		}
		if err := checkUnwrittenGenerators(l, plan, l == root); err != nil {
			return err
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
	// After the checks above, so a tree they refuse (a ".." segment, two
	// layouts in one directory, a file where a directory is needed) is refused
	// in their words; before the checks below, which read what each build
	// takes in from the same entries.
	if err := checkDirectoryChildEntries(root, plan); err != nil {
		return err
	}
	if err := checkRecursiveLayouts(root, plan); err != nil {
		return err
	}
	if err := checkBuildIdentities(root, plan); err != nil {
		return err
	}
	// Last, so a tree the checks above refuse is refused in their words.
	if err := checkKustomizationNames(root); err != nil {
		return err
	}
	return checkUnappliedLayouts(root, plan)
}

// sameDirectory words the refusal of two layouts that resolve to one
// directory. Two layouts with one parent and one name have one FullRepoPath,
// so a layout that is a bundle's own directory (see ownBundle) is named by
// its bundle's path instead: two umbrella children, or two bundles under
// BundleGrouping GroupByName, that take one directory name are told apart.
func sameDirectory(a, b *ManifestLayout) string {
	ownA, ownB := a.ownBundle(), b.ownBundle()
	if ownA == nil && ownB == nil {
		return fmt.Sprintf("layouts %q and %q resolve to the same directory", a.FullRepoPath(), b.FullRepoPath())
	}
	const rule = "a bundle's directory is named by its DirName, or by its Name without one, and must be the only one of that name in the directory above it"
	if ownA != nil && ownB != nil {
		return fmt.Sprintf("bundles %q and %q resolve to the same directory %q: %s", ownA.GetPath(), ownB.GetPath(), b.FullRepoPath(), rule)
	}
	claimant := func(l *ManifestLayout, own *stack.Bundle) string {
		if own != nil {
			return fmt.Sprintf("bundle %q", own.GetPath())
		}
		return fmt.Sprintf("layout %q", l.FullRepoPath())
	}
	return fmt.Sprintf("%s and %s resolve to the same directory: %s", claimant(a, ownA), claimant(b, ownB), rule)
}

// fluxKustomizationGroup and fluxKustomizationKind identify a Flux
// Kustomization at any version. The fluxcd package imports this one, so its
// types cannot be used here.
const (
	fluxKustomizationGroup = "kustomize.toolkit.fluxcd.io"
	fluxKustomizationKind  = "Kustomization"
)

// checkUnappliedLayouts refuses, in a tree Flux delivers, a layout that is
// written and that nothing applies (go-kure/kure#977). The tree is one Flux
// delivers when its root is marked with SetFluxBuild: the fluxcd package's
// LayoutIntegrator marks the root of every tree it generated a Kustomization
// for, and a caller that places Flux Kustomizations itself marks its tree the
// same way. In any other tree, an Argo CD one included, nothing is checked:
// the placement value does not tell the two apart.
//
// In such a tree a child its parent's kustomization.yaml does not list (see
// childEntry: an umbrella child, a child that renders a bundle, a directory
// child of a FluxIntegratedPerLayout parent, and for WriteToDisk and
// WriteToTar a directory child of another package) is applied only by a Flux
// Kustomization whose spec.path names its directory, so it must be marked as
// well. The rule is childEntry's own, so the check and the listing cannot
// disagree; a child of another package is not exempt, because the writers
// write its directory into this tree and nothing here lists it. An
// AppFileSingle child without resources writes no file, so there is nothing
// to apply and it is not checked.
func checkUnappliedLayouts(root *ManifestLayout, plan writerPlan) error {
	if !root.fluxBuild {
		return nil
	}
	var walk func(l *ManifestLayout) error
	walk = func(l *ManifestLayout) error {
		for _, child := range l.Children {
			if child == nil {
				continue
			}
			if _, single := plan.outDir(child); !child.fluxBuild && plan.childEntry(l, child) == "" && (!single || child.writesSingleFile()) {
				return errors.NewFileError("write", layoutPath(child, plan), fmt.Sprintf(
					"layout %q is not listed by its parent layout %q and no Flux Kustomization is recorded as building it: in a tree Flux delivers nothing would apply it; place a Flux Kustomization whose spec.path names it and mark the layout with SetFluxBuild",
					child.FullRepoPath(), l.FullRepoPath()), nil)
			}
			if err := walk(child); err != nil {
				return err
			}
		}
		return nil
	}
	return walk(root)
}

// checkKustomizationNames refuses two Flux Kustomizations with one namespace
// and name anywhere in the tree, marked or not (go-kure/kure#977). The
// per-layout and per-build checks refuse one object held twice in what one
// kustomize build takes in; two Kustomizations in directories that are applied
// separately pass those, yet they are one object in the cluster, where each
// apply would replace what the other wrote. A Kustomization is matched by its
// group and kind, at any version, and an omitted namespace is "default". One
// inside a List counts, a List opened as kustomize opens it (see builtObjects).
func checkKustomizationNames(root *ManifestLayout) error {
	holders := map[string]*ManifestLayout{}
	var walk func(l *ManifestLayout) error
	walk = func(l *ManifestLayout) error {
		objs, err := builtObjects(l)
		if err != nil {
			return err
		}
		for _, obj := range objs {
			gvk := obj.GetObjectKind().GroupVersionKind()
			if gvk.Group != fluxKustomizationGroup || gvk.Kind != fluxKustomizationKind {
				continue
			}
			acc, err := meta.Accessor(obj)
			if err != nil {
				return errors.Wrapf(err, "layout %q: read object metadata", l.FullRepoPath())
			}
			namespace := acc.GetNamespace()
			if namespace == "" {
				namespace = "default"
			}
			key := namespace + "/" + acc.GetName()
			other, dup := holders[key]
			switch {
			case dup && other == l:
				return errors.NewFileError("write", l.FullRepoPath(), fmt.Sprintf(
					"layout %q holds the Flux Kustomization %s twice: Kustomization names must be unique", l.FullRepoPath(), key), nil)
			case dup:
				return errors.NewFileError("write", l.FullRepoPath(), fmt.Sprintf(
					"layouts %q and %q both hold the Flux Kustomization %s: the two are one object in the cluster, wherever each is applied, so Kustomization names must be unique in a tree",
					other.FullRepoPath(), l.FullRepoPath(), key), nil)
			}
			holders[key] = l
		}
		for _, child := range l.Children {
			if child == nil {
				continue
			}
			if err := walk(child); err != nil {
				return err
			}
		}
		return nil
	}
	return walk(root)
}

// builtObjects returns the objects kustomize builds from l's resources: each
// resource, a List replaced by its items. A List is what kustomize opens as
// one (resource.Factory in sigs.k8s.io/kustomize/api): an object whose kind
// ends in "List" and that has an items field, opened again when an item is
// itself such a List. A kind that does not end in "List" is one object,
// whatever fields it has, and so is a List kind without items. A null items
// field is an empty List. A List whose items field is not an array fails the
// kustomize build; it is returned as the one object it is, since there is
// nothing in it to read.
func builtObjects(l *ManifestLayout) ([]runtime.Object, error) {
	var out []runtime.Object
	queue := make([]runtime.Object, 0, len(l.Resources))
	for _, r := range l.Resources {
		if r != nil {
			queue = append(queue, r)
		}
	}
	for len(queue) > 0 {
		obj := queue[0]
		queue = queue[1:]
		items, isList, err := listItems(obj)
		if err != nil {
			return nil, errors.Wrapf(err, "layout %q: read list items", l.FullRepoPath())
		}
		if !isList {
			out = append(out, obj)
			continue
		}
		queue = append(queue, items...)
	}
	return out, nil
}

// listItems returns the items of obj when it is a List as builtObjects
// defines one, and whether it is.
func listItems(obj runtime.Object) (items []runtime.Object, isList bool, err error) {
	if !strings.HasSuffix(obj.GetObjectKind().GroupVersionKind().Kind, "List") {
		return nil, false, nil
	}
	if u, ok := obj.(*unstructured.Unstructured); ok {
		raw, has := u.Object["items"]
		switch {
		case !has:
			return nil, false, nil
		case raw == nil:
			return nil, true, nil
		case !u.IsList():
			return nil, false, nil
		}
		list, err := u.ToList()
		if err != nil {
			return nil, false, err
		}
		for i := range list.Items {
			items = append(items, &list.Items[i])
		}
		return items, true, nil
	}
	if !meta.IsListType(obj) {
		return nil, false, nil
	}
	extracted, err := typedListItems(obj)
	if err != nil {
		return nil, false, err
	}
	// A typed List can hold an item as raw JSON (runtime.RawExtension), which
	// the writers serialize as the object it encodes: it is read as that
	// object. An empty item holds nothing.
	for _, item := range extracted {
		raw, isRaw := item.(*runtime.Unknown)
		switch {
		case item == nil:
		case !isRaw:
			items = append(items, item)
		default:
			u := &unstructured.Unstructured{}
			if err := json.Unmarshal(raw.Raw, &u.Object); err != nil {
				return nil, false, err
			}
			if u.Object != nil {
				items = append(items, u)
			}
		}
	}
	return items, true, nil
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
	return errors.NewFileError("write", filepath.Join(dir, child.Name+".yaml"), fmt.Sprintf(
		"layout %q is AppFileSingle, which writes no kustomization.yaml, so its ConfigMapGenerators have nowhere to go", child.FullRepoPath()), nil)
}

// checkUnwrittenGenerators refuses a layout with ConfigMapGenerators that the
// writer writes no kustomization.yaml for (go-kure/kure#899): a generator
// exists only inside a kustomization.yaml, so it would be dropped. That is an
// AppFileSingle root with no resources and no children, and, in
// WriteManifest, a layout shaped like the synthetic cluster root (Name "", a
// single-segment Namespace) with no resource file, no bundle and no
// AppFileSingle child that writes a file. An AppFileSingle child is refused in
// its own words by checkSingleChildGenerators before it is walked, and a
// KustomizationRecursive layout by checkRecursiveLayouts.
func checkUnwrittenGenerators(l *ManifestLayout, plan writerPlan, root bool) error {
	if len(l.ConfigMapGenerators) == 0 || plan.writesKustomization(l, root) ||
		plan.kustomizationMode(l) == KustomizationRecursive {
		return nil
	}
	return errors.NewFileError("write", layoutPath(l, plan), fmt.Sprintf(
		"layout %q gets no kustomization.yaml, so its ConfigMapGenerators have nowhere to go", l.FullRepoPath()), nil)
}

// layoutPath is the path an error about l names: its file for an
// AppFileSingle layout, its directory otherwise.
func layoutPath(l *ManifestLayout, plan writerPlan) string {
	dir, single := plan.outDir(l)
	if single {
		return filepath.Join(dir, l.Name+".yaml")
	}
	return dir
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
