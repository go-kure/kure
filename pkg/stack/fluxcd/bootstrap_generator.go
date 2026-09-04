package fluxcd

import (
	"fmt"
	"path/filepath"
	"time"

	fluxv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
	"github.com/fluxcd/flux2/v2/pkg/manifestgen/install"
	kustv1 "github.com/fluxcd/kustomize-controller/api/v1"
	sourcev1 "github.com/fluxcd/source-controller/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-kure/kure/pkg/errors"
	kio "github.com/go-kure/kure/pkg/io"
	pubfluxcd "github.com/go-kure/kure/pkg/kubernetes/fluxcd"
	"github.com/go-kure/kure/pkg/stack"
)

// BootstrapGenerator implements the workflow.BootstrapGenerator interface for Flux.
// It handles the generation of bootstrap resources for setting up Flux.
type BootstrapGenerator struct {
	// DefaultNamespace is the namespace where bootstrap resources are created
	DefaultNamespace string
	// DefaultInterval is the default reconciliation interval
	DefaultInterval time.Duration
	// BootstrapName is the name given to the bootstrap Kustomization and to the
	// FluxInstance. Neither is derived from the root node, and BootstrapConfig
	// carries no equivalent input, so this field is the only way to override
	// [DefaultBootstrapName]. Leaving it empty means the default, not a
	// nameless object — see [BootstrapGenerator.bootstrapName].
	BootstrapName string
}

// bootstrapName is the name the bootstrap Kustomization and the FluxInstance are
// given. An empty BootstrapName resolves to [DefaultBootstrapName] rather than
// emitting an object with no metadata.name.
//
// The field is new: before it, both names were the literal that
// DefaultBootstrapName now holds, so they could not be absent. A caller that
// builds a BootstrapGenerator as a struct literal instead of through
// NewBootstrapGenerator would otherwise have started emitting invalid YAML
// merely because a field appeared. DefaultNamespace and DefaultInterval carry
// the same hazard, but they carried it before this change too and resolving
// them here would alter existing behaviour rather than preserve it.
func (bg *BootstrapGenerator) bootstrapName() string {
	if bg.BootstrapName == "" {
		return DefaultBootstrapName
	}
	return bg.BootstrapName
}

// NewBootstrapGenerator creates a FluxCD bootstrap generator seeded with the
// exported defaults ([DefaultNamespace], [DefaultInterval],
// [DefaultBootstrapName]). Assign the fields afterwards to override any of them.
func NewBootstrapGenerator() *BootstrapGenerator {
	return &BootstrapGenerator{
		DefaultNamespace: DefaultNamespace,
		DefaultInterval:  DefaultInterval,
		BootstrapName:    DefaultBootstrapName,
	}
}

// GenerateBootstrap creates bootstrap resources for setting up Flux.
// When FluxMode is empty, flux-operator is used as the default.
func (bg *BootstrapGenerator) GenerateBootstrap(config *stack.BootstrapConfig, rootNode *stack.Node) ([]client.Object, error) {
	if config == nil || !config.Enabled {
		return nil, nil
	}

	mode := config.FluxMode
	if mode == "" {
		mode = DefaultFluxMode
	}

	switch mode {
	case DefaultFluxMode:
		return bg.generateFluxOperatorBootstrap(config, rootNode)
	case ModeGotk:
		return bg.generateGotkBootstrap(config, rootNode)
	default:
		return nil, errors.NewValidationError("fluxMode", config.FluxMode, "BootstrapConfig",
			bg.SupportedBootstrapModes())
	}
}

// SupportedBootstrapModes returns the bootstrap modes supported by this generator.
// [DefaultFluxMode] is the primary (recommended) mode; [ModeGotk] is the legacy
// one. This is the single list: GenerateBootstrap's validation error reports it
// rather than restating the modes.
func (bg *BootstrapGenerator) SupportedBootstrapModes() []string {
	return []string{DefaultFluxMode, ModeGotk}
}

// generateGotkBootstrap generates bootstrap resources using the standard Flux toolkit.
func (bg *BootstrapGenerator) generateGotkBootstrap(config *stack.BootstrapConfig, rootNode *stack.Node) ([]client.Object, error) {
	var resources []client.Object

	// Generate core Flux components
	gotkResources, err := bg.generateGotkComponents(config)
	if err != nil {
		return nil, errors.ResourceValidationError("BootstrapConfig", "gotk", "components",
			fmt.Sprintf("failed to generate gotk components: %v", err), err)
	}
	resources = append(resources, gotkResources...)

	// Generate flux-system Kustomization
	fluxSystemKust := bg.generateFluxSystemKustomization(config, rootNode)
	resources = append(resources, fluxSystemKust)

	// Generate source for the root node based on SourceKind. An absent
	// SourceURL means the caller supplies the source itself, so no source is
	// emitted; it is not a request for a placeholder.
	if config.SourceURL != "" {
		source, err := bg.generateSource(config, rootNode)
		if err != nil {
			return nil, err
		}
		resources = append(resources, source)
	}

	return resources, nil
}

