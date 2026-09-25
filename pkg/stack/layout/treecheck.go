package layout

import (
	"fmt"
	"path"
	"path/filepath"
	"slices"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/go-kure/kure/pkg/errors"
)

// checkLayoutTree refuses, before anything is written, a tree in which:
//   - two layouts resolve to the same directory, or, for AppFileSingle
//     layouts, the same file (go-kure/kure#771): each writes its own files
//     there, and the later kustomization.yaml silently replaces the earlier
//     one, dropping its resources from the kustomize graph;
//   - an AppFileSingle child has children (see checkSingleChildLeaf);
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
			// The whole file path is cleaned, as the writers clean it, so
			// "x" and "./x" are the same file.
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
		if err := checkResourceIdentities(l); err != nil {
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
// (as checkResourceIdentities defines it) that one kustomize build the writer
// produces takes in together: kustomize refuses to add the second object ("may
// not add resource with an already registered id"), so the kustomization.yaml
// kure writes, or the Flux build of its Recursive directory, would not build
// (go-kure/kure#880). The builds, each checked on its own, are:
//   - every layout the writer writes a kustomization.yaml for: its own
//     resource files, the file of each AppFileSingle child it lists, and the
//     build of each directory it lists, each entry resolved against its
//     directory as kustomize resolves it;
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
	below := func(base, p string) bool {
		rel, ok := relativeTo(base, p)
		return ok && rel != "."
	}
	// recursive adds to in the layouts whose objects the Flux build of the
	// Recursive layout d takes in.
	recursive := func(d laid, in map[*ManifestLayout]bool) {
		// shielded reports whether a directory strictly below d's, at or
		// above p (strictly above it when strict), has a kustomization.yaml.
		shielded := func(p string, strict bool) bool {
			for _, m := range all {
				if m.single || !m.writesK || !below(d.dir, m.dir) {
					continue
				}
				if (!strict && m.dir == p) || below(m.dir, p) {
					return true
				}
			}
			return false
		}
		for _, t := range all {
			if (t.dir != d.dir && !below(d.dir, t.dir)) || shielded(t.dir, !t.single) {
				continue
			}
			if !t.single && t.l != d.l && t.writesK {
				explicit(t.l, in)
			} else {
				in[t.l] = true
			}
		}
	}

	ids := map[*ManifestLayout][]string{}
	for _, li := range all {
		if err := eachIdentity(li.l, func(id string) error {
			ids[li.l] = append(ids[li.l], id)
			return nil
		}); err != nil {
			return err
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
		seen := map[string]*ManifestLayout{}
		for _, li := range all {
			if !in[li.l] {
				continue
			}
			for _, id := range ids[li.l] {
				if other, dup := seen[id]; dup && other != li.l {
					return errors.NewFileError("write", b.l.FullRepoPath(), fmt.Sprintf(
						"layouts %q and %q both hold the object %s, and %s: kustomize refuses one object twice",
						other.FullRepoPath(), li.l.FullRepoPath(), id, what), nil)
				}
				seen[id] = li.l
			}
		}
	}
	return nil
}

// checkSingleChildLeaf refuses an AppFileSingle child (as outDir treats it)
// that has children of its own. Such a child writes one file into its
// parent's directory and no kustomization.yaml, so nothing would list the
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
		fmt.Sprintf("layout %q is AppFileSingle and has child layouts: it writes one file into its parent's directory and no kustomization.yaml, so nothing would list them", child.FullRepoPath()), nil)
}

// checkResourceIdentities refuses a layout that holds two resources with one
// Kubernetes identity (group, kind, namespace and name). Resources that share
// a file name are legitimately written into one multi-document file (every
// FilePerKind file does this), but two objects with one identity in one
// directory make kustomize fail to build it. A grouping axis set to flat
// merges several applications' or nodes' resources into one directory, which
// is where this happens.
func checkResourceIdentities(l *ManifestLayout) error {
	seen := make(map[string]struct{}, len(l.Resources))
	return eachIdentity(l, func(id string) error {
		if _, dup := seen[id]; dup {
			return errors.NewFileError("write", l.FullRepoPath(),
				fmt.Sprintf("layout %q holds the same object %s twice", l.FullRepoPath(), id), nil)
		}
		seen[id] = struct{}{}
		return nil
	})
}

// eachIdentity calls fn, in order, with the identity of every object l holds,
// "<group>/<kind> <namespace>/<name>", a List's items standing in for the
// List. An object without a kind has none and is skipped.
func eachIdentity(l *ManifestLayout, fn func(id string) error) error {
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
		// kustomize reads an omitted namespace as "default", so an object
		// without one and the same object in "default" are one identity.
		ns := acc.GetNamespace()
		if ns == "" {
			ns = "default"
		}
		return fn(fmt.Sprintf("%s/%s %s/%s", gvk.Group, gvk.Kind, ns, acc.GetName()))
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
