package stack_test

import (
	"testing"

	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/go-kure/kure/pkg/stack"
)

func TestNewCluster(t *testing.T) {
	node := &stack.Node{Name: "root"}
	cluster := stack.NewCluster("test-cluster", node)

	if cluster == nil {
		t.Fatal("expected non-nil cluster")
	}

	if cluster.Name != "test-cluster" {
		t.Errorf("Name = %q, want %q", cluster.Name, "test-cluster")
	}

	if cluster.Node != node {
		t.Error("Node was not set correctly")
	}
}

func TestClusterGetters(t *testing.T) {
	gitOps := &stack.GitOpsConfig{Type: "flux"}
	node := &stack.Node{Name: "root"}
	cluster := &stack.Cluster{
		Name:   "test-cluster",
		Node:   node,
		GitOps: gitOps,
	}

	if cluster.GetName() != "test-cluster" {
		t.Errorf("GetName() = %q, want %q", cluster.GetName(), "test-cluster")
	}

	if cluster.GetNode() != node {
		t.Error("GetNode() returned wrong node")
	}

	if cluster.GetGitOps() != gitOps {
		t.Error("GetGitOps() returned wrong config")
	}
}

func TestClusterSetters(t *testing.T) {
	cluster := &stack.Cluster{}

	cluster.SetName("new-name")
	if cluster.Name != "new-name" {
		t.Errorf("SetName() didn't set name correctly")
	}

	node := &stack.Node{Name: "new-node"}
	cluster.SetNode(node)
	if cluster.Node != node {
		t.Error("SetNode() didn't set node correctly")
	}

	gitOps := &stack.GitOpsConfig{Type: "argocd"}
	cluster.SetGitOps(gitOps)
	if cluster.GitOps != gitOps {
		t.Error("SetGitOps() didn't set config correctly")
	}
}

func TestNodeGetters(t *testing.T) {
	parent := &stack.Node{Name: "parent"}
	child := &stack.Node{
		Name:       "child",
		ParentPath: "parent",
		Children:   []*stack.Node{},
		PackageRef: &schema.GroupVersionKind{Group: "test", Version: "v1", Kind: "Test"},
		Bundle:     &stack.Bundle{Name: "test-bundle"},
	}
	child.SetParent(parent)

	if child.GetName() != "child" {
		t.Errorf("GetName() = %q, want %q", child.GetName(), "child")
	}

	if child.GetParent() != parent {
		t.Error("GetParent() returned wrong parent")
	}

	if child.GetParentPath() != "parent" {
		t.Errorf("GetParentPath() = %q, want %q", child.GetParentPath(), "parent")
	}

	if child.GetChildren() == nil {
		t.Error("GetChildren() returned nil")
	}

	if child.GetPackageRef() == nil {
		t.Error("GetPackageRef() returned nil")
	}

	if child.GetBundle() == nil {
		t.Error("GetBundle() returned nil")
	}
}

func TestNodeSetters(t *testing.T) {
	node := &stack.Node{}

	node.SetName("test-node")
	if node.Name != "test-node" {
		t.Errorf("SetName() didn't set name correctly")
	}

	node.SetParentPath("parent/path")
	if node.ParentPath != "parent/path" {
		t.Errorf("SetParentPath() didn't set path correctly")
	}

	children := []*stack.Node{{Name: "child1"}, {Name: "child2"}}
	node.SetChildren(children)
	if len(node.Children) != 2 {
		t.Errorf("SetChildren() didn't set children correctly")
	}

	ref := &schema.GroupVersionKind{Group: "test", Version: "v1", Kind: "Test"}
	node.SetPackageRef(ref)
	if node.PackageRef != ref {
		t.Error("SetPackageRef() didn't set ref correctly")
	}

	bundle := &stack.Bundle{Name: "test-bundle"}
	node.SetBundle(bundle)
	if node.Bundle != bundle {
		t.Error("SetBundle() didn't set bundle correctly")
	}
}

