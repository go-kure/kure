package v1alpha1

import (
	"fmt"

	"github.com/go-kure/kure/internal/gvk"
	"github.com/go-kure/kure/pkg/errors"
)

// BundleConfig represents a versioned bundle configuration
// +gvk:group=stack.gokure.dev
// +gvk:version=v1alpha1
// +gvk:kind=Bundle
type BundleConfig struct {
	APIVersion string           `yaml:"apiVersion" json:"apiVersion"`
	Kind       string           `yaml:"kind" json:"kind"`
	Metadata   gvk.BaseMetadata `yaml:"metadata" json:"metadata"`
	Spec       BundleSpec       `yaml:"spec" json:"spec"`
}

// BundleSpec defines the specification for a bundle
type BundleSpec struct {
	// ParentPath is the hierarchical path to the parent bundle (e.g., "cluster/infrastructure")
	// Empty for root bundles. This avoids circular references while maintaining hierarchy.
	ParentPath string `yaml:"parentPath,omitempty" json:"parentPath,omitempty"`

	// DependsOn lists other bundles this bundle depends on
	DependsOn []BundleReference `yaml:"dependsOn,omitempty" json:"dependsOn,omitempty"`

	// Interval controls how often Flux reconciles the bundle
	Interval string `yaml:"interval,omitempty" json:"interval,omitempty"`

	// SourceRef specifies the source for the bundle
	SourceRef *SourceRef `yaml:"sourceRef,omitempty" json:"sourceRef,omitempty"`

	// Applications holds the application configurations that belong to the bundle
	Applications []ApplicationReference `yaml:"applications,omitempty" json:"applications,omitempty"`

	// Description provides a human-readable description of the bundle
	Description string `yaml:"description,omitempty" json:"description,omitempty"`

	// Labels are common labels that should be applied to each resource
	Labels map[string]string `yaml:"labels,omitempty" json:"labels,omitempty"`

	// Annotations are common annotations that should be applied to each resource
	Annotations map[string]string `yaml:"annotations,omitempty" json:"annotations,omitempty"`

	// Prune specifies whether to prune resources when they are removed from the bundle
	Prune bool `yaml:"prune,omitempty" json:"prune,omitempty"`

	// Wait specifies whether to wait for resources to be ready before considering the bundle reconciled
	Wait bool `yaml:"wait,omitempty" json:"wait,omitempty"`

	// Timeout is the maximum time to wait for resources to be ready
	Timeout string `yaml:"timeout,omitempty" json:"timeout,omitempty"`

	// RetryInterval is the interval to retry failed reconciliations
	RetryInterval string `yaml:"retryInterval,omitempty" json:"retryInterval,omitempty"`
}

// SourceRef defines a reference to a Flux source
type SourceRef struct {
	// Kind of the source (e.g., GitRepository, OCIRepository, Bucket)
	Kind string `yaml:"kind" json:"kind"`

	// Name of the source
	Name string `yaml:"name" json:"name"`

	// Namespace of the source
	Namespace string `yaml:"namespace,omitempty" json:"namespace,omitempty"`

	// APIVersion of the source (for future cross-version references)
	APIVersion string `yaml:"apiVersion,omitempty" json:"apiVersion,omitempty"`
}

// ApplicationReference references an Application configuration
type ApplicationReference struct {
	// Name of the application
	Name string `yaml:"name" json:"name"`

	// APIVersion of the referenced application
	APIVersion string `yaml:"apiVersion,omitempty" json:"apiVersion,omitempty"`

	// Kind of the referenced application (for supporting different app types)
	Kind string `yaml:"kind,omitempty" json:"kind,omitempty"`
}

// GetAPIVersion returns the API version of the bundle config
func (b *BundleConfig) GetAPIVersion() string {
	if b.APIVersion == "" {
		return "stack.gokure.dev/v1alpha1"
	}
	return b.APIVersion
}

// GetKind returns the kind of the bundle config
func (b *BundleConfig) GetKind() string {
	if b.Kind == "" {
		return "Bundle"
	}
	return b.Kind
}

// GetName returns the name of the bundle
func (b *BundleConfig) GetName() string {
	return b.Metadata.Name
}

// SetName sets the name of the bundle
func (b *BundleConfig) SetName(name string) {
	b.Metadata.Name = name
}

// GetNamespace returns the namespace of the bundle
func (b *BundleConfig) GetNamespace() string {
	return b.Metadata.Namespace
}

// SetNamespace sets the namespace of the bundle
func (b *BundleConfig) SetNamespace(namespace string) {
	b.Metadata.Namespace = namespace
}

