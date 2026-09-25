package layout

import (
	"fmt"
	"path"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-kure/kure/pkg/errors"
)

// writerPlan is how one writer lays out a layout: the directory it writes
// to, the resource files it groups the layout's objects into, its
// kustomization mode, and whether it writes a kustomization.yaml there. Each
// writer writes from its own plan, and checkLayoutTree checks the tree
// against the same plan before anything is written, so the check and the
// write cannot disagree about a path or a file name.
type writerPlan struct {
	// outDir is the directory the writer writes l's files to, and whether
	// it treats l as AppFileSingle.
	outDir outDirFunc
	// files groups l's resources into the files the writer writes and
	// returns their names, sorted.
	files func(l *ManifestLayout) (sorted []string, groups map[string][]client.Object)
	// kustomizationMode is the writer's effective KustomizationMode for l.
	kustomizationMode func(l *ManifestLayout) KustomizationMode
	// writesKustomization reports whether the writer writes a
	// kustomization.yaml into l's directory; root is true for the tree root.
	writesKustomization func(l *ManifestLayout, root bool) bool
}

// groupResourceFiles groups l's resources into files: all of them into
// <Name>.yaml when single, otherwise by nameFn. It returns the file names
// sorted, for deterministic output.
func groupResourceFiles(l *ManifestLayout, single bool, fileMode FileExportMode, nameFn ManifestFileNameFunc) ([]string, map[string][]client.Object) {
	groups := map[string][]client.Object{}
	for _, obj := range l.Resources {
		ns := obj.GetNamespace()
		if ns == "" {
			ns = "cluster"
		}
		kind := strings.ToLower(obj.GetObjectKind().GroupVersionKind().Kind)
		var fileName string
		if single {
			fileName = fmt.Sprintf("%s.yaml", l.Name)
		} else {
			fileName = nameFn(ns, kind, obj.GetName(), fileMode)
		}
		groups[fileName] = append(groups[fileName], obj)
	}
	sorted := make([]string, 0, len(groups))
	for fileName := range groups {
		sorted = append(sorted, fileName)
	}
	sort.Strings(sorted)
	return sorted, groups
}

// layoutPlan is the plan WriteToDisk and WriteToTar share apart from the
// directory: every mode is the layout's own. An AppFileSingle child writes
// no kustomization.yaml (its parent's lists its file, go-kure/kure#860); an
// AppFileSingle root, which has no parent, writes one when it has anything
// to list.
func layoutPlan(outDir outDirFunc) writerPlan {
	files := func(l *ManifestLayout) ([]string, map[string][]client.Object) {
		fileMode := l.FilePer
		if fileMode == FilePerUnset {
			fileMode = FilePerResource
		}
		return groupResourceFiles(l, l.ApplicationFileMode == AppFileSingle, fileMode, l.resolveManifestFileName())
	}
	return writerPlan{
		outDir: outDir,
		files:  files,
		kustomizationMode: func(l *ManifestLayout) KustomizationMode {
			if l.Mode == KustomizationUnset {
				return KustomizationExplicit
			}
			return l.Mode
		},
		writesKustomization: func(l *ManifestLayout, root bool) bool {
			if l.ApplicationFileMode != AppFileSingle {
				return true
			}
			sorted, _ := files(l)
			return root && (len(sorted) > 0 || len(l.Children) > 0)
		},
	}
}

// diskPlan is WriteToDisk's plan.
func diskPlan(basePath string) writerPlan { return layoutPlan(diskOutDir(basePath)) }

// tarPlan is WriteToTar's plan.
func tarPlan(basePath string) writerPlan { return layoutPlan(tarOutDir(basePath)) }

