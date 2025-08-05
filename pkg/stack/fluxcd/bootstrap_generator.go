package fluxcd

import (
	"fmt"
	"path/filepath"
	"time"

	fluxv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
	"github.com/fluxcd/flux2/v2/pkg/manifestgen/install"
	kustv1 "github.com/fluxcd/kustomize-controller/api/v1"
	sourcev1beta2 "github.com/fluxcd/source-controller/api/v1beta2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	intfluxcd "github.com/go-kure/kure/internal/fluxcd"
	kio "github.com/go-kure/kure/pkg/io"
	"github.com/go-kure/kure/pkg/errors"
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

// CreateBootstrapGenerator creates a FluxCD bootstrap generator.
func CreateBootstrapGenerator() *BootstrapGenerator {
	return &BootstrapGenerator{
		DefaultNamespace: "flux-system",
		DefaultInterval:  10 * time.Minute,
	}
}

// GenerateBootstrap creates bootstrap resources for setting up Flux.
func (bg *BootstrapGenerator) GenerateBootstrap(config *stack.BootstrapConfig, rootNode *stack.Node) ([]client.Object, error) {
	if config == nil || !config.Enabled {
		return nil, nil
	}
	
	switch config.FluxMode {
	case "gotk":
		return bg.generateGotkBootstrap(config, rootNode)
	case "flux-operator":
		return bg.generateFluxOperatorBootstrap(config, rootNode)
	default:
		return nil, errors.NewValidationError("fluxMode", config.FluxMode, "BootstrapConfig",
			[]string{"gotk", "flux-operator"})
	}
}

// SupportedBootstrapModes returns the bootstrap modes supported by this generator.
func (bg *BootstrapGenerator) SupportedBootstrapModes() []string {
	return []string{"gotk", "flux-operator"}
}

// generateGotkBootstrap generates bootstrap resources using the standard Flux toolkit.
func (bg *BootstrapGenerator) generateGotkBootstrap(config *stack.BootstrapConfig, rootNode *stack.Node) ([]client.Object, error) {
	var resources []client.Object
	
	// Generate core Flux components
	gotkResources, err := bg.generateGotkComponents(config)
	if err != nil {
		return nil, errors.NewResourceValidationError("BootstrapConfig", "gotk", "components",
			fmt.Sprintf("failed to generate gotk components: %v", err), err)
	}
	resources = append(resources, gotkResources...)
	
	// Generate flux-system Kustomization
	fluxSystemKust := bg.generateFluxSystemKustomization(rootNode)
	resources = append(resources, fluxSystemKust)
	
	// Generate OCI source for the root node
	if config.SourceURL != "" {
		source := bg.generateOCISource(config, rootNode)
		if source != nil {
			resources = append(resources, source)
		}
	}
	
	return resources, nil
}

// generateFluxOperatorBootstrap generates bootstrap resources using the Flux Operator.
func (bg *BootstrapGenerator) generateFluxOperatorBootstrap(config *stack.BootstrapConfig, rootNode *stack.Node) ([]client.Object, error) {
	// Generate FluxInstance resource
	fluxInstance := bg.generateFluxInstance(config, rootNode)
	
	return []client.Object{fluxInstance}, nil
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
		return nil, errors.NewResourceValidationError("BootstrapConfig", "gotk", "install",
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
func (bg *BootstrapGenerator) generateFluxSystemKustomization(rootNode *stack.Node) client.Object {
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
				Kind: "GitRepository",
				Name: "flux-system",
			},
		},
	}
	
	return kust
}

// generateOCISource creates an OCI source for bootstrap from config.
func (bg *BootstrapGenerator) generateOCISource(config *stack.BootstrapConfig, rootNode *stack.Node) client.Object {
	// Default values
	url := "oci://registry.example.com/flux-system"
	ref := "latest"
	sourceName := "flux-system"
	
	// Use configuration from BootstrapConfig if available
	if config.SourceURL != "" {
		url = config.SourceURL
	}
	if config.SourceRef != "" {
		ref = config.SourceRef
	}
	
	// Create source name based on node
	if rootNode != nil && rootNode.Name != "" {
		sourceName = rootNode.Name
	}
	
	spec := sourcev1beta2.OCIRepositorySpec{
		URL:      url,
		Interval: metav1.Duration{Duration: bg.DefaultInterval},
		Reference: &sourcev1beta2.OCIRepositoryRef{
			Tag: ref,
		},
	}
	
	return intfluxcd.CreateOCIRepository(sourceName, bg.DefaultNamespace, spec)
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
		
		spec.Sync = &fluxv1.Sync{
			Kind:     "OCIRepository",
			URL:      config.SourceURL,
			Ref:      config.SourceRef,
			Path:     path,
			Interval: &metav1.Duration{Duration: bg.DefaultInterval},
		}
	}
	
	return intfluxcd.CreateFluxInstance("flux-system", bg.DefaultNamespace, spec)
}