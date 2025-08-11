package v1alpha1

import (
	"github.com/go-kure/kure/internal/gvk"
	"github.com/go-kure/kure/pkg/errors"
)

// ClusterConfig represents a versioned cluster configuration
// +gvk:group=stack.gokure.dev
// +gvk:version=v1alpha1
// +gvk:kind=Cluster
type ClusterConfig struct {
	APIVersion string              `yaml:"apiVersion" json:"apiVersion"`
	Kind       string              `yaml:"kind" json:"kind"`
	Metadata   gvk.BaseMetadata    `yaml:"metadata" json:"metadata"`
	Spec       ClusterSpec         `yaml:"spec" json:"spec"`
}

// ClusterSpec defines the specification for a cluster
type ClusterSpec struct {
	// Node is the root node of the cluster configuration tree
	Node *NodeReference `yaml:"node,omitempty" json:"node,omitempty"`
	
	// GitOps defines the GitOps tool configuration for the cluster
	GitOps *GitOpsConfig `yaml:"gitops,omitempty" json:"gitops,omitempty"`
	
	// Description provides a human-readable description of the cluster
	Description string `yaml:"description,omitempty" json:"description,omitempty"`
	
	// Labels are key-value pairs for cluster metadata
	Labels map[string]string `yaml:"labels,omitempty" json:"labels,omitempty"`
}

// NodeReference references a Node configuration
type NodeReference struct {
	// Name of the node
	Name string `yaml:"name" json:"name"`
	
	// APIVersion of the referenced node (for future cross-version references)
	APIVersion string `yaml:"apiVersion,omitempty" json:"apiVersion,omitempty"`
}

// GitOpsConfig defines the GitOps tool configuration for the cluster
type GitOpsConfig struct {
	// Type specifies the GitOps tool: "flux" or "argocd"
	Type string `yaml:"type" json:"type"`
	
	// Bootstrap configuration for the GitOps tool
	Bootstrap *BootstrapConfig `yaml:"bootstrap,omitempty" json:"bootstrap,omitempty"`
}

// BootstrapConfig defines the bootstrap configuration for GitOps tools
type BootstrapConfig struct {
	// Common fields
	Enabled bool `yaml:"enabled" json:"enabled"`
	
	// Flux-specific
	FluxMode        string   `yaml:"fluxMode,omitempty" json:"fluxMode,omitempty"`           // "gitops-toolkit" or "flux-operator"
	FluxVersion     string   `yaml:"fluxVersion,omitempty" json:"fluxVersion,omitempty"`
	Components      []string `yaml:"components,omitempty" json:"components,omitempty"`
	Registry        string   `yaml:"registry,omitempty" json:"registry,omitempty"`
	ImagePullSecret string   `yaml:"imagePullSecret,omitempty" json:"imagePullSecret,omitempty"`
	
	// Source configuration
	SourceURL string `yaml:"sourceURL,omitempty" json:"sourceURL,omitempty"` // OCI/Git repository URL
	SourceRef string `yaml:"sourceRef,omitempty" json:"sourceRef,omitempty"` // Tag/branch/ref
	
	// ArgoCD-specific
	ArgoCDVersion   string `yaml:"argoCDVersion,omitempty" json:"argoCDVersion,omitempty"`
	ArgoCDNamespace string `yaml:"argoCDNamespace,omitempty" json:"argoCDNamespace,omitempty"`
}

// GetAPIVersion returns the API version of the cluster config
func (c *ClusterConfig) GetAPIVersion() string {
	if c.APIVersion == "" {
		return "stack.gokure.dev/v1alpha1"
	}
	return c.APIVersion
}

// GetKind returns the kind of the cluster config
func (c *ClusterConfig) GetKind() string {
	if c.Kind == "" {
		return "Cluster"
	}
	return c.Kind
}

// GetName returns the name of the cluster
func (c *ClusterConfig) GetName() string {
	return c.Metadata.Name
}

// SetName sets the name of the cluster
func (c *ClusterConfig) SetName(name string) {
	c.Metadata.Name = name
}

// GetNamespace returns the namespace of the cluster
func (c *ClusterConfig) GetNamespace() string {
	return c.Metadata.Namespace
}

// SetNamespace sets the namespace of the cluster
func (c *ClusterConfig) SetNamespace(namespace string) {
	c.Metadata.Namespace = namespace
}

// Validate performs validation on the cluster configuration
func (c *ClusterConfig) Validate() error {
	if c == nil {
		return errors.New("cluster config is nil")
	}
	
	if c.Metadata.Name == "" {
		return errors.NewValidationError("metadata.name", "", "Cluster", nil)
	}
	
	if c.Spec.GitOps != nil {
		if c.Spec.GitOps.Type != "flux" && c.Spec.GitOps.Type != "argocd" {
			return errors.ResourceValidationError("Cluster", c.Metadata.Name, "spec.gitops.type", 
				"must be 'flux' or 'argocd', got: " + c.Spec.GitOps.Type, nil)
		}
	}
	
	return nil
}

// ConvertTo converts this cluster config to another version
func (c *ClusterConfig) ConvertTo(version string) (interface{}, error) {
	switch version {
	case "v1alpha1":
		return c, nil
	default:
		return nil, errors.New("unsupported version: " + version)
	}
}

// ConvertFrom converts from another version to this cluster config
func (c *ClusterConfig) ConvertFrom(from interface{}) error {
	switch src := from.(type) {
	case *ClusterConfig:
		*c = *src
		return nil
	default:
		return errors.New("unsupported conversion source type")
	}
}

// NewClusterConfig creates a new ClusterConfig with default values
func NewClusterConfig(name string) *ClusterConfig {
	return &ClusterConfig{
		APIVersion: "stack.gokure.dev/v1alpha1",
		Kind:       "Cluster",
		Metadata: gvk.BaseMetadata{
			Name: name,
		},
		Spec: ClusterSpec{},
	}
}