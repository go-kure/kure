package internal

import (
	"encoding/json"
	"fmt"
	"time"

	helmv2 "github.com/fluxcd/helm-controller/api/v2"
	"github.com/fluxcd/pkg/apis/kustomize"
	"github.com/fluxcd/pkg/apis/meta"
	sourcev1 "github.com/fluxcd/source-controller/api/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// SourceType represents the type of Helm chart source
type SourceType string

const (
	// HelmRepositorySource indicates a Helm repository source
	HelmRepositorySource SourceType = "HelmRepository"
	// GitRepositorySource indicates a Git repository source
	GitRepositorySource SourceType = "GitRepository"
	// BucketSource indicates an S3-compatible bucket source
	BucketSource SourceType = "Bucket"
	// OCIRepositorySource indicates an OCI registry source
	OCIRepositorySource SourceType = "OCIRepository"
)

// Config represents the FluxHelm generator configuration
type Config struct {
	Name      string `yaml:"name" json:"name"`
	Namespace string `yaml:"namespace,omitempty" json:"namespace,omitempty"`

	// HelmRelease metadata overrides
	TargetNamespace string `yaml:"targetNamespace,omitempty" json:"targetNamespace,omitempty"`
	ReleaseName     string `yaml:"releaseName,omitempty" json:"releaseName,omitempty"`

	// Chart configuration
	Chart      ChartConfig       `yaml:"chart" json:"chart"`
	Version    string            `yaml:"version,omitempty" json:"version,omitempty"`
	Values     interface{}       `yaml:"values,omitempty" json:"values,omitempty"`
	ValuesFrom []ValuesReference `yaml:"valuesFrom,omitempty" json:"valuesFrom,omitempty"`

	// Source configuration
	Source SourceConfig `yaml:"source" json:"source"`

	// Release configuration
	Release ReleaseConfig `yaml:"release,omitempty" json:"release,omitempty"`

	// ChartRef references an existing OCIRepository or HelmChart (mutually exclusive with Chart)
	ChartRef *ChartRefConfig `yaml:"chartRef,omitempty" json:"chartRef,omitempty"`

	// Advanced options
	Interval       string         `yaml:"interval,omitempty" json:"interval,omitempty"`
	Timeout        string         `yaml:"timeout,omitempty" json:"timeout,omitempty"`
	MaxHistory     int            `yaml:"maxHistory,omitempty" json:"maxHistory,omitempty"`
	ServiceAccount string         `yaml:"serviceAccount,omitempty" json:"serviceAccount,omitempty"`
	Suspend        bool           `yaml:"suspend,omitempty" json:"suspend,omitempty"`
	DependsOn      []string       `yaml:"dependsOn,omitempty" json:"dependsOn,omitempty"`
	PostRenderers  []PostRenderer `yaml:"postRenderers,omitempty" json:"postRenderers,omitempty"`
}

// ChartConfig defines the Helm chart to deploy
type ChartConfig struct {
	Name    string `yaml:"name" json:"name"`
	Version string `yaml:"version,omitempty" json:"version,omitempty"`
}

// SourceConfig defines where to fetch the Helm chart from
type SourceConfig struct {
	Type SourceType `yaml:"type" json:"type"`

	// HelmRepository specific
	URL string `yaml:"url,omitempty" json:"url,omitempty"`

	// GitRepository specific
	GitURL  string `yaml:"gitUrl,omitempty" json:"gitUrl,omitempty"`
	GitRef  string `yaml:"gitRef,omitempty" json:"gitRef,omitempty"`
	GitPath string `yaml:"gitPath,omitempty" json:"gitPath,omitempty"`

	// OCIRepository specific
	OCIUrl string `yaml:"ociUrl,omitempty" json:"ociUrl,omitempty"`

	// Bucket specific
	BucketName string `yaml:"bucketName,omitempty" json:"bucketName,omitempty"`
	Endpoint   string `yaml:"endpoint,omitempty" json:"endpoint,omitempty"`
	Region     string `yaml:"region,omitempty" json:"region,omitempty"`

	// Authentication
	SecretRef string `yaml:"secretRef,omitempty" json:"secretRef,omitempty"`

	// Common
	Interval string `yaml:"interval,omitempty" json:"interval,omitempty"`
}

// CRDsPolicy defines the install/upgrade approach for CRDs bundled with a Helm chart.
type CRDsPolicy string

const (
	// CRDsPolicySkip skips CRD installation and updates.
	CRDsPolicySkip CRDsPolicy = "Skip"
	// CRDsPolicyCreate creates new CRDs but does not update existing ones.
	CRDsPolicyCreate CRDsPolicy = "Create"
	// CRDsPolicyCreateReplace creates new CRDs and replaces (updates) existing ones.
	CRDsPolicyCreateReplace CRDsPolicy = "CreateReplace"
)

// ReleaseConfig defines Helm release options
type ReleaseConfig struct {
	CreateNamespace          bool `yaml:"createNamespace,omitempty" json:"createNamespace,omitempty"`
	DisableWait              bool `yaml:"disableWait,omitempty" json:"disableWait,omitempty"`
	DisableWaitForJobs       bool `yaml:"disableWaitForJobs,omitempty" json:"disableWaitForJobs,omitempty"`
	DisableHooks             bool `yaml:"disableHooks,omitempty" json:"disableHooks,omitempty"`
	DisableOpenAPIValidation bool `yaml:"disableOpenAPIValidation,omitempty" json:"disableOpenAPIValidation,omitempty"`
	ResetValues              bool `yaml:"resetValues,omitempty" json:"resetValues,omitempty"`
	ForceUpgrade             bool `yaml:"forceUpgrade,omitempty" json:"forceUpgrade,omitempty"`
	PreserveValues           bool `yaml:"preserveValues,omitempty" json:"preserveValues,omitempty"`
	CleanupOnFail            bool `yaml:"cleanupOnFail,omitempty" json:"cleanupOnFail,omitempty"`
	Replace                  bool `yaml:"replace,omitempty" json:"replace,omitempty"`

	// CRD lifecycle policies
	InstallCRDs CRDsPolicy `yaml:"installCRDs,omitempty" json:"installCRDs,omitempty"`
	UpgradeCRDs CRDsPolicy `yaml:"upgradeCRDs,omitempty" json:"upgradeCRDs,omitempty"`
}

// ValuesReference defines a reference to a resource from which Helm values are sourced.
type ValuesReference struct {
	// Kind of the values referent (ConfigMap or Secret).
	Kind string `yaml:"kind" json:"kind"`
	// Name of the values referent.
	Name string `yaml:"name" json:"name"`
	// ValuesKey is the data key where values.yaml can be found. Defaults to "values.yaml".
	ValuesKey string `yaml:"valuesKey,omitempty" json:"valuesKey,omitempty"`
	// TargetPath is the YAML dot notation path at which the value should be merged.
	TargetPath string `yaml:"targetPath,omitempty" json:"targetPath,omitempty"`
	// Optional marks the reference as optional — missing referent does not cause failure.
	Optional bool `yaml:"optional,omitempty" json:"optional,omitempty"`
}

// PostRenderer defines a post-renderer for the Helm release
type PostRenderer struct {
	Kustomize *KustomizePostRenderer `yaml:"kustomize,omitempty" json:"kustomize,omitempty"`
}

// KustomizePostRenderer applies Kustomize patches as a post-render step
type KustomizePostRenderer struct {
	Patches []KustomizePatch `yaml:"patches,omitempty" json:"patches,omitempty"`
	Images  []KustomizeImage `yaml:"images,omitempty" json:"images,omitempty"`
}

// KustomizePatch defines a Kustomize patch
type KustomizePatch struct {
	Target *kustomize.Selector `yaml:"target,omitempty" json:"target,omitempty"`
	Patch  string              `yaml:"patch" json:"patch"`
}

// KustomizeImage defines a Kustomize image substitution
type KustomizeImage struct {
	Name    string `yaml:"name" json:"name"`
	NewName string `yaml:"newName,omitempty" json:"newName,omitempty"`
	NewTag  string `yaml:"newTag,omitempty" json:"newTag,omitempty"`
}

// ChartRefConfig defines a reference to an existing OCIRepository or HelmChart resource
type ChartRefConfig struct {
	Kind      string `yaml:"kind" json:"kind"`
	Name      string `yaml:"name" json:"name"`
	Namespace string `yaml:"namespace,omitempty" json:"namespace,omitempty"`
}

// GenerateResources creates Flux HelmRelease and source resources
func GenerateResources(c *Config) ([]*client.Object, error) {
	if err := c.validateChartRef(); err != nil {
		return nil, err
	}

	var objects []*client.Object

	// Skip source generation when ChartRef is used — ChartRef references an
	// existing source, so generating a new one would create an orphan.
	if c.ChartRef == nil {
		source, err := c.generateSource()
		if err != nil {
			return nil, fmt.Errorf("failed to generate source: %w", err)
		}
		if source != nil {
			objects = append(objects, source)
		}
	}

	// Generate the HelmRelease
	release, err := c.generateHelmRelease()
	if err != nil {
		return nil, fmt.Errorf("failed to generate HelmRelease: %w", err)
	}
	objects = append(objects, &release)

	return objects, nil
}

func (c *Config) validateChartRef() error {
	if c.ChartRef == nil {
		return nil
	}
	if c.Chart.Name != "" {
		return fmt.Errorf("chartRef and chart are mutually exclusive: remove either chartRef or chart.name (%q)", c.Chart.Name)
	}
	if c.ChartRef.Name == "" {
		return fmt.Errorf("chartRef.name is required")
	}
	switch c.ChartRef.Kind {
	case "OCIRepository", "HelmChart":
	default:
		return fmt.Errorf("chartRef.kind must be OCIRepository or HelmChart, got %q", c.ChartRef.Kind)
	}
	return nil
}

// generateSource creates the appropriate source resource based on type
func (c *Config) generateSource() (*client.Object, error) {
	switch c.Source.Type {
	case HelmRepositorySource:
		return c.generateHelmRepository()
	case GitRepositorySource:
		return c.generateGitRepository()
	case OCIRepositorySource:
		return c.generateOCIRepository()
	case BucketSource:
		return c.generateBucket()
	default:
		// If no source type specified, try to infer from URL
		if c.Source.URL != "" {
			return c.generateHelmRepository()
		}
		if c.Source.OCIUrl != "" {
			return c.generateOCIRepository()
		}
		// No source needed (might be using an existing source)
		return nil, nil
	}
}

// generateHelmRepository creates a HelmRepository resource
func (c *Config) generateHelmRepository() (*client.Object, error) {
	interval := c.Source.Interval
	if interval == "" {
		interval = "10m"
	}

	duration, err := time.ParseDuration(interval)
	if err != nil {
		return nil, fmt.Errorf("invalid interval: %w", err)
	}

	repo := &sourcev1.HelmRepository{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "source.toolkit.fluxcd.io/v1",
			Kind:       "HelmRepository",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      c.Name + "-source",
			Namespace: c.Namespace,
		},
		Spec: sourcev1.HelmRepositorySpec{
			URL:      c.Source.URL,
			Interval: metav1.Duration{Duration: duration},
		},
	}

	if c.Source.SecretRef != "" {
		repo.Spec.SecretRef = &meta.LocalObjectReference{
			Name: c.Source.SecretRef,
		}
	}

	var obj client.Object = repo
	return &obj, nil
}

