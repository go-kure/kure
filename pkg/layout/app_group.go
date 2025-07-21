package layout

import (
	"fmt"

	"github.com/go-kure/kure/pkg/api"
)

func NewAppGroup(group api.AppGroup) (*ManifestLayout, *FluxLayout, error) {
	manifestGroup := &ManifestLayout{
		Name:      group.Name,
		Namespace: group.Namespace,
		FilePer:   group.FilePer,
	}

	fluxGroup := &FluxLayout{
		Name:      group.Name,
		DependsOn: group.FluxDependsOn,
		Manifest:  manifestGroup,
	}

	for _, appCfg := range group.Apps {
		if appCfg.Namespace == "" {
			appCfg.Namespace = group.Namespace
		}
		appCfg.FilePer = group.FilePer

		manifest, flux, err := NewAppDeployment(appCfg)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to generate app '%s': %w", appCfg.Name, err)
		}

		if flux.TargetPath == "" && flux.Manifest != nil {
			flux.TargetPath = flux.Manifest.FullRepoPath()
		}

		flux.DependsOn = append(flux.DependsOn, fluxGroup.Name)

		manifestGroup.Children = append(manifestGroup.Children, manifest)
		fluxGroup.Children = append(fluxGroup.Children, flux)
	}

	return manifestGroup, fluxGroup, nil
}

func NewAppDeployment(appCfg api.AppDeploymentConfig) (*ManifestLayout, *FluxLayout, error) {
	manifestLayout := &ManifestLayout{
		Name:      appCfg.Name,
		Namespace: appCfg.Namespace,
		FilePer:   appCfg.FilePer,
	}

	fluxLayout := &FluxLayout{
		Name:     appCfg.Name,
		Manifest: manifestLayout,
	}

	//if appCfg.Flux != nil {
	//	fluxLayout.TargetPath = appCfg.Flux.TargetPath
	//	fluxLayout.DependsOn = appCfg.Flux.DependsOn
	//	fluxLayout.Interval = appCfg.Flux.Interval
	//	fluxLayout.Prune = appCfg.Flux.Prune
	//}

	return manifestLayout, fluxLayout, nil
}
