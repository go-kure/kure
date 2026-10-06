package fluxcd_test

import (
	"context"
	"maps"
	"path"
	"strings"
	"testing"

	kustv1 "github.com/fluxcd/kustomize-controller/api/v1"
	fluxkustomize "github.com/fluxcd/pkg/kustomize"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/kustomize/api/provider"

	"github.com/go-kure/kure/pkg/stack"
	fluxstack "github.com/go-kure/kure/pkg/stack/fluxcd"
	"github.com/go-kure/kure/pkg/stack/layout"
)

// Tests for a per-layout Kustomization's own postBuild and patches in the
// build of the Kustomization that applies it (go-kure/kure#1021): the
// parent's postBuild substitution runs over them first.

const substituteOptOut = "kustomize.toolkit.fluxcd.io/substitute"

// postBuildTree integrates shopSettings with the given substitution and one
// patch with a target that writes value into each ConfigMap's data, and extra
// bundle annotations.
func postBuildTree(pb *stack.PostBuild, value string, annotations map[string]string) (*layout.ManifestLayout, error) {
	b := shopSettings(nil)
	b.PostBuild = pb
	b.Patches = []stack.Patch{{
		Patch:  "- op: add\n  path: /data/note\n  value: \"" + value + "\"\n",
		Target: &stack.PatchSelector{Kind: "ConfigMap"},
	}}
	maps.Copy(b.Annotations, annotations)
	return fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).CreateLayoutWithResources(oneBundleCluster(b), perLayoutRules())
}

// parentOf is the Kustomization whose build holds k: the one of the directory
// above k's.
func parentOf(t *testing.T, ml *layout.ManifestLayout, k *kustv1.Kustomization) *kustv1.Kustomization {
	t.Helper()
	dir := path.Dir(k.Spec.Path)
	for _, other := range kustomizationsByName(ml) {
		if other.Spec.Path == dir {
			return other
		}
	}
	t.Fatalf("no Kustomization builds %q, the directory that holds %s", dir, k.Name)
	return nil
}

// substituted is obj's content after k's postBuild substitution, run as Flux
// runs it, offline.
func substituted(t *testing.T, k *kustv1.Kustomization, obj client.Object) map[string]any {
	t.Helper()
	content, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		t.Fatal(err)
	}
	res, err := provider.NewDefaultDepProvider().GetResourceFactory().FromMap(content)
	if err != nil {
		t.Fatal(err)
	}
	kust, err := runtime.DefaultUnstructuredConverter.ToUnstructured(k)
	if err != nil {
		t.Fatal(err)
	}
	out, err := fluxkustomize.SubstituteVariables(context.Background(), nil, unstructured.Unstructured{Object: kust}, res, fluxkustomize.SubstituteWithDryRun(true))
	if err != nil {
		t.Fatal(err)
	}
	if out == nil {
		out = res
	}
	m, err := out.Map()
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func firstPatch(t *testing.T, content map[string]any) string {
	t.Helper()
	patches, _, err := unstructured.NestedSlice(content, "spec", "patches")
	if err != nil || len(patches) == 0 {
		t.Fatalf("no patches in %v (%v)", content, err)
	}
	return patches[0].(map[string]any)["patch"].(string)
}

