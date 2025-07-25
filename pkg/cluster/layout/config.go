package layout

import (
	"fmt"
	"strings"

	"github.com/go-kure/kure/pkg/cluster/api"
)

// ManifestFileNameFunc returns a file name for the given namespace, kind and resource name.
type ManifestFileNameFunc func(namespace, kind, name string, mode api.FileExportMode) string

// KustomizationFileNameFunc returns the file name for a Flux Kustomization manifest.
type KustomizationFileNameFunc func(name string) string

// LayoutConfig defines rules for generating a cluster layout.
type LayoutConfig struct {
	// ManifestsDir is the directory under which Kubernetes manifests are written.
	ManifestsDir string
	// FluxDir is the directory under which Flux manifests are written.
	FluxDir string
	// FilePer determines how resources are grouped into files when writing manifests.
	FilePer api.FileExportMode
	// ManifestFileName formats the file name for a resource manifest.
	ManifestFileName ManifestFileNameFunc
	// KustomizationFileName formats the file name for a Flux Kustomization.
	KustomizationFileName KustomizationFileNameFunc
}

// DefaultLayoutConfig returns a configuration that matches the directory layout
// expected by FluxCD when writing manifests and Kustomizations.
func DefaultLayoutConfig() LayoutConfig {
	return LayoutConfig{
		ManifestsDir:          "clusters",
		FluxDir:               "clusters",
		FilePer:               api.FilePerResource,
		ManifestFileName:      DefaultManifestFileName,
		KustomizationFileName: DefaultKustomizationFileName,
	}
}

// DefaultManifestFileName implements the standard file naming convention used
// by Kure. It writes either one file per resource or groups by kind depending
// on the FileExportMode.
func DefaultManifestFileName(namespace, kind, name string, mode api.FileExportMode) string {
	kind = strings.ToLower(kind)
	switch mode {
	case api.FilePerKind:
		return fmt.Sprintf("%s-%s.yaml", namespace, kind)
	default:
		return fmt.Sprintf("%s-%s-%s.yaml", namespace, kind, name)
	}
}

// DefaultKustomizationFileName returns the standard Flux Kustomization file name.
func DefaultKustomizationFileName(name string) string {
	return fmt.Sprintf("kustomization-%s.yaml", name)
}
