package stack

import (
	"errors"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime/schema"
)

// FluentBuilder interfaces for method chaining with copy-on-write pattern

// ClusterBuilder provides fluent interface for building Cluster configurations.
type ClusterBuilder interface {
	WithNode(name string) NodeBuilder
	WithGitOps(gitops *GitOpsConfig) ClusterBuilder
	Build() (*Cluster, error)
}

// NodeBuilder provides fluent interface for building Node configurations.
type NodeBuilder interface {
	WithChild(name string) NodeBuilder
	WithBundle(name string) BundleBuilder
	WithPackageRef(ref *schema.GroupVersionKind) NodeBuilder
	End() ClusterBuilder
	Build() (*Cluster, error)
}

// BundleBuilder provides fluent interface for building Bundle configurations.
type BundleBuilder interface {
	WithApplication(name string, appConfig ApplicationConfig) BundleBuilder
	WithDependency(bundle *Bundle) BundleBuilder
	WithSourceRef(sourceRef *SourceRef) BundleBuilder
	End() NodeBuilder
	Build() (*Cluster, error)
}

// --- implementation structs ---

type clusterBuilderImpl struct {
	cluster *Cluster
	errors  []error
}

type nodeBuilderImpl struct {
	cluster  *Cluster
	nodePath string // re-derived via findNodeByPath after copy
	errors   []error
}

type bundleBuilderImpl struct {
	cluster  *Cluster
	nodePath string // node that owns the bundle
	errors   []error
}

// --- package-level helpers ---

// findNodeByPath locates a node within the tree by its full path.
func findNodeByPath(root *Node, path string) *Node {
	if root == nil {
		return nil
	}
	if root.GetPath() == path {
		return root
	}
	for _, child := range root.Children {
		if found := findNodeByPath(child, path); found != nil {
			return found
		}
	}
	return nil
}

// deepCopyCluster creates a deep copy of a Cluster.
func deepCopyCluster(c *Cluster) *Cluster {
	if c == nil {
		return nil
	}
	newCluster := &Cluster{
		Name:   c.Name,
		GitOps: c.GitOps, // shared; GitOps is set-once, not mutated
	}
	if c.Node != nil {
		newCluster.Node = deepCopyNode(c.Node)
	}
	return newCluster
}

// deepCopyNode creates a deep copy of a Node and its subtree.
func deepCopyNode(n *Node) *Node {
	if n == nil {
		return nil
	}
	newNode := &Node{
		Name:       n.Name,
		ParentPath: n.ParentPath,
		PackageRef: n.PackageRef, // GVK is effectively immutable
	}
	if n.Bundle != nil {
		newNode.Bundle = deepCopyBundle(n.Bundle)
	}
	if n.Children != nil {
		newNode.Children = make([]*Node, len(n.Children))
		for i, child := range n.Children {
			newNode.Children[i] = deepCopyNode(child)
		}
	}
	return newNode
}

// deepCopyBundle creates a deep copy of a Bundle.
func deepCopyBundle(b *Bundle) *Bundle {
	if b == nil {
		return nil
	}
	newBundle := &Bundle{
		Name:          b.Name,
		ParentPath:    b.ParentPath,
		SourceRef:     b.SourceRef,
		Interval:      b.Interval,
		Labels:        b.Labels,
		Annotations:   b.Annotations,
		Description:   b.Description,
		Prune:         b.Prune,
		Wait:          b.Wait,
		Timeout:       b.Timeout,
		RetryInterval: b.RetryInterval,
	}
	if b.Applications != nil {
		newBundle.Applications = make([]*Application, len(b.Applications))
		copy(newBundle.Applications, b.Applications)
	}
	if b.DependsOn != nil {
		newBundle.DependsOn = make([]*Bundle, len(b.DependsOn))
		copy(newBundle.DependsOn, b.DependsOn)
	}
	return newBundle
}

// copyErrors returns a fresh copy of an error slice.
func copyErrors(errs []error) []error {
	if len(errs) == 0 {
		return nil
	}
	out := make([]error, len(errs))
	copy(out, errs)
	return out
}

// --- ensureOwned helpers ---

// ensureOwned returns a deep-copied cluster and error slice that are safe to
// mutate without affecting the original builder's state.
func (cb *clusterBuilderImpl) ensureOwned() (*Cluster, []error) {
	return deepCopyCluster(cb.cluster), copyErrors(cb.errors)
}

