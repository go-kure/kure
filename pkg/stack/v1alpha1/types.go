package v1alpha1

import (
	"github.com/go-kure/kure/internal/gvk"
	"github.com/go-kure/kure/pkg/stack"
)

// API Group and Version constants
const (
	GroupName = "stack.gokure.dev"
	Version   = "v1alpha1"
)

// ClusterV1Alpha1 represents a versioned Cluster configuration
type ClusterV1Alpha1 struct {
	gvk.BaseMetadata `yaml:",inline" json:",inline"`
	
	// Spec contains the cluster specification
	Spec ClusterSpec `yaml:"spec" json:"spec"`
}

// ClusterSpec defines the specification for a cluster
type ClusterSpec struct {
	// GitOps configuration for the cluster
	GitOps *GitOpsConfig `yaml:"gitops,omitempty" json:"gitops,omitempty"`
	
	// Nodes contains the top-level nodes in the cluster
	Nodes []NodeReference `yaml:"nodes,omitempty" json:"nodes,omitempty"`
}

// GitOpsConfig defines the GitOps tool configuration
type GitOpsConfig struct {
	Type      string           `yaml:"type" json:"type"` // "flux" or "argocd"
	Bootstrap *BootstrapConfig `yaml:"bootstrap,omitempty" json:"bootstrap,omitempty"`
}

// BootstrapConfig defines the bootstrap configuration for GitOps tools
type BootstrapConfig struct {
	// Common fields
	Enabled bool `yaml:"enabled" json:"enabled"`

	// Flux-specific
	FluxMode        string   `yaml:"fluxMode,omitempty" json:"fluxMode,omitempty"`
	FluxVersion     string   `yaml:"fluxVersion,omitempty" json:"fluxVersion,omitempty"`
	Components      []string `yaml:"components,omitempty" json:"components,omitempty"`
	Registry        string   `yaml:"registry,omitempty" json:"registry,omitempty"`
	ImagePullSecret string   `yaml:"imagePullSecret,omitempty" json:"imagePullSecret,omitempty"`

	// Source configuration
	SourceURL string `yaml:"sourceURL,omitempty" json:"sourceURL,omitempty"`
	SourceRef string `yaml:"sourceRef,omitempty" json:"sourceRef,omitempty"`

	// ArgoCD-specific
	ArgoCDVersion   string `yaml:"argoCDVersion,omitempty" json:"argoCDVersion,omitempty"`
	ArgoCDNamespace string `yaml:"argoCDNamespace,omitempty" json:"argoCDNamespace,omitempty"`
}

// NodeReference references a Node by name or inline definition
type NodeReference struct {
	// Name references an external Node resource
	Name string `yaml:"name,omitempty" json:"name,omitempty"`
	
	// Inline allows embedding a Node definition
	Inline *NodeSpec `yaml:"inline,omitempty" json:"inline,omitempty"`
}

// GetAPIVersion returns the API version for ClusterV1Alpha1
func (c *ClusterV1Alpha1) GetAPIVersion() string {
	return GroupName + "/" + Version
}

// GetKind returns the kind for ClusterV1Alpha1
func (c *ClusterV1Alpha1) GetKind() string {
	return "Cluster"
}

// ToUnversioned converts to the unversioned Cluster type
func (c *ClusterV1Alpha1) ToUnversioned() *stack.Cluster {
	cluster := &stack.Cluster{
		Name:   c.Name,
		GitOps: c.Spec.GitOps.toUnversioned(),
	}
	
	// Convert nodes if present
	if len(c.Spec.Nodes) > 0 && c.Spec.Nodes[0].Inline != nil {
		cluster.Node = c.Spec.Nodes[0].Inline.toUnversioned(c.Name)
	}
	
	return cluster
}

func (g *GitOpsConfig) toUnversioned() *stack.GitOpsConfig {
	if g == nil {
		return nil
	}
	return &stack.GitOpsConfig{
		Type:      g.Type,
		Bootstrap: g.Bootstrap.toUnversioned(),
	}
}

func (b *BootstrapConfig) toUnversioned() *stack.BootstrapConfig {
	if b == nil {
		return nil
	}
	return &stack.BootstrapConfig{
		Enabled:         b.Enabled,
		FluxMode:        b.FluxMode,
		FluxVersion:     b.FluxVersion,
		Components:      b.Components,
		Registry:        b.Registry,
		ImagePullSecret: b.ImagePullSecret,
		SourceURL:       b.SourceURL,
		SourceRef:       b.SourceRef,
		ArgoCDVersion:   b.ArgoCDVersion,
		ArgoCDNamespace: b.ArgoCDNamespace,
	}
}

// NodeV1Alpha1 represents a versioned Node configuration
type NodeV1Alpha1 struct {
	gvk.BaseMetadata `yaml:",inline" json:",inline"`
	
	// Spec contains the node specification
	Spec NodeSpec `yaml:"spec" json:"spec"`
}

