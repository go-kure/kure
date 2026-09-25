package layout

import (
	"fmt"
	"path"
	"path/filepath"

	"github.com/go-kure/kure/pkg/errors"
)

// checkRecursiveLayouts refuses what a KustomizationRecursive layout, which
// gets every file except kustomization.yaml (go-kure/kure#868), makes
// contradictory in the writer's own output:
//   - ConfigMapGenerators on it: they exist only inside a kustomization.yaml;
//   - a kustomization.yaml the writer writes that lists its directory, which
//     kustomize cannot build without one.
//
// A Recursive directory marked with SetFluxBuild is built by a Flux
// Kustomization kure generated, and Flux generates the kustomization.yaml:
// every .yaml and .yml file below the directory, where a subdirectory that
// has a kustomization file is added as a whole and not descended into. For
// such a directory D it also refuses what makes that build differ from the
// one Explicit mode would give:
//   - a marked layout T below D with no kustomization.yaml the writer writes
//     in any directory strictly between D and T (T's own does not count, as
//     Flux adds T's directory as a resource): D's Kustomization would apply
//     T's objects, and T's would too;
//   - an extra file named *.yaml or *.yml in D's build, outside every such
//     directory: Flux would apply it (or fail the whole build on one it
//     cannot decode), and an Explicit kustomization.yaml never lists it.
//
// Directories are compared as normDir compares them.
func checkRecursiveLayouts(root *ManifestLayout, plan writerPlan) error {
	type dirLayout struct {
		l       *ManifestLayout
		dir     string // normDir'ed
		writesK bool
	}
	type extraFile struct {
		l    *ManifestLayout
		name string
		dir  string // normDir'ed directory the file lands in
	}
	var dirs []dirLayout
	var extras []extraFile
	var walk func(l, parent *ManifestLayout) error
	walk = func(l, parent *ManifestLayout) error {
		dir, single := plan.outDir(l)
		for _, ef := range l.ExtraFiles {
			extras = append(extras, extraFile{l, ef.Name, normDir(path.Dir(path.Join(filepath.ToSlash(dir), ef.Name)))})
		}
		if !single {
			dirs = append(dirs, dirLayout{l, normDir(dir), plan.writesKustomization(l, parent == nil)})
			if plan.kustomizationMode(l) == KustomizationRecursive {
				if len(l.ConfigMapGenerators) > 0 {
					return errors.NewFileError("write", dir, fmt.Sprintf(
						"layout %q is KustomizationRecursive, which writes no kustomization.yaml, so its ConfigMapGenerators have nowhere to go", l.FullRepoPath()), nil)
				}
				if parent != nil && plan.writesKustomization(parent, parent == root) && plan.childEntry(parent, l) != "" {
					return errors.NewFileError("write", dir, fmt.Sprintf(
						"layout %q is KustomizationRecursive and writes no kustomization.yaml, but the kustomization.yaml of layout %q lists its directory", l.FullRepoPath(), parent.FullRepoPath()), nil)
				}
			}
		}
		for _, child := range l.Children {
			if child == nil {
				continue
			}
			if err := walk(child, l); err != nil {
				return err
			}
		}
		return nil
	}
	if err := walk(root, nil); err != nil {
		return err
	}

	below := func(base, p string) bool {
		rel, ok := relativeTo(base, p)
		return ok && rel != "."
	}
	for _, d := range dirs {
		if !d.l.fluxBuild || plan.kustomizationMode(d.l) != KustomizationRecursive {
			continue
		}
		// shielded reports whether a directory strictly below d.dir, at or
		// above p (strictly above it when strict), has a kustomization.yaml.
		shielded := func(p string, strict bool) bool {
			for _, m := range dirs {
				if !m.writesK || !below(d.dir, m.dir) {
					continue
				}
				if (!strict && m.dir == p) || below(m.dir, p) {
					return true
				}
			}
			return false
		}
		for _, t := range dirs {
			if t.l.fluxBuild && below(d.dir, t.dir) && !shielded(t.dir, true) {
				return errors.NewFileError("write", d.l.FullRepoPath(), fmt.Sprintf(
					"the Flux build of KustomizationRecursive layout %q would include layout %q, which a Flux Kustomization kure generated builds as well: its objects would be applied twice", d.l.FullRepoPath(), t.l.FullRepoPath()), nil)
			}
		}
		for _, e := range extras {
			// Flux's own test: the file extension, as it is.
			if ext := path.Ext(e.name); ext != ".yaml" && ext != ".yml" {
				continue
			}
			if (e.dir == d.dir || below(d.dir, e.dir)) && !shielded(e.dir, false) {
				return errors.NewFileError("write", path.Join(e.dir, e.name), fmt.Sprintf(
					"the Flux build of KustomizationRecursive layout %q would apply the extra file %q of layout %q, which the Explicit mode never lists", d.l.FullRepoPath(), e.name, e.l.FullRepoPath()), nil)
			}
		}
	}
	return nil
}
