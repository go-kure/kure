package v1alpha1_test

import (
	"strings"
	"testing"

	"github.com/go-kure/kure/pkg/stack"
	"github.com/go-kure/kure/pkg/stack/v1alpha1"
	_ "github.com/go-kure/kure/pkg/stack/generators/appworkload" // Register AppWorkload generator
)

func TestClusterV1Alpha1(t *testing.T) {
	t.Run("GetAPIVersion", func(t *testing.T) {
		cluster := &v1alpha1.ClusterV1Alpha1{}
		expected := "stack.gokure.dev/v1alpha1"
		if got := cluster.GetAPIVersion(); got != expected {
			t.Errorf("GetAPIVersion() = %v, want %v", got, expected)
		}
	})

	t.Run("GetKind", func(t *testing.T) {
		cluster := &v1alpha1.ClusterV1Alpha1{}
		expected := "Cluster"
		if got := cluster.GetKind(); got != expected {
			t.Errorf("GetKind() = %v, want %v", got, expected)
		}
	})

	t.Run("ToUnversioned", func(t *testing.T) {
		cluster := &v1alpha1.ClusterV1Alpha1{
			Spec: v1alpha1.ClusterSpec{
				GitOps: &v1alpha1.GitOpsConfig{
					Type: "flux",
					Bootstrap: &v1alpha1.BootstrapConfig{
						Enabled:     true,
						FluxVersion: "v2.0.0",
					},
				},
			},
		}
		cluster.Name = "test-cluster"

		unversioned := cluster.ToUnversioned()
		if unversioned.Name != "test-cluster" {
			t.Errorf("Name = %v, want test-cluster", unversioned.Name)
		}
		if unversioned.GitOps.Type != "flux" {
			t.Errorf("GitOps.Type = %v, want flux", unversioned.GitOps.Type)
		}
		if !unversioned.GitOps.Bootstrap.Enabled {
			t.Error("GitOps.Bootstrap.Enabled should be true")
		}
	})
}

func TestNodeV1Alpha1(t *testing.T) {
	t.Run("GetAPIVersion", func(t *testing.T) {
		node := &v1alpha1.NodeV1Alpha1{}
		expected := "stack.gokure.dev/v1alpha1"
		if got := node.GetAPIVersion(); got != expected {
			t.Errorf("GetAPIVersion() = %v, want %v", got, expected)
		}
	})

	t.Run("GetKind", func(t *testing.T) {
		node := &v1alpha1.NodeV1Alpha1{}
		expected := "Node"
		if got := node.GetKind(); got != expected {
			t.Errorf("GetKind() = %v, want %v", got, expected)
		}
	})

	t.Run("ToUnversioned", func(t *testing.T) {
		node := &v1alpha1.NodeV1Alpha1{
			Spec: v1alpha1.NodeSpec{
				ParentPath: "cluster/infrastructure",
				Labels: map[string]string{
					"tier": "infrastructure",
				},
				Interval: "5m",
			},
		}
		node.Name = "test-node"

		unversioned := node.ToUnversioned()
		if unversioned.Name != "test-node" {
			t.Errorf("Name = %v, want test-node", unversioned.Name)
		}
		if unversioned.ParentPath != "cluster/infrastructure" {
			t.Errorf("ParentPath = %v, want cluster/infrastructure", unversioned.ParentPath)
		}
		// Note: Labels, Interval, etc. are not directly supported in the current Node struct
		// They are preserved in the v1alpha1 format for future use
	})
}

