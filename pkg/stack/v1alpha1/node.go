package v1alpha1

import (
	"github.com/go-kure/kure/internal/gvk"
	"github.com/go-kure/kure/pkg/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// NodeConfig represents a versioned node configuration
// +gvk:group=stack.gokure.dev
// +gvk:version=v1alpha1
// +gvk:kind=Node
type NodeConfig struct {
	APIVersion string           `yaml:"apiVersion" json:"apiVersion"`
	Kind       string           `yaml:"kind" json:"kind"`
	Metadata   gvk.BaseMetadata `yaml:"metadata" json:"metadata"`
	Spec       NodeSpec         `yaml:"spec" json:"spec"`
}

// NodeSpec defines the specification for a node
type NodeSpec struct {
	// ParentPath is the hierarchical path to the parent node (e.g., "cluster/infrastructure")
	// Empty for root nodes. This avoids circular references while maintaining hierarchy.
	ParentPath string `yaml:"parentPath,omitempty" json:"parentPath,omitempty"`

	// Children list child nodes
	Children []NodeReference `yaml:"children,omitempty" json:"children,omitempty"`

	// PackageRef identifies in which package the tree of resources get bundled together
	// If undefined, the PackageRef of the parent is inherited
	PackageRef *schema.GroupVersionKind `yaml:"packageRef,omitempty" json:"packageRef,omitempty"`

	// Bundle holds the applications that get deployed on this level
	Bundle *BundleReference `yaml:"bundle,omitempty" json:"bundle,omitempty"`

	// Description provides a human-readable description of the node
	Description string `yaml:"description,omitempty" json:"description,omitempty"`

	// Labels are key-value pairs for node metadata
	Labels map[string]string `yaml:"labels,omitempty" json:"labels,omitempty"`

	// Annotations are key-value pairs for node annotations
	Annotations map[string]string `yaml:"annotations,omitempty" json:"annotations,omitempty"`
}

// BundleReference references a Bundle configuration
type BundleReference struct {
	// Name of the bundle
	Name string `yaml:"name" json:"name"`

	// APIVersion of the referenced bundle (for future cross-version references)
	APIVersion string `yaml:"apiVersion,omitempty" json:"apiVersion,omitempty"`
}

// GetAPIVersion returns the API version of the node config
func (n *NodeConfig) GetAPIVersion() string {
	if n.APIVersion == "" {
		return "stack.gokure.dev/v1alpha1"
	}
	return n.APIVersion
}

// GetKind returns the kind of the node config
func (n *NodeConfig) GetKind() string {
	if n.Kind == "" {
		return "Node"
	}
	return n.Kind
}

// GetName returns the name of the node
func (n *NodeConfig) GetName() string {
	return n.Metadata.Name
}

// SetName sets the name of the node
func (n *NodeConfig) SetName(name string) {
	n.Metadata.Name = name
}

// GetNamespace returns the namespace of the node
func (n *NodeConfig) GetNamespace() string {
	return n.Metadata.Namespace
}

// SetNamespace sets the namespace of the node
func (n *NodeConfig) SetNamespace(namespace string) {
	n.Metadata.Namespace = namespace
}

// GetPath returns the full hierarchical path of this node
func (n *NodeConfig) GetPath() string {
	if n.Spec.ParentPath == "" {
		return n.Metadata.Name
	}
	return n.Spec.ParentPath + "/" + n.Metadata.Name
}

// Validate performs validation on the node configuration
func (n *NodeConfig) Validate() error {
	if n == nil {
		return errors.New("node config is nil")
	}

	if n.Metadata.Name == "" {
		return errors.NewValidationError("metadata.name", "", "Node", nil)
	}

	// Validate PackageRef if present
	if n.Spec.PackageRef != nil {
		if n.Spec.PackageRef.Kind == "" {
			return errors.ResourceValidationError("Node", n.Metadata.Name, "spec.packageRef.kind",
				"packageRef kind cannot be empty", nil)
		}
	}

	// Check for circular references in children
	childNames := make(map[string]bool)
	for _, child := range n.Spec.Children {
		if child.Name == "" {
			return errors.ResourceValidationError("Node", n.Metadata.Name, "spec.children",
				"child node name cannot be empty", nil)
		}
		if childNames[child.Name] {
			return errors.ResourceValidationError("Node", n.Metadata.Name, "spec.children",
				"duplicate child node name: "+child.Name, nil)
		}
		childNames[child.Name] = true
	}

	return nil
}

// ConvertTo converts this node config to another version
func (n *NodeConfig) ConvertTo(version string) (interface{}, error) {
	switch version {
	case "v1alpha1":
		return n, nil
	default:
		return nil, errors.New("unsupported version: " + version)
	}
}

// ConvertFrom converts from another version to this node config
func (n *NodeConfig) ConvertFrom(from interface{}) error {
	switch src := from.(type) {
	case *NodeConfig:
		*n = *src
		return nil
	default:
		return errors.New("unsupported conversion source type")
	}
}

// NewNodeConfig creates a new NodeConfig with default values
func NewNodeConfig(name string) *NodeConfig {
	return &NodeConfig{
		APIVersion: "stack.gokure.dev/v1alpha1",
		Kind:       "Node",
		Metadata: gvk.BaseMetadata{
			Name: name,
		},
		Spec: NodeSpec{},
	}
}

// AddChild adds a child node reference
func (n *NodeConfig) AddChild(childName string) {
	n.Spec.Children = append(n.Spec.Children, NodeReference{
		Name:       childName,
		APIVersion: n.GetAPIVersion(),
	})
}

// SetBundle sets the bundle reference
func (n *NodeConfig) SetBundle(bundleName string) {
	n.Spec.Bundle = &BundleReference{
		Name:       bundleName,
		APIVersion: "stack.gokure.dev/v1alpha1",
	}
}

// SetPackageRef sets the package reference
func (n *NodeConfig) SetPackageRef(group, version, kind string) {
	n.Spec.PackageRef = &schema.GroupVersionKind{
		Group:   group,
		Version: version,
		Kind:    kind,
	}
}
