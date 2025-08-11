package internal

import (
	"fmt"
	"time"

	helmv2 "github.com/fluxcd/helm-controller/api/v2"
	"github.com/fluxcd/pkg/apis/kustomize"
	"github.com/fluxcd/pkg/apis/meta"
	sourcev1 "github.com/fluxcd/source-controller/api/v1"
	"gopkg.in/yaml.v3"
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

	// Chart configuration
	Chart   ChartConfig `yaml:"chart" json:"chart"`
	Version string      `yaml:"version,omitempty" json:"version,omitempty"`
	Values  interface{} `yaml:"values,omitempty" json:"values,omitempty"`

	// Source configuration
	Source SourceConfig `yaml:"source" json:"source"`

	// Release configuration
	Release ReleaseConfig `yaml:"release,omitempty" json:"release,omitempty"`

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

// GenerateResources creates Flux HelmRelease and source resources
func GenerateResources(c *Config) ([]*client.Object, error) {
	var objects []*client.Object

	// Generate the source resource
	source, err := c.generateSource()
	if err != nil {
		return nil, fmt.Errorf("failed to generate source: %w", err)
	}
	if source != nil {
		objects = append(objects, source)
	}

	// Generate the HelmRelease
	release, err := c.generateHelmRelease()
	if err != nil {
		return nil, fmt.Errorf("failed to generate HelmRelease: %w", err)
	}
	objects = append(objects, &release)

	return objects, nil
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
			Chart: &helmv2.HelmChartTemplate{
				Spec: helmv2.HelmChartTemplateSpec{
					Chart:   c.Chart.Name,
					Version: c.Chart.Version,
					SourceRef: helmv2.CrossNamespaceObjectReference{
						Name: c.Name + "-source",
					},
				},
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

	// Set values if provided
	if c.Values != nil {
		// Convert values to JSON
		valuesJSON := &apiextensionsv1.JSON{}
		valuesJSON.Raw, _ = yaml.Marshal(c.Values)
		hr.Spec.Values = valuesJSON
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
	}

	hr.Spec.Upgrade = &helmv2.Upgrade{
		DisableWait:              c.Release.DisableWait,
		DisableWaitForJobs:       c.Release.DisableWaitForJobs,
		DisableHooks:             c.Release.DisableHooks,
		DisableOpenAPIValidation: c.Release.DisableOpenAPIValidation,
		Force:                    c.Release.ForceUpgrade,
		PreserveValues:           c.Release.PreserveValues,
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
