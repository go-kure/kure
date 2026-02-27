package v1alpha1

import (
	"fmt"
	"regexp"
	"time"

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
	// Supports Go duration format (e.g., "5m", "1h", "30s", "1h30m")
	// Valid range: 1s to 24h. Empty value uses system defaults.
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
	// Supports Go duration format (e.g., "10m", "1h", "30s")
	// Valid range: 1s to 24h. Empty value uses system defaults.
	Timeout string `yaml:"timeout,omitempty" json:"timeout,omitempty"`

	// RetryInterval is the interval to retry failed reconciliations
	// Supports Go duration format (e.g., "2m", "30s", "5m")
	// Valid range: 1s to 24h. Empty value uses system defaults.
	RetryInterval string `yaml:"retryInterval,omitempty" json:"retryInterval,omitempty"`

	// HealthChecks lists resources whose health is monitored during reconciliation.
	// When specified, the Kustomization waits for these resources to become ready.
	HealthChecks []HealthCheckReference `yaml:"healthChecks,omitempty" json:"healthChecks,omitempty"`
}

// HealthCheckReference defines a resource to be monitored for health during reconciliation.
type HealthCheckReference struct {
	// APIVersion of the resource (e.g. "apps/v1", "helm.toolkit.fluxcd.io/v2").
	APIVersion string `yaml:"apiVersion,omitempty" json:"apiVersion,omitempty"`
	// Kind of the resource (e.g. "Deployment", "HelmRelease").
	Kind string `yaml:"kind" json:"kind"`
	// Name of the resource.
	Name string `yaml:"name" json:"name"`
	// Namespace of the resource. Defaults to the Kustomization namespace.
	Namespace string `yaml:"namespace,omitempty" json:"namespace,omitempty"`
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

	// URL is the repository URL (OCI or Git). When set, the resource generator
	// creates the source CRD in addition to referencing it.
	URL string `yaml:"url,omitempty" json:"url,omitempty"`

	// Tag is the tag or semver reference for OCI/Git sources.
	Tag string `yaml:"tag,omitempty" json:"tag,omitempty"`

	// Branch is the branch reference for Git sources.
	Branch string `yaml:"branch,omitempty" json:"branch,omitempty"`
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

// validIntervalPattern defines the regex pattern for valid time intervals
// Supports formats like: 1s, 30s, 1m, 5m, 1h, 24h, 1m30s, 1h30m, etc.
var validIntervalPattern = regexp.MustCompile(`^(\d+(\.\d+)?[a-z]+)+$`)

// validateInterval checks if an interval string is valid according to Go's time.Duration format
// and GitOps best practices (minimum 1 second, maximum 24 hours)
func validateInterval(interval string) error {
	if interval == "" {
		return nil // Empty intervals are allowed
	}

	// Check basic format with regex
	if !validIntervalPattern.MatchString(interval) {
		return fmt.Errorf("invalid interval format: %q, expected format like '5m', '1h', '30s'", interval)
	}

	// Parse using Go's time.Duration to ensure validity
	duration, err := time.ParseDuration(interval)
	if err != nil {
		return fmt.Errorf("invalid interval format: %q, %w", interval, err)
	}

	// Validate range: minimum 1 second, maximum 24 hours
	const minInterval = 1 * time.Second
	const maxInterval = 24 * time.Hour

	if duration < minInterval {
		return fmt.Errorf("interval %q is too short, minimum is %v", interval, minInterval)
	}

	if duration > maxInterval {
		return fmt.Errorf("interval %q is too long, maximum is %v", interval, maxInterval)
	}

	return nil
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
	if err := validateInterval(b.Spec.Interval); err != nil {
		return errors.ResourceValidationError("Bundle", b.Metadata.Name, "spec.interval",
			err.Error(), nil)
	}

	// Validate timeout format if specified
	if err := validateInterval(b.Spec.Timeout); err != nil {
		return errors.ResourceValidationError("Bundle", b.Metadata.Name, "spec.timeout",
			err.Error(), nil)
	}

	// Validate retry interval format if specified
	if err := validateInterval(b.Spec.RetryInterval); err != nil {
		return errors.ResourceValidationError("Bundle", b.Metadata.Name, "spec.retryInterval",
			err.Error(), nil)
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