// ensureOwned returns a deep-copied cluster, the target node within that copy,
// and a copied error slice. Returns nil node if the node path cannot be resolved.
func (nb *nodeBuilderImpl) ensureOwned() (*Cluster, *Node, []error) {
	cluster := deepCopyCluster(nb.cluster)
	errs := copyErrors(nb.errors)
	node := findNodeByPath(cluster.Node, nb.nodePath)
	return cluster, node, errs
}

// ensureOwned returns a deep-copied cluster, the target node, the target bundle,
// and a copied error slice.
func (bb *bundleBuilderImpl) ensureOwned() (*Cluster, *Node, *Bundle, []error) {
	cluster := deepCopyCluster(bb.cluster)
	errs := copyErrors(bb.errors)
	node := findNodeByPath(cluster.Node, bb.nodePath)
	var bundle *Bundle
	if node != nil {
		bundle = node.Bundle
	}
	return cluster, node, bundle, errs
}

// --- ClusterBuilder ---

// NewClusterBuilder creates a new fluent cluster builder.
func NewClusterBuilder(name string) ClusterBuilder {
	return &clusterBuilderImpl{
		cluster: &Cluster{Name: name},
	}
}

// WithNode sets the root node and returns a NodeBuilder for chaining.
func (cb *clusterBuilderImpl) WithNode(name string) NodeBuilder {
	cluster, errs := cb.ensureOwned()

	if name == "" {
		errs = append(errs, fmt.Errorf("node name must not be empty"))
		return &nodeBuilderImpl{cluster: cluster, errors: errs}
	}

	node := &Node{
		Name:     name,
		Children: []*Node{},
	}
	cluster.Node = node

	return &nodeBuilderImpl{
		cluster:  cluster,
		nodePath: node.GetPath(),
		errors:   errs,
	}
}

// WithGitOps sets GitOps configuration.
func (cb *clusterBuilderImpl) WithGitOps(gitops *GitOpsConfig) ClusterBuilder {
	cluster, errs := cb.ensureOwned()
	cluster.GitOps = gitops
	return &clusterBuilderImpl{
		cluster: cluster,
		errors:  errs,
	}
}

// Build finalizes the cluster construction.
func (cb *clusterBuilderImpl) Build() (*Cluster, error) {
	errs := cb.errors

	// Final validations
	if cb.cluster.Name == "" {
		errs = append(errs, fmt.Errorf("cluster name must not be empty"))
	}

	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}

	// Initialize path maps if we have a root node
	if cb.cluster.Node != nil {
		cb.cluster.Node.InitializePathMap()
	}

	return cb.cluster, nil
}

// --- NodeBuilder ---

// WithChild adds a child node to the current node.
func (nb *nodeBuilderImpl) WithChild(name string) NodeBuilder {
	cluster, currentNode, errs := nb.ensureOwned()

	if name == "" {
		errs = append(errs, fmt.Errorf("child node name must not be empty"))
		return &nodeBuilderImpl{cluster: cluster, nodePath: nb.nodePath, errors: errs}
	}

	if currentNode == nil {
		errs = append(errs, fmt.Errorf("cannot add child %q: parent node not found", name))
		return &nodeBuilderImpl{cluster: cluster, errors: errs}
	}

	childNode := &Node{
		Name:       name,
		ParentPath: currentNode.GetPath(),
		Children:   []*Node{},
	}
	currentNode.Children = append(currentNode.Children, childNode)

	return &nodeBuilderImpl{
		cluster:  cluster,
		nodePath: childNode.GetPath(),
		errors:   errs,
	}
}

// WithBundle adds a bundle to the current node.
func (nb *nodeBuilderImpl) WithBundle(name string) BundleBuilder {
	cluster, currentNode, errs := nb.ensureOwned()

	if name == "" {
		errs = append(errs, fmt.Errorf("bundle name must not be empty"))
		return &bundleBuilderImpl{cluster: cluster, nodePath: nb.nodePath, errors: errs}
	}

	if currentNode == nil {
		errs = append(errs, fmt.Errorf("cannot add bundle %q: node not found", name))
		return &bundleBuilderImpl{cluster: cluster, errors: errs}
	}

	bundle := &Bundle{
		Name:         name,
		ParentPath:   currentNode.GetPath(),
		Applications: []*Application{},
		DependsOn:    []*Bundle{},
	}
	currentNode.Bundle = bundle

	return &bundleBuilderImpl{
		cluster:  cluster,
		nodePath: currentNode.GetPath(),
		errors:   errs,
	}
}

