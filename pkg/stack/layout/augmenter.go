package layout

import (
	"archive/tar"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/go-kure/kure/pkg/errors"
)

// LayoutAugmenter is an optional interface that ApplicationConfig
// implementations can implement to attach extra files or configMapGenerator
// entries to their per-app ManifestLayout after resource generation. The
// walker invokes AugmentLayout when app.Config satisfies this interface.
//
// The interface lives in the layout package (rather than pkg/stack alongside
// Validator) because ApplicationConfig — defined in pkg/stack — cannot
// reference *ManifestLayout without creating an import cycle: the layout
// package already imports pkg/stack.
type LayoutAugmenter interface {
	AugmentLayout(layout *ManifestLayout) error
}

// LayoutIntentAugmenter is an optional companion to LayoutAugmenter for a
// config whose desire for its own per-app layout varies per instance rather
// than being fixed for the whole type. Implementing LayoutAugmenter alone is
// a per-type, presence-only signal: the method either exists or it doesn't,
// so a config that only wants its own layout for some instance
// configurations has no way to express that without a separate wrapper type
// per case. LayoutIntentAugmenter lets such a config implement AugmentLayout
// unconditionally and answer "do I want the walker to carve me a directory"
// per instance instead.
//
// WantsOwnLayout() gates placement only, and only where ApplicationGrouping
// is GroupFlat (in every walker, WalkClusterByPackage included): false is
// treated as-if-absent for that decision — the app's resources merge into
// the bundle's layout instead of getting a per-app child, and AugmentLayout
// is not invoked because no per-app layout exists to pass it. With
// ApplicationGrouping GroupByName every app already gets its own layout and
// AugmentLayout runs unconditionally, regardless of augmenter status.
//
// A config that does not implement this interface keeps LayoutAugmenter's
// existing presence-only behaviour unchanged.
//
// The interface lives in the layout package for the same import-cycle
// reason as LayoutAugmenter above.
type LayoutIntentAugmenter interface {
	LayoutAugmenter
	WantsOwnLayout() bool
}

// renderConfigMapGeneratorBlock renders the kustomization.yaml
// configMapGenerator: section for the given specs. Returns the empty string
// when no specs are present, so callers can append unconditionally.
func renderConfigMapGeneratorBlock(specs []ConfigMapGeneratorSpec) string {
	if len(specs) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("configMapGenerator:\n")
	for _, spec := range specs {
		b.WriteString(fmt.Sprintf("  - name: %s\n", spec.Name))
		if len(spec.Files) > 0 {
			b.WriteString("    files:\n")
			for _, f := range spec.Files {
				b.WriteString(fmt.Sprintf("      - %s\n", f))
			}
		}
	}
	return b.String()
}

// extraFileSegment is one segment of an ExtraFile name: a portable file-name
// character set, so a name means the same path on every supported filesystem
// and needs no normalisation before it is compared.
var extraFileSegment = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

// kustomizeControlFiles are the file names kustomize accepts as a directory's
// kustomization; an extra file may take none of them.
var kustomizeControlFiles = []string{"kustomization.yaml", "kustomization.yml", "Kustomization"}

