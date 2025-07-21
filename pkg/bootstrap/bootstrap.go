package bootstrap

import "github.com/go-kure/kure/pkg/layout"

// NewFluxBootstrap returns a FluxLayout that bootstraps a cluster's
// "flux-system" Kustomization.
func NewFluxBootstrap(clusterName, sourceRef, interval, targetPath string) (*layout.FluxLayout, error) {
	if targetPath == "" {
		targetPath = "clusters/" + clusterName
	}
	return &layout.FluxLayout{
		Name:       "flux-system",
		TargetPath: targetPath,
		Interval:   interval,
		SourceRef:  sourceRef,
	}, nil
}
