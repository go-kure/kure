package cluster

import (
	"github.com/go-kure/kure/pkg/api"
	"github.com/go-kure/kure/pkg/bootstrap"
	"github.com/go-kure/kure/pkg/layout"
)

func NewClusterLayouts(cfg api.ClusterConfig) ([]*layout.ManifestLayout, []*layout.FluxLayout, *layout.FluxLayout, error) {
	var manifests []*layout.ManifestLayout
	var fluxes []*layout.FluxLayout

	for _, group := range cfg.AppGroups {
		manifest, flux, err := layout.NewAppGroup(group)
		if err != nil {
			return nil, nil, nil, err
		}
		manifests = append(manifests, manifest)
		fluxes = append(fluxes, flux)
	}

	bootstrapFlux, err := bootstrap.NewFluxBootstrap(cfg.Name, cfg.SourceRef, cfg.Interval, "flux-system")
	if err != nil {
		return nil, nil, nil, err
	}

	return manifests, fluxes, bootstrapFlux, nil
}
