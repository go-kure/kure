package fluxcd_test

import (
	"strings"
	"testing"

	kustv1 "github.com/fluxcd/kustomize-controller/api/v1"

	"github.com/go-kure/kure/pkg/stack"
	fluxstack "github.com/go-kure/kure/pkg/stack/fluxcd"
	"github.com/go-kure/kure/pkg/stack/layout"
)

// A non-empty duration that does not parse is an error on every generation
// path, never a silent fallback to the default (go-kure/kure#762). Empty keeps meaning
// "use the default" for Interval and "leave unset" for Timeout/RetryInterval.

var invalidDurations = []string{"5 minutes", "5min", "5"}

func bundleWith(field, value string) *stack.Bundle {
	b := &stack.Bundle{Name: "b"}
	switch field {
	case "interval":
		b.Interval = value
	case "timeout":
		b.Timeout = value
	case "retryInterval":
		b.RetryInterval = value
	}
	return b
}

func TestGenerateForBundle_RejectsInvalidDurations(t *testing.T) {
	gen := fluxstack.NewResourceGenerator()
	for _, field := range []string{"interval", "timeout", "retryInterval"} {
		for _, v := range invalidDurations {
			t.Run(field+"="+v, func(t *testing.T) {
				_, err := gen.GenerateForBundle(bundleWith(field, v), "b")
				if err == nil {
					t.Fatalf("expected an error for %s %q", field, v)
				}
				if !strings.Contains(err.Error(), field) || !strings.Contains(err.Error(), v) {
					t.Errorf("error %q does not name the field and the rejected value", err)
				}
			})
		}
	}
}

func TestGenerateForBundle_EmptyDurationsKeepDefaults(t *testing.T) {
	gen := fluxstack.NewResourceGenerator()
	objs, err := gen.GenerateForBundle(&stack.Bundle{Name: "b"}, "b")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	k := objs[0].(*kustv1.Kustomization)
	if k.Spec.Interval.Duration != gen.DefaultInterval {
		t.Errorf("Interval = %v, want DefaultInterval %v", k.Spec.Interval.Duration, gen.DefaultInterval)
	}
	if k.Spec.Timeout != nil || k.Spec.RetryInterval != nil {
		t.Errorf("empty Timeout/RetryInterval must stay unset, got %v / %v", k.Spec.Timeout, k.Spec.RetryInterval)
	}
}

func TestGenerateFromLayout_RejectsInvalidDurationOnNestedUmbrellaChild(t *testing.T) {
	// GenerateFromLayout does not run ValidateCluster; the generator itself
	// must refuse the value on a grandchild of the umbrella. Walk a valid
	// cluster, then corrupt the grandchild.
	grandchild := &stack.Bundle{Name: "gc"}
	child := &stack.Bundle{Name: "c", Children: []*stack.Bundle{grandchild}}
	umbrella := &stack.Bundle{Name: "u", Children: []*stack.Bundle{child}}
	c := &stack.Cluster{Name: "c", Node: &stack.Node{Name: "n", Bundle: umbrella}}
	ml, err := layout.WalkCluster(c, layout.DefaultLayoutRules())
	if err != nil {
		t.Fatalf("WalkCluster: %v", err)
	}
	grandchild.Timeout = "5min"
	_, err = fluxstack.NewResourceGenerator().GenerateFromLayout(ml, c)
	if err == nil || !strings.Contains(err.Error(), `"5min"`) {
		t.Fatalf("expected an error naming the rejected timeout, got %v", err)
	}
}

func TestGenerateFromCluster_RejectsInvalidInterval(t *testing.T) {
	c := &stack.Cluster{Name: "c", Node: &stack.Node{Name: "n", Bundle: &stack.Bundle{Name: "b", Interval: "5"}}}
	if _, err := fluxstack.NewResourceGenerator().GenerateFromCluster(c); err == nil {
		t.Fatal("expected an error for an unparsable interval")
	}
}

func TestIntegrateWithLayout_RejectsInvalidDurationOnUmbrellaChild(t *testing.T) {
	// Walk a valid cluster, then corrupt an umbrella child's RetryInterval:
	// IntegrateWithLayout places child CRs without re-validating, so the
	// generator must refuse the value itself.
	child := &stack.Bundle{Name: "child", SourceRef: testSR()}
	umbrella := &stack.Bundle{Name: "platform", SourceRef: testSR(), Children: []*stack.Bundle{child}}
	node := &stack.Node{Name: "apps", Bundle: umbrella}
	root := &stack.Node{Name: "demo", Children: []*stack.Node{node}}
	node.SetParent(root)
	cluster := &stack.Cluster{Name: "demo", Node: root}
	rules := layout.LayoutRules{
		BundleGrouping:      layout.GroupFlat,
		ApplicationGrouping: layout.GroupFlat,
		FluxPlacement:       layout.FluxIntegratedPerLayout,
	}
	ml, err := layout.WalkCluster(cluster, rules)
	if err != nil {
		t.Fatalf("WalkCluster: %v", err)
	}
	child.RetryInterval = "2 minutes"
	err = fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).IntegrateWithLayout(ml, cluster, rules)
	if err == nil || !strings.Contains(err.Error(), "retryInterval") {
		t.Fatalf("expected an error naming retryInterval, got %v", err)
	}
}
