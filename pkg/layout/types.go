// Package api defines configuration structures used to generate
// Kubernetes manifests and Flux resources.
package layout

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

// LayoutRules control how layouts are generated.
type LayoutRules struct {
	// FilePer sets the default file export mode for resources.
	FilePer FileExportMode
}
