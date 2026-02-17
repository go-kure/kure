package layout

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-kure/kure/pkg/errors"
	kio "github.com/go-kure/kure/pkg/io"
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

	var fullPath string
	if appMode == AppFileSingle {
		fullPath = filepath.Join(basePath, cfg.ManifestsDir, ml.Namespace)
	} else {
		fullPath = filepath.Join(basePath, cfg.ManifestsDir, ml.FullRepoPath())
	}
	if err := os.MkdirAll(fullPath, 0755); err != nil {
		return errors.NewFileError("create", fullPath, "directory creation failed", err)
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

	// Sort file names for deterministic output
	sortedFileNames := make([]string, 0, len(fileGroups))
	for fileName := range fileGroups {
		sortedFileNames = append(sortedFileNames, fileName)
	}
	sort.Strings(sortedFileNames)

	for _, fileName := range sortedFileNames {
		objs := fileGroups[fileName]
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

	// Don't generate root kustomization.yaml at cluster level (when namespace is just the cluster name)
	isClusterRoot := strings.Count(ml.Namespace, string(filepath.Separator)) == 0 && ml.Name == ""

	// Generate kustomization.yaml if there are resources or children, but not at cluster root
	// Every directory with manifests should have a kustomization.yaml for proper GitOps workflow
	if !isClusterRoot && (len(fileGroups) > 0 || len(ml.Children) > 0) {
		kustomPath := filepath.Join(fullPath, "kustomization.yaml")
		kf, err := os.Create(kustomPath)
		if err != nil {
			return err
		}

		var writeErr error
		writeStr := func(s string) {
			if writeErr != nil {
				return
			}
			_, writeErr = kf.WriteString(s)
		}

		// Write proper YAML header
		writeStr("apiVersion: kustomize.config.kubernetes.io/v1beta1\n")
		writeStr("kind: Kustomization\n")
		writeStr("resources:\n")

		// Add resource files if in explicit mode OR if it's a leaf directory with no children
		if kMode == KustomizationExplicit || len(ml.Children) == 0 {
			for _, file := range sortedFileNames {
				writeStr(fmt.Sprintf("  - %s\n", file))
			}
		}
		// In recursive mode, only reference child directories, not files

		// Add child references
		for _, child := range ml.Children {
			if child.ApplicationFileMode == AppFileSingle {
				writeStr(fmt.Sprintf("  - %s.yaml\n", child.Name))
			} else {
				// For FluxIntegrated mode, reference Flux Kustomization YAML files instead of directories
				if ml.FluxPlacement == FluxIntegrated {
					fluxKustName := fmt.Sprintf("flux-system-kustomization-%s.yaml", child.Name)
					writeStr(fmt.Sprintf("  - %s\n", fluxKustName))
				} else {
					writeStr(fmt.Sprintf("  - %s\n", child.Name))
				}
			}
		}

		if writeErr != nil {
			_ = kf.Close()
			return errors.Wrapf(writeErr, "writing kustomization.yaml at %s", kustomPath)
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
