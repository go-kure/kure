// Package api defines configuration structures used to generate
// Kubernetes manifests and Flux resources.
package layout

import "fmt"

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

// GroupingMode controls how nodes, bundles and applications are laid out on disk.
//
// The default for all grouping modes is GroupByName which creates a directory per
// entity. GroupFlat places all entities in the same directory.
type GroupingMode string

const (
	// GroupByName creates a directory for each item in the hierarchy.
	GroupByName GroupingMode = "name"
	// GroupFlat flattens the hierarchy placing all items in the same directory.
	GroupFlat GroupingMode = "flat"
	// GroupUnset indicates that no grouping preference was specified.
	GroupUnset GroupingMode = ""
)

// ApplicationFileMode specifies how resources within an application are written.
//
// The default is AppFilePerResource which mirrors the behaviour of FilePerResource
// and writes each generated resource to its own file. AppFileSingle groups all
// resources belonging to an application into a single manifest file.
type ApplicationFileMode string

const (
	// AppFilePerResource writes each application resource to its own file.
	AppFilePerResource ApplicationFileMode = "resource"
	// AppFileSingle writes all resources for an application into one file.
	AppFileSingle ApplicationFileMode = "single"
	// AppFileUnset indicates that no application file mode was specified.
	AppFileUnset ApplicationFileMode = ""
)

// KustomizationMode determines how kustomization.yaml files reference manifests.
type KustomizationMode string

const (
	// KustomizationExplicit lists each manifest file in kustomization.yaml.
	KustomizationExplicit KustomizationMode = "explicit"
	// KustomizationRecursive references only subdirectories in kustomization.yaml.
	KustomizationRecursive KustomizationMode = "recursive"
	// KustomizationUnset indicates no kustomization mode preference.
	KustomizationUnset KustomizationMode = ""
)

// FluxPlacement determines how Flux Kustomizations are placed in the layout.
type FluxPlacement string

const (
	// FluxSeparate places all Flux Kustomizations in a separate directory.
	FluxSeparate FluxPlacement = "separate"
	// FluxIntegrated distributes Flux Kustomizations across their target nodes.
	FluxIntegrated FluxPlacement = "integrated"
	// FluxUnset indicates no flux placement preference.
	FluxUnset FluxPlacement = ""
)

// LayoutRules control how layouts are generated.
//
// Zero values are interpreted as the defaults described in the field
// documentation.
type LayoutRules struct {
	// NodeGrouping controls how nodes are written to disk. Defaults to
	// GroupByName.
	NodeGrouping GroupingMode
	// BundleGrouping controls how bundles are written to disk. Defaults to
	// GroupByName.
	BundleGrouping GroupingMode
	// ApplicationGrouping controls how applications are written to disk.
	// Defaults to GroupByName.
	ApplicationGrouping GroupingMode
	// ApplicationFileMode controls whether application resources are
	// combined into a single file or split per resource. Defaults to
	// AppFilePerResource.
	ApplicationFileMode ApplicationFileMode
	// FilePer sets the default file export mode for resources. Defaults to
	// FilePerResource.
	FilePer FileExportMode
	// ClusterName specifies a cluster name to prepend to all paths.
	// When set, creates clusters/{ClusterName}/... structure.
	ClusterName string
	// FluxPlacement determines how Flux Kustomizations are placed.
	// Defaults to FluxSeparate.
	FluxPlacement FluxPlacement
}

// DefaultLayoutRules returns a LayoutRules instance populated with the
// documented default values.
func DefaultLayoutRules() LayoutRules {
	return LayoutRules{
		NodeGrouping:        GroupByName,
		BundleGrouping:      GroupByName,
		ApplicationGrouping: GroupByName,
		ApplicationFileMode: AppFilePerResource,
		FilePer:             FilePerResource,
		FluxPlacement:       FluxSeparate,
	}
}

// Validate ensures the LayoutRules contain known option values.
func (lr LayoutRules) Validate() error {
	validGrouping := func(g GroupingMode) bool {
		switch g {
		case GroupByName, GroupFlat, GroupUnset:
			return true
		default:
			return false
		}
	}

	if !validGrouping(lr.NodeGrouping) {
		return fmt.Errorf("invalid node grouping: %s", lr.NodeGrouping)
	}
	if !validGrouping(lr.BundleGrouping) {
		return fmt.Errorf("invalid bundle grouping: %s", lr.BundleGrouping)
	}
	if !validGrouping(lr.ApplicationGrouping) {
		return fmt.Errorf("invalid application grouping: %s", lr.ApplicationGrouping)
	}

	switch lr.ApplicationFileMode {
	case AppFilePerResource, AppFileSingle, AppFileUnset:
		// valid
	default:
		return fmt.Errorf("invalid application file mode: %s", lr.ApplicationFileMode)
	}

	switch lr.FilePer {
	case FilePerResource, FilePerKind, FilePerUnset:
		// valid
	default:
		return fmt.Errorf("invalid file export mode: %s", lr.FilePer)
	}

	switch lr.FluxPlacement {
	case FluxSeparate, FluxIntegrated, FluxUnset:
		// valid
	default:
		return fmt.Errorf("invalid flux placement: %s", lr.FluxPlacement)
	}

	return nil
}
