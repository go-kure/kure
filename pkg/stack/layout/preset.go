package layout

import "fmt"

// LayoutPreset identifies a named layout pattern that configures all layout
// dimensions into a known-valid combination. Presets are based on the layout
// patterns defined in the Wharf FluxCD layout research.
type LayoutPreset string

const (
	// PresetCentralizedControlPlane implements Pattern A: all Flux KS CRs in
	// dedicated aggregator directories, completely separate from payload. Best
	// suited for fleet management with many applications. Uses flat directory
	// grouping and {kind}-{name}.yaml file naming.
	PresetCentralizedControlPlane LayoutPreset = "CentralizedControlPlane"

	// PresetSiblingControlPlane implements Pattern B: Flux KS CRs in a
	// flux-system/ sibling directory within each artifact. Designed for
	// single-artifact or per-app artifact scenarios.
	PresetSiblingControlPlane LayoutPreset = "SiblingControlPlane"

	// PresetParentDeployedControl implements Pattern C: KS CRs live in the
	// payload of their parent Kustomization. Best suited for simple,
	// single-app deployments.
	PresetParentDeployedControl LayoutPreset = "ParentDeployedControl"
)

// LayoutRulesForPreset returns LayoutRules configured for the given preset.
// Unknown presets return an error.
func LayoutRulesForPreset(p LayoutPreset) (LayoutRules, error) {
	switch p {
	case PresetCentralizedControlPlane:
		return LayoutRules{
			NodeGrouping:        GroupFlat,
			BundleGrouping:      GroupFlat,
			ApplicationGrouping: GroupFlat,
			ApplicationFileMode: AppFilePerResource,
			FilePer:             FilePerResource,
			FluxPlacement:       FluxSeparate,
		}, nil

	case PresetSiblingControlPlane:
		return LayoutRules{
			NodeGrouping:        GroupByName,
			BundleGrouping:      GroupFlat,
			ApplicationGrouping: GroupFlat,
			ApplicationFileMode: AppFilePerResource,
			FilePer:             FilePerResource,
			FluxPlacement:       FluxSeparate,
		}, nil

	case PresetParentDeployedControl:
		return LayoutRules{
			NodeGrouping:        GroupByName,
			BundleGrouping:      GroupFlat,
			ApplicationGrouping: GroupFlat,
			ApplicationFileMode: AppFilePerResource,
			FilePer:             FilePerResource,
			FluxPlacement:       FluxIntegrated,
		}, nil

	default:
		return LayoutRules{}, fmt.Errorf("unknown layout preset: %q", p)
	}
}

// KindNameManifestFileName returns file names using {kind}-{name}.yaml format,
// without the namespace prefix. This is the default for Pattern A
// (CentralizedControlPlane) where per-app artifact directories make the
// namespace prefix redundant.
func KindNameManifestFileName(_, kind, name string, mode FileExportMode) string {
	switch mode {
	case FilePerKind:
		return fmt.Sprintf("%s.yaml", kind)
	default:
		return fmt.Sprintf("%s-%s.yaml", kind, name)
	}
}

// ConfigForPreset returns a Config configured for the given preset.
// Unknown presets return an error.
func ConfigForPreset(p LayoutPreset) (Config, error) {
	switch p {
	case PresetCentralizedControlPlane:
		return Config{
			ManifestsDir:        "clusters",
			FluxDir:             "clusters",
			FilePer:             FilePerResource,
			ApplicationFileMode: AppFilePerResource,
			KustomizationMode:   KustomizationExplicit,
			ManifestFileName:    KindNameManifestFileName,
			KustomizationFileName: func(name string) string {
				return fmt.Sprintf("kustomization-%s.yaml", name)
			},
		}, nil

	case PresetSiblingControlPlane:
		return Config{
			ManifestsDir:          "clusters",
			FluxDir:               "clusters",
			FilePer:               FilePerResource,
			ApplicationFileMode:   AppFilePerResource,
			KustomizationMode:     KustomizationExplicit,
			ManifestFileName:      DefaultManifestFileName,
			KustomizationFileName: DefaultKustomizationFileName,
		}, nil

	case PresetParentDeployedControl:
		return Config{
			ManifestsDir:        "clusters",
			FluxDir:             "clusters",
			FilePer:             FilePerResource,
			ApplicationFileMode: AppFilePerResource,
			KustomizationMode:   KustomizationExplicit,
			ManifestFileName:    DefaultManifestFileName,
			KustomizationFileName: func(name string) string {
				return fmt.Sprintf("flux-ks-%s.yaml", name)
			},
		}, nil

	default:
		return Config{}, fmt.Errorf("unknown layout preset: %q", p)
	}
}