// generateFluxOperatorBootstrap generates bootstrap resources using the Flux Operator.
//
// Output order (also a valid apply order):
//  1. Flux Operator install bundle — Namespace, CRDs, RBAC, ServiceAccount,
//     Service, controller Deployment (from the embedded upstream install.yaml,
//     see FluxOperatorInstallObjects / FluxOperatorVersion).
//  2. FluxInstance CR — configured from BootstrapConfig.
//
// Prior to kure v0.1.0-rc.5 only the FluxInstance was emitted, which
// required every caller to provide the Flux Operator install bundle
// separately. Emitting the full set here makes the generator self-sufficient
// so callers can return a
// single apply-ready bundle.
func (bg *BootstrapGenerator) generateFluxOperatorBootstrap(config *stack.BootstrapConfig, rootNode *stack.Node) ([]client.Object, error) {
	installObjs, err := FluxOperatorInstallObjects()
	if err != nil {
		return nil, errors.ResourceValidationError("BootstrapConfig", "flux-operator", "install",
			fmt.Sprintf("failed to load vendored flux-operator install bundle: %v", err), err)
	}

	resources := make([]client.Object, 0, len(installObjs)+1)
	resources = append(resources, installObjs...)
	resources = append(resources, bg.generateFluxInstance(config, rootNode))
	return resources, nil
}

// generateGotkComponents generates the standard Flux toolkit components.
func (bg *BootstrapGenerator) generateGotkComponents(config *stack.BootstrapConfig) ([]client.Object, error) {
	// Create install options with defaults
	opts := install.MakeDefaultOptions()

	// Set version if specified
	if config.FluxVersion != "" {
		opts.Version = config.FluxVersion
	}

	// Set registry if specified
	if config.Registry != "" {
		opts.Registry = config.Registry
	}

	// Set image pull secret if specified
	if config.ImagePullSecret != "" {
		opts.ImagePullSecret = config.ImagePullSecret
	}

	// Set components if specified
	if len(config.Components) > 0 {
		opts.Components = config.Components
	}

	// Generate manifests
	content, err := install.Generate(opts, "")
	if err != nil {
		return nil, errors.ResourceValidationError("BootstrapConfig", "gotk", "install",
			fmt.Sprintf("failed to generate Flux installation manifests: %v", err), err)
	}

	// Parse the generated manifests
	objects, err := kio.ParseYAML([]byte(content.Content))
	if err != nil {
		return nil, errors.NewParseError("gotk manifests", "failed to parse generated manifests", 0, 0, err)
	}

	return objects, nil
}

// rootName returns the root node's name, or "" when there is no root node or it
// is unnamed. A nil root node is legitimate — the caller may be bootstrapping
// before any node exists — so every site that reads the name goes through here
// rather than dereferencing. The bootstrap Kustomization's spec.path previously
// dereferenced it directly, one line below a nil-guarded call, and panicked.
func rootName(rootNode *stack.Node) string {
	if rootNode == nil {
		return ""
	}
	return rootNode.Name
}

// sourceName returns the name a generated GitRepository or OCIRepository
// carries: the root node's name when it has one, [DefaultSourceName] otherwise.
// The bootstrap Kustomization's sourceRef must resolve through this same
// function: it previously hardcoded [DefaultSourceName] while the source object
// took the root node's name, so a named root node produced a Kustomization
// pointing at a source that was never emitted.
func sourceName(rootNode *stack.Node) string {
	if name := rootName(rootNode); name != "" {
		return name
	}
	return DefaultSourceName
}

// resolvedSourceKind returns the kind of source object bootstrap will emit for
// config, and therefore the kind every reference to it must name. A
// GitRepository only when SourceKind says so; an OCIRepository for every other
// value, the empty string included ([DefaultSourceKind]).
//
// This is the sourceName problem in the other field. The three sites that need
// the kind — the source object, the bootstrap Kustomization's sourceRef, and the
// FluxInstance sync block — each decided it independently, and the sourceRef
// used the opposite test ("is it OCIRepository?" rather than "is it
// GitRepository?"). The two agreed only when SourceKind named one of them
// exactly; an empty or unrecognised kind emitted an OCIRepository beneath a
// sourceRef pointing at a GitRepository of the same name, which does not exist.
func resolvedSourceKind(config *stack.BootstrapConfig) string {
	if config != nil && config.SourceKind == "GitRepository" {
		return "GitRepository"
	}
	return DefaultSourceKind
}