// generateGitRepository creates a GitRepository resource
func (c *Config) generateGitRepository() (*client.Object, error) {
	interval := c.Source.Interval
	if interval == "" {
		interval = "10m"
	}

	duration, err := time.ParseDuration(interval)
	if err != nil {
		return nil, fmt.Errorf("invalid interval: %w", err)
	}

	repo := &sourcev1.GitRepository{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "source.toolkit.fluxcd.io/v1",
			Kind:       "GitRepository",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      c.Name + "-source",
			Namespace: c.Namespace,
		},
		Spec: sourcev1.GitRepositorySpec{
			URL:      c.Source.GitURL,
			Interval: metav1.Duration{Duration: duration},
		},
	}

	if c.Source.GitRef != "" {
		repo.Spec.Reference = &sourcev1.GitRepositoryRef{
			Branch: c.Source.GitRef,
		}
	}

	if c.Source.SecretRef != "" {
		repo.Spec.SecretRef = &meta.LocalObjectReference{
			Name: c.Source.SecretRef,
		}
	}

	var obj client.Object = repo
	return &obj, nil
}

// generateOCIRepository creates an OCIRepository resource
func (c *Config) generateOCIRepository() (*client.Object, error) {
	interval := c.Source.Interval
	if interval == "" {
		interval = "10m"
	}

	duration, err := time.ParseDuration(interval)
	if err != nil {
		return nil, fmt.Errorf("invalid interval: %w", err)
	}

	repo := &sourcev1.OCIRepository{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "source.toolkit.fluxcd.io/v1beta2",
			Kind:       "OCIRepository",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      c.Name + "-source",
			Namespace: c.Namespace,
		},
		Spec: sourcev1.OCIRepositorySpec{
			URL:      c.Source.OCIUrl,
			Interval: metav1.Duration{Duration: duration},
		},
	}

	if c.Source.SecretRef != "" {
		repo.Spec.SecretRef = &meta.LocalObjectReference{
			Name: c.Source.SecretRef,
		}
	}

	var obj client.Object = repo
	return &obj, nil
}