// checkExtraFiles fails when an ExtraFile of ml would take a path the writer
// owns in ml's output directory, or leave it. outDir is the writer's own
// directory function (see outDirFunc), so ml's directory, its generated file
// names and its children's directories are all resolved exactly as the
// writer resolves them. Writers call it with the resource file names they
// are about to write, before writing any file of ml. Before this check an
// extra file named like a generated resource silently replaced it on disk,
// or shadowed it as a later tar entry, while kustomization.yaml still listed
// the path (go-kure/kure#752).
//
// Names are compared case-insensitively, since default macOS volumes treat
// names that differ only in case as one file.
func checkExtraFiles(ml *ManifestLayout, outDir outDirFunc, resourceFiles []string) error {
	if len(ml.ExtraFiles) == 0 {
		return nil
	}
	// No legitimate layout identity or generated file name contains "..",
	// so one that does is refused rather than resolved: everything below can
	// then compare paths that never climb out of the base and back in.
	if err := refuseTraversal("layout namespace", ml.Namespace); err != nil {
		return err
	}
	if err := refuseTraversal("layout name", ml.Name); err != nil {
		return err
	}
	for _, f := range resourceFiles {
		if err := refuseTraversal("generated resource file name", f); err != nil {
			return err
		}
	}
	for _, child := range ml.Children {
		if child == nil {
			continue
		}
		if err := refuseTraversal("child layout namespace", child.Namespace); err != nil {
			return err
		}
		if err := refuseTraversal("child layout name", child.Name); err != nil {
			return err
		}
	}
	mlDir, _ := outDir(ml)
	base := normDir(mlDir)
	taken := map[string]string{} // lower-cased path relative to base -> what owns it
	// A generated name is joined onto base as the writer joins it (path.Join
	// cleans the result, so a rooted "/x" resolves under base).
	for _, f := range resourceFiles {
		if rel, ok := relativeTo(base, normDir(path.Join(base, filepath.ToSlash(f)))); ok && rel != "." {
			taken[rel] = fmt.Sprintf("the generated resource file %q", f)
		}
	}
	for _, f := range kustomizeControlFiles {
		taken[strings.ToLower(f)] = "a kustomize control file"
	}
	// A child whose output directory lies at or below base reserves that
	// directory, or its file for an AppFileSingle child (umbrella children
	// included: kustomization.yaml does not list them, but the writers still
	// write them).
	childDirs := map[string]string{} // lower-cased directory relative to base -> child name
	for _, child := range ml.Children {
		if child == nil {
			continue
		}
		dir, single := outDir(child)
		if single {
			// The file is joined onto the child's directory as the writer
			// joins it, then made relative to base. A child without
			// resources writes no file.
			file := normDir(path.Join(filepath.ToSlash(dir), child.Name+".yaml"))
			if rel, ok := relativeTo(base, file); ok && rel != "." && child.writesSingleFile() {
				taken[rel] = fmt.Sprintf("the child file %q", rel)
			}
			continue
		}
		if rel, ok := relativeTo(base, normDir(dir)); ok && rel != "." {
			childDirs[rel] = child.Name
		}
	}

	// Every directory a reserved file sits in (a generated name in a
	// subdirectory, an AppFileSingle child written below ml's directory) must
	// stay a directory, so no extra may be a file there. With no ".." in a
	// child's Name, a single-file child's file always lies below its own
	// directory, so this also reserves that directory.
	neededDirs := map[string]string{}
	for p, what := range taken {
		for d := path.Dir(p); d != "." && d != "/"; d = path.Dir(d) {
			neededDirs[d] = what
		}
	}
	extras := map[string]bool{}
	for _, ef := range ml.ExtraFiles {
		segs := strings.Split(ef.Name, "/")
		for _, s := range segs {
			if s == "." || s == ".." || !extraFileSegment.MatchString(s) {
				return errors.NewFileError("write", ef.Name, fmt.Sprintf(
					"extra file name %q must be a relative path of segments made of letters, digits, '.', '_' and '-' (no '.' or '..' segments)", ef.Name), nil)
			}
		}
		key := strings.ToLower(ef.Name)
		if what, ok := neededDirs[key]; ok {
			return errors.NewFileError("write", ef.Name, fmt.Sprintf("extra file %q would take a directory that %s needs", ef.Name, what), nil)
		}
		if what, ok := taken[key]; ok {
			return errors.NewFileError("write", ef.Name, fmt.Sprintf("extra file %q would replace %s", ef.Name, what), nil)
		}
		for dir, child := range childDirs {
			// Inside the child's directory, or a file where one of the
			// directories leading to it must be.
			if key == dir || strings.HasPrefix(key, dir+"/") || strings.HasPrefix(dir, key+"/") {
				return errors.NewFileError("write", ef.Name, fmt.Sprintf("extra file %q would take the directory of child layout %q", ef.Name, child), nil)
			}
		}
		if extras[key] {
			return errors.NewFileError("write", ef.Name, fmt.Sprintf("extra file %q is listed twice", ef.Name), nil)
		}
		extras[key] = true
	}
	// No file, generated or extra, can also be the directory an extra sits in.
	for key := range extras {
		for d := path.Dir(key); d != "." && d != "/"; d = path.Dir(d) {
			if what, ok := taken[d]; ok {
				return errors.NewFileError("write", key, fmt.Sprintf("extra file %q would use %s as a directory", key, what), nil)
			}
			if extras[d] {
				return errors.NewFileError("write", key, fmt.Sprintf("extra file %q would sit beneath the extra file %q", key, d), nil)
			}
		}
	}
	return nil
}

