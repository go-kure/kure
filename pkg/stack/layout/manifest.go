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
	FluxPlacement       FluxPlacement // Track flux placement mode for kustomization generation
	// FileNaming is the naming pattern of this layout's resource files under
	// WriteToDisk and WriteToTar. The writers read it from the layout itself,
	// never from its parent: the walkers set it from LayoutRules.FileNaming,
	// and give a layout an augmenter added and left unset its parent's.
	FileNaming FileNamingMode
	Resources  []client.Object
	Children   []*ManifestLayout
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
	// DependsOn lists what this layout's Flux Kustomization CR waits for.
	// Augmenters (LayoutAugmenter) set this field; on a node's own layout the
	// walker fills it from Node.NamedDependsOn.
	//
	// On an application or augmenter layout an entry is read as a layout name
	// first: that of a sibling, or of another layout below the same bundle's
	// directory. The Flux layout integrator then writes the name of that
	// layout's CR, which is not the layout's ("<unit>-<layout name>", or its
	// KustomizationName). An entry that names no such layout is written as
	// given, as a Kustomization name: the CR of a node layout that renders no
	// bundle is named "<path with / replaced by ->-node" unless the node names
	// it, and that is the name to list. On a node's own layout every entry is
	// a Kustomization name and is written as given; an entry the node lists
	// in NamedDependsOn and this field lacks (the node got it after the walk)
	// is refused by the integrator.
	//
	// The integrator writes spec.dependsOn only on the Kustomization CR it
	// generates for this layout itself, which it does only under
	// FluxIntegratedPerLayout and only for a child layout that is not an
	// umbrella child, is not AppFileSingle and renders no bundle. In every
	// other case the field is dropped without an error: under
	// FluxIntegratedPerBundle and FluxSeparate an augmenter's ordering
	// produces nothing. (What a node sets is refused there instead, on the
	// node's own fields.)
	DependsOn []string
	// KustomizationName names the Flux Kustomization the layout integrator
	// generates for this layout in FluxIntegratedPerLayout mode, instead of
	// the default: "<unit>-<layout name>" for an application or augmenter
	// layout, the "-node" name for a node's own layout. An augmenter sets it
	// on a layout it creates; the walker fills it from Node.KustomizationName
	// on a node's own layout. A name a caller sets on a walked node layout
	// wins over the node's; a node that has a name while its layout carries
	// none (the node got it after the walk) is refused by the integrator.
	// Like DependsOn it is read only where the
	// layout gets a Kustomization of its own. The name in effect, set or
	// default, must be one Flux can reconcile
	// (stack.ValidateKustomizationName): the integrator refuses any other
	// where it creates the Kustomization. It shortens a "<unit>-<Name>"
	// default over the limit, keeping the "-<Name>" tail where Name has at
	// most 54 characters, and keeping the first 54 bytes of a longer Name,
	// without the hyphens and dots that end them, before the hash (the hash
	// alone where nothing is left of them). A name set here is not
	// shortened, so one that is too long is refused.
	KustomizationName string
	// Wait, Timeout, RetryInterval, Labels and Annotations are settings of the
	// Flux Kustomization the layout integrator generates for this layout in
	// FluxIntegratedPerLayout mode: its spec.wait, spec.timeout and
	// spec.retryInterval, and the labels and annotations of the Kustomization
	// object itself. No object in the layout's directory gets them. Like
	// KustomizationName they are read only where the layout gets a
	// Kustomization of its own, and are dropped without an error anywhere
	// else.
	//
	// An application's own layout, and every layout below it (the ones its
	// LayoutAugmenter added), inherits the five from the bundle that holds the
	// application. Wait, Timeout and RetryInterval left unset here are the
	// bundle's, and set here they replace it: a Wait that points at false
	// turns an inherited wait off, while an inherited Timeout or RetryInterval
	// can be replaced but not removed. Labels and Annotations are merged with
	// the bundle's per key and the value set here wins, so a key the bundle
	// sets can be given another value but not dropped. Any other layout (a
	// node's own, one a caller added to the tree) inherits nothing: its
	// Kustomization carries what is set here.
	//
	// Timeout and RetryInterval must parse as a Go duration ("5m"). Where the
	// integrator creates the Kustomization it refuses any other value, and a
	// label or annotation the Kubernetes API does not accept.
	Wait          *bool
	Timeout       string
	RetryInterval string
	Labels        map[string]string
	Annotations   map[string]string
	// Interval, Prune, Force and Suspend are four more settings of that
	// Kustomization (go-kure/kure#1021): its spec.interval, spec.prune,
	// spec.force and spec.suspend. They are read where the five above are and
	// dropped without an error anywhere else, and they follow the same rule:
	// set here, a value replaces the one of the bundle that holds the
	// application; left unset, it is that bundle's. A Prune, Force or Suspend
	// that points at false turns an inherited true off.
	//
	// Where neither the layout nor a holding bundle sets one, Interval and
	// Prune are the generator's (ResourceGenerator.DefaultInterval and
	// ResourceGenerator.Prune), and Force and Suspend are left out. Interval
	// must parse as a Go duration and be one Flux takes; the integrator
	// refuses any other value where it creates the Kustomization.
	//
	// The patches and the postBuild substitution of that Kustomization have
	// no field here: they are the holding bundle's (Bundle.Patches,
	// Bundle.PostBuild), and a layout without a holding bundle has none.
	Interval string
	Prune    *bool
	Force    *bool
	Suspend  *bool
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
// layout so marked the writers refuse a directory that holds no file and a
// build that differs from what the Explicit mode would build (see
// checkRecursiveLayouts).
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
// generated ConfigMap. Annotations are written as the entry's
// options.annotations, which kustomize puts on the ConfigMap it generates:
// the only way to annotate an object that exists only after the build.
type ConfigMapGeneratorSpec struct {
	Name        string
	Files       []string
	Annotations map[string]string
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
// Namespace means "cluster". A child whose Namespace is already its own path
// nests one level deeper (go-kure/kure#771), and the writers refuse it where
// its parent lists it (go-kure/kure#979); set Namespace to the parent path.
func (ml *ManifestLayout) FullRepoPath() string {
	ns := ml.Namespace
	if ns == "" {
		ns = "cluster"
	}
	return filepath.ToSlash(filepath.Join(ns, ml.Name))
}

// SameDirectory reports whether the directories of ml and other, each written
// as a directory, are one directory to the writers: the FullRepoPath resolved
// under the output directory (a leading slash dropped) and cleaned, compared
// without regard to case. A Name such as "/web" or "./web" resolves to the
// directory of "web", and the writers refuse a tree in which two layouts share
// a directory. It says nothing about a layout a writer writes as a single file
// (AppFileSingle), which has no directory of its own. WriteToDisk with an
// empty base path is the one writer call it does not describe: that call has
// no output directory to resolve a rooted path under, and writes it as an
// absolute path.
func (ml *ManifestLayout) SameDirectory(other *ManifestLayout) bool {
	if ml == nil || other == nil {
		return false
	}
	return normDir(unrooted(ml.FullRepoPath())) == normDir(unrooted(other.FullRepoPath()))
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
