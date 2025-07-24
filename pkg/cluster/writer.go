package cluster

import (
	"fmt"
	"path/filepath"

	"github.com/go-kure/kure/pkg/fluxcd"
)

// WriteCluster writes the manifests and Flux layouts for the cluster to disk.
func WriteCluster(c *Cluster, manifestsBasePath, fluxBasePath string) error {
	manifests, fluxes, bootstrapFlux, err := NewClusterLayouts(c)
	if err != nil {
		return err
	}

	for _, m := range manifests {
		if err := m.WriteToDisk(filepath.Join(manifestsBasePath, "clusters", c.Name)); err != nil {
			return fmt.Errorf("write manifest: %w", err)
		}
	}

	for _, f := range fluxes {
		if err := f.WriteToDisk(filepath.Join(fluxBasePath, "clusters", c.Name)); err != nil {
			return fmt.Errorf("write flux layout: %w", err)
		}
	}

	if err := bootstrapFlux.WriteToDisk(filepath.Join(fluxBasePath, "clusters", c.Name)); err != nil {
		return fmt.Errorf("write bootstrap flux: %w", err)
	}

	if c.OCIRepo != nil {
		ocirepo := fluxcd.NewOCIRepositoryYAML(c.OCIRepo)
		ocipath := filepath.Join(fluxBasePath, "clusters", c.Name, "flux-system", "ocirepository-"+c.Name+".yaml")
		if err := fluxcd.WriteYAMLResource(ocipath, ocirepo); err != nil {
			return fmt.Errorf("write OCI repo: %w", err)
		}
	}

	return nil
}
