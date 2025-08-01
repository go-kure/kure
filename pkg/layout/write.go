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
	appMode := ml.ApplicationFileMode
	if appMode == AppFileUnset {
		appMode = cfg.ApplicationFileMode
	}
	kMode := ml.Mode
	if kMode == KustomizationUnset {
		kMode = cfg.KustomizationMode
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

		var fileName string
		if appMode == AppFileSingle {
			fileName = fmt.Sprintf("%s.yaml", ml.Name)
		} else {
			fileName = cfg.ManifestFileName(ns, kind, name, mode)
		}

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

	if kMode == KustomizationExplicit || len(ml.Children) > 0 {
		kustomPath := filepath.Join(fullPath, "kustomization.yaml")
		kf, err := os.Create(kustomPath)
		if err != nil {
			return err
		}
		_, _ = kf.WriteString("resources: ")
		if kMode == KustomizationExplicit {
			for file := range fileGroups {
				_, _ = kf.WriteString(fmt.Sprintf("  - %s ", file))
			}
		}
		for _, child := range ml.Children {
			_, _ = kf.WriteString(fmt.Sprintf("  - ../%s ", child.Name))
		}
		if err := kf.Close(); err != nil {
			return err
		}
	}

	for _, child := range ml.Children {
		if err := WriteManifest(basePath, cfg, child); err != nil {
			return err
		}
	}

	return nil
}
