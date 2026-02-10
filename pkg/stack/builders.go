package stack

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// FluentBuilder interfaces for method chaining with immutable pattern

// ClusterBuilder provides fluent interface for building Cluster configurations
type ClusterBuilder interface {
	WithNode(name string) NodeBuilder
	WithGitOps(gitops *GitOpsConfig) ClusterBuilder
	Build() *Cluster
}

// NodeBuilder provides fluent interface for building Node configurations
type NodeBuilder interface {
	WithChild(name string) NodeBuilder
	WithBundle(name string) BundleBuilder
	WithPackageRef(ref *schema.GroupVersionKind) NodeBuilder
	End() ClusterBuilder
	Build() *Cluster
}

// BundleBuilder provides fluent interface for building Bundle configurations
type BundleBuilder interface {
	WithApplication(name string, appConfig ApplicationConfig) BundleBuilder
	WithDependency(bundle *Bundle) BundleBuilder
	WithSourceRef(sourceRef *SourceRef) BundleBuilder
	End() NodeBuilder
	Build() *Cluster
}

// ApplicationConfig is already defined in application.go - using existing interface

// ClusterBuilderImpl implements ClusterBuilder with immutable pattern
type ClusterBuilderImpl struct {
	cluster *Cluster
	errors  []error
}

// NodeBuilderImpl implements NodeBuilder with immutable pattern
type NodeBuilderImpl struct {
	cluster       *Cluster
	currentNode   *Node
	parentBuilder ClusterBuilder
	errors        []error
}

// BundleBuilderImpl implements BundleBuilder with immutable pattern
type BundleBuilderImpl struct {
	cluster       *Cluster
	currentNode   *Node
	currentBundle *Bundle
	parentBuilder NodeBuilder
	errors        []error
}

// NewClusterBuilder creates a new fluent cluster builder
func NewClusterBuilder(name string) ClusterBuilder {
	cluster := &Cluster{
		Name: name,
	}
	return &ClusterBuilderImpl{
		cluster: cluster,
		errors:  []error{},
	}
}

// WithNode adds a child node and returns a NodeBuilder for chaining
func (cb *ClusterBuilderImpl) WithNode(name string) NodeBuilder {
	// Create immutable copy
	newCluster := cb.copyCluster()

	node := &Node{
		Name:     name,
		Children: []*Node{},
	}

	newCluster.Node = node

	return &NodeBuilderImpl{
		cluster:       newCluster,
		currentNode:   node,
		parentBuilder: &ClusterBuilderImpl{cluster: newCluster, errors: cb.errors},
		errors:        cb.errors,
	}
}

// WithGitOps sets GitOps configuration
func (cb *ClusterBuilderImpl) WithGitOps(gitops *GitOpsConfig) ClusterBuilder {
	newCluster := cb.copyCluster()
	newCluster.GitOps = gitops

	return &ClusterBuilderImpl{
		cluster: newCluster,
		errors:  cb.errors,
	}
}

// Build finalizes the cluster construction
func (cb *ClusterBuilderImpl) Build() *Cluster {
	if len(cb.errors) > 0 {
		// In a real implementation, you'd want to handle errors appropriately
		// For now, we'll return the cluster even with errors
	}

	// Initialize path maps if we have a root node
	if cb.cluster.Node != nil {
		cb.cluster.Node.InitializePathMap()
	}

	return cb.cluster
}

// copyCluster creates a deep copy of the cluster
func (cb *ClusterBuilderImpl) copyCluster() *Cluster {
	newCluster := &Cluster{
		Name:   cb.cluster.Name,
		GitOps: cb.cluster.GitOps,
	}

	if cb.cluster.Node != nil {
		newCluster.Node = cb.copyNode(cb.cluster.Node)
	}

	return newCluster
}

// copyNode creates a deep copy of a node
func (cb *ClusterBuilderImpl) copyNode(node *Node) *Node {
	newNode := &Node{
		Name:       node.Name,
		ParentPath: node.ParentPath,
		PackageRef: node.PackageRef,
	}

	if node.Bundle != nil {
		newNode.Bundle = cb.copyBundle(node.Bundle)
	}

	if node.Children != nil {
		newNode.Children = make([]*Node, len(node.Children))
		for i, child := range node.Children {
			newNode.Children[i] = cb.copyNode(child)
		}
	}

	return newNode
}