// generateBucket creates a Bucket resource
func (c *Config) generateBucket() (*client.Object, error) {
	interval := c.Source.Interval
	if interval == "" {
		interval = "10m"
	}

	duration, err := time.ParseDuration(interval)
	if err != nil {
		return nil, fmt.Errorf("invalid interval: %w", err)
	}

	bucket := &sourcev1.Bucket{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "source.toolkit.fluxcd.io/v1beta2",
			Kind:       "Bucket",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      c.Name + "-source",
			Namespace: c.Namespace,
		},
		Spec: sourcev1.BucketSpec{
			BucketName: c.Source.BucketName,
			Endpoint:   c.Source.Endpoint,
			Region:     c.Source.Region,
			Interval:   metav1.Duration{Duration: duration},
		},
	}

	if c.Source.SecretRef != "" {
		bucket.Spec.SecretRef = &meta.LocalObjectReference{
			Name: c.Source.SecretRef,
		}
	}

	var obj client.Object = bucket
	return &obj, nil
}

// generateHelmRelease creates a HelmRelease resource
func (c *Config) generateHelmRelease() (client.Object, error) {
	interval := c.Interval
	if interval == "" {
		interval = "10m"
	}

	duration, err := time.ParseDuration(interval)
	if err != nil {
		return nil, fmt.Errorf("invalid interval: %w", err)
	}

	hr := &helmv2.HelmRelease{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "helm.toolkit.fluxcd.io/v2",
			Kind:       "HelmRelease",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      c.Name,
			Namespace: c.Namespace,
		},
		Spec: helmv2.HelmReleaseSpec{
			Interval: metav1.Duration{Duration: duration},
		},
	}

	// Set either ChartRef or Chart (mutually exclusive)
	if c.ChartRef != nil {
		hr.Spec.ChartRef = &helmv2.CrossNamespaceSourceReference{
			Kind: c.ChartRef.Kind,
			Name: c.ChartRef.Name,
		}
		if c.ChartRef.Namespace != "" {
			hr.Spec.ChartRef.Namespace = c.ChartRef.Namespace
		}
	} else {
		sourceName := c.Name + "-source"
		hr.Spec.Chart = &helmv2.HelmChartTemplate{
			Spec: helmv2.HelmChartTemplateSpec{
				Chart:   c.Chart.Name,
				Version: c.Chart.Version,
				SourceRef: helmv2.CrossNamespaceObjectReference{
					Name: sourceName,
				},
			},
		}

		// Set source reference kind based on source type
		switch c.Source.Type {
		case HelmRepositorySource:
			hr.Spec.Chart.Spec.SourceRef.Kind = "HelmRepository"
		case GitRepositorySource:
			hr.Spec.Chart.Spec.SourceRef.Kind = "GitRepository"
		case OCIRepositorySource:
			hr.Spec.Chart.Spec.SourceRef.Kind = "OCIRepository"
		case BucketSource:
			hr.Spec.Chart.Spec.SourceRef.Kind = "Bucket"
		default:
			// Default to HelmRepository if not specified
			hr.Spec.Chart.Spec.SourceRef.Kind = "HelmRepository"
		}
	}

	// Set values if provided
	if c.Values != nil {
		// Convert values to JSON for the apiextensionsv1.JSON raw field
		valuesJSON := &apiextensionsv1.JSON{}
		raw, err := json.Marshal(c.Values)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal Helm values: %w", err)
		}
		valuesJSON.Raw = raw
		hr.Spec.Values = valuesJSON
	}

	// Set valuesFrom references
	if len(c.ValuesFrom) > 0 {
		hr.Spec.ValuesFrom = make([]helmv2.ValuesReference, 0, len(c.ValuesFrom))
		for _, vf := range c.ValuesFrom {
			hr.Spec.ValuesFrom = append(hr.Spec.ValuesFrom, helmv2.ValuesReference{
				Kind:       vf.Kind,
				Name:       vf.Name,
				ValuesKey:  vf.ValuesKey,
				TargetPath: vf.TargetPath,
				Optional:   vf.Optional,
			})
		}
	}

	// Set timeout if provided
	if c.Timeout != "" {
		timeout, err := time.ParseDuration(c.Timeout)
		if err != nil {
			return nil, fmt.Errorf("invalid timeout: %w", err)
		}
		hr.Spec.Timeout = &metav1.Duration{Duration: timeout}
	}

	// Set max history
	if c.MaxHistory > 0 {
		hr.Spec.MaxHistory = &c.MaxHistory
	}

	// Set service account
	if c.ServiceAccount != "" {
		hr.Spec.ServiceAccountName = c.ServiceAccount
	}

	// Set suspend
	hr.Spec.Suspend = c.Suspend

	// Set target namespace and release name overrides
	if c.TargetNamespace != "" {
		hr.Spec.TargetNamespace = c.TargetNamespace
	}
	if c.ReleaseName != "" {
		hr.Spec.ReleaseName = c.ReleaseName
	}

	// Set release options
	if c.Release.CreateNamespace {
		hr.Spec.Install = &helmv2.Install{
			CreateNamespace: c.Release.CreateNamespace,
		}
	}

	// Set install/upgrade options
	hr.Spec.Install = &helmv2.Install{
		CreateNamespace:          c.Release.CreateNamespace,
		DisableWait:              c.Release.DisableWait,
		DisableWaitForJobs:       c.Release.DisableWaitForJobs,
		DisableHooks:             c.Release.DisableHooks,
		DisableOpenAPIValidation: c.Release.DisableOpenAPIValidation,
		Replace:                  c.Release.Replace,
		CRDs:                     helmv2.CRDsPolicy(c.Release.InstallCRDs),
	}

	hr.Spec.Upgrade = &helmv2.Upgrade{
		DisableWait:              c.Release.DisableWait,
		DisableWaitForJobs:       c.Release.DisableWaitForJobs,
		DisableHooks:             c.Release.DisableHooks,
		DisableOpenAPIValidation: c.Release.DisableOpenAPIValidation,
		Force:                    c.Release.ForceUpgrade,
		PreserveValues:           c.Release.PreserveValues,
		CRDs:                     helmv2.CRDsPolicy(c.Release.UpgradeCRDs),
		CleanupOnFail:            c.Release.CleanupOnFail,
	}

	// Set dependencies
	if len(c.DependsOn) > 0 {
		hr.Spec.DependsOn = make([]meta.NamespacedObjectReference, 0, len(c.DependsOn))
		for _, dep := range c.DependsOn {
			hr.Spec.DependsOn = append(hr.Spec.DependsOn, meta.NamespacedObjectReference{
				Name: dep,
			})
		}
	}

	// Set post-renderers
	if len(c.PostRenderers) > 0 {
		hr.Spec.PostRenderers = make([]helmv2.PostRenderer, 0, len(c.PostRenderers))
		for _, pr := range c.PostRenderers {
			if pr.Kustomize != nil {
				kustomizeObj := &helmv2.Kustomize{}

				// Add patches
				for _, patch := range pr.Kustomize.Patches {
					kustomizeObj.Patches = append(kustomizeObj.Patches, kustomize.Patch{
						Target: patch.Target,
						Patch:  patch.Patch,
					})
				}

				// Add images
				for _, img := range pr.Kustomize.Images {
					kustomizeObj.Images = append(kustomizeObj.Images, kustomize.Image{
						Name:    img.Name,
						NewName: img.NewName,
						NewTag:  img.NewTag,
					})
				}

				hr.Spec.PostRenderers = append(hr.Spec.PostRenderers, helmv2.PostRenderer{
					Kustomize: kustomizeObj,
				})
			}
		}
	}

	return hr, nil
}
