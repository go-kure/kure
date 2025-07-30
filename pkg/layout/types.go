// Package api defines configuration structures used to generate
// Kubernetes manifests and Flux resources.
package layout

import (
	"github.com/go-kure/kure/pkg/application"
)

// FileExportMode determines how resources are written to disk.
type FileExportMode string

const (
	// FilePerResource writes each resource to its own file.
	FilePerResource FileExportMode = "resource"
	// FilePerKind groups resources by kind into a single file.
	FilePerKind FileExportMode = "kind"
	// FilePerUnset indicates that no export mode is specified.
	FilePerUnset FileExportMode = ""
)

// AppGroup groups related applications under a single namespace.
type AppGroup struct {
	Name          string                          `yaml:"name"`
	Namespace     string                          `yaml:"namespace,omitempty"`
	Apps          []application.AppWorkloadConfig `yaml:"apps,omitempty"`
	FilePer       FileExportMode                  `yaml:"filePer,omitempty"`
	FluxDependsOn []string                        `yaml:"fluxDependsOn,omitempty"`
}

// LayoutRules control how layouts are generated.
type LayoutRules struct {
	// FilePer sets the default file export mode for resources.
	FilePer FileExportMode
}
