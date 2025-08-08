package appworkload

import (
	"sigs.k8s.io/controller-runtime/pkg/client"
	
	"github.com/go-kure/kure/internal/gvk"
	"github.com/go-kure/kure/pkg/stack"
	"github.com/go-kure/kure/pkg/stack/generators"
	"github.com/go-kure/kure/pkg/stack/generators/appworkload/internal"
)

func init() {
	// Register the AppWorkload v1alpha1 generator with both registries
	gvkObj := gvk.GVK{
		Group:   "generators.gokure.dev",
		Version: "v1alpha1",
		Kind:    "AppWorkload",
	}
	
	factory := func() stack.ApplicationConfig {
		return &ConfigV1Alpha1{}
	}
	
	// Register with generators package for backward compatibility
	generators.Register(generators.GVK(gvkObj), factory)
	
	// Register with stack package for direct usage
	stack.RegisterApplicationConfig(gvkObj, factory)
}

// ConfigV1Alpha1 describes a single deployable application with GVK support
type ConfigV1Alpha1 struct {
	generators.BaseMetadata `yaml:",inline" json:",inline"`
	
	Workload  internal.WorkloadType      `yaml:"workload,omitempty" json:"workload,omitempty"`
	Replicas  int32                      `yaml:"replicas,omitempty" json:"replicas,omitempty"`
	Labels    map[string]string          `yaml:"labels,omitempty" json:"labels,omitempty"`

	Containers           []internal.ContainerConfig     `yaml:"containers" json:"containers"`
	Volumes              []internal.Volume              `yaml:"volumes,omitempty" json:"volumes,omitempty"`
	VolumeClaimTemplates []internal.VolumeClaimTemplate `yaml:"volumeClaimTemplates,omitempty" json:"volumeClaimTemplates,omitempty"`

	Services []internal.ServiceConfig `yaml:"services,omitempty" json:"services,omitempty"`
	Ingress  *internal.IngressConfig  `yaml:"ingress,omitempty" json:"ingress,omitempty"`
}

// GetAPIVersion returns the API version for this config
func (c *ConfigV1Alpha1) GetAPIVersion() string {
	return "generators.gokure.dev/v1alpha1"
}

// GetKind returns the kind for this config
func (c *ConfigV1Alpha1) GetKind() string {
	return "AppWorkload"
}

// Generate creates Kubernetes objects from the AppWorkloadConfig
func (c *ConfigV1Alpha1) Generate(app *stack.Application) ([]*client.Object, error) {
	// Delegate to the internal implementation
	return internal.GenerateResources(&internal.Config{
		Name:                 c.Name,
		Namespace:            c.Namespace,
		Workload:             c.Workload,
		Replicas:             c.Replicas,
		Labels:               c.Labels,
		Containers:           c.Containers,
		Volumes:              c.Volumes,
		VolumeClaimTemplates: c.VolumeClaimTemplates,
		Services:             c.Services,
		Ingress:              c.Ingress,
	}, app)
}