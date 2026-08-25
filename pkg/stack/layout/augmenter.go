package layout

import (
	"archive/tar"
	"fmt"
	"os"
	"path"
	"path/filepath"
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
// WantsOwnLayout() gates placement only, on the flat-bundle walker path
// (processFlatBundleApps): false is treated as-if-absent for that decision —
// the app's resources merge flat into the parent layout instead of getting a
// per-app child, and AugmentLayout is not invoked because no per-app layout
// exists to pass it. It has no effect on the GroupByName or by-package
// walker paths, where an app already gets its own layout and AugmentLayout
// already runs unconditionally, regardless of augmenter status.
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

// writeExtraFilesToDisk writes each ExtraFile into dir.
func writeExtraFilesToDisk(dir string, files []ExtraFile) error {
	for _, ef := range files {
		fp := filepath.Join(dir, ef.Name)
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
