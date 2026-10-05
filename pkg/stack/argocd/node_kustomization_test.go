package argocd

import (
	"strings"
	"testing"

	"github.com/go-kure/kure/pkg/stack"
	"github.com/go-kure/kure/pkg/stack/layout"
)

// TestGenerateFromCluster_NodeKustomizationFieldsRefused: the ArgoCD workflow
// generates nothing for a node, so a node's KustomizationName, DependsOn and
// NamedDependsOn would be ignored. Each is refused, naming the node and the
// field; the same cluster without them is accepted.
func TestGenerateFromCluster_NodeKustomizationFieldsRefused(t *testing.T) {
	build := func(set func(apps, platform *stack.Node)) *stack.Cluster {
		platform := &stack.Node{Name: "platform", Children: []*stack.Node{
			{Name: "cert-manager", Bundle: &stack.Bundle{Name: "cert-manager"}},
		}}
		apps := &stack.Node{Name: "apps", Children: []*stack.Node{
			{Name: "shop", Bundle: &stack.Bundle{Name: "shop"}},
		}}
		if set != nil {
			set(apps, platform)
		}
		return &stack.Cluster{Name: "demo", Node: &stack.Node{Name: "prod", Children: []*stack.Node{platform, apps}}}
	}

	if _, err := Engine().GenerateFromCluster(build(nil), layout.DefaultLayoutRules()); err != nil {
		t.Fatalf("GenerateFromCluster without the fields: %v", err)
	}
	for field, set := range map[string]func(apps, platform *stack.Node){
		"KustomizationName": func(apps, _ *stack.Node) { apps.KustomizationName = "apps" },
		"DependsOn":         func(apps, platform *stack.Node) { apps.DependsOn = []*stack.Node{platform} },
		"NamedDependsOn":    func(apps, _ *stack.Node) { apps.NamedDependsOn = []string{"platform"} },
	} {
		t.Run(field, func(t *testing.T) {
			_, err := Engine().GenerateFromCluster(build(set), layout.DefaultLayoutRules())
			if err == nil {
				t.Fatalf("GenerateFromCluster accepted a node that sets %s", field)
			}
			for _, want := range []string{`node "prod/apps"`, field, "ignored"} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error %q does not contain %s", err, want)
				}
			}
		})
	}
}

// TestIntegrateWithLayout_NodeKustomizationFieldsRefused: IntegrateWithLayout
// adds nothing to a layout, and it refuses the node fields like generation
// does instead of returning success for a model it would not honour.
func TestIntegrateWithLayout_NodeKustomizationFieldsRefused(t *testing.T) {
	cluster := func(apps *stack.Node) *stack.Cluster {
		return &stack.Cluster{Name: "demo", Node: &stack.Node{Name: "prod", Children: []*stack.Node{apps}}}
	}
	rules := layout.DefaultLayoutRules()

	plain := &stack.Node{Name: "apps"}
	if err := Engine().IntegrateWithLayout(&layout.ManifestLayout{Name: "prod"}, cluster(plain), rules); err != nil {
		t.Fatalf("IntegrateWithLayout without the fields: %v", err)
	}
	if err := Engine().IntegrateWithLayout(&layout.ManifestLayout{Name: "prod"}, nil, rules); err != nil {
		t.Fatalf("IntegrateWithLayout without a cluster: %v", err)
	}

	// A node tree that loops back is read once, not forever, and a field set
	// in it is still found.
	looping := &stack.Node{Name: "apps"}
	looping.Children = []*stack.Node{{Name: "shop", Children: []*stack.Node{looping}}}
	if err := Engine().IntegrateWithLayout(&layout.ManifestLayout{Name: "prod"}, cluster(looping), rules); err != nil {
		t.Fatalf("IntegrateWithLayout with a looping node tree: %v", err)
	}
	looping.Children[0].NamedDependsOn = []string{"platform"}
	if err := Engine().IntegrateWithLayout(&layout.ManifestLayout{Name: "prod"}, cluster(looping), rules); err == nil || !strings.Contains(err.Error(), `node "prod/apps/shop"`) {
		t.Fatalf("IntegrateWithLayout with a looping node tree and a field set = %v, want node \"prod/apps/shop\" refused", err)
	}

	named := &stack.Node{Name: "apps", KustomizationName: "apps"}
	err := Engine().IntegrateWithLayout(&layout.ManifestLayout{Name: "prod"}, cluster(named), rules)
	if err == nil {
		t.Fatal("IntegrateWithLayout accepted a node that sets KustomizationName")
	}
	for _, want := range []string{`node "prod/apps"`, "KustomizationName", "ignored"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not contain %s", err, want)
		}
	}
}
