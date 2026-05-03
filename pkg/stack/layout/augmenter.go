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
