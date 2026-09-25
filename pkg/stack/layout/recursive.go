package layout

import (
	"fmt"
	"path"
	"path/filepath"
	"slices"

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
//   - an extra file in D's build, outside every such directory, that Flux
//     reads: one named *.yaml or *.yml, which Flux would apply (or fail the
//     whole build on one it cannot decode), or one named like a kustomization
//     file, which Flux would build as a kustomization of its own. An Explicit
//     kustomization.yaml lists neither;
//   - a resource file in D's build, outside every such directory, that Flux
//     does not read as a manifest: one not named *.yaml or *.yml, which Flux
//     skips, or one named like a kustomization file. An Explicit
//     kustomization.yaml lists it by name. Only a Config's ManifestFileName
//     can produce either;
//   - the file of an AppFileSingle child in D's build that its parent's
//     kustomization.yaml does not list (an umbrella child, or one that
//     renders bundles): Flux would apply it, where the Explicit mode leaves
//     it to the child's own Kustomization.
//
// Flux's tests are its own: the file extension as it is, and the base name
// against the kustomization file names exactly. Directories are compared as
// normDir compares them.
func checkRecursiveLayouts(root *ManifestLayout, plan writerPlan) error {
	type dirLayout struct {
		l       *ManifestLayout
		dir     string // normDir'ed
		writesK bool
	}
	type file struct {
		l        *ManifestLayout
		name     string
		dir      string // normDir'ed directory the file lands in
		raw      string // that directory as written, for messages
		extra    bool   // an ExtraFile, not a resource file
		unlisted bool   // a resource file its directory's kustomization.yaml does not list
	}
	var dirs []dirLayout
	var files []file
	var walk func(l, parent *ManifestLayout) error
	walk = func(l, parent *ManifestLayout) error {
		dir, single := plan.outDir(l)
		add := func(name string, extra, unlisted bool) {
			raw := path.Dir(path.Join(filepath.ToSlash(dir), name))
			files = append(files, file{l, name, normDir(raw), raw, extra, unlisted})
		}
		for _, ef := range l.ExtraFiles {
			add(ef.Name, true, false)
		}
		if single && parent != nil && l.writesSingleFile() && plan.childEntry(parent, l) == "" {
			add(l.Name+".yaml", false, true)
		}
		if !single {
			names, _ := plan.files(l)
			for _, name := range names {
				add(name, false, false)
			}
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
		for _, f := range files {
			if (f.dir != d.dir && !below(d.dir, f.dir)) || shielded(f.dir, false) {
				continue
			}
			ext := path.Ext(f.name)
			manifest := ext == ".yaml" || ext == ".yml"
			control := slices.Contains(kustomizeControlFiles, path.Base(f.name))
			var why string
			switch {
			case control && f.extra:
				why = fmt.Sprintf("would build the extra file %q of layout %q as a kustomization, which the Explicit mode never lists", f.name, f.l.FullRepoPath())
			case control:
				why = fmt.Sprintf("would read the resource file %q of layout %q as a kustomization, where the Explicit mode lists it as a manifest", f.name, f.l.FullRepoPath())
			case f.extra && manifest:
				why = fmt.Sprintf("would apply the extra file %q of layout %q, which the Explicit mode never lists", f.name, f.l.FullRepoPath())
			case f.unlisted && manifest:
				why = fmt.Sprintf("would apply the file %q of layout %q, which the Explicit mode does not list", f.name, f.l.FullRepoPath())
			case !f.extra && !manifest:
				why = fmt.Sprintf("would skip the resource file %q of layout %q, as Flux reads only *.yaml and *.yml files, where the Explicit mode lists it", f.name, f.l.FullRepoPath())
			default:
				continue
			}
			return errors.NewFileError("write", path.Join(f.raw, path.Base(f.name)), fmt.Sprintf(
				"the Flux build of KustomizationRecursive layout %q %s", d.l.FullRepoPath(), why), nil)
		}
	}
	return nil
}