// refuseTraversal fails when name (a layout's Namespace or Name, or a
// generated resource file name) has a ".." path segment. A rooted name is
// not refused: path.Join and filepath.Join treat a later "/"-prefixed
// argument as an ordinary component, so it resolves under the base like any
// other relative name.
func refuseTraversal(what, name string) error {
	for _, seg := range strings.Split(filepath.ToSlash(name), "/") {
		if seg == ".." {
			return errors.NewFileError("write", name, fmt.Sprintf("%s %q must not contain a \"..\" path segment", what, name), nil)
		}
	}
	return nil
}

// writeExtraFilesToDisk writes each ExtraFile into dir, creating any
// subdirectory its name contains. checkExtraFiles has already accepted them.
func writeExtraFilesToDisk(dir string, files []ExtraFile) error {
	for _, ef := range files {
		fp := filepath.Join(dir, filepath.FromSlash(ef.Name))
		if err := os.MkdirAll(filepath.Dir(fp), 0755); err != nil {
			return errors.NewFileError("create", filepath.Dir(fp), "directory creation failed", err)
		}
		if err := os.WriteFile(fp, ef.Content, 0644); err != nil {
			return errors.NewFileError("write", fp, "extra file write failed", err)
		}
	}
	return nil
}

// writeExtraFilesToTar writes each ExtraFile as a tar entry under fullPath.
func writeExtraFilesToTar(tw *tar.Writer, fullPath string, files []ExtraFile) error {
	for _, ef := range files {
		if err := writeTarFile(tw, path.Join(fullPath, ef.Name), ef.Content); err != nil {
			return err
		}
	}
	return nil
}

// outDirFunc returns the directory a writer writes l's files to, exactly as
// that writer computes it, and whether it treats l as AppFileSingle. Each
// writer derives its own fullPath from the same function it hands to
// checkExtraFiles, so the check and the write cannot disagree about a path.
type outDirFunc func(l *ManifestLayout) (dir string, single bool)

// normDir is the lower-cased, cleaned, slash-separated form of a directory,
// for comparisons made the way a case-insensitive volume would.
func normDir(p string) string { return strings.ToLower(path.Clean(filepath.ToSlash(p))) }

// relativeTo returns p relative to base (both normDir'ed) and whether p lies
// at or below base.
func relativeTo(base, p string) (string, bool) {
	switch {
	case p == base:
		return ".", true
	case base == "/":
		return p[1:], strings.HasPrefix(p, "/")
	case base == ".":
		return p, p != ".." && !strings.HasPrefix(p, "../") && !path.IsAbs(p)
	case strings.HasPrefix(p, base+"/"):
		return p[len(base)+1:], true
	}
	return "", false
}

// diskOutDir is WriteToDisk's outDirFunc.
func diskOutDir(basePath string) outDirFunc {
	return func(l *ManifestLayout) (string, bool) {
		if l.ApplicationFileMode == AppFileSingle {
			return filepath.Join(basePath, l.Namespace), true
		}
		return filepath.Join(basePath, l.FullRepoPath()), false
	}
}

// tarOutDir is WriteToTar's outDirFunc (archive paths are slash-separated).
func tarOutDir(basePath string) outDirFunc {
	return func(l *ManifestLayout) (string, bool) {
		if l.ApplicationFileMode == AppFileSingle {
			return path.Join(basePath, l.Namespace), true
		}
		return path.Join(basePath, l.FullRepoPath()), false
	}
}

// manifestOutDir is WriteManifest's outDirFunc: the application mode is
// manifestAppMode's, and every directory sits under cfg.ManifestsDir.
func manifestOutDir(basePath string, cfg Config) outDirFunc {
	return func(l *ManifestLayout) (string, bool) {
		if manifestAppMode(l, cfg) == AppFileSingle {
			return filepath.Join(basePath, cfg.ManifestsDir, l.Namespace), true
		}
		return filepath.Join(basePath, cfg.ManifestsDir, l.FullRepoPath()), false
	}
}
