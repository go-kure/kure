package layout

import (
	"fmt"
	"path/filepath"

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