// GetPath returns the full hierarchical path of this bundle
func (b *BundleConfig) GetPath() string {
	if b.Spec.ParentPath == "" {
		return b.Metadata.Name
	}
	return b.Spec.ParentPath + "/" + b.Metadata.Name
}

// Validate performs validation on the bundle configuration
func (b *BundleConfig) Validate() error {
	if b == nil {
		return errors.ErrNilBundle
	}

	if b.Metadata.Name == "" {
		return errors.NewValidationError("metadata.name", "", "Bundle", nil)
	}

	// Validate interval format if specified
	if b.Spec.Interval != "" {
		// TODO: Add interval format validation (e.g., "5m", "1h")
	}

	// Validate source ref if present
	if b.Spec.SourceRef != nil {
		if b.Spec.SourceRef.Kind == "" {
			return errors.ResourceValidationError("Bundle", b.Metadata.Name, "spec.sourceRef.kind",
				"sourceRef kind cannot be empty", nil)
		}
		if b.Spec.SourceRef.Name == "" {
			return errors.ResourceValidationError("Bundle", b.Metadata.Name, "spec.sourceRef.name",
				"sourceRef name cannot be empty", nil)
		}
	}

	// Check for duplicate applications
	appNames := make(map[string]bool)
	for i, app := range b.Spec.Applications {
		if app.Name == "" {
			return errors.ResourceValidationError("Bundle", b.Metadata.Name, "spec.applications",
				fmt.Sprintf("application at index %d has empty name", i), nil)
		}
		key := fmt.Sprintf("%s:%s:%s", app.APIVersion, app.Kind, app.Name)
		if appNames[key] {
			return errors.ResourceValidationError("Bundle", b.Metadata.Name, "spec.applications",
				fmt.Sprintf("duplicate application reference: %s", app.Name), nil)
		}
		appNames[key] = true
	}

	// Check for circular dependencies
	depNames := make(map[string]bool)
	for _, dep := range b.Spec.DependsOn {
		if dep.Name == "" {
			return errors.ResourceValidationError("Bundle", b.Metadata.Name, "spec.dependsOn",
				"dependency name cannot be empty", nil)
		}
		if dep.Name == b.Metadata.Name {
			return errors.ResourceValidationError("Bundle", b.Metadata.Name, "spec.dependsOn",
				"bundle cannot depend on itself", nil)
		}
		if depNames[dep.Name] {
			return errors.ResourceValidationError("Bundle", b.Metadata.Name, "spec.dependsOn",
				fmt.Sprintf("duplicate dependency: %s", dep.Name), nil)
		}
		depNames[dep.Name] = true
	}

	return nil
}

// ConvertTo converts this bundle config to another version
func (b *BundleConfig) ConvertTo(version string) (interface{}, error) {
	switch version {
	case "v1alpha1":
		return b, nil
	default:
		return nil, errors.New("unsupported version: " + version)
	}
}

// ConvertFrom converts from another version to this bundle config
func (b *BundleConfig) ConvertFrom(from interface{}) error {
	switch src := from.(type) {
	case *BundleConfig:
		*b = *src
		return nil
	default:
		return errors.New("unsupported conversion source type")
	}
}

// NewBundleConfig creates a new BundleConfig with default values
func NewBundleConfig(name string) *BundleConfig {
	return &BundleConfig{
		APIVersion: "stack.gokure.dev/v1alpha1",
		Kind:       "Bundle",
		Metadata: gvk.BaseMetadata{
			Name: name,
		},
		Spec: BundleSpec{
			Interval: "5m",
			Prune:    true,
			Wait:     true,
			Timeout:  "5m",
		},
	}
}

// AddApplication adds an application reference to the bundle
func (b *BundleConfig) AddApplication(name, apiVersion, kind string) {
	b.Spec.Applications = append(b.Spec.Applications, ApplicationReference{
		Name:       name,
		APIVersion: apiVersion,
		Kind:       kind,
	})
}

// AddDependency adds a dependency to another bundle
func (b *BundleConfig) AddDependency(bundleName string) {
	b.Spec.DependsOn = append(b.Spec.DependsOn, BundleReference{
		Name:       bundleName,
		APIVersion: b.GetAPIVersion(),
	})
}

// SetSourceRef sets the source reference for the bundle
func (b *BundleConfig) SetSourceRef(kind, name, namespace string) {
	b.Spec.SourceRef = &SourceRef{
		Kind:       kind,
		Name:       name,
		Namespace:  namespace,
		APIVersion: "source.toolkit.fluxcd.io/v1",
	}
}