func TestLayoutPostBuild_ThroughTheParentsBuild(t *testing.T) {
	t.Run("a plain ${VAR} in a patch: written as before", func(t *testing.T) {
		ml, err := postBuildTree(&stack.PostBuild{Substitute: map[string]string{"REGION": "eu"}}, "${REGION}", nil)
		if err != nil {
			t.Fatalf("CreateLayoutWithResources: %v", err)
		}
		for _, name := range shopLayoutNames {
			if got := mustKustomization(t, ml, name).Annotations; !maps.Equal(got, map[string]string{"owner": "shop"}) {
				t.Errorf("%s: annotations = %v, want the bundle's alone", name, got)
			}
		}
	})

	t.Run("a var only substituteFrom sets, in a patch and an annotation: written as before", func(t *testing.T) {
		// The value from the cluster is taken to hold no $; one that does is
		// substituted twice, the stated limit.
		pb := &stack.PostBuild{SubstituteFrom: []stack.SubstituteRef{{Kind: "ConfigMap", Name: "cluster-vars"}}}
		ml, err := postBuildTree(pb, "${FROM_CLUSTER}", map[string]string{"note": "${FROM_CLUSTER}"})
		if err != nil {
			t.Fatalf("CreateLayoutWithResources: %v", err)
		}
		for _, name := range shopLayoutNames {
			if got := mustKustomization(t, ml, name).Annotations; !maps.Equal(got, map[string]string{"owner": "shop", "note": "${FROM_CLUSTER}"}) {
				t.Errorf("%s: annotations = %v, want the bundle's alone", name, got)
			}
		}
	})

	t.Run("an escape in a patch survives the parent's build", func(t *testing.T) {
		ml, err := postBuildTree(&stack.PostBuild{Substitute: map[string]string{"VAR": "x"}}, "$${VAR}", nil)
		if err != nil {
			t.Fatalf("CreateLayoutWithResources: %v", err)
		}
		for _, name := range shopLayoutNames {
			k := mustKustomization(t, ml, name)
			if got := k.Annotations[substituteOptOut]; got != "disabled" {
				t.Fatalf("%s: %s = %q, want disabled", name, substituteOptOut, got)
			}
			parent := parentOf(t, ml, k)
			if got := firstPatch(t, substituted(t, parent, k)); !strings.Contains(got, `"$${VAR}"`) {
				t.Errorf("%s through %s's build: patch %q, want the escape as written", name, parent.Name, got)
			}
			// Without the opt-out, the parent unescapes it, and the child would
			// substitute what is left.
			bare := k.DeepCopy()
			delete(bare.Annotations, substituteOptOut)
			if got := firstPatch(t, substituted(t, parent, bare)); !strings.Contains(got, `"${VAR}"`) {
				t.Errorf("%s through %s's build without the opt-out: patch %q, want the escape consumed", name, parent.Name, got)
			}
		}
		// The child's own substitution, run over what the patch wrote, gives
		// the literal the escape stands for.
		cm := cmObj("db")
		(*cm).(*unstructured.Unstructured).Object["data"] = map[string]any{"note": "$${VAR}"}
		out := substituted(t, mustKustomization(t, ml, "shop-db"), *cm)
		if got, _, _ := unstructured.NestedString(out, "data", "note"); got != "${VAR}" {
			t.Errorf("shop-db's substitution of the patched ConfigMap: note = %q, want the literal ${VAR}", got)
		}
	})

	t.Run("a substitute value holding ${...} reaches the child as written", func(t *testing.T) {
		pb := &stack.PostBuild{Substitute: map[string]string{"NAME": "x", "GREETING": "hello ${NAME}"}}
		ml, err := postBuildTree(pb, "plain", nil)
		if err != nil {
			t.Fatalf("CreateLayoutWithResources: %v", err)
		}
		for _, name := range shopLayoutNames {
			k := mustKustomization(t, ml, name)
			if got := k.Annotations[substituteOptOut]; got != "disabled" {
				t.Fatalf("%s: %s = %q, want disabled", name, substituteOptOut, got)
			}
			parent := parentOf(t, ml, k)
			got, _, _ := unstructured.NestedString(substituted(t, parent, k), "spec", "postBuild", "substitute", "GREETING")
			if got != "hello ${NAME}" {
				t.Errorf("%s through %s's build: GREETING = %q, want it as written", name, parent.Name, got)
			}
		}
	})

	t.Run("refused: an escape, and another field the parent substitutes", func(t *testing.T) {
		_, err := postBuildTree(&stack.PostBuild{Substitute: map[string]string{"VAR": "x"}}, "$${VAR}", map[string]string{"note": "${VAR}"})
		mustContainAll(t, err, `"shop-db"`, `"prod/shop/db"`, "(spec.patches)", "metadata.annotations", "opt-out")
	})
}
