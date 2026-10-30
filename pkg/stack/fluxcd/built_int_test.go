package fluxcd_test

import (
	"slices"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/go-kure/kure/pkg/stack"
	"github.com/go-kure/kure/pkg/stack/layout"
)

// A build kure runs as kustomize-controller does (a patch makes a's build
// one) holds an integer, here a Service port, as kustomize decodes it: a Go
// int. Reading that build, with a postBuild or without, refuses the tree as
// it refuses it without the integer, and never panics (go-kure/kure#1199).
func TestBuiltInteger_IsRead(t *testing.T) {
	service := unstructuredObj("v1", "Service", "default", "dns")
	if err := unstructured.SetNestedSlice(service.Object, []any{map[string]any{"name": "dns", "port": int64(53)}}, "spec", "ports"); err != nil {
		t.Fatal(err)
	}
	patched := func(c *stack.Cluster) *stack.Cluster {
		c.Node.Children[0].Bundle.Patches = []stack.Patch{{
			Patch:  "- op: add\n  path: /metadata/labels\n  value: {patched: \"true\"}\n",
			Target: &stack.PatchSelector{Kind: "ConfigMap", Name: "a-cm"},
		}}
		return c
	}
	cases := map[string]func() *stack.Cluster{
		"with a postBuild":    func() *stack.Cluster { return patched(withPostBuild(service.DeepCopy())) },
		"without a postBuild": func() *stack.Cluster { return patched(baseCluster(service.DeepCopy())) },
	}
	for name, cluster := range cases {
		for _, placement := range append(slices.Clone(integratedPlacements), layout.FluxSeparate) {
			t.Run(name+"/"+string(placement), func(t *testing.T) {
				err := integrateWalked(t, cluster(), placement, nil)
				if err == nil || !strings.Contains(err.Error(), "can never all become Ready") || !strings.Contains(err.Error(), "apply flux-system/b-longer") {
					t.Fatalf("got %v, want the cycle through b-longer's apply refused", err)
				}
			})
		}
	}
}
