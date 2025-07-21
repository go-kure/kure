package layout

import (
	"fmt"
	"gopkg.in/yaml.v3"
	"os"
	"path/filepath"
	"strings"
)

type FluxLayout struct {
	Name       string
	DependsOn  []string
	TargetPath string
	Manifest   *ManifestLayout
	Children   []*FluxLayout
	// Interval controls how often Flux reconciles the Kustomization.
	Interval string
	// SourceRef specifies the source reference name for the Kustomization.
	SourceRef string
}

func (fl *FluxLayout) WriteToDisk(basePath string) error {
	if fl.TargetPath == "" && fl.Manifest != nil {
		fl.TargetPath = fl.Manifest.FullRepoPath()
	}

	dir := filepath.Join(basePath, fl.TargetPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	fileName := fmt.Sprintf("kustomization-%s.yaml", fl.Name)
	fullPath := filepath.Join(dir, fileName)

	interval := fl.Interval
	if interval == "" {
		interval = "5m"
	}
	source := fl.SourceRef
	if source == "" {
		source = "flux-system"
	}

	var kustom = map[string]interface{}{
		"apiVersion": "kustomize.toolkit.fluxcd.io/v1",
		"kind":       "Kustomization",
		"metadata": map[string]string{
			"name":      fl.Name,
			"namespace": "flux-system",
		},
		"spec": map[string]interface{}{
			"interval": interval,
			"path":     "./" + strings.TrimPrefix(fl.TargetPath, basePath+"/"),
			"prune":    true,
			"sourceRef": map[string]string{
				"kind":      "OCIRepository",
				"name":      source,
				"namespace": "flux-system",
			},
		},
	}

	if len(fl.DependsOn) > 0 {
		var deps []map[string]string
		for _, d := range fl.DependsOn {
			deps = append(deps, map[string]string{"name": d})
		}
		kustom["spec"].(map[string]interface{})["dependsOn"] = deps
	}

	data, err := yaml.Marshal(kustom)
	if err != nil {
		return err
	}

	return os.WriteFile(fullPath, data, 0644)
}