// copyBundle creates a deep copy of a bundle
func (cb *ClusterBuilderImpl) copyBundle(bundle *Bundle) *Bundle {
	newBundle := &Bundle{
		Name:          bundle.Name,
		ParentPath:    bundle.ParentPath,
		SourceRef:     bundle.SourceRef,
		Interval:      bundle.Interval,
		Labels:        bundle.Labels,
		Annotations:   bundle.Annotations,
		Description:   bundle.Description,
		Prune:         bundle.Prune,
		Wait:          bundle.Wait,
		Timeout:       bundle.Timeout,
		RetryInterval: bundle.RetryInterval,
	}

	if bundle.Applications != nil {
		newBundle.Applications = make([]*Application, len(bundle.Applications))
		copy(newBundle.Applications, bundle.Applications)
	}

	if bundle.DependsOn != nil {
		newBundle.DependsOn = make([]*Bundle, len(bundle.DependsOn))
		copy(newBundle.DependsOn, bundle.DependsOn)
	}

	return newBundle
}

// NodeBuilder implementation

// WithChild adds a child node
func (nb *NodeBuilderImpl) WithChild(name string) NodeBuilder {
	newCluster := nb.copyClusterFromNode()

	// Create the child with proper parent path
	parentPath := nb.currentNode.GetPath()
	childNode := &Node{
		Name:       name,
		ParentPath: parentPath,
		Children:   []*Node{},
	}

	// Find the current node in the new cluster and add child
	if newCluster.Node != nil {
		currentNodeInCopy := nb.findNodeByPath(newCluster.Node, parentPath)
		if currentNodeInCopy != nil {
			currentNodeInCopy.Children = append(currentNodeInCopy.Children, childNode)
		}
	}

	return &NodeBuilderImpl{
		cluster:       newCluster,
		currentNode:   childNode,
		parentBuilder: &ClusterBuilderImpl{cluster: newCluster, errors: nb.errors},
		errors:        nb.errors,
	}
}

// WithBundle adds a bundle to the current node
func (nb *NodeBuilderImpl) WithBundle(name string) BundleBuilder {
	newCluster := nb.copyClusterFromNode()

	bundle := &Bundle{
		Name:         name,
		ParentPath:   nb.currentNode.GetPath(),
		Applications: []*Application{},
		DependsOn:    []*Bundle{},
	}

	// Find the current node in the new cluster and set bundle
	if newCluster.Node != nil {
		currentNodeInCopy := nb.findNodeByPath(newCluster.Node, nb.currentNode.GetPath())
		if currentNodeInCopy != nil {
			currentNodeInCopy.Bundle = bundle
		}
	}

	// Find the current node in the new cluster to keep references consistent
	currentNodeInCopy := nb.findNodeByPath(newCluster.Node, nb.currentNode.GetPath())

	return &BundleBuilderImpl{
		cluster:       newCluster,
		currentNode:   currentNodeInCopy, // Use the node from the new cluster
		currentBundle: bundle,
		parentBuilder: &NodeBuilderImpl{
			cluster:       newCluster,
			currentNode:   currentNodeInCopy, // Use the node from the new cluster
			parentBuilder: &ClusterBuilderImpl{cluster: newCluster, errors: nb.errors},
			errors:        nb.errors,
		},
		errors: nb.errors,
	}
}

// WithPackageRef sets the package reference
func (nb *NodeBuilderImpl) WithPackageRef(ref *schema.GroupVersionKind) NodeBuilder {
	newCluster := nb.copyClusterFromNode()

	// Find the current node in the new cluster and set package ref
	if newCluster.Node != nil {
		currentNodeInCopy := nb.findNodeByPath(newCluster.Node, nb.currentNode.GetPath())
		if currentNodeInCopy != nil {
			currentNodeInCopy.PackageRef = ref
		}
	}

	return &NodeBuilderImpl{
		cluster:       newCluster,
		currentNode:   nb.currentNode,
		parentBuilder: &ClusterBuilderImpl{cluster: newCluster, errors: nb.errors},
		errors:        nb.errors,
	}
}

// End returns to the parent ClusterBuilder
func (nb *NodeBuilderImpl) End() ClusterBuilder {
	// Return a parent builder with the updated cluster state
	return &ClusterBuilderImpl{
		cluster: nb.cluster,
		errors:  nb.errors,
	}
}

// Build finalizes the cluster construction
func (nb *NodeBuilderImpl) Build() *Cluster {
	// Use the updated cluster state
	clusterBuilder := &ClusterBuilderImpl{cluster: nb.cluster, errors: nb.errors}
	return clusterBuilder.Build()
}

// Helper methods for NodeBuilder

func (nb *NodeBuilderImpl) copyClusterFromNode() *Cluster {
	cb := &ClusterBuilderImpl{cluster: nb.cluster, errors: nb.errors}
	return cb.copyCluster()
}

func (nb *NodeBuilderImpl) findNodeByPath(root *Node, path string) *Node {
	if root.GetPath() == path {
		return root
	}

	for _, child := range root.Children {
		if found := nb.findNodeByPath(child, path); found != nil {
			return found
		}
	}

	return nil
}

// BundleBuilder implementation

