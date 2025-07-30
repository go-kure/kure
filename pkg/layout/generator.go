package layout

import (
	"github.com/go-kure/kure/pkg/cluster"
)

// ClusterLayout groups the manifest and Flux layouts produced for a cluster.
type ClusterLayout struct {
	Manifests []*ManifestLayout
	Fluxes    []*FluxLayout
	Bootstrap *FluxLayout
}

// FluxLayoutGenerator generates a cluster layout from a configuration and LayoutConfig.
type FluxLayoutGenerator interface {
	Generate(cluster.ClusterConfig, Config) (*ClusterLayout, error)
}

// DefaultGenerator implements FluxLayoutGenerator using the current Kure behaviour.
type DefaultGenerator struct{}

// Generate builds ManifestLayout and FluxLayout trees from the given cluster configuration.
func (DefaultGenerator) Generate(cfg cluster.ClusterConfig, lc Config) (*ClusterLayout, error) {
	var manifests []*ManifestLayout
	var fluxes []*FluxLayout

	for _, group := range cfg.ApplicationGroups {
		m, f, err := NewAppGroup(group)
		if err != nil {
			return nil, err
		}
		if m.FilePer == FilePerUnset {
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
