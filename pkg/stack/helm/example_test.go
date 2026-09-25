package helm_test

import (
	"fmt"

	"github.com/go-kure/kure/pkg/io"
	"github.com/go-kure/kure/pkg/stack/helm"
)

// The README's Go blocks are generated from these functions
// (scripts/gen-doc-examples.sh). The RenderChart examples have no output
// comment, so go test compiles them without running them: they pull a chart
// from a registry. ExampleSplitByHookWeight runs offline and checks its
// output.

func ExampleRenderChart() {
	manifests, err := helm.RenderChart(
		"oci://registry.example.com/charts/cilium", // OCI chart URL
		"1.16.5", // chart version
		map[string]any{ // value overrides (merged on top of chart defaults)
			"kubeProxyReplacement": true,
			"ipam": map[string]any{
				"mode": "kubernetes",
			},
		},
	)
	if err != nil {
		panic(err)
	}
	// manifests is multi-doc YAML suitable for kubectl apply -f -
	fmt.Print(string(manifests))
}

func ExampleRenderChart_http() {
	manifests, err := helm.RenderChart(
		"https://charts.bitnami.com/bitnami/redis", // repo base URL + chart name
		"19.0.0",
		map[string]any{"replicaCount": 3},
	)
	if err != nil {
		panic(err)
	}
	fmt.Print(string(manifests))
}

func ExampleWithReleaseName() {
	values := map[string]any{"kubeProxyReplacement": true}

	manifests, err := helm.RenderChart(
		"oci://registry.example.com/charts/cilium",
		"1.16.5",
		values,
		helm.WithReleaseName("my-cilium"),
		helm.WithNamespace("kube-system"),
	)
	if err != nil {
		panic(err)
	}
	fmt.Print(string(manifests))
}

func ExampleSplitByHookWeight() {
	// Multi-doc YAML as RenderChart returns it
	rendered := []byte(`apiVersion: batch/v1
kind: Job
metadata:
  name: db-migrate
  annotations:
    helm.sh/hook: pre-install
    helm.sh/hook-weight: "-5"
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: app-config
---
apiVersion: v1
kind: Pod
metadata:
  name: app-test
  annotations:
    helm.sh/hook: test
`)
	parsed, err := io.ParseYAML(rendered) // []client.Object
	if err != nil {
		panic(err)
	}

	groups := helm.SplitByHookWeight(parsed)
	for _, g := range groups {
		// each group becomes one FluxCD Kustomization, deployed in order
		fmt.Printf("phase=%q weight=%d resources=%d\n", g.Phase, g.Weight, len(g.Resources))
	}
	// Output:
	// phase="pre-install" weight=-5 resources=1
	// phase="" weight=0 resources=1
}
