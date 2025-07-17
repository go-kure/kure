package cluster

import (
	"fmt"
	"path/filepath"

	"github.com/go-kure/kure/pkg/api"
	"github.com/go-kure/kure/pkg/bootstrap"
)

func WriteCluster(cfg api.ClusterConfig, manifestsBasePath, fluxBasePath string) error {
	manifests, fluxes, bootstrapFlux, err := NewClusterLayouts(cfg)
	if err != nil {
		return err
	}

	for _, m := range manifests {
		if err := m.WriteToDisk(filepath.Join(manifestsBasePath, "clusters", cfg.Name)); err != nil {
			return fmt.Errorf("write manifest: %w", err)
		}
	}

	for _, f := range fluxes {
		if err := f.WriteToDisk(filepath.Join(fluxBasePath, "clusters", cfg.Name)); err != nil {
			return fmt.Errorf("write flux layout: %w", err)
		}
	}

	if err := bootstrapFlux.WriteToDisk(filepath.Join(fluxBasePath, "clusters", cfg.Name)); err != nil {
		return fmt.Errorf("write bootstrap flux: %w", err)
	}

	if cfg.OCIRepo != nil {
		ocirepo := bootstrap.NewOCIRepositoryYAML(cfg.OCIRepo)
		ocipath := filepath.Join(fluxBasePath, "clusters", cfg.Name, "flux-system", "ocirepository-"+cfg.Name+".yaml")
		if err := bootstrap.WriteYAMLResource(ocipath, ocirepo); err != nil {
			return fmt.Errorf("write OCI repo: %w", err)
		}
	}

	return nil
}
