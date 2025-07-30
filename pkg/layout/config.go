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
	// ManifestFileName formats the file name for a resource manifest.
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
