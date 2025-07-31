package application

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// Cluster describes a cluster configuration.
// A cluster configuration is a set of configurations that are packaged in one or more package units
type Cluster struct {
	Name string `yaml:"name"`
	Node *Node  `yaml:"node,omitempty"`
}

// Node represents a hierarchic structure holding all deployment bundles
// each tree has a list of children, which can be a deployment, or a subtree
// It could match a kubernetes cluster's full configuration, or it could be just
// a part of that, when parts are e.g. packaged in different OCI artifacts
// Tree's with a common PackageRef are packaged together
type Node struct {
	// Name identifies the application set.
	Name string `yaml:"name"`
	// Parent is the parent node. Only the otp level node will have a nil value here
	Parent *Node `yaml:"parent"`
	// Children list child bundles
	Children []*Node `yaml:"children,omitempty"`
	// PackageRef identifies in which package the tree of resources get bundled together
	// If undefined, the PackageRef of the parent is inherited
	PackageRef *schema.GroupVersionKind `yaml:"packageref,omitempty"`
	// Bundle holds the applications that get deployed on this level
	Bundle *Bundle `yaml:"bundle,omitempty"`
}

// NewCluster creates a Cluster with the provided metadata.
func NewCluster(name string, tree *Node) *Cluster {
	return &Cluster{Name: name, Node: tree}
}

// GetName Helper getters.
func (c *Cluster) GetName() string { return c.Name }
func (c *Cluster) GetNode() *Node  { return c.Node }

// SetName Setters for metadata fields.
func (c *Cluster) SetName(n string) { c.Name = n }
func (c *Cluster) SetNode(t *Node)  { c.Node = t }

func (n *Node) GetName() string                         { return n.Name }
func (n *Node) GetParent() *Node                        { return n.Parent }
func (n *Node) GetChildren() []*Node                    { return n.Children }
func (n *Node) GetPackageRef() *schema.GroupVersionKind { return n.PackageRef }
func (n *Node) GetBundle() *Bundle                      { return n.Bundle }

func (n *Node) SetName(name string)                        { n.Name = name }
func (n *Node) SetParent(parent *Node)                     { n.Parent = parent }
func (n *Node) SetChildren(children []*Node)               { n.Children = children }
func (n *Node) SetPackageRef(ref *schema.GroupVersionKind) { n.PackageRef = ref }
func (n *Node) SetBundle(bundle *Bundle)                   { n.Bundle = bundle }
