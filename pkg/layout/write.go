package layout

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// WriteManifest writes a ManifestLayout to disk using the provided configuration.
func WriteManifest(basePath string, cfg Config, ml *ManifestLayout) error {
	if cfg.ManifestFileName == nil {
		cfg.ManifestFileName = DefaultManifestFileName
	}
	if cfg.ManifestsDir == "" {
		cfg.ManifestsDir = "clusters"
	}
	mode := ml.FilePer
	if mode == FilePerUnset {
		mode = cfg.FilePer
	}

	fullPath := filepath.Join(basePath, cfg.ManifestsDir, ml.FullRepoPath())
	if err := os.MkdirAll(fullPath, 0755); err != nil {
		return fmt.Errorf("create dir: %w", err)
	}

	fileGroups := map[string][]client.Object{}
	for _, obj := range ml.Resources {
		ns := obj.GetNamespace()
		if ns == "" {
			ns = "cluster"
		}
		kind := strings.ToLower(obj.GetObjectKind().GroupVersionKind().Kind)
		name := obj.GetName()
		fileName := cfg.ManifestFileName(ns, kind, name, mode)
		fileGroups[fileName] = append(fileGroups[fileName], obj)
	}

	for fileName, objs := range fileGroups {
		f, err := os.Create(filepath.Join(fullPath, fileName))
		if err != nil {
			return err
		}
		for _, obj := range objs {
			data, err := yaml.Marshal(obj)
			if err != nil {
				_ = f.Close()
				return err
			}
			if _, err = f.Write(data); err != nil {
				_ = f.Close()
				return err
			}
			_, _ = f.Write([]byte("---"))
		}
		if err := f.Close(); err != nil {
			return err
		}
	}

	kustomPath := filepath.Join(fullPath, "kustomization.yaml")
	kf, err := os.Create(kustomPath)
	if err != nil {
		return err
	}
	_, _ = kf.WriteString("resources: ")
	for file := range fileGroups {
		_, _ = kf.WriteString(fmt.Sprintf("  - %s ", file))
	}

	for _, child := range ml.Children {
		_, _ = kf.WriteString(fmt.Sprintf("  - ../%s ", child.Name))
	}

	for _, child := range ml.Children {
		if err := WriteManifest(basePath, cfg, child); err != nil {
			_ = kf.Close()
			return err
		}
	}

	return kf.Close()
}

// WriteFlux writes a FluxLayout to disk using the provided configuration.
func WriteFlux(basePath string, cfg Config, fl *FluxLayout) error {
	if cfg.KustomizationFileName == nil {
		cfg.KustomizationFileName = DefaultKustomizationFileName
	}
	if cfg.FluxDir == "" {
		cfg.FluxDir = "clusters"
	}

	if fl.TargetPath == "" && fl.Manifest != nil {
		fl.TargetPath = fl.Manifest.FullRepoPath()
	}

	dir := filepath.Join(basePath, cfg.FluxDir, fl.TargetPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	fileName := cfg.KustomizationFileName(fl.Name)
	fullPath := filepath.Join(dir, fileName)

	interval := fl.Interval
	if interval == "" {
		interval = "5m"
	}
	source := fl.SourceRef
	if source == "" {
		source = "flux-system"
	}

	pathSpec := "./" + filepath.ToSlash(fl.TargetPath)

	kustom := map[string]interface{}{
		"apiVersion": "kustomize.toolkit.fluxcd.io/v1",
		"kind":       "Kustomization",
		"metadata": map[string]string{
			"name":      fl.Name,
			"namespace": "flux-system",
		},
		"spec": map[string]interface{}{
			"interval": interval,
			"path":     pathSpec,
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
