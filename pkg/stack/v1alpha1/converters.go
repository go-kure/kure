package v1alpha1

import (
	"github.com/go-kure/kure/internal/gvk"
	"github.com/go-kure/kure/pkg/stack"
)

// ConvertClusterToV1Alpha1 converts an unversioned Cluster to a v1alpha1 ClusterConfig
func ConvertClusterToV1Alpha1(c *stack.Cluster) *ClusterConfig {
	if c == nil {
		return nil
	}

	config := &ClusterConfig{
		APIVersion: "stack.gokure.dev/v1alpha1",
		Kind:       "Cluster",
		Metadata: gvk.BaseMetadata{
			Name: c.Name,
		},
		Spec: ClusterSpec{},
	}

	// Convert Node reference
	if c.Node != nil {
		config.Spec.Node = &NodeReference{
			Name:       c.Node.Name,
			APIVersion: "stack.gokure.dev/v1alpha1",
		}
	}

	// Convert GitOps config
	if c.GitOps != nil {
		config.Spec.GitOps = &GitOpsConfig{
			Type: c.GitOps.Type,
		}

		if c.GitOps.Bootstrap != nil {
			config.Spec.GitOps.Bootstrap = &BootstrapConfig{
				Enabled:         c.GitOps.Bootstrap.Enabled,
				FluxMode:        c.GitOps.Bootstrap.FluxMode,
				FluxVersion:     c.GitOps.Bootstrap.FluxVersion,
				Components:      c.GitOps.Bootstrap.Components,
				Registry:        c.GitOps.Bootstrap.Registry,
				ImagePullSecret: c.GitOps.Bootstrap.ImagePullSecret,
				SourceURL:       c.GitOps.Bootstrap.SourceURL,
				SourceRef:       c.GitOps.Bootstrap.SourceRef,
				ArgoCDVersion:   c.GitOps.Bootstrap.ArgoCDVersion,
				ArgoCDNamespace: c.GitOps.Bootstrap.ArgoCDNamespace,
			}
		}
	}

	return config
}

// ConvertV1Alpha1ToCluster converts a v1alpha1 ClusterConfig to an unversioned Cluster
func ConvertV1Alpha1ToCluster(config *ClusterConfig) *stack.Cluster {
	if config == nil {
		return nil
	}

	c := &stack.Cluster{
		Name: config.Metadata.Name,
	}

	// Note: We don't convert the Node here as it would require full tree traversal
	// This should be handled at a higher level that has access to all nodes

	// Convert GitOps config
	if config.Spec.GitOps != nil {
		c.GitOps = &stack.GitOpsConfig{
			Type: config.Spec.GitOps.Type,
		}

		if config.Spec.GitOps.Bootstrap != nil {
			c.GitOps.Bootstrap = &stack.BootstrapConfig{
				Enabled:         config.Spec.GitOps.Bootstrap.Enabled,
				FluxMode:        config.Spec.GitOps.Bootstrap.FluxMode,
				FluxVersion:     config.Spec.GitOps.Bootstrap.FluxVersion,
				Components:      config.Spec.GitOps.Bootstrap.Components,
				Registry:        config.Spec.GitOps.Bootstrap.Registry,
				ImagePullSecret: config.Spec.GitOps.Bootstrap.ImagePullSecret,
				SourceURL:       config.Spec.GitOps.Bootstrap.SourceURL,
				SourceRef:       config.Spec.GitOps.Bootstrap.SourceRef,
				ArgoCDVersion:   config.Spec.GitOps.Bootstrap.ArgoCDVersion,
				ArgoCDNamespace: config.Spec.GitOps.Bootstrap.ArgoCDNamespace,
			}
		}
	}

	return c
}

// ConvertNodeToV1Alpha1 converts an unversioned Node to a v1alpha1 NodeConfig
func ConvertNodeToV1Alpha1(n *stack.Node) *NodeConfig {
	if n == nil {
		return nil
	}

	config := &NodeConfig{
		APIVersion: "stack.gokure.dev/v1alpha1",
		Kind:       "Node",
		Metadata: gvk.BaseMetadata{
			Name: n.Name,
		},
		Spec: NodeSpec{
			ParentPath: n.ParentPath,
			PackageRef: n.PackageRef,
		},
	}

	// Convert children references
	for _, child := range n.Children {
		if child != nil {
			config.Spec.Children = append(config.Spec.Children, NodeReference{
				Name:       child.Name,
				APIVersion: "stack.gokure.dev/v1alpha1",
			})
		}
	}

	// Convert bundle reference
	if n.Bundle != nil {
		config.Spec.Bundle = &BundleReference{
			Name:       n.Bundle.Name,
			APIVersion: "stack.gokure.dev/v1alpha1",
		}
	}

	return config
}