// WithPackageRef sets the package reference on the current node.
func (nb *nodeBuilderImpl) WithPackageRef(ref *schema.GroupVersionKind) NodeBuilder {
	cluster, currentNode, errs := nb.ensureOwned()

	if currentNode != nil {
		currentNode.PackageRef = ref
	} else {
		errs = append(errs, fmt.Errorf("cannot set package ref: node not found"))
	}

	return &nodeBuilderImpl{
		cluster:  cluster,
		nodePath: nb.nodePath,
		errors:   errs,
	}
}

// End returns to the parent ClusterBuilder.
func (nb *nodeBuilderImpl) End() ClusterBuilder {
	return &clusterBuilderImpl{
		cluster: nb.cluster,
		errors:  nb.errors,
	}
}

// Build finalizes the cluster construction from a NodeBuilder.
func (nb *nodeBuilderImpl) Build() (*Cluster, error) {
	return (&clusterBuilderImpl{cluster: nb.cluster, errors: nb.errors}).Build()
}

// --- BundleBuilder ---

// WithApplication adds an application to the bundle.
func (bb *bundleBuilderImpl) WithApplication(name string, appConfig ApplicationConfig) BundleBuilder {
	cluster, _, bundle, errs := bb.ensureOwned()

	if name == "" {
		errs = append(errs, fmt.Errorf("application name must not be empty"))
		return &bundleBuilderImpl{cluster: cluster, nodePath: bb.nodePath, errors: errs}
	}
	if appConfig == nil {
		errs = append(errs, fmt.Errorf("application config for %q must not be nil", name))
		return &bundleBuilderImpl{cluster: cluster, nodePath: bb.nodePath, errors: errs}
	}

	if bundle == nil {
		errs = append(errs, fmt.Errorf("cannot add application %q: bundle not found", name))
		return &bundleBuilderImpl{cluster: cluster, nodePath: bb.nodePath, errors: errs}
	}

	app := &Application{
		Name:   name,
		Config: appConfig,
	}
	bundle.Applications = append(bundle.Applications, app)

	return &bundleBuilderImpl{
		cluster:  cluster,
		nodePath: bb.nodePath,
		errors:   errs,
	}
}

// WithDependency adds a bundle dependency.
func (bb *bundleBuilderImpl) WithDependency(dep *Bundle) BundleBuilder {
	cluster, _, bundle, errs := bb.ensureOwned()

	if dep == nil {
		errs = append(errs, fmt.Errorf("dependency must not be nil"))
		return &bundleBuilderImpl{cluster: cluster, nodePath: bb.nodePath, errors: errs}
	}

	if bundle == nil {
		errs = append(errs, fmt.Errorf("cannot add dependency: bundle not found"))
		return &bundleBuilderImpl{cluster: cluster, nodePath: bb.nodePath, errors: errs}
	}

	bundle.DependsOn = append(bundle.DependsOn, dep)

	return &bundleBuilderImpl{
		cluster:  cluster,
		nodePath: bb.nodePath,
		errors:   errs,
	}
}

// WithSourceRef sets the source reference on the bundle.
func (bb *bundleBuilderImpl) WithSourceRef(sourceRef *SourceRef) BundleBuilder {
	cluster, _, bundle, errs := bb.ensureOwned()

	if bundle != nil {
		bundle.SourceRef = sourceRef
	} else {
		errs = append(errs, fmt.Errorf("cannot set source ref: bundle not found"))
	}

	return &bundleBuilderImpl{
		cluster:  cluster,
		nodePath: bb.nodePath,
		errors:   errs,
	}
}

// End returns to the parent NodeBuilder.
func (bb *bundleBuilderImpl) End() NodeBuilder {
	return &nodeBuilderImpl{
		cluster:  bb.cluster,
		nodePath: bb.nodePath,
		errors:   bb.errors,
	}
}

// Build finalizes the cluster construction from a BundleBuilder.
func (bb *bundleBuilderImpl) Build() (*Cluster, error) {
	return (&clusterBuilderImpl{cluster: bb.cluster, errors: bb.errors}).Build()
}
