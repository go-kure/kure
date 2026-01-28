package stack

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// TestBundleValidate exercises the Bundle validation logic against
// the various failure modes as well as the happy path.
func TestBundleValidate(t *testing.T) {
	// nil bundle should error
	var nilBundle *Bundle
	if err := nilBundle.Validate(); err == nil {
		t.Fatalf("expected error for nil bundle")
	}

	// empty name should error
	b := &Bundle{Name: "", Applications: []*Application{}}
	if err := b.Validate(); err == nil {
		t.Fatalf("expected validation error for empty name")
	}

	// nil application inside the slice should error
	b = &Bundle{Name: "test", Applications: []*Application{nil}}
	if err := b.Validate(); err == nil {
		t.Fatalf("expected error for nil application entry")
	}

	// valid bundle should pass
	app := NewApplication("app", "ns", &fakeConfig{})
	b = &Bundle{Name: "ok", Applications: []*Application{app}}
	if err := b.Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBundleGenerate(t *testing.T) {
	// Test empty bundle
	b := &Bundle{Name: "empty", Applications: []*Application{}}
	resources, err := b.Generate()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resources) != 0 {
		t.Fatalf("expected 0 resources, got %d", len(resources))
	}

	// Test bundle with applications
	obj1 := client.Object(&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod1"}})
	obj2 := client.Object(&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod2"}})
	app1 := NewApplication("app1", "ns1", &fakeConfig{objs: []*client.Object{&obj1}})
	app2 := NewApplication("app2", "ns2", &fakeConfig{objs: []*client.Object{&obj2}})
	b = &Bundle{
		Name:         "test-bundle",
		Applications: []*Application{app1, app2},
	}

	resources, err = b.Generate()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Each fakeConfig generates one resource
	expectedCount := 2
	if len(resources) != expectedCount {
		t.Fatalf("expected %d resources, got %d", expectedCount, len(resources))
	}
}

func TestBundleGetParentPath(t *testing.T) {
	tests := []struct {
		name       string
		bundle     *Bundle
		expected   string
	}{
		{
			name: "root bundle",
			bundle: &Bundle{
				Name:       "root",
				ParentPath: "",
			},
			expected: "",
		},
		{
			name: "child bundle",
			bundle: &Bundle{
				Name:       "child",
				ParentPath: "root",
			},
			expected: "root",
		},
		{
			name: "nested bundle",
			bundle: &Bundle{
				Name:       "nested",
				ParentPath: "root/child",
			},
			expected: "root/child",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := test.bundle.GetParentPath()
			if result != test.expected {
				t.Errorf("expected %q, got %q", test.expected, result)
			}
		})
	}
}

func TestBundleInitializePathMap(t *testing.T) {
	// Create a hierarchy of bundles
	root := &Bundle{Name: "root", ParentPath: ""}
	child1 := &Bundle{Name: "child1", ParentPath: "root"}
	child2 := &Bundle{Name: "child2", ParentPath: "root"}
	grandchild := &Bundle{Name: "grandchild", ParentPath: "root/child1"}

	allBundles := []*Bundle{root, child1, child2, grandchild}

	// Initialize path map on root
	root.InitializePathMap(allBundles)

	// Verify path map was set on all bundles
	for _, bundle := range allBundles {
		if bundle.pathMap == nil {
			t.Errorf("pathMap not set for bundle %s", bundle.Name)
		}
	}

	// Verify parent references were set correctly
	if child1.parent != root {
		t.Error("child1 parent should be root")
	}
	if child2.parent != root {
		t.Error("child2 parent should be root")
	}
	if grandchild.parent != child1 {
		t.Error("grandchild parent should be child1")
	}
	if root.parent != nil {
		t.Error("root parent should be nil")
	}

	// Verify path map contents
	if root.pathMap["root"] != root {
		t.Error("root not in path map")
	}
	if root.pathMap["root/child1"] != child1 {
		t.Error("child1 not in path map")
	}
	if root.pathMap["root/child2"] != child2 {
		t.Error("child2 not in path map")
	}
	if root.pathMap["root/child1/grandchild"] != grandchild {
		t.Error("grandchild not in path map")
	}
}

func TestBundleGetPath(t *testing.T) {
	tests := []struct {
		name     string
		bundle   *Bundle
		expected string
	}{
		{
			name: "root bundle",
			bundle: &Bundle{
				Name:       "root",
				ParentPath: "",
			},
			expected: "root",
		},
		{
			name: "child bundle",
			bundle: &Bundle{
				Name:       "child",
				ParentPath: "root",
			},
			expected: "root/child",
		},
		{
			name: "deeply nested bundle",
			bundle: &Bundle{
				Name:       "leaf",
				ParentPath: "root/branch1/branch2",
			},
			expected: "root/branch1/branch2/leaf",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := test.bundle.GetPath()
			if result != test.expected {
				t.Errorf("expected %q, got %q", test.expected, result)
			}
		})
	}
}

func TestBundleSetParent(t *testing.T) {
	parent := &Bundle{Name: "parent", ParentPath: ""}
	child := &Bundle{Name: "child", ParentPath: ""}

	// Set parent
	child.SetParent(parent)

	// Verify parent reference
	if child.GetParent() != parent {
		t.Error("parent reference not set correctly")
	}

	// Verify ParentPath was updated
	if child.ParentPath != "parent" {
		t.Errorf("expected ParentPath 'parent', got %q", child.ParentPath)
	}

	// Test setting parent to nil
	child.SetParent(nil)
	if child.GetParent() != nil {
		t.Error("parent should be nil")
	}
	if child.ParentPath != "" {
		t.Errorf("expected empty ParentPath, got %q", child.ParentPath)
	}
}
