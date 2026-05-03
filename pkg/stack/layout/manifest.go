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
	// entry. When true, kustomization.yaml writers emit a
	// flux-system-kustomization-{Name}.yaml reference in the parent directory
	// (regardless of FluxPlacement), and the layout integrator places the
	// child's Flux Kustomization CR at the parent layout node rather than in
	// the child's own directory.
	UmbrellaChild bool
	// flattenInfo carries the redirects produced by FlattenSingleTier when
	// this layout absorbed a collapsed child. Set only on the absorbing
	// layout; never serialised. Consulted by the Flux integrator's
	// findLayoutNode fallback and by ApplyFlattenPathRewrites; remains
	// populated after rewrite so that IntegrateWithLayout can be invoked
	// multiple times on the same flattened layout without losing the alias
	// state needed by integrated placement.
	flattenInfo *flattenInfo
}

// flattenInfo records the redirects produced by a FlattenSingleTier collapse.
// Two distinct keying schemes are needed because the integrator looks up
// layouts by node paths while Flux Kustomization Spec.Path values are
// layout-tree paths (cluster-name-prefixed); a single map cannot serve both.
type flattenInfo struct {
	// nodeAliases maps node.GetPath() of the collapsed child node to the
	// absorbing layout. Used by findLayoutNode (FluxIntegrated mode only).
	nodeAliases map[string]*ManifestLayout
	// pathRewrites maps pre-collapse layout repo path → post-collapse layout
	// repo path. Used to rewrite Spec.Path strings on Flux Kustomization
	// CRs (both modes). Handles exact-match and prefix-match (path/...).
	pathRewrites map[string]string
}

// FlattenInfoNodeAlias returns the absorbing layout for the given node path
// recorded on this layout's flattenInfo, or nil if no alias matches. Exposed
// for the Flux integrator's findLayoutNode fallback.
func (ml *ManifestLayout) FlattenInfoNodeAlias(nodePath string) *ManifestLayout {
	if ml == nil || ml.flattenInfo == nil {
		return nil
	}
	return ml.flattenInfo.nodeAliases[nodePath]
}

// FlattenInfoPathRewrites returns the path-rewrite map recorded on this
// layout's flattenInfo, or nil. Exposed for ApplyFlattenPathRewrites.
func (ml *ManifestLayout) FlattenInfoPathRewrites() map[string]string {
	if ml == nil || ml.flattenInfo == nil {
		return nil
	}
	return ml.flattenInfo.pathRewrites
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

		// Add resource files if in explicit mode OR if it's a leaf directory with no children
		if kMode == KustomizationExplicit || len(ml.Children) == 0 {
			for _, file := range sortedFileNames {
				writeStr(fmt.Sprintf("  - %s\n", file))
			}
		}

		// Add child references
		for _, child := range ml.Children {
			if child.UmbrellaChild {
				// Umbrella children are not referenced from the parent
				// kustomization.yaml's Children loop:
				//   - FluxIntegrated: the child's Kustomization CR is
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
			} else if ml.FluxPlacement == FluxIntegrated {
				// FluxIntegrated: reference Flux Kustomization YAML files.
				// Use FilePerResource to force per-resource naming even when
				// the parent directory uses FilePerKind.
				nameFn := ml.resolveManifestFileName()
				fluxKustName := nameFn("flux-system", "kustomization", child.Name, FilePerResource)
				writeStr(fmt.Sprintf("  - %s\n", fluxKustName))
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
		if err := child.WriteToDisk(basePath); err != nil {
			return err
		}
	}
	return nil
}
