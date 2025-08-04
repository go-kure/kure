package layout

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kio "github.com/go-kure/kure/pkg/io"
)

type ManifestLayout struct {
	Name                string
	Namespace           string
	PackageRef          *schema.GroupVersionKind
	FilePer             FileExportMode
	ApplicationFileMode ApplicationFileMode
	Mode                KustomizationMode
	FluxPlacement       FluxPlacement // Track flux placement mode for kustomization generation
	Resources           []client.Object
	Children            []*ManifestLayout
}

func (ml *ManifestLayout) FullRepoPath() string {
	ns := ml.Namespace
	if ns == "" {
		ns = "cluster"
	}
	
	// Don't duplicate the name if it's already at the end of the namespace
	if ml.Name != "" && strings.HasSuffix(ns, ml.Name) {
		return filepath.ToSlash(ns)
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
		
		// Create package-specific subdirectory with proper sanitization
		packageDirName := sanitizePackageKey(packageKey)
		packagePath := filepath.Join(basePath, packageDirName)
		
		if err := layout.WriteToDisk(packagePath); err != nil {
			return fmt.Errorf("write package %s to disk: %w", packageKey, err)
		}
	}
	return nil
}

// sanitizePackageKey converts package reference strings to valid directory names
func sanitizePackageKey(packageKey string) string {
	if packageKey == "default" {
		return "default"
	}
	
	// Replace problematic characters with safe alternatives
	sanitized := packageKey
	
	// Replace slashes and backslashes with dashes
	sanitized = strings.ReplaceAll(sanitized, "/", "-")
	sanitized = strings.ReplaceAll(sanitized, "\\", "-")
	
	// Replace colons with dashes
	sanitized = strings.ReplaceAll(sanitized, ":", "-")
	
	// Replace spaces with dashes
	sanitized = strings.ReplaceAll(sanitized, " ", "-")
	
	// Replace commas with dashes
	sanitized = strings.ReplaceAll(sanitized, ",", "-")
	
	// Remove or replace other problematic characters
	sanitized = strings.ReplaceAll(sanitized, "=", "-")
	sanitized = strings.ReplaceAll(sanitized, "&", "-")
	sanitized = strings.ReplaceAll(sanitized, "?", "-")
	sanitized = strings.ReplaceAll(sanitized, "#", "-")
	sanitized = strings.ReplaceAll(sanitized, "!", "-")
	sanitized = strings.ReplaceAll(sanitized, "@", "-")
	sanitized = strings.ReplaceAll(sanitized, "%", "-")
	sanitized = strings.ReplaceAll(sanitized, "^", "-")
	sanitized = strings.ReplaceAll(sanitized, "*", "-")
	sanitized = strings.ReplaceAll(sanitized, "(", "-")
	sanitized = strings.ReplaceAll(sanitized, ")", "-")
	sanitized = strings.ReplaceAll(sanitized, "+", "-")
	sanitized = strings.ReplaceAll(sanitized, "|", "-")
	sanitized = strings.ReplaceAll(sanitized, "[", "-")
	sanitized = strings.ReplaceAll(sanitized, "]", "-")
	sanitized = strings.ReplaceAll(sanitized, "{", "-")
	sanitized = strings.ReplaceAll(sanitized, "}", "-")
	sanitized = strings.ReplaceAll(sanitized, ";", "-")
	sanitized = strings.ReplaceAll(sanitized, "'", "-")
	sanitized = strings.ReplaceAll(sanitized, "\"", "-")
	sanitized = strings.ReplaceAll(sanitized, "<", "-")
	sanitized = strings.ReplaceAll(sanitized, ">", "-")
	sanitized = strings.ReplaceAll(sanitized, "`", "-")
	sanitized = strings.ReplaceAll(sanitized, "~", "-")
	sanitized = strings.ReplaceAll(sanitized, "$", "-")
	
	// Clean up multiple consecutive dashes
	for strings.Contains(sanitized, "--") {
		sanitized = strings.ReplaceAll(sanitized, "--", "-")
	}
	
	// Trim leading/trailing dashes
	sanitized = strings.Trim(sanitized, "-")
	
	// Ensure it's not empty and doesn't contain only special characters
	if sanitized == "" || sanitized == "-" {
		sanitized = "unknown-package"
	}
	
	return sanitized
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
		
		// Convert to []*client.Object for the kio encoder
		var objPtrs []*client.Object
		for _, obj := range objs {
			objPtr := &obj
			objPtrs = append(objPtrs, objPtr)
		}
		
		// Use proper Kubernetes YAML encoder
		data, err := kio.EncodeObjectsToYAML(objPtrs)
		if err != nil {
			_ = f.Close()
			return err
		}
		
		if _, err = f.Write(data); err != nil {
			_ = f.Close()
			return err
		}
		
		if err := f.Close(); err != nil {
			return err
		}
	}

	kMode := ml.Mode
	if kMode == KustomizationUnset {
		kMode = KustomizationExplicit
	}

	// Generate kustomization.yaml if there are resources or children
	if (kMode == KustomizationExplicit && len(fileGroups) > 0) || len(ml.Children) > 0 {
		kustomPath := filepath.Join(fullPath, "kustomization.yaml")
		kf, err := os.Create(kustomPath)
		if err != nil {
			return err
		}
		
		// Write proper YAML header
		_, _ = kf.WriteString("apiVersion: kustomize.config.k8s.io/v1beta1\n")
		_, _ = kf.WriteString("kind: Kustomization\n")
		_, _ = kf.WriteString("resources:\n")
		
		// Add resource files if in explicit mode
		if kMode == KustomizationExplicit {
			for file := range fileGroups {
				_, _ = kf.WriteString(fmt.Sprintf("  - %s\n", file))
			}
		}
		
		// Add child references
		for _, child := range ml.Children {
			if child.ApplicationFileMode == AppFileSingle {
				_, _ = kf.WriteString(fmt.Sprintf("  - %s.yaml\n", child.Name))
			} else {
				// For package-aware layouts, use relative path
				if ml.PackageRef != nil && child.PackageRef != nil && ml.PackageRef != child.PackageRef {
					// Different packages - skip cross-package references in kustomization
					continue
				}
				_, _ = kf.WriteString(fmt.Sprintf("  - %s\n", child.Name))
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
