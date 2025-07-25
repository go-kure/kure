package layout

import (
	"github.com/go-kure/kure/pkg/cluster/api"
)

// ClusterLayout groups the manifest and Flux layouts produced for a cluster.
type ClusterLayout struct {
	Manifests []*ManifestLayout
	Fluxes    []*FluxLayout
	Bootstrap *FluxLayout
}

// FluxLayoutGenerator generates a cluster layout from a configuration and LayoutConfig.
type FluxLayoutGenerator interface {
	Generate(api.ClusterConfig, LayoutConfig) (*ClusterLayout, error)
}

// DefaultGenerator implements FluxLayoutGenerator using the current Kure behaviour.
type DefaultGenerator struct{}

// Generate builds ManifestLayout and FluxLayout trees from the given cluster configuration.
func (DefaultGenerator) Generate(cfg api.ClusterConfig, lc LayoutConfig) (*ClusterLayout, error) {
	var manifests []*ManifestLayout
	var fluxes []*FluxLayout

	for _, group := range cfg.AppGroups {
		m, f, err := NewAppGroup(group)
		if err != nil {
			return nil, err
		}
		if m.FilePer == api.FilePerUnset {
			m.FilePer = lc.FilePer
		}
		manifests = append(manifests, m)
		fluxes = append(fluxes, f)
	}

	bs, err := NewFluxBootstrap(cfg.Name, cfg.SourceRef, cfg.Interval, "flux-system")
	if err != nil {
		return nil, err
	}

	return &ClusterLayout{Manifests: manifests, Fluxes: fluxes, Bootstrap: bs}, nil
}
