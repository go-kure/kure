package layout

import (
	"fmt"
	"path/filepath"
	"slices"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/go-kure/kure/pkg/errors"
)

// checkLayoutTree refuses a tree in which two layouts resolve to the same
// directory (or, for AppFileSingle layouts, the same file), an AppFileSingle
// child has children (see checkSingleChildLeaf), an extra file takes a path
// the writer owns (see checkExtraFiles), or an AppFileSingle layout's file or
// extra file would replace another file in its directory (see checkSingleFiles),
// before anything is written (go-kure/kure#771). Each such layout writes its own files there,
// and the later kustomization.yaml silently replaces the earlier one, dropping
// its resources from the kustomize graph. Directories are compared
// case-insensitively, as on default macOS volumes. plan is the writer's own,
// so the check and the write agree on every path and file name.
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
	return checkSingleFiles(root, plan)
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
		id := fmt.Sprintf("%s/%s %s/%s", gvk.Group, gvk.Kind, ns, acc.GetName())
		if _, dup := seen[id]; dup {
			return errors.NewFileError("write", l.FullRepoPath(),
				fmt.Sprintf("layout %q holds the same object %s twice", l.FullRepoPath(), id), nil)
		}
		seen[id] = struct{}{}
		return nil
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
