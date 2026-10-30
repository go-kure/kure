package fluxcd_test

import (
	"slices"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-kure/kure/pkg/stack"
	"github.com/go-kure/kure/pkg/stack/layout"
)

// A Service port is an integer kure reads in a's build, and reading it never
// panics (go-kure/kure#1199): the tree is refused as it is without the port.
// The integer is held two ways. A build kure runs as kustomize-controller
// does (a patch makes a's build one) holds it as kustomize decodes it, a Go
// int, with a postBuild or without. A caller may build the object, or a
// List's item, in Go with a plain int, which a held build keeps as it is.
func TestBuiltInteger_IsRead(t *testing.T) {
	ports := func(port any) map[string]any {
		return map[string]any{"ports": []any{map[string]any{"name": "dns", "port": port}}}
	}
	service := func(port any) client.Object {
		s := unstructuredObj("v1", "Service", "default", "dns")
		s.Object["spec"] = ports(port)
		return s
	}
	list := func(port any) client.Object {
		return &unstructured.Unstructured{Object: map[string]any{"apiVersion": "v1", "kind": "List", "items": []any{
			map[string]any{"apiVersion": "v1", "kind": "Service",
				"metadata": map[string]any{"name": "dns", "namespace": "default"}, "spec": ports(port)},
		}}}
	}
	patched := func(c *stack.Cluster) *stack.Cluster {
		c.Node.Children[0].Bundle.Patches = []stack.Patch{{
			Patch:  "- op: add\n  path: /metadata/labels\n  value: {patched: \"true\"}\n",
			Target: &stack.PatchSelector{Kind: "ConfigMap", Name: "a-cm"},
		}}
		return c
	}
	cases := map[string]func() *stack.Cluster{
		"built with a postBuild":          func() *stack.Cluster { return patched(withPostBuild(service(int64(53)))) },
		"built without a postBuild":       func() *stack.Cluster { return patched(baseCluster(service(int64(53)))) },
		"a caller's plain int":            func() *stack.Cluster { return baseCluster(service(53)) },
		"a caller's List item":            func() *stack.Cluster { return baseCluster(list(53)) },
		"a caller's List item, postBuild": func() *stack.Cluster { return withPostBuild(list(53)) },
		"a caller's List item, built":     func() *stack.Cluster { return patched(withPostBuild(list(53))) },
		"a caller's int32 in a postBuild": func() *stack.Cluster { return withPostBuild(service(int32(53))) },
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
