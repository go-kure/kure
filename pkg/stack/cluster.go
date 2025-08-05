package stack

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// Cluster describes a cluster configuration.
// A cluster configuration is a set of configurations that are packaged in one or more package units
type Cluster struct {
	Name   string        `yaml:"name"`
	Node   *Node         `yaml:"node,omitempty"`
	GitOps *GitOpsConfig `yaml:"gitops,omitempty"`
}

// GitOpsConfig defines the GitOps tool configuration for the cluster
type GitOpsConfig struct {
	Type      string           `yaml:"type"` // "flux" or "argocd"
	Bootstrap *BootstrapConfig `yaml:"bootstrap,omitempty"`
}

// BootstrapConfig defines the bootstrap configuration for GitOps tools
type BootstrapConfig struct {
	// Common fields
	Enabled bool `yaml:"enabled"`

	// Flux-specific
	FluxMode        string   `yaml:"fluxMode,omitempty"` // "gitops-toolkit" or "flux-operator"
	FluxVersion     string   `yaml:"fluxVersion,omitempty"`
	Components      []string `yaml:"components,omitempty"`
	Registry        string   `yaml:"registry,omitempty"`
	ImagePullSecret string   `yaml:"imagePullSecret,omitempty"`

	// Source configuration
	SourceURL string `yaml:"sourceURL,omitempty"` // OCI/Git repository URL
	SourceRef string `yaml:"sourceRef,omitempty"` // Tag/branch/ref

	// ArgoCD-specific (mock for now)
	ArgoCDVersion   string `yaml:"argoCDVersion,omitempty"`
	ArgoCDNamespace string `yaml:"argoCDNamespace,omitempty"`
}

// Node represents a hierarchic structure holding all deployment bundles
// each tree has a list of children, which can be a deployment, or a subtree
// It could match a kubernetes cluster's full configuration, or it could be just
// a part of that, when parts are e.g. packaged in different OCI artifacts
// Tree's with a common PackageRef are packaged together
type Node struct {
	// Name identifies the application set.
	Name string `yaml:"name"`
	// ParentPath is the hierarchical path to the parent node (e.g., "cluster/infrastructure")
	// Empty for root nodes. This avoids circular references while maintaining hierarchy.
	ParentPath string `yaml:"parentPath,omitempty"`
	// Children list child bundles
	Children []*Node `yaml:"children,omitempty"`
	// PackageRef identifies in which package the tree of resources get bundled together
	// If undefined, the PackageRef of the parent is inherited
	PackageRef *schema.GroupVersionKind `yaml:"packageref,omitempty"`
	// Bundle holds the applications that get deployed on this level
	Bundle *Bundle `yaml:"bundle,omitempty"`

	// Internal fields for runtime hierarchy navigation (not serialized)
	parent  *Node            `yaml:"-"` // Runtime parent reference for efficient traversal
	pathMap map[string]*Node `yaml:"-"` // Runtime path lookup map (shared across tree)
}

// NewCluster creates a Cluster with the provided metadata.
func NewCluster(name string, tree *Node) *Cluster {
	return &Cluster{Name: name, Node: tree}
}

// GetName Helper getters.
func (c *Cluster) GetName() string          { return c.Name }
func (c *Cluster) GetNode() *Node           { return c.Node }
func (c *Cluster) GetGitOps() *GitOpsConfig { return c.GitOps }

// SetName Setters for metadata fields.
func (c *Cluster) SetName(n string)          { c.Name = n }
func (c *Cluster) SetNode(t *Node)           { c.Node = t }
func (c *Cluster) SetGitOps(g *GitOpsConfig) { c.GitOps = g }

func (n *Node) GetName() string                         { return n.Name }
func (n *Node) GetParent() *Node                        { return n.parent }
func (n *Node) GetParentPath() string                   { return n.ParentPath }
func (n *Node) GetChildren() []*Node                    { return n.Children }
func (n *Node) GetPackageRef() *schema.GroupVersionKind { return n.PackageRef }
func (n *Node) GetBundle() *Bundle                      { return n.Bundle }

func (n *Node) SetName(name string)                        { n.Name = name }
func (n *Node) SetParentPath(path string)                  { n.ParentPath = path }
func (n *Node) SetChildren(children []*Node)               { n.Children = children }
func (n *Node) SetPackageRef(ref *schema.GroupVersionKind) { n.PackageRef = ref }
func (n *Node) SetBundle(bundle *Bundle)                   { n.Bundle = bundle }

// SetParent sets the parent node and updates the ParentPath accordingly.
// This method maintains both the serializable path and runtime reference.
func (n *Node) SetParent(parent *Node) {
	n.parent = parent
	if parent == nil {
		n.ParentPath = ""
	} else {
		n.ParentPath = parent.GetPath()
	}
}

// GetPath returns the full hierarchical path of this node.
func (n *Node) GetPath() string {
	if n.ParentPath == "" {
		return n.Name
	}
	return n.ParentPath + "/" + n.Name
}

// InitializePathMap builds the runtime path lookup map for efficient hierarchy navigation.
// This should be called on the root node after the tree structure is complete.
func (n *Node) InitializePathMap() {
	pathMap := make(map[string]*Node)
	n.buildPathMap(pathMap, "")
	n.setPathMapRecursive(pathMap)
}

// buildPathMap recursively builds the path to node mapping.
func (n *Node) buildPathMap(pathMap map[string]*Node, parentPath string) {
	currentPath := n.Name
	if parentPath != "" {
		currentPath = parentPath + "/" + n.Name
	}

	pathMap[currentPath] = n

	for _, child := range n.Children {
		child.buildPathMap(pathMap, currentPath)
	}
}

// setPathMapRecursive sets the path map reference on all nodes in the tree.
func (n *Node) setPathMapRecursive(pathMap map[string]*Node) {
	n.pathMap = pathMap

	// Set parent runtime reference from path
	if n.ParentPath != "" {
		if parent, exists := pathMap[n.ParentPath]; exists {
			n.parent = parent
		}
	}

	for _, child := range n.Children {
		child.setPathMapRecursive(pathMap)
	}
}
