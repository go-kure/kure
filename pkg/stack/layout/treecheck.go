package layout

import (
	"fmt"
	"path/filepath"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/go-kure/kure/pkg/errors"
)

// checkLayoutTree refuses a tree in which two layouts resolve to the same
// directory (or, for AppFileSingle layouts, the same file), before anything
// is written (go-kure/kure#771). Each such layout writes its own files there,
// and the later kustomization.yaml silently replaces the earlier one, dropping
// its resources from the kustomize graph. Directories are compared
// case-insensitively, as on default macOS volumes. outDir is the writer's own
// outDirFunc, so the check and the write agree on every path.
func checkLayoutTree(root *ManifestLayout, outDir outDirFunc) error {
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