// ConvertV1Alpha1ToNode converts a v1alpha1 NodeConfig to an unversioned Node
func ConvertV1Alpha1ToNode(config *NodeConfig) *stack.Node {
	if config == nil {
		return nil
	}

	n := &stack.Node{
		Name:       config.Metadata.Name,
		ParentPath: config.Spec.ParentPath,
		PackageRef: config.Spec.PackageRef,
	}

	// Note: We don't convert children and bundle here as they would require
	// access to the full set of nodes and bundles. This should be handled
	// at a higher level that has access to all configurations

	return n
}

// ConvertBundleToV1Alpha1 converts an unversioned Bundle to a v1alpha1 BundleConfig
func ConvertBundleToV1Alpha1(b *stack.Bundle) *BundleConfig {
	if b == nil {
		return nil
	}

	config := &BundleConfig{
		APIVersion: "stack.gokure.dev/v1alpha1",
		Kind:       "Bundle",
		Metadata: gvk.BaseMetadata{
			Name: b.Name,
		},
		Spec: BundleSpec{
			ParentPath:    b.ParentPath,
			Interval:      b.Interval,
			Labels:        b.Labels,
			Annotations:   b.Annotations,
			Description:   b.Description,
			Timeout:       b.Timeout,
			RetryInterval: b.RetryInterval,
		},
	}

	// Convert bool pointers
	if b.Prune != nil {
		config.Spec.Prune = *b.Prune
	}
	if b.Wait != nil {
		config.Spec.Wait = *b.Wait
	}

	// Convert source ref
	if b.SourceRef != nil {
		config.Spec.SourceRef = &SourceRef{
			Kind:      b.SourceRef.Kind,
			Name:      b.SourceRef.Name,
			Namespace: b.SourceRef.Namespace,
		}
	}

	// Convert dependencies
	for _, dep := range b.DependsOn {
		if dep != nil {
			config.Spec.DependsOn = append(config.Spec.DependsOn, BundleReference{
				Name:       dep.Name,
				APIVersion: "stack.gokure.dev/v1alpha1",
			})
		}
	}

	// Convert applications
	for _, app := range b.Applications {
		if app != nil {
			// Use the application's name and default GVK
			// In the future, we could enhance Application to track its GVK
			apiVersion := "generators.gokure.dev/v1alpha1"
			kind := "Application"

			config.Spec.Applications = append(config.Spec.Applications, ApplicationReference{
				Name:       app.Name,
				APIVersion: apiVersion,
				Kind:       kind,
			})
		}
	}

	return config
}

// ConvertV1Alpha1ToBundle converts a v1alpha1 BundleConfig to an unversioned Bundle
func ConvertV1Alpha1ToBundle(config *BundleConfig) *stack.Bundle {
	if config == nil {
		return nil
	}

	prune := config.Spec.Prune
	wait := config.Spec.Wait
	b := &stack.Bundle{
		Name:          config.Metadata.Name,
		ParentPath:    config.Spec.ParentPath,
		Interval:      config.Spec.Interval,
		Labels:        config.Spec.Labels,
		Annotations:   config.Spec.Annotations,
		Description:   config.Spec.Description,
		Prune:         &prune,
		Wait:          &wait,
		Timeout:       config.Spec.Timeout,
		RetryInterval: config.Spec.RetryInterval,
	}

	// Convert source ref
	if config.Spec.SourceRef != nil {
		b.SourceRef = &stack.SourceRef{
			Kind:      config.Spec.SourceRef.Kind,
			Name:      config.Spec.SourceRef.Name,
			Namespace: config.Spec.SourceRef.Namespace,
		}
	}

	// Note: We don't convert dependencies and applications here as they would require
	// access to the full set of bundles and applications. This should be handled
	// at a higher level that has access to all configurations

	return b
}

// StackConverter provides methods to convert between versioned and unversioned stack types
type StackConverter struct {
	// nodeMap maps node names to their configs for reconstruction
	nodeMap map[string]*NodeConfig

	// bundleMap maps bundle names to their configs for reconstruction
	bundleMap map[string]*BundleConfig

	// appMap maps application references to their actual applications
	appMap map[string]*stack.Application
}

