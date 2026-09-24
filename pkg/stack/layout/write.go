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

// WriteManifest writes a ManifestLayout to disk using the provided
// configuration, every directory under basePath/<cfg.ManifestsDir>: a Flux
// source for a walked layout must be rooted there, because every Flux
// Kustomization spec.path is a layout directory relative to that root. It
// refuses, before writing anything, a tree in which two layouts resolve to the
// same directory (see checkLayoutTree) and a layout that renders a node or
// bundle but would be written in AppFileSingle mode (its own mode, or cfg's
// when unset): that layout's files go into its Namespace, so no directory
// exists at the path its Flux Kustomization or ArgoCD Application names.
func WriteManifest(basePath string, cfg Config, ml *ManifestLayout) error {
	if cfg.ManifestsDir == "" {
		cfg.ManifestsDir = "clusters"
	}
	if err := checkOriginFileModes(ml, cfg); err != nil {
		return err
	}
	if err := checkLayoutTree(ml, manifestOutDir(basePath, cfg)); err != nil {
		return err
	}
	return writeManifest(basePath, cfg, ml, false)
}

// checkOriginFileModes refuses a node- or bundle-rendering layout that
// WriteManifest would write in AppFileSingle mode.
func checkOriginFileModes(ml *ManifestLayout, cfg Config) error {
	if ml == nil {
		return nil
	}
	mode := ml.ApplicationFileMode
	if mode == AppFileUnset {
		mode = cfg.ApplicationFileMode
	}
	if mode == AppFileSingle && ml.hasNodeOrBundleOrigin() {
		return errors.Errorf("layout %q renders a node or bundle and cannot be written as AppFileSingle: its files would go into %q, not into the directory its Flux or ArgoCD path names", ml.FullRepoPath(), ml.Namespace)
	}
	for _, child := range ml.Children {
		if err := checkOriginFileModes(child, cfg); err != nil {
			return err
		}
	}
	return nil
}

// fluxTarget is set for a child of a FluxIntegratedPerLayout layout: its
// directory is a Flux Kustomization's spec.path (see ManifestLayout.writeToDisk).
func writeManifest(basePath string, cfg Config, ml *ManifestLayout, fluxTarget bool) error {
	manifestFileName := cfg.ResolveManifestFileName()
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
		kMode = cfg.ResolveKustomizationMode(ml.FluxPlacement)
	}

	outDir := manifestOutDir(basePath, cfg)
	fullPath, _ := outDir(ml)

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
			fileName = manifestFileName(ns, kind, name, mode)
		}

		fileGroups[fileName] = append(fileGroups[fileName], obj)
	}

	// Sort file names for deterministic output
	sortedFileNames := make([]string, 0, len(fileGroups))
	for fileName := range fileGroups {
		sortedFileNames = append(sortedFileNames, fileName)
	}
	sort.Strings(sortedFileNames)
	if err := checkExtraFiles(ml, outDir, sortedFileNames); err != nil {
		return err
	}
	// Created only after the check, so a refused layout leaves nothing behind.
	if err := os.MkdirAll(fullPath, 0755); err != nil {
		return errors.NewFileError("create", fullPath, "directory creation failed", err)
	}

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

	if err := writeExtraFilesToDisk(fullPath, ml.ExtraFiles); err != nil {
		return err
	}

	// Skip the kustomization.yaml at the synthetic cluster root only when it
	// has no resources of its own. The synthetic root is the cluster-name
	// container created by walkClusterWithClusterName: Name="",
	// single-segment Namespace. With FlattenSingleTier the root may absorb a
	// collapsed child's Resources, in which case it does need a
	// kustomization.yaml.
	skipClusterRoot := ml.Namespace != "" &&
		strings.Count(ml.Namespace, string(filepath.Separator)) == 0 &&
		ml.Name == "" &&
		len(fileGroups) == 0 &&
		!(appMode != AppFileSingle && (fluxTarget || ml.rendersBundle()))

	// Generate kustomization.yaml if there are resources or children, except at the empty cluster root.
	// Every directory with manifests should have a kustomization.yaml for proper GitOps workflow.
	if !skipClusterRoot && (len(fileGroups) > 0 || len(ml.Children) > 0 || (appMode != AppFileSingle && (fluxTarget || ml.rendersBundle()))) {
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
		writeStr("apiVersion: kustomize.config.k8s.io/v1beta1\n")
		writeStr("kind: Kustomization\n")
		writeStr("resources:\n")

		// Every resource file in explicit mode or for a leaf; see
		// listedResourceFiles for recursive mode.
		for _, file := range listedResourceFiles(ml, kMode, sortedFileNames, fileGroups) {
			writeStr(fmt.Sprintf("  - %s\n", file))
		}

		// Add child references
		for _, child := range ml.Children {
			if child.UmbrellaChild {
				// Umbrella children are not referenced from the parent
				// kustomization.yaml's Children loop:
				//   - FluxIntegratedPerLayout: the child's Kustomization CR is
				//     already in ml.Resources (placed there by the
				//     LayoutIntegrator), so the Resources loop above
				//     emits the filename exactly once.
				//   - FluxSeparate: the child is applied by its own CR
				//     under flux-system/ with spec.path pointing directly
				//     at the child subdir, so the parent must not
				//     reference it at all.
				// The sub-layout is still walked below to write its
				// workloads + own kustomization.yaml.
				continue
			}
			if child.ApplicationFileMode == AppFileSingle {
				writeStr(fmt.Sprintf("  - %s.yaml\n", child.Name))
			} else if ml.FluxPlacement != FluxIntegratedPerLayout {
				writeStr(fmt.Sprintf("  - %s\n", child.Name))
			}
			// FluxIntegratedPerLayout: the child is applied by the Flux
			// Kustomization the integrator placed in ml.Resources, which the
			// resource list above already names. Nothing is guessed from the
			// child's name.
		}

		writeStr(renderConfigMapGeneratorBlock(ml.ConfigMapGenerators))

		if writeErr != nil {
			_ = kf.Close()
			return errors.Wrapf(writeErr, "writing kustomization.yaml at %s", kustomPath)
		}

		if err := kf.Close(); err != nil {
			return err
		}
	}

	for _, child := range ml.Children {
		if err := writeManifest(basePath, cfg, child, ml.FluxPlacement == FluxIntegratedPerLayout); err != nil {
			return err
		}
	}

	return nil
}
