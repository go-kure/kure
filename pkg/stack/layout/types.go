// Package api defines configuration structures used to generate
// Kubernetes manifests and Flux resources.
package layout

import (
	"github.com/go-kure/kure/pkg/errors"
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
	// KustomizationRecursive writes no kustomization.yaml into the layout's
	// directory, and every other file as KustomizationExplicit does: a Flux
	// Kustomization builds the directory from every .yaml and .yml file below
	// it (go-kure/kure#868).
	KustomizationRecursive KustomizationMode = "recursive"
	// KustomizationUnset indicates no kustomization mode preference.
	KustomizationUnset KustomizationMode = ""
)

// FileNamingMode controls the file naming pattern for manifest files.
type FileNamingMode string

const (
	// FileNamingDefault uses the standard {namespace}-{kind}-{name}.yaml format.
	FileNamingDefault FileNamingMode = "default"
	// FileNamingKindName uses the {kind}-{name}.yaml format, omitting the namespace prefix.
	FileNamingKindName FileNamingMode = "kind-name"
	// FileNamingUnset indicates no file naming preference.
	FileNamingUnset FileNamingMode = ""
)

// FluxPlacement determines how Flux Kustomizations are placed in the layout.
type FluxPlacement string

const (
	// FluxSeparate places all Flux Kustomizations in a separate directory.
	FluxSeparate FluxPlacement = "separate"
	// FluxIntegratedPerLayout places a Flux Kustomization CR inline for every
	// layout node, including augmenter-added child layouts. Each child's CR is
	// hosted in its parent layout and listed there as a resource file; the
	// parent does not reference the child directory. Finest granularity; use
	// when each child should be reconciled by
	// its own Flux Kustomization (e.g. hook-group dependsOn).
	FluxIntegratedPerLayout FluxPlacement = "integrated"
	// FluxIntegratedPerBundle places Flux Kustomization CRs inline at bundle
	// boundaries only, each hosted in the parent of the directory it applies;
	// a bundle's interior (application and augmenter-added child layouts) is a
	// single kustomize build, with those children referenced as directories.
	// A child that renders bundles is never referenced: its own CR applies
	// it. Coarser than PerLayout: Flux reconciles per bundle, kustomize
	// handles the interior. Use when the unit of Flux reconciliation is the
	// bundle, not each layout node.
	FluxIntegratedPerBundle FluxPlacement = "integrated-per-bundle"
	// FluxUnset indicates no flux placement preference.
	FluxUnset FluxPlacement = ""
)

// LayoutRules control how layouts are generated.
//
// Zero values are interpreted as the defaults described in the field
// documentation.
type LayoutRules struct {
	// NodeGrouping says whether each child node gets its own directory
	// (GroupByName) or is merged into its parent's (GroupFlat): its bundle,
	// child nodes and origins then render in the parent's directory. The root
	// node always keeps its directory. Defaults to GroupByName.
	NodeGrouping GroupingMode
	// BundleGrouping says whether each bundle gets its own directory inside
	// its node's (GroupByName) or is rendered in the node's directory
	// (GroupFlat). Defaults to GroupFlat.
	BundleGrouping GroupingMode
	// ApplicationGrouping says whether each application gets its own
	// directory inside its bundle's (GroupByName) or writes its resources into
	// the bundle's directory (GroupFlat). An augmenter application that wants
	// its own layout gets a directory either way, and so does every umbrella
	// child bundle. The three axes are independent. Defaults to GroupFlat.
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
	// FileNaming controls the file naming pattern for manifest files.
	// Defaults to FileNamingDefault ({namespace}-{kind}-{name}.yaml).
	FileNaming FileNamingMode

	// FlattenSingleTier collapses a vestigial intermediate directory layer
	// produced by the walker when it adds no semantic value: a parent layout
	// with exactly one named child whose own children are empty and which is
	// not an UmbrellaChild, where the parent itself is a top-level layout
	// (Namespace has no path separator) with no own Resources.
	//
	// Typical case: flat single-bundle apps where the caller wraps the bundle
	// in an extra Node (e.g. the caller's "apps" Node). Multi-tier apps with sub-
	// Kustomizations are unaffected — the collapse rules require the
	// intermediate to be terminal.
	//
	// Only effective for WalkCluster (not WalkClusterByPackage, which uses
	// synthetic unnamed wrappers to express package boundaries).
	//
	// The absorbing layout takes over the collapsed layout's origins (the
	// nodes, bundles and application it rendered), so every Flux
	// Kustomization and ArgoCD Application generated from the layout names
	// the post-collapse directory. Nothing is rewritten afterwards: a Flux
	// CR a caller adds to the walked tree keeps the spec.path it was given.
	FlattenSingleTier bool
}

// DefaultLayoutRules returns a LayoutRules instance populated with the
// documented default values.
func DefaultLayoutRules() LayoutRules {
	return LayoutRules{
		NodeGrouping:        GroupByName,
		BundleGrouping:      GroupFlat, // Avoid bundle/app/app nesting
		ApplicationGrouping: GroupFlat, // Avoid double nesting
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
		return errors.NewValidationError("NodeGrouping", string(lr.NodeGrouping), "LayoutRules", []string{string(GroupByName), string(GroupFlat)})
	}
	if !validGrouping(lr.BundleGrouping) {
		return errors.NewValidationError("BundleGrouping", string(lr.BundleGrouping), "LayoutRules", []string{string(GroupByName), string(GroupFlat)})
	}
	if !validGrouping(lr.ApplicationGrouping) {
		return errors.NewValidationError("ApplicationGrouping", string(lr.ApplicationGrouping), "LayoutRules", []string{string(GroupByName), string(GroupFlat)})
	}

	switch lr.ApplicationFileMode {
	case AppFilePerResource, AppFileSingle, AppFileUnset:
		// valid
	default:
		return errors.NewValidationError("ApplicationFileMode", string(lr.ApplicationFileMode), "LayoutRules", []string{string(AppFilePerResource), string(AppFileSingle)})
	}

	switch lr.FilePer {
	case FilePerResource, FilePerKind, FilePerUnset:
		// valid
	default:
		return errors.NewValidationError("FilePer", string(lr.FilePer), "LayoutRules", []string{string(FilePerResource), string(FilePerKind)})
	}

	switch lr.FluxPlacement {
	case FluxSeparate, FluxIntegratedPerLayout, FluxIntegratedPerBundle, FluxUnset:
		// valid
	default:
		return errors.NewValidationError("FluxPlacement", string(lr.FluxPlacement), "LayoutRules", []string{string(FluxSeparate), string(FluxIntegratedPerLayout), string(FluxIntegratedPerBundle)})
	}

	switch lr.FileNaming {
	case FileNamingDefault, FileNamingKindName, FileNamingUnset:
		// valid
	default:
		return errors.NewValidationError("FileNaming", string(lr.FileNaming), "LayoutRules", []string{string(FileNamingDefault), string(FileNamingKindName)})
	}

	return nil
}