// WithApplication adds an application to the bundle
func (bb *BundleBuilderImpl) WithApplication(name string, appConfig ApplicationConfig) BundleBuilder {
	// Create deep copy of cluster
	newCluster := bb.copyClusterFromBundle()

	// Create the application
	app := &Application{
		Name:   name,
		Config: appConfig,
	}

	// Create new bundle with the application added
	newCurrentBundle := &Bundle{
		Name:         bb.currentBundle.Name,
		ParentPath:   bb.currentBundle.ParentPath,
		SourceRef:    bb.currentBundle.SourceRef,
		Applications: make([]*Application, 0, len(bb.currentBundle.Applications)+1),
		DependsOn:    bb.currentBundle.DependsOn,
	}

	// Copy existing applications and add the new one
	copy(newCurrentBundle.Applications, bb.currentBundle.Applications)
	newCurrentBundle.Applications = append(newCurrentBundle.Applications, app)

	// Update the bundle in the cluster
	if newCluster.Node != nil {
		targetPath := bb.currentNode.GetPath()
		currentNodeInCopy := bb.findNodeByPath(newCluster.Node, targetPath)
		if currentNodeInCopy != nil {
			currentNodeInCopy.Bundle = newCurrentBundle
		}
	}

	return &BundleBuilderImpl{
		cluster:       newCluster,
		currentNode:   bb.currentNode,
		currentBundle: newCurrentBundle,
		parentBuilder: bb.parentBuilder,
		errors:        bb.errors,
	}
}

// WithDependency adds a bundle dependency
func (bb *BundleBuilderImpl) WithDependency(bundle *Bundle) BundleBuilder {
	newCluster := bb.copyClusterFromBundle()

	// Find the current bundle in the new cluster and add dependency
	if newCluster.Node != nil {
		currentNodeInCopy := bb.findNodeByPath(newCluster.Node, bb.currentNode.GetPath())
		if currentNodeInCopy != nil && currentNodeInCopy.Bundle != nil {
			currentNodeInCopy.Bundle.DependsOn = append(currentNodeInCopy.Bundle.DependsOn, bundle)
		}
	}

	return &BundleBuilderImpl{
		cluster:       newCluster,
		currentNode:   bb.currentNode,
		currentBundle: bb.currentBundle,
		parentBuilder: bb.parentBuilder,
		errors:        bb.errors,
	}
}

// WithSourceRef sets the source reference
func (bb *BundleBuilderImpl) WithSourceRef(sourceRef *SourceRef) BundleBuilder {
	// Create deep copy of cluster
	newCluster := bb.copyClusterFromBundle()

	// Create new bundle with the source ref set
	newCurrentBundle := &Bundle{
		Name:         bb.currentBundle.Name,
		ParentPath:   bb.currentBundle.ParentPath,
		SourceRef:    sourceRef, // Set the new source ref
		Applications: bb.currentBundle.Applications,
		DependsOn:    bb.currentBundle.DependsOn,
	}

	// Update the bundle in the cluster
	if newCluster.Node != nil {
		targetPath := bb.currentNode.GetPath()
		currentNodeInCopy := bb.findNodeByPath(newCluster.Node, targetPath)
		if currentNodeInCopy != nil {
			currentNodeInCopy.Bundle = newCurrentBundle
		}
	}

	return &BundleBuilderImpl{
		cluster:       newCluster,
		currentNode:   bb.currentNode,
		currentBundle: newCurrentBundle,
		parentBuilder: bb.parentBuilder,
		errors:        bb.errors,
	}
}

// End returns to the parent NodeBuilder
func (bb *BundleBuilderImpl) End() NodeBuilder {
	// Return a parent builder with the updated cluster state
	return &NodeBuilderImpl{
		cluster:       bb.cluster,
		currentNode:   bb.currentNode,
		parentBuilder: &ClusterBuilderImpl{cluster: bb.cluster, errors: bb.errors},
		errors:        bb.errors,
	}
}

// Build finalizes the cluster construction
func (bb *BundleBuilderImpl) Build() *Cluster {
	// Use the updated cluster state
	clusterBuilder := &ClusterBuilderImpl{cluster: bb.cluster, errors: bb.errors}
	return clusterBuilder.Build()
}

// Helper methods for BundleBuilder

func (bb *BundleBuilderImpl) copyClusterFromBundle() *Cluster {
	cb := &ClusterBuilderImpl{cluster: bb.cluster, errors: bb.errors}
	return cb.copyCluster()
}

func (bb *BundleBuilderImpl) findNodeByPath(root *Node, path string) *Node {
	if root.GetPath() == path {
		return root
	}

	for _, child := range root.Children {
		if found := bb.findNodeByPath(child, path); found != nil {
			return found
		}
	}

	return nil
}
