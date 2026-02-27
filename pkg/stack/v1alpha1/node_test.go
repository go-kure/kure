package v1alpha1

import (
	"testing"

	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/go-kure/kure/internal/gvk"
)

func TestNodeConfig(t *testing.T) {
	tests := []struct {
		name    string
		node    *NodeConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid node config",
			node: &NodeConfig{
				APIVersion: "stack.gokure.dev/v1alpha1",
				Kind:       "Node",
				Metadata: gvk.BaseMetadata{
					Name: "infrastructure",
				},
				Spec: NodeSpec{
					ParentPath: "cluster",
					Children: []NodeReference{
						{Name: "monitoring"},
						{Name: "networking"},
					},
					Bundle: &BundleReference{
						Name: "infra-bundle",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "node with package ref",
			node: &NodeConfig{
				APIVersion: "stack.gokure.dev/v1alpha1",
				Kind:       "Node",
				Metadata: gvk.BaseMetadata{
					Name: "apps",
				},
				Spec: NodeSpec{
					PackageRef: &schema.GroupVersionKind{
						Group:   "packages.example.com",
						Version: "v1",
						Kind:    "AppPackage",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "missing name",
			node: &NodeConfig{
				APIVersion: "stack.gokure.dev/v1alpha1",
				Kind:       "Node",
				Spec:       NodeSpec{},
			},
			wantErr: true,
			errMsg:  "metadata.name",
		},
		{
			name: "empty child name",
			node: &NodeConfig{
				APIVersion: "stack.gokure.dev/v1alpha1",
				Kind:       "Node",
				Metadata: gvk.BaseMetadata{
					Name: "parent",
				},
				Spec: NodeSpec{
					Children: []NodeReference{
						{Name: ""},
					},
				},
			},
			wantErr: true,
			errMsg:  "child node name cannot be empty",
		},
		{
			name: "duplicate children",
			node: &NodeConfig{
				APIVersion: "stack.gokure.dev/v1alpha1",
				Kind:       "Node",
				Metadata: gvk.BaseMetadata{
					Name: "parent",
				},
				Spec: NodeSpec{
					Children: []NodeReference{
						{Name: "child1"},
						{Name: "child1"},
					},
				},
			},
			wantErr: true,
			errMsg:  "duplicate child node name",
		},
		{
			name: "invalid package ref",
			node: &NodeConfig{
				APIVersion: "stack.gokure.dev/v1alpha1",
				Kind:       "Node",
				Metadata: gvk.BaseMetadata{
					Name: "node",
				},
				Spec: NodeSpec{
					PackageRef: &schema.GroupVersionKind{
						Group:   "packages.example.com",
						Version: "v1",
						Kind:    "", // Empty kind
					},
				},
			},
			wantErr: true,
			errMsg:  "packageRef kind cannot be empty",
		},
		{
			name:    "nil node",
			node:    nil,
			wantErr: true,
			errMsg:  "node config is nil",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.node.Validate()

			if tt.wantErr {
				if err == nil {
					t.Error("expected error but got nil")
				} else if tt.errMsg != "" && !contains(err.Error(), tt.errMsg) {
					t.Errorf("expected error containing %q, got %q", tt.errMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestNodeConfig_GettersSetters(t *testing.T) {
	node := NewNodeConfig("test-node")

	// Test initial values
	if node.GetName() != "test-node" {
		t.Errorf("expected name 'test-node', got %s", node.GetName())
	}

	if node.GetPath() != "test-node" {
		t.Errorf("expected path 'test-node', got %s", node.GetPath())
	}

	// Test with parent path
	node.Spec.ParentPath = "cluster/infrastructure"
	if node.GetPath() != "cluster/infrastructure/test-node" {
		t.Errorf("expected path 'cluster/infrastructure/test-node', got %s", node.GetPath())
	}

	// Test setters
	node.SetName("new-name")
	if node.GetName() != "new-name" {
		t.Errorf("expected name 'new-name', got %s", node.GetName())
	}

	node.SetNamespace("test-namespace")
	if node.GetNamespace() != "test-namespace" {
		t.Errorf("expected namespace 'test-namespace', got %s", node.GetNamespace())
	}
}

func TestNodeConfig_Helpers(t *testing.T) {
	node := NewNodeConfig("parent")

	// Test AddChild
	node.AddChild("child1")
	node.AddChild("child2")

	if len(node.Spec.Children) != 2 {
		t.Errorf("expected 2 children, got %d", len(node.Spec.Children))
	}

	if node.Spec.Children[0].Name != "child1" {
		t.Errorf("expected first child 'child1', got %s", node.Spec.Children[0].Name)
	}

	// Test SetBundle
	node.SetBundle("test-bundle")

	if node.Spec.Bundle == nil {
		t.Fatal("expected bundle to be set")
	}

	if node.Spec.Bundle.Name != "test-bundle" {
		t.Errorf("expected bundle name 'test-bundle', got %s", node.Spec.Bundle.Name)
	}

	// Test SetPackageRef
	node.SetPackageRef("packages.example.com", "v1", "TestPackage")

	if node.Spec.PackageRef == nil {
		t.Fatal("expected package ref to be set")
	}

	if node.Spec.PackageRef.Kind != "TestPackage" {
		t.Errorf("expected package ref kind 'TestPackage', got %s", node.Spec.PackageRef.Kind)
	}
}

func TestNodeConfig_Conversion(t *testing.T) {
	node := NewNodeConfig("test-node")
	node.Spec.ParentPath = "cluster"
	node.AddChild("child1")
	node.SetBundle("bundle1")

	// Test ConvertTo
	converted, err := node.ConvertTo("v1alpha1")
	if err != nil {
		t.Errorf("unexpected error converting to v1alpha1: %v", err)
	}

	if converted != node {
		t.Error("expected same instance when converting to same version")
	}

	// Test unsupported version
	_, err = node.ConvertTo("v2")
	if err == nil {
		t.Error("expected error for unsupported version")
	}

	// Test ConvertFrom
	newNode := &NodeConfig{}
	err = newNode.ConvertFrom(node)
	if err != nil {
		t.Errorf("unexpected error converting from NodeConfig: %v", err)
	}

	if newNode.GetName() != node.GetName() {
		t.Errorf("expected name %s, got %s", node.GetName(), newNode.GetName())
	}

	if len(newNode.Spec.Children) != len(node.Spec.Children) {
		t.Errorf("expected %d children, got %d", len(node.Spec.Children), len(newNode.Spec.Children))
	}
}

func TestNodeConfig_HierarchicalPath(t *testing.T) {
	tests := []struct {
		name       string
		nodeName   string
		parentPath string
		wantPath   string
	}{
		{
			name:       "root node",
			nodeName:   "cluster",
			parentPath: "",
			wantPath:   "cluster",
		},
		{
			name:       "first level child",
			nodeName:   "infrastructure",
			parentPath: "cluster",
			wantPath:   "cluster/infrastructure",
		},
		{
			name:       "deep nested node",
			nodeName:   "prometheus",
			parentPath: "cluster/infrastructure/monitoring",
			wantPath:   "cluster/infrastructure/monitoring/prometheus",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := NewNodeConfig(tt.nodeName)
			node.Spec.ParentPath = tt.parentPath

			gotPath := node.GetPath()
			if gotPath != tt.wantPath {
				t.Errorf("expected path %q, got %q", tt.wantPath, gotPath)
			}
		})
	}
}
