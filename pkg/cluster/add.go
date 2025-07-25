package cluster

import (
	"github.com/go-kure/kure/pkg/cluster/api"
	"github.com/go-kure/kure/pkg/cluster/appset"
	"github.com/go-kure/kure/pkg/cluster/layout"
)

// AddAppSet converts the provided AppSet into layout structures and appends
// them to the manifest and Flux layout slices.
func AddAppSet(manifests *[]*layout.ManifestLayout, fluxes *[]*layout.FluxLayout, as *appset.AppSet, filePer api.FileExportMode) error {
	if err := as.Validate(); err != nil {
		return err
	}

	ml := &layout.ManifestLayout{
		Name:      as.Name,
		Namespace: as.Namespace,
		FilePer:   filePer,
		Resources: as.Resources,
	}
	fl := &layout.FluxLayout{
		Name:     as.Name,
		Manifest: ml,
	}

	*manifests = append(*manifests, ml)
	*fluxes = append(*fluxes, fl)
	return nil
}
