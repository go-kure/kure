package layout

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ManifestLayout struct {
	Name                string
	Namespace           string
	PackageRef          *schema.GroupVersionKind
	FilePer             FileExportMode
	ApplicationFileMode ApplicationFileMode
	Mode                KustomizationMode
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

// FullRepoPathWithPackage returns the repository path including package-specific prefix
func (ml *ManifestLayout) FullRepoPathWithPackage() string {
	basePath := ml.FullRepoPath()
	if ml.PackageRef != nil {
		// Use package kind as prefix to avoid path collisions
		prefix := strings.ToLower(ml.PackageRef.Kind)
		if prefix == "ocirepository" {
			prefix = "oci"
		} else if prefix == "gitrepository" {
			prefix = "git"
		}
		return filepath.ToSlash(filepath.Join(prefix, basePath))
	}
	return basePath
}

// WritePackagesToDisk writes multiple package layouts to separate directory structures
func WritePackagesToDisk(packages map[string]*ManifestLayout, basePath string) error {
	for packageKey, layout := range packages {
		if layout == nil {
			continue
		}
		
		// Create package-specific subdirectory
		var packagePath string
		if packageKey == "default" {
			packagePath = filepath.Join(basePath, "default")
		} else {
			// Use a sanitized version of the package key for directory name
			sanitized := strings.ReplaceAll(packageKey, "/", "-")
			sanitized = strings.ReplaceAll(sanitized, ":", "-")
			packagePath = filepath.Join(basePath, sanitized)
		}
		
		if err := layout.WriteToDisk(packagePath); err != nil {
			return fmt.Errorf("write package %s to disk: %w", packageKey, err)
		}
	}
	return nil
}

func (ml *ManifestLayout) WriteToDisk(basePath string) error {
	fileMode := ml.FilePer
	if fileMode == FilePerUnset {
		fileMode = FilePerResource
	}
	appMode := ml.ApplicationFileMode
	if appMode == AppFileUnset {
		appMode = AppFilePerResource
	}

	var fullPath string
	if appMode == AppFileSingle {
		fullPath = filepath.Join(basePath, ml.Namespace)
	} else {
		fullPath = filepath.Join(basePath, ml.FullRepoPath())
	}
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
			switch fileMode {
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

	kMode := ml.Mode
	if kMode == KustomizationUnset {
		kMode = KustomizationExplicit
	}

	if (kMode == KustomizationExplicit || len(ml.Children) > 0) && !(appMode == AppFileSingle && len(ml.Children) == 0) {
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
			if child.ApplicationFileMode == AppFileSingle {
				_, _ = kf.WriteString(fmt.Sprintf("  - %s.yaml ", child.Name))
			} else {
				_, _ = kf.WriteString(fmt.Sprintf("  - ../%s ", child.Name))
			}
		}
		if err := kf.Close(); err != nil {
			return err
		}
	}

	for _, child := range ml.Children {
		if err := child.WriteToDisk(basePath); err != nil {
			return err
		}
	}
	return nil
}
