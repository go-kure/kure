package fluxcd_test

import (
	"context"
	"maps"
	"path"
	"reflect"
	"strings"
	"testing"

	kustv1 "github.com/fluxcd/kustomize-controller/api/v1"
	"github.com/fluxcd/pkg/apis/kustomize"
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
	return postBuildTreeWith(nil, pb, value, annotations)
}

// postBuildTreeWith is postBuildTree with set editing the layouts, as
// shopSettings does.
func postBuildTreeWith(set func(app, pre, main, deep *layout.ManifestLayout), pb *stack.PostBuild, value string, annotations map[string]string) (*layout.ManifestLayout, error) {
	b := shopSettings(set)
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

// optOutObjects gives every object of the four layouts Flux's opt-out.
func optOutObjects(app, pre, main, deep *layout.ManifestLayout) {
	for _, l := range []*layout.ManifestLayout{app, pre, main, deep} {
		for _, obj := range l.Resources {
			annotations := obj.GetAnnotations()
			if annotations == nil {
				annotations = map[string]string{}
			}
			annotations[substituteOptOut] = "disabled"
			obj.SetAnnotations(annotations)
		}
	}
}

func TestLayoutPostBuild_ThroughTheParentsBuild(t *testing.T) {
	t.Run("nothing the parent substitutes: written as before", func(t *testing.T) {
		ml, err := postBuildTree(&stack.PostBuild{Substitute: map[string]string{"REGION": "eu"}}, "plain", nil)
		if err != nil {
			t.Fatalf("CreateLayoutWithResources: %v", err)
		}
		for _, name := range shopLayoutNames {
			k := mustKustomization(t, ml, name)
			if got := k.Annotations; !maps.Equal(got, map[string]string{"owner": "shop"}) {
				t.Errorf("%s: annotations = %v, want the bundle's alone", name, got)
			}
			if pb := k.Spec.PostBuild; pb == nil || !maps.Equal(pb.Substitute, map[string]string{"REGION": "eu"}) || len(pb.SubstituteFrom) != 0 {
				t.Errorf("%s: postBuild = %#v, want the bundle's", name, pb)
			}
			want := []kustomize.Patch{{
				Patch:  "- op: add\n  path: /data/note\n  value: \"plain\"\n",
				Target: &kustomize.Selector{Kind: "ConfigMap"},
			}}
			if !reflect.DeepEqual(k.Spec.Patches, want) {
				t.Errorf("%s: patches = %#v, want the bundle's one as written", name, k.Spec.Patches)
			}
		}
	})

	// Any ${...} in a patch is substituted by the parent first; what the
	// child's own substitution then makes of the patched objects can differ,
	// so each gets the opt-out and the patch reaches the child as written.
	for name, tc := range map[string]struct {
		set   func(app, pre, main, deep *layout.ManifestLayout)
		pb    *stack.PostBuild
		value string
	}{
		"a plain ${VAR}": {nil, &stack.PostBuild{Substitute: map[string]string{"REGION": "eu"}}, "${REGION}"},
		"a var only substituteFrom sets": {nil,
			&stack.PostBuild{SubstituteFrom: []stack.SubstituteRef{{Kind: "ConfigMap", Name: "cluster-vars"}}}, "${FROM_CLUSTER}"},
		// The parent would write x into the patch; the child skips the
		// ConfigMaps it patches, which keep ${REGION} as written.
		"a ${VAR} for objects that carry the opt-out": {optOutObjects,
			&stack.PostBuild{Substitute: map[string]string{"REGION": "x"}}, "${REGION}"},
		// The parent would write the string "true" into the patch; the child
		// substitutes the unquoted ${FLAG} the patch writes into the boolean.
		"a ${VAR} whose value reads as another YAML type": {nil,
			&stack.PostBuild{Substitute: map[string]string{"FLAG": "true"}}, "${FLAG}"},
	} {
		t.Run(name+" in a patch: the opt-out", func(t *testing.T) {
			ml, err := postBuildTreeWith(tc.set, tc.pb, tc.value, nil)
			if err != nil {
				t.Fatalf("CreateLayoutWithResources: %v", err)
			}
			for _, name := range shopLayoutNames {
				k := mustKustomization(t, ml, name)
				if got := k.Annotations[substituteOptOut]; got != "disabled" {
					t.Errorf("%s: %s = %q, want disabled", name, substituteOptOut, got)
				}
				parent := parentOf(t, ml, k)
				if got := firstPatch(t, substituted(t, parent, k)); !strings.Contains(got, `"`+tc.value+`"`) {
					t.Errorf("%s through %s's build: patch %q, want it as written", name, parent.Name, got)
				}
			}
		})
	}

	// An escape whose unescaped form is not a complete expression: the
	// parent's substitution leaves ${BROKEN, a change like any other.
	for name, tc := range map[string]struct {
		pb    *stack.PostBuild
		patch string
	}{
		"in a substitute value": {&stack.PostBuild{Substitute: map[string]string{"TEXT": "$${BROKEN"}}, "plain"},
		"in a patch":            {&stack.PostBuild{Substitute: map[string]string{"VAR": "x"}}, "$${BROKEN"},
	} {
		t.Run("an incomplete escape "+name+": the opt-out", func(t *testing.T) {
			ml, err := postBuildTree(tc.pb, tc.patch, nil)
			if err != nil {
				t.Fatalf("CreateLayoutWithResources: %v", err)
			}
			for _, k := range shopLayoutNames {
				if got := mustKustomization(t, ml, k).Annotations[substituteOptOut]; got != "disabled" {
					t.Errorf("%s: %s = %q, want disabled", k, substituteOptOut, got)
				}
			}
		})
	}

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

	t.Run("a plain ${VAR} in a patch, under a parent with other vars: the opt-out", func(t *testing.T) {
		// The caller's own Kustomization, kept in place of the bundle's, sets
		// VAR to another value: the parent's substitution would write its
		// value into shop-db's patch, where shop-db's own would write x.
		build := func() *stack.Cluster {
			b := shopSettings(nil)
			b.PostBuild = &stack.PostBuild{Substitute: map[string]string{"VAR": "x"}}
			b.Patches = []stack.Patch{{
				Patch:  "- op: add\n  path: /data/note\n  value: \"${VAR}\"\n",
				Target: &stack.PatchSelector{Kind: "ConfigMap"},
			}}
			return oneBundleCluster(b)
		}
		ml, c := keptIn(t, build, perLayoutRules(), "shop", "typed", func(k *kustv1.Kustomization) {
			k.Spec.PostBuild = &kustv1.PostBuild{Substitute: map[string]string{"VAR": "y"}}
		}, nil)
		if err := integrate(ml, c, perLayoutRules()); err != nil {
			t.Fatalf("IntegrateWithLayout: %v", err)
		}
		// shop-db is built by the kept shop, the others by a Kustomization of
		// the bundle, with the bundle's vars.
		for _, name := range shopLayoutNames {
			if got := mustKustomization(t, ml, name).Annotations[substituteOptOut]; got != "disabled" {
				t.Errorf("%s: %s = %q, want disabled", name, substituteOptOut, got)
			}
		}
	})

	t.Run("refused: an escape, and another field the parent substitutes", func(t *testing.T) {
		_, err := postBuildTree(&stack.PostBuild{Substitute: map[string]string{"VAR": "x"}}, "$${VAR}", map[string]string{"note": "${VAR}"})
		mustContainAll(t, err, `"shop-db"`, `"prod/shop/db"`, "(spec.patches)", "metadata.annotations", "opt-out")
	})
}
