package layout

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
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
}

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

// listedResourceFiles returns the resource files a layout's kustomization.yaml
// lists: every one in explicit mode or for a leaf. In recursive mode a layout
// with children lists its child references instead of its files — and under
// FluxIntegratedPerLayout those references are the Flux objects the layout
// hosts (each child's Kustomization and any Source generated for it), so the
// files holding them are listed, and no other.
func listedResourceFiles(ml *ManifestLayout, kMode KustomizationMode, sorted []string, groups map[string][]client.Object) []string {
	if kMode == KustomizationExplicit || len(ml.Children) == 0 {
		return sorted
	}
	if ml.FluxPlacement != FluxIntegratedPerLayout {
		return nil
	}
	var out []string
	for _, f := range sorted {
		for _, o := range groups[f] {
			if g := o.GetObjectKind().GroupVersionKind().Group; g == "kustomize.toolkit.fluxcd.io" || g == "source.toolkit.fluxcd.io" {
				out = append(out, f)
				break
			}
		}
	}
	return out
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
	if err := checkLayoutTree(ml, diskOutDir(basePath)); err != nil {
		return err
	}
	return ml.writeToDisk(basePath)
}

func (ml *ManifestLayout) writeToDisk(basePath string) error {
	fileMode := ml.FilePer
	if fileMode == FilePerUnset {
		fileMode = FilePerResource
	}
	appMode := ml.ApplicationFileMode
	if appMode == AppFileUnset {
		appMode = AppFilePerResource
	}

	outDir := diskOutDir(basePath)
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
			nameFn := ml.resolveManifestFileName()
			fileName = nameFn(ns, kind, name, fileMode)
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

	kMode := ml.Mode
	if kMode == KustomizationUnset {
		kMode = KustomizationExplicit
	}

	// Generate kustomization.yaml if there are resources or children
	// Every directory with manifests should have a kustomization.yaml for proper GitOps workflow
	if len(fileGroups) > 0 || len(ml.Children) > 0 {
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
			} else if ml.FluxPlacement == FluxIntegratedPerLayout {
				// FluxIntegratedPerLayout: the child is applied by the Flux
				// Kustomization the integrator placed in ml.Resources, which
				// the resource list above already names. Nothing is guessed
				// from the child's name.
				continue
			} else {
				// For package-aware layouts, use relative path
				if ml.PackageRef != nil && child.PackageRef != nil && ml.PackageRef != child.PackageRef {
					// Different packages - skip cross-package references in kustomization
					continue
				}
				writeStr(fmt.Sprintf("  - %s\n", child.Name))
			}
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
		if err := child.writeToDisk(basePath); err != nil {
			return err
		}
	}
	return nil
}