// NewStackConverter creates a new stack converter
func NewStackConverter() *StackConverter {
	return &StackConverter{
		nodeMap:   make(map[string]*NodeConfig),
		bundleMap: make(map[string]*BundleConfig),
		appMap:    make(map[string]*stack.Application),
	}
}

// ConvertClusterTreeToV1Alpha1 converts a full cluster tree to versioned configs
func (c *StackConverter) ConvertClusterTreeToV1Alpha1(cluster *stack.Cluster) (*ClusterConfig, []*NodeConfig, []*BundleConfig) {
	if cluster == nil {
		return nil, nil, nil
	}

	// Convert cluster
	clusterConfig := ConvertClusterToV1Alpha1(cluster)

	// Convert all nodes recursively
	var nodes []*NodeConfig
	var bundles []*BundleConfig

	if cluster.Node != nil {
		c.convertNodeTreeToV1Alpha1(cluster.Node, &nodes, &bundles)
	}

	return clusterConfig, nodes, bundles
}

// convertNodeTreeToV1Alpha1 recursively converts nodes and their bundles
func (c *StackConverter) convertNodeTreeToV1Alpha1(node *stack.Node, nodes *[]*NodeConfig, bundles *[]*BundleConfig) {
	if node == nil {
		return
	}

	// Convert this node
	nodeConfig := ConvertNodeToV1Alpha1(node)
	*nodes = append(*nodes, nodeConfig)
	c.nodeMap[node.Name] = nodeConfig

	// Convert bundle if present
	if node.Bundle != nil {
		bundleConfig := ConvertBundleToV1Alpha1(node.Bundle)
		*bundles = append(*bundles, bundleConfig)
		c.bundleMap[node.Bundle.Name] = bundleConfig
	}

	// Recursively convert children
	for _, child := range node.Children {
		c.convertNodeTreeToV1Alpha1(child, nodes, bundles)
	}
}

// ConvertV1Alpha1ToClusterTree converts versioned configs back to a cluster tree
func (c *StackConverter) ConvertV1Alpha1ToClusterTree(
	clusterConfig *ClusterConfig,
	nodeConfigs []*NodeConfig,
	bundleConfigs []*BundleConfig,
	applications []*stack.Application,
) *stack.Cluster {
	if clusterConfig == nil {
		return nil
	}

	// Build maps for lookup
	nodeMap := make(map[string]*stack.Node)
	bundleMap := make(map[string]*stack.Bundle)

	// Convert all bundles first
	for _, bundleConfig := range bundleConfigs {
		bundle := ConvertV1Alpha1ToBundle(bundleConfig)
		bundleMap[bundleConfig.Metadata.Name] = bundle

		// Resolve applications
		for _, appRef := range bundleConfig.Spec.Applications {
			for _, app := range applications {
				if app.Name == appRef.Name {
					bundle.Applications = append(bundle.Applications, app)
					break
				}
			}
		}

		// Dependencies will be resolved after all bundles are created
	}

	// Resolve bundle dependencies
	for _, bundleConfig := range bundleConfigs {
		bundle := bundleMap[bundleConfig.Metadata.Name]
		for _, depRef := range bundleConfig.Spec.DependsOn {
			if dep, exists := bundleMap[depRef.Name]; exists {
				bundle.DependsOn = append(bundle.DependsOn, dep)
			}
		}
	}

	// Convert all nodes
	for _, nodeConfig := range nodeConfigs {
		node := ConvertV1Alpha1ToNode(nodeConfig)
		nodeMap[nodeConfig.Metadata.Name] = node

		// Resolve bundle reference
		if nodeConfig.Spec.Bundle != nil {
			if bundle, exists := bundleMap[nodeConfig.Spec.Bundle.Name]; exists {
				node.Bundle = bundle
			}
		}
	}

	// Build node tree structure
	for _, nodeConfig := range nodeConfigs {
		node := nodeMap[nodeConfig.Metadata.Name]

		// Add children
		for _, childRef := range nodeConfig.Spec.Children {
			if child, exists := nodeMap[childRef.Name]; exists {
				node.Children = append(node.Children, child)
				child.SetParent(node)
			}
		}
	}

	// Convert cluster and attach root node
	cluster := ConvertV1Alpha1ToCluster(clusterConfig)

	// Find and attach root node
	if clusterConfig.Spec.Node != nil {
		if rootNode, exists := nodeMap[clusterConfig.Spec.Node.Name]; exists {
			cluster.Node = rootNode
		}
	}

	return cluster
}
