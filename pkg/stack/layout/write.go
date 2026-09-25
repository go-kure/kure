package layout

import (
	"fmt"
	"os"
	"path/filepath"

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
// bundle but sets AppFileSingle as its own mode: that layout's files go into
// its Namespace, so no directory exists at the path its Flux Kustomization or
// ArgoCD Application names. cfg's AppFileSingle never applies to such a
// layout (see manifestAppMode).
func WriteManifest(basePath string, cfg Config, ml *ManifestLayout) error {
	if cfg.ManifestsDir == "" {
		cfg.ManifestsDir = "clusters"
	}
	if err := checkOriginFileModes(ml, cfg); err != nil {
		return err
	}
	plan := manifestPlan(basePath, cfg)
	if err := checkLayoutTree(ml, plan); err != nil {
		return err
	}
	return writeManifest(plan, cfg, ml, true)
}

// manifestAppMode is the application file mode WriteManifest writes l with.
// A layout's own mode wins. Config's ApplicationFileMode is the default for
// application layouts (and hand-built ones); a layout that renders a node or
// bundle, or that a PerLayout Kustomization targets, never takes it, because
// in AppFileSingle mode its files would go into its Namespace, not the
// directory its Flux or ArgoCD path names. That
// is what lets a profile such as ArgoProfile write single files per
// application while nodes and bundles keep their directories.
func manifestAppMode(l *ManifestLayout, cfg Config) ApplicationFileMode {
	if l.ApplicationFileMode != AppFileUnset {
		return l.ApplicationFileMode
	}
	// A layout that renders a node or bundle, or any walked layout under
	// FluxIntegratedPerLayout (every one gets a Kustomization targeting its
	// directory), must keep that directory.
	if l.hasNodeOrBundleOrigin() || l.FluxPlacement == FluxIntegratedPerLayout {
		return AppFilePerResource
	}
	return cfg.ApplicationFileMode
}

// checkOriginFileModes refuses a node- or bundle-rendering layout whose own
// ApplicationFileMode is AppFileSingle.
func checkOriginFileModes(ml *ManifestLayout, cfg Config) error {
	if ml == nil {
		return nil
	}
	if manifestAppMode(ml, cfg) == AppFileSingle && ml.hasNodeOrBundleOrigin() {
		return errors.Errorf("layout %q renders a node or bundle and cannot be written as AppFileSingle: its files would go into %q, not into the directory its Flux or ArgoCD path names", ml.FullRepoPath(), ml.Namespace)
	}
	for _, child := range ml.Children {
		if err := checkOriginFileModes(child, cfg); err != nil {
			return err
		}
	}
	return nil
}

// writeManifest writes ml and its children; every directory gets a
// kustomization.yaml (see ManifestLayout.writeToDisk), except the empty
// synthetic cluster root. root is false for a child: an AppFileSingle child
// writes its one file into its parent's directory, whose kustomization.yaml
// the parent writes and which lists that file (go-kure/kure#860).
func writeManifest(plan writerPlan, cfg Config, ml *ManifestLayout, root bool) error {
	fullPath, _ := plan.outDir(ml)
	sortedFileNames, fileGroups := plan.files(ml)
	if err := checkExtraFiles(ml, plan.outDir, sortedFileNames); err != nil {
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

	// Generate kustomization.yaml if there are resources or children, except
	// at the empty synthetic cluster root (see manifestPlan). Every directory
	// with manifests should have a kustomization.yaml for proper GitOps
	// workflow. An AppFileSingle child writes none: its file is in its
	// parent's directory, and the parent's kustomization.yaml lists it.
	if plan.writesKustomization(ml, root) {
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
		// The header is written with the first entry: a kustomization that
		// lists nothing says so with "resources: []", since kustomize refuses
		// a bare "resources:" with nothing else as an empty kustomization.
		listed := false
		entry := func(s string) {
			if !listed {
				writeStr("resources:\n")
				listed = true
			}
			writeStr(s)
		}

		for _, file := range sortedFileNames {
			entry(fmt.Sprintf("  - %s\n", file))
		}
		for _, child := range ml.Children {
			if e := plan.childEntry(ml, child); e != "" {
				entry(fmt.Sprintf("  - %s\n", e))
			}
		}
		if !listed {
			writeStr("resources: []\n")
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
		if err := writeManifest(plan, cfg, child, false); err != nil {
			return err
		}
	}

	return nil
}
