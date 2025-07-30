package layout

import (
	"github.com/go-kure/kure/pkg/application"
)

// AddAppSet converts the provided AppSet into layout structures and appends
// them to the manifest and Flux layout slices.
func AddAppSet(manifests *[]*ManifestLayout, fluxes *[]*FluxLayout, as *application.ApplicationGroup, filePer FileExportMode) error {
	if err := as.Validate(); err != nil {
		return err
	}

	ml := &ManifestLayout{
		Name:      as.Name,
		Namespace: as.Namespace,
		FilePer:   filePer,
		Resources: as.Resources,
	}
	fl := &FluxLayout{
		Name:     as.Name,
		Manifest: ml,
	}

	*manifests = append(*manifests, ml)
	*fluxes = append(*fluxes, fl)
	return nil
}