func TestNodeSetParent(t *testing.T) {
	t.Run("set parent", func(t *testing.T) {
		parent := &stack.Node{Name: "parent"}
		child := &stack.Node{Name: "child"}

		child.SetParent(parent)

		if child.GetParent() != parent {
			t.Error("SetParent() didn't set parent correctly")
		}

		if child.ParentPath != "parent" {
			t.Errorf("ParentPath = %q, want %q", child.ParentPath, "parent")
		}
	})

	t.Run("set nil parent", func(t *testing.T) {
		node := &stack.Node{Name: "node", ParentPath: "old/path"}
		node.SetParent(nil)

		if node.GetParent() != nil {
			t.Error("SetParent(nil) didn't clear parent")
		}

		if node.ParentPath != "" {
			t.Errorf("ParentPath = %q, want empty string", node.ParentPath)
		}
	})
}

func TestNodeGetPath(t *testing.T) {
	tests := []struct {
		name         string
		node         *stack.Node
		expectedPath string
	}{
		{
			name:         "root node",
			node:         &stack.Node{Name: "root", ParentPath: ""},
			expectedPath: "root",
		},
		{
			name:         "child node",
			node:         &stack.Node{Name: "child", ParentPath: "parent"},
			expectedPath: "parent/child",
		},
		{
			name:         "deeply nested node",
			node:         &stack.Node{Name: "leaf", ParentPath: "root/parent/grandparent"},
			expectedPath: "root/parent/grandparent/leaf",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := tt.node.GetPath()
			if path != tt.expectedPath {
				t.Errorf("GetPath() = %q, want %q", path, tt.expectedPath)
			}
		})
	}
}

func TestNodeInitializePathMap(t *testing.T) {
	// Create a tree structure
	root := &stack.Node{Name: "root"}
	child1 := &stack.Node{Name: "child1"}
	child2 := &stack.Node{Name: "child2"}
	grandchild := &stack.Node{Name: "grandchild"}

	// Set up hierarchy
	child1.SetParent(root)
	child2.SetParent(root)
	grandchild.SetParent(child1)

	root.Children = []*stack.Node{child1, child2}
	child1.Children = []*stack.Node{grandchild}

	// Initialize path map
	root.InitializePathMap()

	// Verify paths are correct
	if root.GetPath() != "root" {
		t.Errorf("root.GetPath() = %q, want %q", root.GetPath(), "root")
	}

	if child1.GetPath() != "root/child1" {
		t.Errorf("child1.GetPath() = %q, want %q", child1.GetPath(), "root/child1")
	}

	if child2.GetPath() != "root/child2" {
		t.Errorf("child2.GetPath() = %q, want %q", child2.GetPath(), "root/child2")
	}

	if grandchild.GetPath() != "root/child1/grandchild" {
		t.Errorf("grandchild.GetPath() = %q, want %q", grandchild.GetPath(), "root/child1/grandchild")
	}
}

func TestBootstrapConfig(t *testing.T) {
	config := &stack.BootstrapConfig{
		Enabled:         true,
		FluxMode:        "flux-operator",
		FluxVersion:     "v2.0.0",
		Components:      []string{"source-controller", "kustomize-controller"},
		Registry:        "ghcr.io/fluxcd",
		ImagePullSecret: "my-secret",
		SourceURL:       "oci://registry.example.com/flux-system",
		SourceRef:       "latest",
		ArgoCDVersion:   "v2.8.0",
		ArgoCDNamespace: "argocd",
	}

	if !config.Enabled {
		t.Error("expected Enabled to be true")
	}

	if config.FluxMode != "flux-operator" {
		t.Errorf("FluxMode = %q, want %q", config.FluxMode, "flux-operator")
	}

	if len(config.Components) != 2 {
		t.Errorf("Components count = %d, want 2", len(config.Components))
	}
}

func TestGitOpsConfig(t *testing.T) {
	bootstrap := &stack.BootstrapConfig{
		Enabled:  true,
		FluxMode: "gotk",
	}

	config := &stack.GitOpsConfig{
		Type:      "flux",
		Bootstrap: bootstrap,
	}

	if config.Type != "flux" {
		t.Errorf("Type = %q, want %q", config.Type, "flux")
	}

	if config.Bootstrap != bootstrap {
		t.Error("Bootstrap was not set correctly")
	}
}