// NodeSpec defines the specification for a node
type NodeSpec struct {
	// ParentPath is the hierarchical path to the parent node
	ParentPath string `yaml:"parentPath,omitempty" json:"parentPath,omitempty"`
	
	// Labels are common labels for this node
	Labels map[string]string `yaml:"labels,omitempty" json:"labels,omitempty"`
	
	// Children contains child nodes
	Children []NodeReference `yaml:"children,omitempty" json:"children,omitempty"`
	
	// Bundles contains the bundles in this node
	Bundles []BundleReference `yaml:"bundles,omitempty" json:"bundles,omitempty"`
	
	// Interval controls reconciliation frequency
	Interval string `yaml:"interval,omitempty" json:"interval,omitempty"`
	
	// SourceRef specifies the source for this node
	SourceRef *SourceRef `yaml:"sourceRef,omitempty" json:"sourceRef,omitempty"`
	
	// DependsOn lists dependencies
	DependsOn []string `yaml:"dependsOn,omitempty" json:"dependsOn,omitempty"`
}

// BundleReference references a Bundle by name or inline definition
type BundleReference struct {
	// Name references an external Bundle resource
	Name string `yaml:"name,omitempty" json:"name,omitempty"`
	
	// Inline allows embedding a Bundle definition
	Inline *BundleSpec `yaml:"inline,omitempty" json:"inline,omitempty"`
}

// SourceRef defines a reference to a source
type SourceRef struct {
	Kind      string `yaml:"kind" json:"kind"`
	Name      string `yaml:"name" json:"name"`
	Namespace string `yaml:"namespace,omitempty" json:"namespace,omitempty"`
}

// GetAPIVersion returns the API version for NodeV1Alpha1
func (n *NodeV1Alpha1) GetAPIVersion() string {
	return GroupName + "/" + Version
}

// GetKind returns the kind for NodeV1Alpha1
func (n *NodeV1Alpha1) GetKind() string {
	return "Node"
}

// ToUnversioned converts to the unversioned Node type
func (n *NodeV1Alpha1) ToUnversioned() *stack.Node {
	return n.Spec.toUnversioned(n.Name)
}

func (n *NodeSpec) toUnversioned(name string) *stack.Node {
	node := &stack.Node{
		Name:       name,
		ParentPath: n.ParentPath,
	}
	
	// Convert children
	for _, child := range n.Children {
		if child.Inline != nil {
			childNode := child.Inline.toUnversioned(child.Name)
			node.Children = append(node.Children, childNode)
		}
	}
	
	// Convert the first bundle to the Bundle field (current Node structure only supports one bundle)
	if len(n.Bundles) > 0 && n.Bundles[0].Inline != nil {
		node.Bundle = n.Bundles[0].Inline.toUnversioned(n.Bundles[0].Name)
	}
	
	return node
}

func (s *SourceRef) toUnversioned() *stack.SourceRef {
	if s == nil {
		return nil
	}
	return &stack.SourceRef{
		Kind:      s.Kind,
		Name:      s.Name,
		Namespace: s.Namespace,
	}
}

// BundleV1Alpha1 represents a versioned Bundle configuration
type BundleV1Alpha1 struct {
	gvk.BaseMetadata `yaml:",inline" json:",inline"`
	
	// Spec contains the bundle specification
	Spec BundleSpec `yaml:"spec" json:"spec"`
}

// BundleSpec defines the specification for a bundle
type BundleSpec struct {
	// ParentPath is the hierarchical path to the parent bundle
	ParentPath string `yaml:"parentPath,omitempty" json:"parentPath,omitempty"`
	
	// DependsOn lists other bundles this bundle depends on
	DependsOn []string `yaml:"dependsOn,omitempty" json:"dependsOn,omitempty"`
	
	// Interval controls how often reconciliation occurs
	Interval string `yaml:"interval,omitempty" json:"interval,omitempty"`
	
	// SourceRef specifies the source for the bundle
	SourceRef *SourceRef `yaml:"sourceRef,omitempty" json:"sourceRef,omitempty"`
	
	// Applications contains the applications in this bundle
	Applications []ApplicationReference `yaml:"applications,omitempty" json:"applications,omitempty"`
	
	// Labels are common labels for all resources in the bundle
	Labels map[string]string `yaml:"labels,omitempty" json:"labels,omitempty"`
}

// ApplicationReference references an Application by name or inline definition
type ApplicationReference struct {
	// Name references an external Application resource
	Name string `yaml:"name,omitempty" json:"name,omitempty"`
	
	// Inline allows embedding an Application definition using ApplicationWrapper
	Inline *stack.ApplicationWrapper `yaml:"inline,omitempty" json:"inline,omitempty"`
}

// GetAPIVersion returns the API version for BundleV1Alpha1
func (b *BundleV1Alpha1) GetAPIVersion() string {
	return GroupName + "/" + Version
}

// GetKind returns the kind for BundleV1Alpha1
func (b *BundleV1Alpha1) GetKind() string {
	return "Bundle"
}

// ToUnversioned converts to the unversioned Bundle type
func (b *BundleV1Alpha1) ToUnversioned() *stack.Bundle {
	return b.Spec.toUnversioned(b.Name)
}

func (b *BundleSpec) toUnversioned(name string) *stack.Bundle {
	bundle := &stack.Bundle{
		Name:       name,
		ParentPath: b.ParentPath,
		Interval:   b.Interval,
		SourceRef:  b.SourceRef.toUnversioned(),
		Labels:     b.Labels,
	}
	
	// Convert applications
	for _, app := range b.Applications {
		if app.Inline != nil {
			bundle.Applications = append(bundle.Applications, app.Inline.ToApplication())
		}
	}
	
	// Note: DependsOn will need to be resolved at runtime since it references other bundles by name
	
	return bundle
}