func TestBundleV1Alpha1(t *testing.T) {
	t.Run("GetAPIVersion", func(t *testing.T) {
		bundle := &v1alpha1.BundleV1Alpha1{}
		expected := "stack.gokure.dev/v1alpha1"
		if got := bundle.GetAPIVersion(); got != expected {
			t.Errorf("GetAPIVersion() = %v, want %v", got, expected)
		}
	})

	t.Run("GetKind", func(t *testing.T) {
		bundle := &v1alpha1.BundleV1Alpha1{}
		expected := "Bundle"
		if got := bundle.GetKind(); got != expected {
			t.Errorf("GetKind() = %v, want %v", got, expected)
		}
	})

	t.Run("ToUnversioned", func(t *testing.T) {
		bundle := &v1alpha1.BundleV1Alpha1{
			Spec: v1alpha1.BundleSpec{
				ParentPath: "node/apps",
				Interval:   "10m",
				Labels: map[string]string{
					"app": "web",
				},
				SourceRef: &v1alpha1.SourceRef{
					Kind:      "GitRepository",
					Name:      "main-repo",
					Namespace: "flux-system",
				},
			},
		}
		bundle.Name = "test-bundle"

		unversioned := bundle.ToUnversioned()
		if unversioned.Name != "test-bundle" {
			t.Errorf("Name = %v, want test-bundle", unversioned.Name)
		}
		if unversioned.ParentPath != "node/apps" {
			t.Errorf("ParentPath = %v, want node/apps", unversioned.ParentPath)
		}
		if unversioned.SourceRef.Kind != "GitRepository" {
			t.Errorf("SourceRef.Kind = %v, want GitRepository", unversioned.SourceRef.Kind)
		}
	})
}

func TestYAMLParsing(t *testing.T) {
	t.Run("Parse Cluster YAML", func(t *testing.T) {
		yamlContent := `
apiVersion: stack.gokure.dev/v1alpha1
kind: Cluster
name: production
spec:
  gitops:
    type: flux
    bootstrap:
      enabled: true
      fluxVersion: v2.0.0
      components:
        - source-controller
        - kustomize-controller
`

		data, err := v1alpha1.ParseAndConvertCluster([]byte(yamlContent))
		if err != nil {
			t.Fatalf("Failed to parse cluster: %v", err)
		}

		if data.Name != "production" {
			t.Errorf("Name = %v, want production", data.Name)
		}
		if data.GitOps.Type != "flux" {
			t.Errorf("GitOps.Type = %v, want flux", data.GitOps.Type)
		}
		if len(data.GitOps.Bootstrap.Components) != 2 {
			t.Errorf("Components length = %v, want 2", len(data.GitOps.Bootstrap.Components))
		}
	})

	t.Run("Parse Node YAML", func(t *testing.T) {
		yamlContent := `
apiVersion: stack.gokure.dev/v1alpha1
kind: Node
name: infrastructure
spec:
  parentPath: cluster
  labels:
    tier: infrastructure
  interval: 5m
  sourceRef:
    kind: GitRepository
    name: infra-repo
    namespace: flux-system
`

		data, err := v1alpha1.ParseAndConvertNode([]byte(yamlContent))
		if err != nil {
			t.Fatalf("Failed to parse node: %v", err)
		}

		if data.Name != "infrastructure" {
			t.Errorf("Name = %v, want infrastructure", data.Name)
		}
		if data.ParentPath != "cluster" {
			t.Errorf("ParentPath = %v, want cluster", data.ParentPath)
		}
		// Note: Labels are not directly supported in the current Node struct
	})

	t.Run("Parse Bundle YAML", func(t *testing.T) {
		yamlContent := `
apiVersion: stack.gokure.dev/v1alpha1
kind: Bundle
name: web-apps
spec:
  parentPath: node/apps
  interval: 10m
  labels:
    type: web
  sourceRef:
    kind: GitRepository
    name: apps-repo
    namespace: flux-system
  applications:
    - inline:
        apiVersion: generators.gokure.dev/v1alpha1
        kind: AppWorkload
        metadata:
          name: nginx
          namespace: default
        spec:
          workload: Deployment
          replicas: 3
`

		data, err := v1alpha1.ParseAndConvertBundle([]byte(yamlContent))
		if err != nil {
			t.Fatalf("Failed to parse bundle: %v", err)
		}

		if data.Name != "web-apps" {
			t.Errorf("Name = %v, want web-apps", data.Name)
		}
		if data.ParentPath != "node/apps" {
			t.Errorf("ParentPath = %v, want node/apps", data.ParentPath)
		}
		if data.Labels["type"] != "web" {
			t.Errorf("Labels[type] = %v, want web", data.Labels["type"])
		}
		// Note: Applications parsing would require the generators to be registered
	})
}