// generateFluxSystemKustomization creates a Kustomization for the flux-system.
func (bg *BootstrapGenerator) generateFluxSystemKustomization(config *stack.BootstrapConfig, rootNode *stack.Node) client.Object {
	kust := &kustv1.Kustomization{
		TypeMeta: metav1.TypeMeta{
			APIVersion: kustv1.GroupVersion.String(),
			Kind:       "Kustomization",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      bg.bootstrapName(),
			Namespace: bg.DefaultNamespace,
		},
		Spec: kustv1.KustomizationSpec{
			Interval: metav1.Duration{Duration: bg.DefaultInterval},
			Path:     filepath.ToSlash(filepath.Join(DefaultBootstrapPathRoot, rootName(rootNode))),
			Prune:    pruneValue(config.Prune),
			SourceRef: kustv1.CrossNamespaceSourceReference{
				Kind: resolvedSourceKind(config),
				Name: sourceName(rootNode),
			},
		},
	}

	return kust
}

// generateSource creates a source resource of the kind [resolvedSourceKind]
// names, so that the object emitted here and every reference to it elsewhere
// cannot disagree about what was created.
func (bg *BootstrapGenerator) generateSource(config *stack.BootstrapConfig, rootNode *stack.Node) (client.Object, error) {
	if resolvedSourceKind(config) == "GitRepository" {
		return bg.generateGitSource(config, rootNode)
	}
	return bg.generateOCISource(config, rootNode)
}

// generateGitSource creates a GitRepository source for bootstrap from config.
func (bg *BootstrapGenerator) generateGitSource(config *stack.BootstrapConfig, rootNode *stack.Node) (client.Object, error) {
	if config.SourceURL == "" {
		return nil, errors.ResourceValidationError("BootstrapConfig", sourceName(rootNode), "sourceURL",
			"a GitRepository source requires sourceURL; there is no default repository", nil)
	}

	gr := pubfluxcd.CreateGitRepository(sourceName(rootNode), bg.DefaultNamespace)
	gr.Spec.URL = config.SourceURL
	gr.Spec.Interval = metav1.Duration{Duration: bg.DefaultInterval}

	if config.SourceRef != "" {
		pubfluxcd.SetGitRepositoryReference(gr, &sourcev1.GitRepositoryRef{Branch: config.SourceRef})
	}

	return gr, nil
}

// generateOCISource creates an OCI source for bootstrap from config.
func (bg *BootstrapGenerator) generateOCISource(config *stack.BootstrapConfig, rootNode *stack.Node) (client.Object, error) {
	if config.SourceURL == "" {
		return nil, errors.ResourceValidationError("BootstrapConfig", sourceName(rootNode), "sourceURL",
			"an OCIRepository source requires sourceURL; there is no default registry", nil)
	}

	ref := config.SourceRef
	if ref == "" {
		ref = DefaultSourceRef
	}

	or := pubfluxcd.CreateOCIRepository(sourceName(rootNode), bg.DefaultNamespace)
	or.Spec.URL = config.SourceURL
	or.Spec.Interval = metav1.Duration{Duration: bg.DefaultInterval}
	pubfluxcd.SetOCIRepositoryReference(or, &sourcev1.OCIRepositoryRef{Tag: ref})

	return or, nil
}

// GenerateFluxInstance returns only the FluxInstance CR configured for
// the given bootstrap settings, without the full Flux Operator install bundle.
// Returns (nil, nil) when config is nil. Unlike GenerateBootstrap, this method
// does not check config.Enabled — the caller is responsible for that gate.
func (bg *BootstrapGenerator) GenerateFluxInstance(config *stack.BootstrapConfig, rootNode *stack.Node) (*fluxv1.FluxInstance, error) {
	if config == nil {
		return nil, nil
	}
	obj := bg.generateFluxInstance(config, rootNode)
	fi, ok := obj.(*fluxv1.FluxInstance)
	if !ok {
		return nil, errors.Errorf("internal error: generateFluxInstance returned unexpected type %T", obj)
	}
	return fi, nil
}

// generateFluxInstance creates a FluxInstance for flux-operator mode.
func (bg *BootstrapGenerator) generateFluxInstance(config *stack.BootstrapConfig, rootNode *stack.Node) client.Object {
	spec := fluxv1.FluxInstanceSpec{
		Distribution: fluxv1.Distribution{
			Version:  config.FluxVersion,
			Registry: config.Registry,
		},
	}

	// Add components if specified
	for _, comp := range config.Components {
		spec.Components = append(spec.Components, fluxv1.Component(comp))
	}

	// Add sync configuration if source is provided
	if config.SourceURL != "" {
		path := DefaultSyncPath
		if name := rootName(rootNode); name != "" {
			path = DefaultSyncPath + name
		}

		spec.Sync = &fluxv1.Sync{
			Kind:     resolvedSourceKind(config),
			URL:      config.SourceURL,
			Ref:      config.SourceRef,
			Path:     path,
			Interval: &metav1.Duration{Duration: bg.DefaultInterval},
		}
	}

	fi := pubfluxcd.CreateFluxInstance(bg.bootstrapName(), bg.DefaultNamespace)
	fi.Spec = spec
	return fi
}
