package fluxcd

import (
	"testing"

	kustv1 "github.com/fluxcd/kustomize-controller/api/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-kure/kure/pkg/stack/layout"
)

// Tests for go-kure/kure#1173 at the two readers that turn a Flux
// Kustomization held as unstructured content into the typed object: a key is
// read only when its spelling is the API's own, and one that is not is dropped,
// as the API server prunes it.

// misCasedKustomization is a Flux Kustomization in unstructured form whose spec
// spells wait and kubeConfig as `Wait` and `KubeConfig`, beside correctly spelled
// path, interval and sourceRef.
func misCasedKustomization() *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": kustv1.GroupVersion.String(),
		"kind":       "Kustomization",
		"metadata":   map[string]any{"name": "hand-written", "namespace": "flux-system"},
		"spec": map[string]any{
			"path":       "./elsewhere",
			"interval":   "10m",
			"sourceRef":  map[string]any{"kind": "GitRepository", "name": "flux-system"},
			"Wait":       true,
			"KubeConfig": map[string]any{"secretRef": map[string]any{"name": "remote-kubeconfig"}},
			"Prune":      true,
		},
	}}
}

func assertReadAsServed(t *testing.T, k *kustv1.Kustomization) {
	t.Helper()
	if k.Spec.Wait || k.Spec.Prune {
		t.Errorf("wait %v, prune %v: a key not spelled as the API spells it set a field", k.Spec.Wait, k.Spec.Prune)
	}
	if k.Spec.KubeConfig != nil {
		t.Errorf("kubeConfig %+v: a key not spelled as the API spells it set a field", k.Spec.KubeConfig)
	}
	if k.Spec.Path != "./elsewhere" || k.Spec.SourceRef.Name != "flux-system" || k.Spec.Interval.Duration.String() != "10m0s" {
		t.Errorf("path %q, sourceRef %q, interval %v: a key spelled as the API spells it was not read", k.Spec.Path, k.Spec.SourceRef.Name, k.Spec.Interval)
	}
}

func TestTypedKustomization_ReadsKeysAsTheAPIServerDoes(t *testing.T) {
	k, err := typedKustomization(misCasedKustomization())
	if err != nil {
		t.Fatalf("typedKustomization: %v", err)
	}
	assertReadAsServed(t, k)
}

func TestGeneratedKustomizations_ReadsKeysAsTheAPIServerDoes(t *testing.T) {
	var kept client.Object = misCasedKustomization()
	l := &layout.ManifestLayout{Resources: []client.Object{kept}}
	ks, err := generatedKustomizations(l, map[string]bool{crKey("flux-system", "hand-written"): true})
	if err != nil {
		t.Fatalf("generatedKustomizations: %v", err)
	}
	if len(ks) != 1 {
		t.Fatalf("got %d Kustomizations, want the one the integration kept", len(ks))
	}
	assertReadAsServed(t, ks[0])
}
