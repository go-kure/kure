package layout

import (
	"fmt"
	"strings"
)

// ManifestFileNameFunc returns a file name for the given namespace, kind and resource name.
type ManifestFileNameFunc func(namespace, kind, name string, mode FileExportMode) string

// KustomizationFileNameFunc returns the file name for a Flux Kustomization manifest.
type KustomizationFileNameFunc func(name string) string

// Config LayoutConfig defines rules for generating a cluster layout.
type Config struct {
	// ManifestsDir is the directory under which Kubernetes manifests are written.
	ManifestsDir string
	// FluxDir is the directory under which Flux manifests are written.
	FluxDir string
	// FilePer determines how resources are grouped into files when writing manifests.
	FilePer FileExportMode
	// ApplicationFileMode controls whether application resources are written
	// to a single file or split per resource. Defaults to AppFilePerResource.
	ApplicationFileMode ApplicationFileMode
	// KustomizationMode controls how kustomization.yaml files are generated.
	// Defaults to KustomizationExplicit.
	KustomizationMode KustomizationMode
	// FluxKustomizationMode overrides KustomizationMode based on FluxPlacement.
	// When a ManifestLayout's FluxPlacement matches a key in this map, the
	// corresponding KustomizationMode is used instead of the global
	// KustomizationMode. This allows different kustomization.yaml reference
	// styles per flux placement strategy.
	FluxKustomizationMode map[FluxPlacement]KustomizationMode
	// FileNaming controls the file naming pattern. When set, it determines
	// the ManifestFileNameFunc to use. If ManifestFileName is also set, it
	// takes precedence over FileNaming.
	FileNaming FileNamingMode
	// ManifestFileName formats the file name for a resource manifest.
	// Takes precedence over FileNaming when set.
	ManifestFileName ManifestFileNameFunc
	// KustomizationFileName formats the file name for a Flux Kustomization.
	KustomizationFileName KustomizationFileNameFunc
}

// DefaultLayoutConfig returns a configuration that matches the directory layout
// expected by FluxCD when writing manifests and Kustomizations.
func DefaultLayoutConfig() Config {
	return Config{
		ManifestsDir:          "clusters",
		FluxDir:               "clusters",
		FilePer:               FilePerResource,
		ApplicationFileMode:   AppFilePerResource,
		KustomizationMode:     KustomizationExplicit,
		ManifestFileName:      DefaultManifestFileName,
		KustomizationFileName: DefaultKustomizationFileName,
	}
}

// DefaultManifestFileName implements the standard file naming convention used
// by Kure. It writes either one file per resource or groups by kind depending
// on the FileExportMode.
func DefaultManifestFileName(namespace, kind, name string, mode FileExportMode) string {
	kind = strings.ToLower(kind)
	switch mode {
	case FilePerKind:
		return fmt.Sprintf("%s-%s.yaml", namespace, kind)
	default:
		return fmt.Sprintf("%s-%s-%s.yaml", namespace, kind, name)
	}
}

// DefaultKustomizationFileName returns the standard Flux Kustomization file name.
func DefaultKustomizationFileName(name string) string {
	return fmt.Sprintf("kustomization-%s.yaml", name)
}

// ResolveManifestFileName returns the effective ManifestFileNameFunc for this
// Config. If ManifestFileName is set it is returned directly. Otherwise,
// FileNaming is used to select the function. If neither is set,
// DefaultManifestFileName is returned.
func (c Config) ResolveManifestFileName() ManifestFileNameFunc {
	if c.ManifestFileName != nil {
		return c.ManifestFileName
	}
	switch c.FileNaming {
	case FileNamingKindName:
		return KindNameManifestFileName
	default:
		return DefaultManifestFileName
	}
}

// ResolveKustomizationMode returns the effective KustomizationMode for the
// given FluxPlacement. If FluxKustomizationMode contains an override for the
// placement, that value is used. Otherwise, the global KustomizationMode (or
// KustomizationExplicit when unset) is returned.
func (c Config) ResolveKustomizationMode(fp FluxPlacement) KustomizationMode {
	if c.FluxKustomizationMode != nil {
		if mode, ok := c.FluxKustomizationMode[fp]; ok {
			return mode
		}
	}
	if c.KustomizationMode == KustomizationUnset {
		return KustomizationExplicit
	}
	return c.KustomizationMode
}