func TestMultiDocumentParsing(t *testing.T) {
	yamlContent := `
apiVersion: stack.gokure.dev/v1alpha1
kind: Cluster
name: production
spec:
  gitops:
    type: flux
---
apiVersion: stack.gokure.dev/v1alpha1
kind: Node
name: infrastructure
spec:
  parentPath: cluster
---
apiVersion: stack.gokure.dev/v1alpha1
kind: Bundle
name: core-services
spec:
  parentPath: infrastructure
`

	reader := strings.NewReader(yamlContent)
	documents, err := v1alpha1.ParseStackDocuments(reader)
	if err != nil {
		t.Fatalf("Failed to parse documents: %v", err)
	}

	if len(documents) != 3 {
		t.Fatalf("Expected 3 documents, got %d", len(documents))
	}

	// Check first document (Cluster)
	if documents[0].Kind != "Cluster" {
		t.Errorf("Document 0 kind = %v, want Cluster", documents[0].Kind)
	}
	cluster, ok := documents[0].Resource.(*v1alpha1.ClusterV1Alpha1)
	if !ok {
		t.Errorf("Document 0 resource type = %T, want *v1alpha1.ClusterV1Alpha1", documents[0].Resource)
	} else if cluster.Name != "production" {
		t.Errorf("Cluster name = %v, want production", cluster.Name)
	}

	// Check second document (Node)
	if documents[1].Kind != "Node" {
		t.Errorf("Document 1 kind = %v, want Node", documents[1].Kind)
	}
	node, ok := documents[1].Resource.(*v1alpha1.NodeV1Alpha1)
	if !ok {
		t.Errorf("Document 1 resource type = %T, want *v1alpha1.NodeV1Alpha1", documents[1].Resource)
	} else if node.Name != "infrastructure" {
		t.Errorf("Node name = %v, want infrastructure", node.Name)
	}

	// Check third document (Bundle)
	if documents[2].Kind != "Bundle" {
		t.Errorf("Document 2 kind = %v, want Bundle", documents[2].Kind)
	}
	bundle, ok := documents[2].Resource.(*v1alpha1.BundleV1Alpha1)
	if !ok {
		t.Errorf("Document 2 resource type = %T, want *v1alpha1.BundleV1Alpha1", documents[2].Resource)
	} else if bundle.Name != "core-services" {
		t.Errorf("Bundle name = %v, want core-services", bundle.Name)
	}
}

func TestConvertDocument(t *testing.T) {
	cluster := &v1alpha1.ClusterV1Alpha1{}
	cluster.Name = "test"
	cluster.Spec.GitOps = &v1alpha1.GitOpsConfig{Type: "flux"}

	doc := v1alpha1.StackDocument{
		APIVersion: "stack.gokure.dev/v1alpha1",
		Kind:       "Cluster",
		Resource:   cluster,
	}

	converted, err := v1alpha1.ConvertDocument(doc)
	if err != nil {
		t.Fatalf("Failed to convert document: %v", err)
	}

	unversionedCluster, ok := converted.(*stack.Cluster)
	if !ok {
		t.Fatalf("Converted type = %T, want *stack.Cluster", converted)
	}

	if unversionedCluster.Name != "test" {
		t.Errorf("Name = %v, want test", unversionedCluster.Name)
	}
	if unversionedCluster.GitOps.Type != "flux" {
		t.Errorf("GitOps.Type = %v, want flux", unversionedCluster.GitOps.Type)
	}
}