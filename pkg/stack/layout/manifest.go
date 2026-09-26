package layout

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-kure/kure/pkg/errors"
	kio "github.com/go-kure/kure/pkg/io"
)

type ManifestLayout struct {
	Name                string
	Namespace           string
	PackageRef          *schema.GroupVersionKind
	FilePer             FileExportMode
	ApplicationFileMode ApplicationFileMode
	Mode                KustomizationMode
	FluxPlacement       FluxPlacement  // Track flux placement mode for kustomization generation
	FileNaming          FileNamingMode // Controls resource file naming pattern
	Resources           []client.Object
	Children            []*ManifestLayout
	// ExtraFiles are arbitrary files written alongside resource YAMLs in this
	// layout's directory. Typical use: a values.yaml referenced by a
	// configMapGenerator entry. Augmenters (LayoutAugmenter) attach these.
	ExtraFiles []ExtraFile
	// ConfigMapGenerators emit a kustomize configMapGenerator: section in
	// kustomization.yaml. kustomize appends a content-hash suffix to each
	// generated ConfigMap name; resources referencing it (e.g.
	// HelmRelease.spec.valuesFrom) are rewritten to the suffixed name on
	// build, so any change to the source file forces re-reconciliation.
	ConfigMapGenerators []ConfigMapGeneratorSpec
	// UmbrellaChild marks this layout as rendered from a Bundle.Children
	// entry. When true, the parent's kustomization.yaml does not list it: the
	// child is applied by its own Flux Kustomization, which the layout
	// integrator places at the parent layout node (integrated placement) or
	// in flux-system (separate placement), with spec.path = this layout's
	// directory.
	UmbrellaChild bool
	// DependsOn lists sibling layout names whose Kustomization CRs must reconcile
	// before this layout's CR. In FluxIntegratedPerLayout mode the layout integrator
	// translates these into spec.dependsOn on the emitted Kustomization CR.
	// Augmenters (LayoutAugmenter) set this field; the integrator reads it.
	DependsOn []string
	// origin records the stack objects this layout renders (see origin.go).
	// Set only by the walkers and FlattenSingleTier; never serialised.
	origin origin
	// fluxBuild is set through SetFluxBuild.
	fluxBuild bool
}

// SetFluxBuild records whether a Flux Kustomization kure generated builds this
// layout's directory: its spec.path names the directory, or it is the root of
// a tree whose integration generated one (the Flux bootstrap applies the
// root). Only fluxcd's LayoutIntegrator sets it; a caller that places Flux
// Kustomizations itself, from fluxcd's GenerateFromLayout or its own code,
// marks their spec.path layouts here. For a KustomizationRecursive
// layout so marked the writers refuse a build that differs from what the
// Explicit mode would build (see checkRecursiveLayouts).
func (ml *ManifestLayout) SetFluxBuild(b bool) { ml.fluxBuild = b }

// FluxBuild reports what SetFluxBuild last recorded.
func (ml *ManifestLayout) FluxBuild() bool { return ml.fluxBuild }

// ExtraFile is an arbitrary file written into a ManifestLayout's directory
// alongside the resource YAMLs.
type ExtraFile struct {
	Name    string
	Content []byte
}

// ConfigMapGeneratorSpec describes a single kustomize configMapGenerator entry.
// Files are paths (relative to the layout directory) of files included in the
// generated ConfigMap.
type ConfigMapGeneratorSpec struct {
	Name  string
	Files []string
}

// resolveManifestFileName returns the effective ManifestFileNameFunc for this
// layout. It mirrors Config.ResolveManifestFileName but uses the layout's own
// FileNaming field.
func (ml *ManifestLayout) resolveManifestFileName() ManifestFileNameFunc {
	switch ml.FileNaming {
	case FileNamingKindName:
		return KindNameManifestFileName
	default:
		return DefaultManifestFileName
	}
}

// FullRepoPath returns the layout's directory: Namespace joined with Name.
// Namespace is always the parent's path ("." for the tree root); an empty
// Namespace means "cluster". A child whose Namespace already ends in its Name
// nests one level deeper (go-kure/kure#771); set Namespace to the parent path.
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

		// Create package-specific subdirectory with proper sanitization
		packageDirName := sanitizePackageKey(packageKey)
		packagePath := filepath.Join(basePath, packageDirName)

		if err := layout.WriteToDisk(packagePath); err != nil {
			return errors.Wrap(err, fmt.Sprintf("write package %s to disk", packageKey))
		}
	}
	return nil
}

// sanitizePackageKey converts package reference strings to valid directory names
func sanitizePackageKey(packageKey string) string {
	if packageKey == "default" {
		return "default"
	}

	// Convert common GroupVersionKind strings to meaningful names
	if strings.Contains(packageKey, "OCIRepository") {
		return "oci-packages"
	}
	if strings.Contains(packageKey, "GitRepository") {
		return "git-packages"
	}
	if strings.Contains(packageKey, "Bucket") {
		return "bucket-packages"
	}

	// Fallback: sanitize the full string
	sanitized := packageKey

	// Replace problematic characters with safe alternatives
	sanitized = strings.ReplaceAll(sanitized, "/", "-")
	sanitized = strings.ReplaceAll(sanitized, "\\", "-")
	sanitized = strings.ReplaceAll(sanitized, ":", "-")
	sanitized = strings.ReplaceAll(sanitized, " ", "-")
	sanitized = strings.ReplaceAll(sanitized, ",", "-")
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

// WriteToDisk writes the layout tree under basePath. It refuses a tree in
// which two layouts resolve to the same directory (see checkLayoutTree)
// before writing anything.
func (ml *ManifestLayout) WriteToDisk(basePath string) error {
	plan := diskPlan(basePath)
	if err := checkLayoutTree(ml, plan); err != nil {
		return err
	}
	return ml.writeToDisk(plan, true)
}

// writesSingleFile reports whether ml, written AppFileSingle, writes its one
// file, <Name>.yaml: only when it has a resource. Its parent's
// kustomization.yaml lists that file only then, or the entry would name a
// file that does not exist.
func (ml *ManifestLayout) writesSingleFile() bool { return len(ml.Resources) > 0 }

// writeToDisk writes ml and its children. Every layout's directory gets a
// kustomization.yaml, even when it lists nothing ("resources: []"): its parent
// lists the directory, or a Flux Kustomization's spec.path names it, and an
// empty directory does not survive a Git tree either. A KustomizationRecursive
// layout gets none: Flux generates it (go-kure/kure#868). An AppFileSingle child
// (root false) writes its one file into its Namespace, normally its parent's
// directory, and never a kustomization.yaml: the parent's lists that file
// (go-kure/kure#879), and one of the child's
// would replace it (go-kure/kure#860). An AppFileSingle root, which has no
// parent to list its file, still writes one.
func (ml *ManifestLayout) writeToDisk(plan writerPlan, root bool) error {
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

	// Generate kustomization.yaml if there are resources or children
	// Every directory with manifests should have a kustomization.yaml for proper GitOps workflow
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
			entry(fmt.Sprintf("  - %s\n", yamlString(file)))
		}
		for _, child := range ml.Children {
			if e := plan.childEntry(ml, child); e != "" {
				entry(fmt.Sprintf("  - %s\n", yamlString(e)))
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
		if err := child.writeToDisk(plan, false); err != nil {
			return err
		}
	}
	return nil
}
