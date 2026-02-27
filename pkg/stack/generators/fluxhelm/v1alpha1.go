package fluxhelm

import (
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-kure/kure/internal/gvk"
	"github.com/go-kure/kure/pkg/stack"
	"github.com/go-kure/kure/pkg/stack/generators"
	"github.com/go-kure/kure/pkg/stack/generators/fluxhelm/internal"
)

func init() {
	// Register the FluxHelm v1alpha1 generator
	stack.RegisterApplicationConfig(gvk.GVK{
		Group:   "generators.gokure.dev",
		Version: "v1alpha1",
		Kind:    "FluxHelm",
	}, func() stack.ApplicationConfig {
		return &ConfigV1Alpha1{}
	})
}

// ConfigV1Alpha1 generates Flux HelmRelease and source resources
type ConfigV1Alpha1 struct {
	generators.BaseMetadata `yaml:",inline" json:",inline"`

	// HelmRelease metadata overrides
	TargetNamespace string `yaml:"targetNamespace,omitempty" json:"targetNamespace,omitempty"`
	ReleaseName     string `yaml:"releaseName,omitempty" json:"releaseName,omitempty"`

	// Chart configuration
	Chart      internal.ChartConfig       `yaml:"chart" json:"chart"`
	Version    string                     `yaml:"version,omitempty" json:"version,omitempty"`
	Values     interface{}                `yaml:"values,omitempty" json:"values,omitempty"`
	ValuesFrom []internal.ValuesReference `yaml:"valuesFrom,omitempty" json:"valuesFrom,omitempty"`

	// Source configuration
	Source internal.SourceConfig `yaml:"source" json:"source"`

	// Release configuration
	Release internal.ReleaseConfig `yaml:"release,omitempty" json:"release,omitempty"`

	// ChartRef references an existing OCIRepository or HelmChart (mutually exclusive with Chart)
	ChartRef *internal.ChartRefConfig `yaml:"chartRef,omitempty" json:"chartRef,omitempty"`

	// Advanced options
	Interval       string                  `yaml:"interval,omitempty" json:"interval,omitempty"`
	Timeout        string                  `yaml:"timeout,omitempty" json:"timeout,omitempty"`
	MaxHistory     int                     `yaml:"maxHistory,omitempty" json:"maxHistory,omitempty"`
	ServiceAccount string                  `yaml:"serviceAccount,omitempty" json:"serviceAccount,omitempty"`
	Suspend        bool                    `yaml:"suspend,omitempty" json:"suspend,omitempty"`
	DependsOn      []string                `yaml:"dependsOn,omitempty" json:"dependsOn,omitempty"`
	PostRenderers  []internal.PostRenderer `yaml:"postRenderers,omitempty" json:"postRenderers,omitempty"`
}

// GetAPIVersion returns the API version for this config
func (c *ConfigV1Alpha1) GetAPIVersion() string {
	return "generators.gokure.dev/v1alpha1"
}

// GetKind returns the kind for this config
func (c *ConfigV1Alpha1) GetKind() string {
	return "FluxHelm"
}

// Generate creates Flux HelmRelease and source resources
func (c *ConfigV1Alpha1) Generate(app *stack.Application) ([]*client.Object, error) {
	// Delegate to the internal implementation
	return internal.GenerateResources(&internal.Config{
		Name:            c.Name,
		Namespace:       c.Namespace,
		TargetNamespace: c.TargetNamespace,
		ReleaseName:     c.ReleaseName,
		Chart:           c.Chart,
		Version:         c.Version,
		Values:          c.Values,
		ValuesFrom:      c.ValuesFrom,
		Source:          c.Source,
		Release:         c.Release,
		ChartRef:        c.ChartRef,
		Interval:        c.Interval,
		Timeout:         c.Timeout,
		MaxHistory:      c.MaxHistory,
		ServiceAccount:  c.ServiceAccount,
		Suspend:         c.Suspend,
		DependsOn:       c.DependsOn,
		PostRenderers:   c.PostRenderers,
	})
}
