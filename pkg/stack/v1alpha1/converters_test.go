package v1alpha1

import (
	"reflect"
	"testing"

	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/go-kure/kure/internal/gvk"
	"github.com/go-kure/kure/pkg/stack"
)

func TestConvertClusterToV1Alpha1(t *testing.T) {
	tests := []struct {
		name     string
		cluster  *stack.Cluster
		expected *ClusterConfig
	}{
		{
			name:     "nil cluster",
			cluster:  nil,
			expected: nil,
		},
		{
			name: "simple cluster",
			cluster: &stack.Cluster{
				Name: "test-cluster",
			},
			expected: &ClusterConfig{
				APIVersion: "stack.gokure.dev/v1alpha1",
				Kind:       "Cluster",
				Metadata: gvk.BaseMetadata{
					Name: "test-cluster",
				},
				Spec: ClusterSpec{},
			},
		},
		{
			name: "cluster with node",
			cluster: &stack.Cluster{
				Name: "test-cluster",
				Node: &stack.Node{
					Name: "root-node",
				},
			},
			expected: &ClusterConfig{
				APIVersion: "stack.gokure.dev/v1alpha1",
				Kind:       "Cluster",
				Metadata: gvk.BaseMetadata{
					Name: "test-cluster",
				},
				Spec: ClusterSpec{
					Node: &NodeReference{
						Name:       "root-node",
						APIVersion: "stack.gokure.dev/v1alpha1",
					},
				},
			},
		},
		{
			name: "cluster with flux gitops",
			cluster: &stack.Cluster{
				Name: "flux-cluster",
				GitOps: &stack.GitOpsConfig{
					Type: "flux",
					Bootstrap: &stack.BootstrapConfig{
						Enabled:         true,
						FluxMode:        "gitops-toolkit",
						FluxVersion:     "v2.0.0",
						Components:      []string{"source-controller", "kustomize-controller"},
						Registry:        "ghcr.io/fluxcd",
						ImagePullSecret: "flux-pull-secret",
						SourceURL:       "oci://ghcr.io/example/flux-manifests",
						SourceRef:       "v1.0.0",
					},
				},
			},
			expected: &ClusterConfig{
				APIVersion: "stack.gokure.dev/v1alpha1",
				Kind:       "Cluster",
				Metadata: gvk.BaseMetadata{
					Name: "flux-cluster",
				},
				Spec: ClusterSpec{
					GitOps: &GitOpsConfig{
						Type: "flux",
						Bootstrap: &BootstrapConfig{
							Enabled:         true,
							FluxMode:        "gitops-toolkit",
							FluxVersion:     "v2.0.0",
							Components:      []string{"source-controller", "kustomize-controller"},
							Registry:        "ghcr.io/fluxcd",
							ImagePullSecret: "flux-pull-secret",
							SourceURL:       "oci://ghcr.io/example/flux-manifests",
							SourceRef:       "v1.0.0",
						},
					},
				},
			},
		},
		{
			name: "cluster with argocd gitops",
			cluster: &stack.Cluster{
				Name: "argo-cluster",
				GitOps: &stack.GitOpsConfig{
					Type: "argocd",
					Bootstrap: &stack.BootstrapConfig{
						Enabled:         true,
						ArgoCDVersion:   "v2.8.0",
						ArgoCDNamespace: "argocd",
					},
				},
			},
			expected: &ClusterConfig{
				APIVersion: "stack.gokure.dev/v1alpha1",
				Kind:       "Cluster",
				Metadata: gvk.BaseMetadata{
					Name: "argo-cluster",
				},
				Spec: ClusterSpec{
					GitOps: &GitOpsConfig{
						Type: "argocd",
						Bootstrap: &BootstrapConfig{
							Enabled:         true,
							ArgoCDVersion:   "v2.8.0",
							ArgoCDNamespace: "argocd",
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ConvertClusterToV1Alpha1(tt.cluster)

			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("ConvertClusterToV1Alpha1() = %+v, want %+v", result, tt.expected)
			}
		})
	}
}

func TestConvertV1Alpha1ToCluster(t *testing.T) {
	tests := []struct {
		name     string
		config   *ClusterConfig
		expected *stack.Cluster
	}{
		{
			name:     "nil config",
			config:   nil,
			expected: nil,
		},
		{
			name: "simple cluster config",
			config: &ClusterConfig{
				APIVersion: "stack.gokure.dev/v1alpha1",
				Kind:       "Cluster",
				Metadata: gvk.BaseMetadata{
					Name: "test-cluster",
				},
			},
			expected: &stack.Cluster{
				Name: "test-cluster",
			},
		},
		{
			name: "cluster config with gitops",
			config: &ClusterConfig{
				Metadata: gvk.BaseMetadata{
					Name: "gitops-cluster",
				},
				Spec: ClusterSpec{
					GitOps: &GitOpsConfig{
						Type: "flux",
						Bootstrap: &BootstrapConfig{
							Enabled:     true,
							FluxVersion: "v2.0.0",
						},
					},
				},
			},
			expected: &stack.Cluster{
				Name: "gitops-cluster",
				GitOps: &stack.GitOpsConfig{
					Type: "flux",
					Bootstrap: &stack.BootstrapConfig{
						Enabled:     true,
						FluxVersion: "v2.0.0",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ConvertV1Alpha1ToCluster(tt.config)

			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("ConvertV1Alpha1ToCluster() = %+v, want %+v", result, tt.expected)
			}
		})
	}
}

func TestConvertNodeToV1Alpha1(t *testing.T) {
	tests := []struct {
		name     string
		node     *stack.Node
		expected *NodeConfig
	}{
		{
			name:     "nil node",
			node:     nil,
			expected: nil,
		},
		{
			name: "simple node",
			node: &stack.Node{
				Name:       "test-node",
				ParentPath: "cluster/infrastructure",
				PackageRef: &schema.GroupVersionKind{
					Group:   "apps",
					Version: "v1",
					Kind:    "Package",
				},
			},
			expected: &NodeConfig{
				APIVersion: "stack.gokure.dev/v1alpha1",
				Kind:       "Node",
				Metadata: gvk.BaseMetadata{
					Name: "test-node",
				},
				Spec: NodeSpec{
					ParentPath: "cluster/infrastructure",
					PackageRef: &schema.GroupVersionKind{
						Group:   "apps",
						Version: "v1",
						Kind:    "Package",
					},
				},
			},
		},
		{
			name: "node with children",
			node: &stack.Node{
				Name: "parent-node",
				Children: []*stack.Node{
					{Name: "child1"},
					{Name: "child2"},
				},
			},
			expected: &NodeConfig{
				APIVersion: "stack.gokure.dev/v1alpha1",
				Kind:       "Node",
				Metadata: gvk.BaseMetadata{
					Name: "parent-node",
				},
				Spec: NodeSpec{
					Children: []NodeReference{
						{Name: "child1", APIVersion: "stack.gokure.dev/v1alpha1"},
						{Name: "child2", APIVersion: "stack.gokure.dev/v1alpha1"},
					},
				},
			},
		},
		{
			name: "node with bundle",
			node: &stack.Node{
				Name: "node-with-bundle",
				Bundle: &stack.Bundle{
					Name: "app-bundle",
				},
			},
			expected: &NodeConfig{
				APIVersion: "stack.gokure.dev/v1alpha1",
				Kind:       "Node",
				Metadata: gvk.BaseMetadata{
					Name: "node-with-bundle",
				},
				Spec: NodeSpec{
					Bundle: &BundleReference{
						Name:       "app-bundle",
						APIVersion: "stack.gokure.dev/v1alpha1",
					},
				},
			},
		},
		{
			name: "node with nil children filtered",
			node: &stack.Node{
				Name: "parent",
				Children: []*stack.Node{
					{Name: "child1"},
					nil,
					{Name: "child2"},
				},
			},
			expected: &NodeConfig{
				APIVersion: "stack.gokure.dev/v1alpha1",
				Kind:       "Node",
				Metadata: gvk.BaseMetadata{
					Name: "parent",
				},
				Spec: NodeSpec{
					Children: []NodeReference{
						{Name: "child1", APIVersion: "stack.gokure.dev/v1alpha1"},
						{Name: "child2", APIVersion: "stack.gokure.dev/v1alpha1"},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ConvertNodeToV1Alpha1(tt.node)

			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("ConvertNodeToV1Alpha1() = %+v, want %+v", result, tt.expected)
			}
		})
	}
}

func TestConvertBundleToV1Alpha1(t *testing.T) {
	tests := []struct {
		name     string
		bundle   *stack.Bundle
		expected *BundleConfig
	}{
		{
			name:     "nil bundle",
			bundle:   nil,
			expected: nil,
		},
		{
			name: "simple bundle",
			bundle: &stack.Bundle{
				Name:       "test-bundle",
				ParentPath: "cluster/apps",
				Interval:   "5m",
				Labels: map[string]string{
					"env": "prod",
				},
			},
			expected: &BundleConfig{
				APIVersion: "stack.gokure.dev/v1alpha1",
				Kind:       "Bundle",
				Metadata: gvk.BaseMetadata{
					Name: "test-bundle",
				},
				Spec: BundleSpec{
					ParentPath: "cluster/apps",
					Interval:   "5m",
					Labels: map[string]string{
						"env": "prod",
					},
				},
			},
		},
		{
			name: "bundle with source ref",
			bundle: &stack.Bundle{
				Name: "sourced-bundle",
				SourceRef: &stack.SourceRef{
					Kind:      "GitRepository",
					Name:      "app-repo",
					Namespace: "flux-system",
				},
			},
			expected: &BundleConfig{
				APIVersion: "stack.gokure.dev/v1alpha1",
				Kind:       "Bundle",
				Metadata: gvk.BaseMetadata{
					Name: "sourced-bundle",
				},
				Spec: BundleSpec{
					SourceRef: &SourceRef{
						Kind:      "GitRepository",
						Name:      "app-repo",
						Namespace: "flux-system",
					},
				},
			},
		},
		{
			name: "bundle with source ref URL/Tag/Branch",
			bundle: &stack.Bundle{
				Name: "oci-bundle",
				SourceRef: &stack.SourceRef{
					Kind:      "OCIRepository",
					Name:      "oci-source",
					Namespace: "flux-system",
					URL:       "oci://registry.example.com/manifests",
					Tag:       "v1.0.0",
				},
			},
			expected: &BundleConfig{
				APIVersion: "stack.gokure.dev/v1alpha1",
				Kind:       "Bundle",
				Metadata: gvk.BaseMetadata{
					Name: "oci-bundle",
				},
				Spec: BundleSpec{
					SourceRef: &SourceRef{
						Kind:      "OCIRepository",
						Name:      "oci-source",
						Namespace: "flux-system",
						URL:       "oci://registry.example.com/manifests",
						Tag:       "v1.0.0",
					},
				},
			},
		},
		{
			name: "bundle with dependencies",
			bundle: &stack.Bundle{
				Name: "dependent-bundle",
				DependsOn: []*stack.Bundle{
					{Name: "infra"},
					{Name: "monitoring"},
				},
			},
			expected: &BundleConfig{
				APIVersion: "stack.gokure.dev/v1alpha1",
				Kind:       "Bundle",
				Metadata: gvk.BaseMetadata{
					Name: "dependent-bundle",
				},
				Spec: BundleSpec{
					DependsOn: []BundleReference{
						{Name: "infra", APIVersion: "stack.gokure.dev/v1alpha1"},
						{Name: "monitoring", APIVersion: "stack.gokure.dev/v1alpha1"},
					},
				},
			},
		},
		{
			name: "bundle with applications",
			bundle: &stack.Bundle{
				Name: "app-bundle",
				Applications: []*stack.Application{
					{Name: "app1"},
					{Name: "app2"},
				},
			},
			expected: &BundleConfig{
				APIVersion: "stack.gokure.dev/v1alpha1",
				Kind:       "Bundle",
				Metadata: gvk.BaseMetadata{
					Name: "app-bundle",
				},
				Spec: BundleSpec{
					Applications: []ApplicationReference{
						{Name: "app1", APIVersion: "generators.gokure.dev/v1alpha1", Kind: "Application"},
						{Name: "app2", APIVersion: "generators.gokure.dev/v1alpha1", Kind: "Application"},
					},
				},
			},
		},
		{
			name: "bundle with nil dependencies filtered",
			bundle: &stack.Bundle{
				Name: "bundle",
				DependsOn: []*stack.Bundle{
					{Name: "dep1"},
					nil,
					{Name: "dep2"},
				},
				Applications: []*stack.Application{
					{Name: "app1"},
					nil,
					{Name: "app2"},
				},
			},
			expected: &BundleConfig{
				APIVersion: "stack.gokure.dev/v1alpha1",
				Kind:       "Bundle",
				Metadata: gvk.BaseMetadata{
					Name: "bundle",
				},
				Spec: BundleSpec{
					DependsOn: []BundleReference{
						{Name: "dep1", APIVersion: "stack.gokure.dev/v1alpha1"},
						{Name: "dep2", APIVersion: "stack.gokure.dev/v1alpha1"},
					},
					Applications: []ApplicationReference{
						{Name: "app1", APIVersion: "generators.gokure.dev/v1alpha1", Kind: "Application"},
						{Name: "app2", APIVersion: "generators.gokure.dev/v1alpha1", Kind: "Application"},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ConvertBundleToV1Alpha1(tt.bundle)

			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("ConvertBundleToV1Alpha1() = %+v, want %+v", result, tt.expected)
			}
		})
	}
}

func TestStackConverter_ConvertClusterTreeToV1Alpha1(t *testing.T) {
	// Create a complex cluster tree
	cluster := &stack.Cluster{
		Name: "test-cluster",
		Node: &stack.Node{
			Name: "root",
			Children: []*stack.Node{
				{
					Name: "infrastructure",
					Bundle: &stack.Bundle{
						Name:     "infra-bundle",
						Interval: "10m",
						Applications: []*stack.Application{
							{Name: "cert-manager"},
							{Name: "external-dns"},
						},
					},
					Children: []*stack.Node{
						{
							Name: "monitoring",
							Bundle: &stack.Bundle{
								Name: "monitoring-bundle",
								DependsOn: []*stack.Bundle{
									{Name: "infra-bundle"},
								},
								Applications: []*stack.Application{
									{Name: "prometheus"},
									{Name: "grafana"},
								},
							},
						},
					},
				},
				{
					Name: "applications",
					Children: []*stack.Node{
						{
							Name: "frontend",
							Bundle: &stack.Bundle{
								Name: "frontend-bundle",
								Applications: []*stack.Application{
									{Name: "web-app"},
								},
							},
						},
						{
							Name: "backend",
							Bundle: &stack.Bundle{
								Name: "backend-bundle",
								Applications: []*stack.Application{
									{Name: "api-server"},
									{Name: "database"},
								},
							},
						},
					},
				},
			},
		},
		GitOps: &stack.GitOpsConfig{
			Type: "flux",
			Bootstrap: &stack.BootstrapConfig{
				Enabled:     true,
				FluxVersion: "v2.0.0",
			},
		},
	}

	converter := NewStackConverter()
	clusterConfig, nodeConfigs, bundleConfigs := converter.ConvertClusterTreeToV1Alpha1(cluster)

	// Verify cluster config
	if clusterConfig == nil {
		t.Fatal("expected non-nil cluster config")
	}
	if clusterConfig.GetName() != "test-cluster" {
		t.Errorf("expected cluster name 'test-cluster', got %s", clusterConfig.GetName())
	}
	if clusterConfig.Spec.GitOps == nil || clusterConfig.Spec.GitOps.Type != "flux" {
		t.Error("expected flux gitops config")
	}

	// Verify node count
	expectedNodeCount := 6 // root, infrastructure, monitoring, applications, frontend, backend
	if len(nodeConfigs) != expectedNodeCount {
		t.Errorf("expected %d nodes, got %d", expectedNodeCount, len(nodeConfigs))
	}

	// Verify bundle count
	expectedBundleCount := 4 // infra-bundle, monitoring-bundle, frontend-bundle, backend-bundle
	if len(bundleConfigs) != expectedBundleCount {
		t.Errorf("expected %d bundles, got %d", expectedBundleCount, len(bundleConfigs))
	}

	// Verify node names
	nodeNames := make(map[string]bool)
	for _, node := range nodeConfigs {
		nodeNames[node.GetName()] = true
	}
	expectedNames := []string{"root", "infrastructure", "monitoring", "applications", "frontend", "backend"}
	for _, name := range expectedNames {
		if !nodeNames[name] {
			t.Errorf("missing expected node: %s", name)
		}
	}

	// Verify bundle names and applications
	bundleApps := make(map[string]int)
	for _, bundle := range bundleConfigs {
		bundleApps[bundle.GetName()] = len(bundle.Spec.Applications)
	}

	if bundleApps["infra-bundle"] != 2 {
		t.Errorf("expected 2 apps in infra-bundle, got %d", bundleApps["infra-bundle"])
	}
	if bundleApps["monitoring-bundle"] != 2 {
		t.Errorf("expected 2 apps in monitoring-bundle, got %d", bundleApps["monitoring-bundle"])
	}
	if bundleApps["frontend-bundle"] != 1 {
		t.Errorf("expected 1 app in frontend-bundle, got %d", bundleApps["frontend-bundle"])
	}
	if bundleApps["backend-bundle"] != 2 {
		t.Errorf("expected 2 apps in backend-bundle, got %d", bundleApps["backend-bundle"])
	}
}

func TestStackConverter_ConvertV1Alpha1ToClusterTree(t *testing.T) {
	// Create versioned configs
	clusterConfig := &ClusterConfig{
		APIVersion: "stack.gokure.dev/v1alpha1",
		Kind:       "Cluster",
		Metadata: gvk.BaseMetadata{
			Name: "test-cluster",
		},
		Spec: ClusterSpec{
			Node: &NodeReference{
				Name: "root",
			},
			GitOps: &GitOpsConfig{
				Type: "argocd",
			},
		},
	}

	nodeConfigs := []*NodeConfig{
		{
			Metadata: gvk.BaseMetadata{Name: "root"},
			Spec: NodeSpec{
				Children: []NodeReference{
					{Name: "child1"},
					{Name: "child2"},
				},
			},
		},
		{
			Metadata: gvk.BaseMetadata{Name: "child1"},
			Spec: NodeSpec{
				Bundle: &BundleReference{Name: "bundle1"},
			},
		},
		{
			Metadata: gvk.BaseMetadata{Name: "child2"},
			Spec: NodeSpec{
				Bundle: &BundleReference{Name: "bundle2"},
			},
		},
	}

	bundleConfigs := []*BundleConfig{
		{
			Metadata: gvk.BaseMetadata{Name: "bundle1"},
			Spec: BundleSpec{
				Applications: []ApplicationReference{
					{Name: "app1"},
					{Name: "app2"},
				},
			},
		},
		{
			Metadata: gvk.BaseMetadata{Name: "bundle2"},
			Spec: BundleSpec{
				DependsOn: []BundleReference{
					{Name: "bundle1"},
				},
				Applications: []ApplicationReference{
					{Name: "app3"},
				},
			},
		},
	}

	applications := []*stack.Application{
		{Name: "app1"},
		{Name: "app2"},
		{Name: "app3"},
	}

	converter := NewStackConverter()
	cluster := converter.ConvertV1Alpha1ToClusterTree(clusterConfig, nodeConfigs, bundleConfigs, applications)

	// Verify cluster
	if cluster == nil {
		t.Fatal("expected non-nil cluster")
	}
	if cluster.Name != "test-cluster" {
		t.Errorf("expected cluster name 'test-cluster', got %s", cluster.Name)
	}
	if cluster.GitOps == nil || cluster.GitOps.Type != "argocd" {
		t.Error("expected argocd gitops config")
	}

	// Verify root node
	if cluster.Node == nil {
		t.Fatal("expected non-nil root node")
	}
	if cluster.Node.Name != "root" {
		t.Errorf("expected root node name 'root', got %s", cluster.Node.Name)
	}

	// Verify children
	if len(cluster.Node.Children) != 2 {
		t.Fatalf("expected 2 children, got %d", len(cluster.Node.Children))
	}

	// Verify child1
	child1 := cluster.Node.Children[0]
	if child1.Name != "child1" {
		t.Errorf("expected child1 name 'child1', got %s", child1.Name)
	}
	if child1.Bundle == nil || child1.Bundle.Name != "bundle1" {
		t.Error("expected bundle1 on child1")
	}
	if len(child1.Bundle.Applications) != 2 {
		t.Errorf("expected 2 applications in bundle1, got %d", len(child1.Bundle.Applications))
	}

	// Verify child2
	child2 := cluster.Node.Children[1]
	if child2.Name != "child2" {
		t.Errorf("expected child2 name 'child2', got %s", child2.Name)
	}
	if child2.Bundle == nil || child2.Bundle.Name != "bundle2" {
		t.Error("expected bundle2 on child2")
	}
	if len(child2.Bundle.Applications) != 1 {
		t.Errorf("expected 1 application in bundle2, got %d", len(child2.Bundle.Applications))
	}

	// Verify bundle dependencies
	if len(child2.Bundle.DependsOn) != 1 {
		t.Fatalf("expected 1 dependency for bundle2, got %d", len(child2.Bundle.DependsOn))
	}
	if child2.Bundle.DependsOn[0].Name != "bundle1" {
		t.Errorf("expected dependency on bundle1, got %s", child2.Bundle.DependsOn[0].Name)
	}
}

func TestStackConverter_HandleNilValues(t *testing.T) {
	converter := NewStackConverter()

	t.Run("nil cluster", func(t *testing.T) {
		clusterConfig, nodeConfigs, bundleConfigs := converter.ConvertClusterTreeToV1Alpha1(nil)
		if clusterConfig != nil || nodeConfigs != nil || bundleConfigs != nil {
			t.Error("expected all nil outputs for nil cluster")
		}
	})

	t.Run("nil cluster config", func(t *testing.T) {
		cluster := converter.ConvertV1Alpha1ToClusterTree(nil, nil, nil, nil)
		if cluster != nil {
			t.Error("expected nil cluster for nil config")
		}
	})

	t.Run("empty configs", func(t *testing.T) {
		clusterConfig := &ClusterConfig{
			Metadata: gvk.BaseMetadata{Name: "empty"},
		}
		cluster := converter.ConvertV1Alpha1ToClusterTree(clusterConfig, []*NodeConfig{}, []*BundleConfig{}, []*stack.Application{})

		if cluster == nil {
			t.Fatal("expected non-nil cluster")
		}
		if cluster.Name != "empty" {
			t.Errorf("expected cluster name 'empty', got %s", cluster.Name)
		}
		if cluster.Node != nil {
			t.Error("expected nil node for empty configs")
		}
	})
}

func TestStackConverter_ComplexScenarios(t *testing.T) {
	t.Run("deeply nested tree", func(t *testing.T) {
		// Create a deeply nested tree
		depth := 10
		root := &stack.Node{Name: "level0"}
		current := root

		for i := 1; i < depth; i++ {
			child := &stack.Node{
				Name: string(rune('a' + i)),
				Bundle: &stack.Bundle{
					Name: string(rune('a'+i)) + "-bundle",
					Applications: []*stack.Application{
						{Name: string(rune('a'+i)) + "-app"},
					},
				},
			}
			current.Children = []*stack.Node{child}
			current = child
		}

		cluster := &stack.Cluster{
			Name: "deep-cluster",
			Node: root,
		}

		converter := NewStackConverter()
		clusterConfig, nodeConfigs, bundleConfigs := converter.ConvertClusterTreeToV1Alpha1(cluster)

		if clusterConfig == nil {
			t.Fatal("expected non-nil cluster config")
		}
		if len(nodeConfigs) != depth {
			t.Errorf("expected %d nodes, got %d", depth, len(nodeConfigs))
		}
		if len(bundleConfigs) != depth-1 {
			t.Errorf("expected %d bundles, got %d", depth-1, len(bundleConfigs))
		}
	})

	t.Run("circular dependency detection", func(t *testing.T) {
		// Note: This test documents current behavior.
		// The converter doesn't prevent circular dependencies during conversion.
		// This would need to be handled at validation time.

		bundle1 := &stack.Bundle{Name: "bundle1"}
		bundle2 := &stack.Bundle{Name: "bundle2"}

		// Create circular dependency
		bundle1.DependsOn = []*stack.Bundle{bundle2}
		bundle2.DependsOn = []*stack.Bundle{bundle1}

		cluster := &stack.Cluster{
			Name: "circular-cluster",
			Node: &stack.Node{
				Name: "root",
				Children: []*stack.Node{
					{Name: "node1", Bundle: bundle1},
					{Name: "node2", Bundle: bundle2},
				},
			},
		}

		converter := NewStackConverter()
		_, _, bundleConfigs := converter.ConvertClusterTreeToV1Alpha1(cluster)

		// The converter should still convert the structure
		if len(bundleConfigs) != 2 {
			t.Errorf("expected 2 bundles despite circular dependency, got %d", len(bundleConfigs))
		}

		// Verify the circular dependency is preserved in the configs
		var b1, b2 *BundleConfig
		for _, b := range bundleConfigs {
			if b.GetName() == "bundle1" {
				b1 = b
			} else if b.GetName() == "bundle2" {
				b2 = b
			}
		}

		if b1 == nil || b2 == nil {
			t.Fatal("expected both bundles to be converted")
		}

		if len(b1.Spec.DependsOn) != 1 || b1.Spec.DependsOn[0].Name != "bundle2" {
			t.Error("expected bundle1 to depend on bundle2")
		}
		if len(b2.Spec.DependsOn) != 1 || b2.Spec.DependsOn[0].Name != "bundle1" {
			t.Error("expected bundle2 to depend on bundle1")
		}
	})

	t.Run("wide tree", func(t *testing.T) {
		// Create a tree with many siblings
		width := 100
		children := make([]*stack.Node, width)

		for i := 0; i < width; i++ {
			children[i] = &stack.Node{
				Name: string(rune('a'+(i%26))) + string(rune('0'+(i/26))),
				Bundle: &stack.Bundle{
					Name: "bundle" + string(rune('0'+i)),
				},
			}
		}

		cluster := &stack.Cluster{
			Name: "wide-cluster",
			Node: &stack.Node{
				Name:     "root",
				Children: children,
			},
		}

		converter := NewStackConverter()
		_, nodeConfigs, bundleConfigs := converter.ConvertClusterTreeToV1Alpha1(cluster)

		if len(nodeConfigs) != width+1 { // +1 for root
			t.Errorf("expected %d nodes, got %d", width+1, len(nodeConfigs))
		}
		if len(bundleConfigs) != width {
			t.Errorf("expected %d bundles, got %d", width, len(bundleConfigs))
		}
	})
}

func BenchmarkConvertClusterTreeToV1Alpha1(b *testing.B) {
	// Create a moderately complex cluster
	cluster := &stack.Cluster{
		Name: "bench-cluster",
		Node: &stack.Node{
			Name: "root",
			Children: []*stack.Node{
				{
					Name: "infra",
					Bundle: &stack.Bundle{
						Name: "infra-bundle",
						Applications: []*stack.Application{
							{Name: "app1"},
							{Name: "app2"},
						},
					},
					Children: []*stack.Node{
						{Name: "monitoring"},
						{Name: "logging"},
					},
				},
				{
					Name: "apps",
					Children: []*stack.Node{
						{Name: "frontend"},
						{Name: "backend"},
					},
				},
			},
		},
	}

	converter := NewStackConverter()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		converter.ConvertClusterTreeToV1Alpha1(cluster)
	}
}

func BenchmarkConvertV1Alpha1ToClusterTree(b *testing.B) {
	clusterConfig := &ClusterConfig{
		Metadata: gvk.BaseMetadata{Name: "bench"},
		Spec: ClusterSpec{
			Node: &NodeReference{Name: "root"},
		},
	}

	nodeConfigs := []*NodeConfig{
		{
			Metadata: gvk.BaseMetadata{Name: "root"},
			Spec: NodeSpec{
				Children: []NodeReference{{Name: "child1"}, {Name: "child2"}},
			},
		},
		{
			Metadata: gvk.BaseMetadata{Name: "child1"},
			Spec:     NodeSpec{Bundle: &BundleReference{Name: "bundle1"}},
		},
		{
			Metadata: gvk.BaseMetadata{Name: "child2"},
			Spec:     NodeSpec{Bundle: &BundleReference{Name: "bundle2"}},
		},
	}

	bundleConfigs := []*BundleConfig{
		{
			Metadata: gvk.BaseMetadata{Name: "bundle1"},
			Spec:     BundleSpec{Applications: []ApplicationReference{{Name: "app1"}}},
		},
		{
			Metadata: gvk.BaseMetadata{Name: "bundle2"},
			Spec:     BundleSpec{Applications: []ApplicationReference{{Name: "app2"}}},
		},
	}

	applications := []*stack.Application{{Name: "app1"}, {Name: "app2"}}

	converter := NewStackConverter()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		converter.ConvertV1Alpha1ToClusterTree(clusterConfig, nodeConfigs, bundleConfigs, applications)
	}
}

func TestSourceRefRoundTrip(t *testing.T) {
	original := &stack.Bundle{
		Name: "roundtrip-bundle",
		SourceRef: &stack.SourceRef{
			Kind:      "GitRepository",
			Name:      "my-repo",
			Namespace: "flux-system",
			URL:       "https://github.com/example/repo",
			Tag:       "v2.0.0",
			Branch:    "main",
		},
	}

	// Convert to v1alpha1
	v1alpha1Config := ConvertBundleToV1Alpha1(original)
	if v1alpha1Config == nil {
		t.Fatal("expected non-nil v1alpha1 config")
	}

	// Verify v1alpha1 SourceRef has URL/Tag/Branch
	sr := v1alpha1Config.Spec.SourceRef
	if sr == nil {
		t.Fatal("expected non-nil source ref in v1alpha1")
	}
	if sr.URL != "https://github.com/example/repo" {
		t.Errorf("expected URL to be preserved, got %q", sr.URL)
	}
	if sr.Tag != "v2.0.0" {
		t.Errorf("expected Tag to be preserved, got %q", sr.Tag)
	}
	if sr.Branch != "main" {
		t.Errorf("expected Branch to be preserved, got %q", sr.Branch)
	}

	// Convert back
	roundTripped := ConvertV1Alpha1ToBundle(v1alpha1Config)
	if roundTripped == nil {
		t.Fatal("expected non-nil bundle after round-trip")
	}

	// Verify all SourceRef fields survived the round-trip
	if roundTripped.SourceRef.Kind != original.SourceRef.Kind {
		t.Errorf("Kind mismatch: %q vs %q", roundTripped.SourceRef.Kind, original.SourceRef.Kind)
	}
	if roundTripped.SourceRef.Name != original.SourceRef.Name {
		t.Errorf("Name mismatch: %q vs %q", roundTripped.SourceRef.Name, original.SourceRef.Name)
	}
	if roundTripped.SourceRef.Namespace != original.SourceRef.Namespace {
		t.Errorf("Namespace mismatch: %q vs %q", roundTripped.SourceRef.Namespace, original.SourceRef.Namespace)
	}
	if roundTripped.SourceRef.URL != original.SourceRef.URL {
		t.Errorf("URL mismatch: %q vs %q", roundTripped.SourceRef.URL, original.SourceRef.URL)
	}
	if roundTripped.SourceRef.Tag != original.SourceRef.Tag {
		t.Errorf("Tag mismatch: %q vs %q", roundTripped.SourceRef.Tag, original.SourceRef.Tag)
	}
	if roundTripped.SourceRef.Branch != original.SourceRef.Branch {
		t.Errorf("Branch mismatch: %q vs %q", roundTripped.SourceRef.Branch, original.SourceRef.Branch)
	}
}
