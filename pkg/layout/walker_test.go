package layout_test

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-kure/kure/pkg/layout"
	"github.com/go-kure/kure/pkg/stack"
)

// fakeConfig implements stack.ApplicationConfig for testing purposes.
type fakeConfig struct {
	objs []*client.Object
	err  error
}

func (f *fakeConfig) Generate(*stack.Application) ([]*client.Object, error) {
	return f.objs, f.err
}

func TestWalkCluster(t *testing.T) {
	obj := &unstructured.Unstructured{}
	obj.SetAPIVersion("v1")
	obj.SetKind("ConfigMap")
	obj.SetName("cm")
	obj.SetNamespace("default")
	var o client.Object = obj

	app := stack.NewApplication("app", "ns", &fakeConfig{objs: []*client.Object{&o}})
	bundle := &stack.Bundle{Name: "bundle", Applications: []*stack.Application{app}}
	node := &stack.Node{Name: "apps", Bundle: bundle}
	root := &stack.Node{Name: "root", Children: []*stack.Node{node}}
	node.Parent = root
	cluster := &stack.Cluster{Name: "demo", Node: root}

	ml, err := layout.WalkCluster(cluster)
	if err != nil {
		t.Fatalf("walk cluster: %v", err)
	}
	if ml == nil {
		t.Fatalf("nil layout returned")
	}

	if len(ml.Children) != 1 {
		t.Fatalf("expected 1 child, got %d", len(ml.Children))
	}
	nodeLayout := ml.Children[0]
	if nodeLayout.Name != "apps" {
		t.Fatalf("unexpected node name: %s", nodeLayout.Name)
	}
	if nodeLayout.Namespace != "root" {
		t.Fatalf("unexpected node namespace: %s", nodeLayout.Namespace)
	}
	if len(nodeLayout.Children) != 1 {
		t.Fatalf("expected bundle child")
	}
	bundleLayout := nodeLayout.Children[0]
	if bundleLayout.Name != "bundle" {
		t.Fatalf("unexpected bundle name: %s", bundleLayout.Name)
	}
	if bundleLayout.Namespace != "root/apps" {
		t.Fatalf("unexpected bundle namespace: %s", bundleLayout.Namespace)
	}
	if len(bundleLayout.Children) != 1 {
		t.Fatalf("expected application child")
	}
	appLayout := bundleLayout.Children[0]
	if appLayout.Name != "app" {
		t.Fatalf("unexpected application name: %s", appLayout.Name)
	}
	if appLayout.Namespace != "root/apps/bundle" {
		t.Fatalf("unexpected application namespace: %s", appLayout.Namespace)
	}
	if len(appLayout.Resources) != 1 {
		t.Fatalf("expected one resource, got %d", len(appLayout.Resources))
	}
}
