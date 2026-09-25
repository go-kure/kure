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
	// It never does for a KustomizationRecursive layout (go-kure/kure#868).
	writesKustomization func(l *ManifestLayout, root bool) bool
	// childEntry is the entry parent's kustomization.yaml lists for child,
	// or "" when it lists none (see childEntry).
	childEntry func(parent, child *ManifestLayout) string
}

// childEntry returns the entry a parent's kustomization.yaml lists for a
// child, as outDir resolves the child: "<Name>.yaml" for an AppFileSingle
// child that writes its file, "<Name>" for a directory child, and "" for a
// child it does not list:
//   - an umbrella child: under FluxIntegratedPerLayout its Kustomization CR is
//     already in the parent's Resources, whose file the parent lists; under
//     FluxSeparate its CR in flux-system names its directory, so the parent
//     must not reference it at all;
//   - a child that renders bundles: it is a reconciliation unit, applied by its
//     own Flux Kustomization (or ArgoCD Application) and only that one, and
//     listing it here too would apply its objects twice, under two owners, and
//     put them in reach of this directory's patches;
//   - a directory child of a FluxIntegratedPerLayout parent: the Flux
//     Kustomization the integrator placed in the parent's Resources applies
//     it, and nothing is guessed from the child's name;
//   - with skipCrossPackage (WriteToDisk, WriteToTar), a directory child of
//     another package: both PackageRefs set and their values differ.
//
// Such children are still written, each into its own directory.
func childEntry(outDir outDirFunc, skipCrossPackage bool) func(parent, child *ManifestLayout) string {
	return func(parent, child *ManifestLayout) string {
		if child == nil || child.UmbrellaChild || child.rendersBundle() {
			return ""
		}
		if _, single := outDir(child); single {
			if child.writesSingleFile() {
				return child.Name + ".yaml"
			}
			return ""
		}
		if parent.FluxPlacement == FluxIntegratedPerLayout {
			return ""
		}
		if skipCrossPackage && parent.PackageRef != nil && child.PackageRef != nil && *parent.PackageRef != *child.PackageRef {
			return ""
		}
		return child.Name
	}
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
// to list. A KustomizationRecursive layout writes none.
func layoutPlan(outDir outDirFunc) writerPlan {
	files := func(l *ManifestLayout) ([]string, map[string][]client.Object) {
		fileMode := l.FilePer
		if fileMode == FilePerUnset {
			fileMode = FilePerResource
		}
		return groupResourceFiles(l, l.ApplicationFileMode == AppFileSingle, fileMode, l.resolveManifestFileName())
	}
	kustomizationMode := func(l *ManifestLayout) KustomizationMode {
		if l.Mode == KustomizationUnset {
			return KustomizationExplicit
		}
		return l.Mode
	}
	return writerPlan{
		outDir:            outDir,
		files:             files,
		kustomizationMode: kustomizationMode,
		childEntry:        childEntry(outDir, true),
		writesKustomization: func(l *ManifestLayout, root bool) bool {
			if kustomizationMode(l) == KustomizationRecursive {
				return false
			}
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
// and the kustomization mode from the layout or else cfg. A layout whose mode
// so resolves to KustomizationRecursive writes no kustomization.yaml.
func manifestPlan(basePath string, cfg Config) writerPlan {
	nameFn := cfg.ResolveManifestFileName()
	files := func(l *ManifestLayout) ([]string, map[string][]client.Object) {
		fileMode := l.FilePer
		if fileMode == FilePerUnset {
			fileMode = cfg.FilePer
		}
		return groupResourceFiles(l, manifestAppMode(l, cfg) == AppFileSingle, fileMode, nameFn)
	}
	kustomizationMode := func(l *ManifestLayout) KustomizationMode {
		if l.Mode == KustomizationUnset {
			return cfg.ResolveKustomizationMode(l.FluxPlacement)
		}
		return l.Mode
	}
	outDir := manifestOutDir(basePath, cfg)
	return writerPlan{
		outDir:            outDir,
		files:             files,
		kustomizationMode: kustomizationMode,
		childEntry:        childEntry(outDir, false),
		writesKustomization: func(l *ManifestLayout, root bool) bool {
			if kustomizationMode(l) == KustomizationRecursive {
				return false
			}
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
// (go-kure/kure#871), or another AppFileSingle layout's file. In the
// directory of a KustomizationRecursive layout, which has no
// kustomization.yaml, a kustomize control file is refused as well. The file lands
// in the owner's directory, so the owner's file would be silently replaced,
// dropping its objects or its whole kustomization. An AppFileSingle root is
// checked against the kustomization.yaml it writes next to its own file.
// checkLayoutTree runs checkExtraFiles first, so a single child with
// resources named like its parent's extra file is refused there, in its words.
//
// It also refuses such a file where one path needs a directory and another is
// a file (go-kure/kure#878): a file beneath one another layout writes, a file
// that another layout's file needs as a directory, a file at or above a
// directory the writers create for a layout, or a file inside the directory
// of a non-single layout below the one it lands in, as checkExtraFiles
// refuses for a layout's own directory. Without it WriteToDisk and
// WriteManifest failed partway with "not a directory", and WriteToTar wrote
// an archive tar could not extract.
//
// Every file another layout writes is indexed first, since the directory's
// owner may come later in the walk than the single-file layout. Paths are
// compared as normDir compares them: case-insensitively, as on default macOS
// volumes.
func checkSingleFiles(root *ManifestLayout, plan writerPlan) error {
	owned := map[string]string{}      // normDir'ed file path -> what a file there would do
	ownedFiles := map[string]string{} // normDir'ed path of a file another layout writes -> that file
	neededDirs := map[string]string{} // normDir'ed directory a written file or a layout needs -> what needs it
	layoutDirs := map[string]string{} // normDir'ed directory of a non-single layout -> that layout
	// need records that every directory above p, and p itself when self is
	// set, must stay a directory. Every directory above a recorded one is
	// recorded too, so the walk up stops there, and the first to need a
	// directory is the one a refusal names.
	need := func(p string, self bool, what string) {
		d := p
		if !self {
			d = path.Dir(p)
		}
		for ; d != "." && d != "/"; d = path.Dir(d) {
			if _, ok := neededDirs[d]; ok {
				return
			}
			neededDirs[d] = what
		}
	}
	own := func(dir, f, what string) {
		key := normDir(path.Join(filepath.ToSlash(dir), f))
		owned[key] = "replace " + what
		ownedFiles[key] = what
		need(key, false, what)
	}
	type single struct {
		l         *ManifestLayout
		dir, file string
		what      string // "file" or "extra file"
	}
	var singles []single
	var walk func(l *ManifestLayout, root bool)
	walk = func(l *ManifestLayout, root bool) {
		dir, isSingle := plan.outDir(l)
		// The writers create every layout's directory, a single layout's
		// included, whether or not they write a file into it.
		need(normDir(dir), true, fmt.Sprintf("layout %q", l.FullRepoPath()))
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
			layoutDirs[normDir(dir)] = l.FullRepoPath()
			for _, f := range sorted {
				own(dir, f, fmt.Sprintf("the resource file %q of layout %q", f, l.FullRepoPath()))
			}
			for _, ef := range l.ExtraFiles {
				own(dir, ef.Name, fmt.Sprintf("the extra file %q of layout %q", ef.Name, l.FullRepoPath()))
			}
		}
		if plan.writesKustomization(l, root) {
			for _, f := range kustomizeControlFiles {
				own(dir, f, fmt.Sprintf("the %s of layout %q", f, l.FullRepoPath()))
			}
		} else if !isSingle && plan.kustomizationMode(l) == KustomizationRecursive {
			// A Recursive directory has no kustomization.yaml, and must not
			// get one from a single layout's file.
			for _, f := range kustomizeControlFiles {
				owned[normDir(path.Join(filepath.ToSlash(dir), f))] = fmt.Sprintf("make the directory of KustomizationRecursive layout %q a kustomization", l.FullRepoPath())
			}
		}
		for _, child := range l.Children {
			if child != nil {
				walk(child, false)
			}
		}
	}
	walk(root, true)
	// A single layout's own paths are checkExtraFiles', so its files are
	// checked only against other layouts' here.
	landed := map[string]single{}     // normDir'ed file path -> the single layout's file there
	landedDirs := map[string]single{} // normDir'ed directory a landed file needs -> the first such file
	for _, s := range singles {
		key := normDir(path.Join(filepath.ToSlash(s.dir), s.file))
		what, ok := owned[key]
		if other, landedThere := landed[key]; !ok && landedThere && other.l != s.l {
			what, ok = fmt.Sprintf("replace the %s %q of AppFileSingle layout %q", other.what, other.file, other.l.FullRepoPath()), true
		}
		if !ok {
			what, ok = singleDirClash(normDir(s.dir), key, ownedFiles, neededDirs, layoutDirs)
		}
		if other, landedThere := landedDirs[key]; !ok && landedThere && other.l != s.l {
			what, ok = fmt.Sprintf("take a directory that the %s %q of AppFileSingle layout %q needs", other.what, other.file, other.l.FullRepoPath()), true
		}
		for d := path.Dir(key); !ok && d != "." && d != "/"; d = path.Dir(d) {
			if other, landedThere := landed[d]; landedThere && other.l != s.l {
				what, ok = fmt.Sprintf("use the %s %q of AppFileSingle layout %q as a directory", other.what, other.file, other.l.FullRepoPath()), true
			}
		}
		if ok {
			return errors.NewFileError("write", filepath.Join(s.dir, s.file), fmt.Sprintf(
				"layout %q is AppFileSingle and its %s %q would %s", s.l.FullRepoPath(), s.what, s.file, what), nil)
		}
		landed[key] = s
		for d := path.Dir(key); d != "." && d != "/"; d = path.Dir(d) {
			if _, recorded := landedDirs[d]; recorded {
				break
			}
			landedDirs[d] = s
		}
	}
	return nil
}

// singleDirClash reports what an AppFileSingle layout's file at key, landing
// in the directory base (both normDir'ed), would do where it needs a
// directory that another layout's path makes a file, or is a file where
// another layout's path needs a directory: take the directory of a non-single
// layout, or land in one that lies below base; take a directory that another
// layout's file, or a layout's own directory, needs; or use another layout's
// file as a directory.
func singleDirClash(base, key string, ownedFiles, neededDirs, layoutDirs map[string]string) (string, bool) {
	if other, ok := layoutDirs[key]; ok {
		return fmt.Sprintf("take the directory of layout %q", other), true
	}
	for d := path.Dir(key); d != base && d != "." && d != "/"; d = path.Dir(d) {
		if other, ok := layoutDirs[d]; ok {
			return fmt.Sprintf("land in the directory of layout %q", other), true
		}
	}
	if what, ok := neededDirs[key]; ok {
		return fmt.Sprintf("take a directory that %s needs", what), true
	}
	for d := path.Dir(key); d != "." && d != "/"; d = path.Dir(d) {
		if what, ok := ownedFiles[d]; ok {
			return fmt.Sprintf("use %s as a directory", what), true
		}
	}
	return "", false
}
