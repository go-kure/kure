package layout

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ManifestLayout struct {
	Name                string
	Namespace           string
	FilePer             FileExportMode
	ApplicationFileMode ApplicationFileMode
	Resources           []client.Object
	Children            []*ManifestLayout
}

func (ml *ManifestLayout) FullRepoPath() string {
	ns := ml.Namespace
	if ns == "" {
		ns = "cluster"
	}
	return filepath.ToSlash(filepath.Join(ns, ml.Name))
}

func (ml *ManifestLayout) WriteToDisk(basePath string) error {
	fullPath := filepath.Join(basePath, ml.FullRepoPath())
	if err := os.MkdirAll(fullPath, 0755); err != nil {
		return fmt.Errorf("create dir: %w", err)
	}

	mode := ml.FilePer
	if mode == FilePerUnset {
		mode = FilePerResource
	}
	appMode := ml.ApplicationFileMode
	if appMode == AppFileUnset {
		appMode = AppFilePerResource
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
			switch mode {
			case FilePerKind:
				fileName = fmt.Sprintf("%s-%s.yaml", ns, kind)
			default:
				fileName = fmt.Sprintf("%s-%s-%s.yaml", ns, kind, name)
			}
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

	// Write kustomization.yaml
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
		if err := child.WriteToDisk(basePath); err != nil {
			_ = kf.Close()
			return err
		}
	}
	return kf.Close()
}
