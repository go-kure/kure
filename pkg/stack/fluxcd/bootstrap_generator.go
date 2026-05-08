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
}

// NewBootstrapGenerator creates a FluxCD bootstrap generator.
func NewBootstrapGenerator() *BootstrapGenerator {
	return &BootstrapGenerator{
		DefaultNamespace: "flux-system",
		DefaultInterval:  10 * time.Minute,
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
		mode = "flux-operator"
	}

	switch mode {
	case "flux-operator":
		return bg.generateFluxOperatorBootstrap(config, rootNode)
	case "gotk":
		return bg.generateGotkBootstrap(config, rootNode)
	default:
		return nil, errors.NewValidationError("fluxMode", config.FluxMode, "BootstrapConfig",
			[]string{"flux-operator", "gotk"})
	}
}

// SupportedBootstrapModes returns the bootstrap modes supported by this generator.
// flux-operator is the primary (recommended) mode; gotk is the legacy mode.
func (bg *BootstrapGenerator) SupportedBootstrapModes() []string {
	return []string{"flux-operator", "gotk"}
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

	// Generate source for the root node based on SourceKind
	if config.SourceURL != "" {
		source := bg.generateSource(config, rootNode)
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
// separately (see crane's bootstrap-chain design §9). Emitting the full
// set here makes the generator self-sufficient so callers can return a
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

// generateFluxSystemKustomization creates a Kustomization for the flux-system.
func (bg *BootstrapGenerator) generateFluxSystemKustomization(config *stack.BootstrapConfig, rootNode *stack.Node) client.Object {
	sourceKind := "GitRepository"
	if config.SourceKind == "OCIRepository" {
		sourceKind = "OCIRepository"
	}

	kust := &kustv1.Kustomization{
		TypeMeta: metav1.TypeMeta{
			APIVersion: kustv1.GroupVersion.String(),
			Kind:       "Kustomization",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "flux-system",
			Namespace: bg.DefaultNamespace,
		},
		Spec: kustv1.KustomizationSpec{
			Interval: metav1.Duration{Duration: bg.DefaultInterval},
			Path:     filepath.ToSlash(filepath.Join("manifests", rootNode.Name)),
			Prune:    true,
			SourceRef: kustv1.CrossNamespaceSourceReference{
				Kind: sourceKind,
				Name: "flux-system",
			},
		},
	}

	return kust
}

// generateSource creates a source resource based on SourceKind.
// When SourceKind is "GitRepository", a GitRepository is created.
// Otherwise (including when SourceKind is empty), an OCIRepository is created for backward compatibility.
func (bg *BootstrapGenerator) generateSource(config *stack.BootstrapConfig, rootNode *stack.Node) client.Object {
	if config.SourceKind == "GitRepository" {
		return bg.generateGitSource(config, rootNode)
	}
	return bg.generateOCISource(config, rootNode)
}

// generateGitSource creates a GitRepository source for bootstrap from config.
func (bg *BootstrapGenerator) generateGitSource(config *stack.BootstrapConfig, rootNode *stack.Node) client.Object {
	sourceName := "flux-system"
	if rootNode != nil && rootNode.Name != "" {
		sourceName = rootNode.Name
	}

	gr := pubfluxcd.CreateGitRepository(sourceName, bg.DefaultNamespace)
	pubfluxcd.SetGitRepositoryURL(gr, config.SourceURL)
	pubfluxcd.SetGitRepositoryInterval(gr, metav1.Duration{Duration: bg.DefaultInterval})

	if config.SourceRef != "" {
		pubfluxcd.SetGitRepositoryReference(gr, &sourcev1.GitRepositoryRef{Branch: config.SourceRef})
	}

	return gr
}

// generateOCISource creates an OCI source for bootstrap from config.
func (bg *BootstrapGenerator) generateOCISource(config *stack.BootstrapConfig, rootNode *stack.Node) client.Object {
	url := "oci://registry.example.com/flux-system"
	ref := "latest"
	sourceName := "flux-system"

	if config.SourceURL != "" {
		url = config.SourceURL
	}
	if config.SourceRef != "" {
		ref = config.SourceRef
	}
	if rootNode != nil && rootNode.Name != "" {
		sourceName = rootNode.Name
	}

	or := pubfluxcd.CreateOCIRepository(sourceName, bg.DefaultNamespace)
	pubfluxcd.SetOCIRepositoryURL(or, url)
	pubfluxcd.SetOCIRepositoryInterval(or, metav1.Duration{Duration: bg.DefaultInterval})
	pubfluxcd.SetOCIRepositoryReference(or, &sourcev1.OCIRepositoryRef{Tag: ref})

	return or
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
		path := "./"
		if rootNode != nil && rootNode.Name != "" {
			path = "./" + rootNode.Name
		}

		syncKind := "OCIRepository"
		if config.SourceKind == "GitRepository" {
			syncKind = "GitRepository"
		}

		spec.Sync = &fluxv1.Sync{
			Kind:     syncKind,
			URL:      config.SourceURL,
			Ref:      config.SourceRef,
			Path:     path,
			Interval: &metav1.Duration{Duration: bg.DefaultInterval},
		}
	}

	fi := pubfluxcd.CreateFluxInstance("flux-system", bg.DefaultNamespace)
	pubfluxcd.SetFluxInstanceSpec(fi, spec)
	return fi
}
