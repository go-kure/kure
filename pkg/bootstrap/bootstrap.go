package bootstrap

import (
	"github.com/go-kure/kure/pkg/layout"
)

func NewFluxBootstrap(clusterName, sourceRef, interval, targetPath string) (*layout.FluxLayout, error) {
	return &layout.FluxLayout{
		Name:       "flux-system",
		DependsOn:  nil,
		TargetPath: targetPath,
		Manifest:   nil,
	}, nil
}