// manifestPlan is WriteManifest's plan: file naming and FilePer come from
// cfg (a layout's FilePer wins), the application mode from manifestAppMode,
// and the kustomization mode from the layout or else cfg.
func manifestPlan(basePath string, cfg Config) writerPlan {
	nameFn := cfg.ResolveManifestFileName()
	files := func(l *ManifestLayout) ([]string, map[string][]client.Object) {
		fileMode := l.FilePer
		if fileMode == FilePerUnset {
			fileMode = cfg.FilePer
		}
		return groupResourceFiles(l, manifestAppMode(l, cfg) == AppFileSingle, fileMode, nameFn)
	}
	return writerPlan{
		outDir: manifestOutDir(basePath, cfg),
		files:  files,
		kustomizationMode: func(l *ManifestLayout) KustomizationMode {
			if l.Mode == KustomizationUnset {
				return cfg.ResolveKustomizationMode(l.FluxPlacement)
			}
			return l.Mode
		},
		writesKustomization: func(l *ManifestLayout, root bool) bool {
			sorted, _ := files(l)
			// The synthetic cluster root is the cluster-name container
			// created by walkClusterWithClusterName: Name="", single-segment
			// Namespace. It gets no kustomization.yaml when it has no
			// resources of its own. With FlattenSingleTier the root may
			// absorb a collapsed child's Resources, in which case it does
			// need one, and so does one that holds an AppFileSingle child's
			// file (a child with no resources writes none).
			skipClusterRoot := l.Namespace != "" &&
				strings.Count(l.Namespace, string(filepath.Separator)) == 0 &&
				l.Name == "" &&
				len(sorted) == 0 &&
				!l.rendersBundle() &&
				!slices.ContainsFunc(l.Children, func(c *ManifestLayout) bool {
					return c != nil && manifestAppMode(c, cfg) == AppFileSingle && c.writesSingleFile()
				})
			if skipClusterRoot {
				return false
			}
			return manifestAppMode(l, cfg) != AppFileSingle || (root && (len(sorted) > 0 || len(l.Children) > 0))
		},
	}
}

// checkSingleFiles refuses an AppFileSingle layout whose one file, or one of
// whose extra files, as plan writes it, would replace another file the writer
// writes into the same directory: the kustomization.yaml of the layout that
// owns that directory, one of that layout's resource or extra files
// (go-kure/kure#871), or another AppFileSingle layout's file. The file lands
// in the owner's directory, so the owner's file would be silently replaced,
// dropping its objects or its whole kustomization. An AppFileSingle root is
// checked against the kustomization.yaml it writes next to its own file.
// checkLayoutTree runs checkExtraFiles first, so a single child named like
// its parent's extra file is refused there, in its words.
//
// Every file another layout writes is indexed first, since the directory's
// owner may come later in the walk than the single-file layout. Paths are
// compared as normDir compares them: case-insensitively, as on default macOS
// volumes.
func checkSingleFiles(root *ManifestLayout, plan writerPlan) error {
	owned := map[string]string{} // normDir'ed file path -> what writes it
	type single struct {
		l         *ManifestLayout
		dir, file string
		what      string // "file" or "extra file"
	}
	var singles []single
	var walk func(l *ManifestLayout, root bool)
	walk = func(l *ManifestLayout, root bool) {
		dir, isSingle := plan.outDir(l)
		sorted, _ := plan.files(l)
		if isSingle {
			// sorted is empty for a single layout without resources,
			// which writes no file and so replaces nothing.
			for _, f := range sorted {
				singles = append(singles, single{l, dir, f, "file"})
			}
			for _, ef := range l.ExtraFiles {
				singles = append(singles, single{l, dir, ef.Name, "extra file"})
			}
		} else {
			for _, f := range sorted {
				owned[normDir(path.Join(filepath.ToSlash(dir), f))] = fmt.Sprintf("the resource file %q of layout %q", f, l.FullRepoPath())
			}
			for _, ef := range l.ExtraFiles {
				owned[normDir(path.Join(filepath.ToSlash(dir), ef.Name))] = fmt.Sprintf("the extra file %q of layout %q", ef.Name, l.FullRepoPath())
			}
		}
		if plan.writesKustomization(l, root) {
			for _, f := range kustomizeControlFiles {
				owned[normDir(path.Join(filepath.ToSlash(dir), f))] = fmt.Sprintf("the %s of layout %q", f, l.FullRepoPath())
			}
		}
		for _, child := range l.Children {
			if child != nil {
				walk(child, false)
			}
		}
	}
	walk(root, true)
	landed := map[string]single{} // normDir'ed file path -> the single layout's file there
	for _, s := range singles {
		key := normDir(path.Join(filepath.ToSlash(s.dir), s.file))
		what, ok := owned[key]
		if other, landedThere := landed[key]; !ok && landedThere && other.l != s.l {
			what, ok = fmt.Sprintf("the %s %q of AppFileSingle layout %q", other.what, other.file, other.l.FullRepoPath()), true
		}
		if ok {
			return errors.NewFileError("write", filepath.Join(s.dir, s.file), fmt.Sprintf(
				"layout %q is AppFileSingle and its %s %q would replace %s", s.l.FullRepoPath(), s.what, s.file, what), nil)
		}
		landed[key] = s
	}
	return nil
}
