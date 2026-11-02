package fluxcd

import (
	"context"
	"testing"

	kustv1 "github.com/fluxcd/kustomize-controller/api/v1"
	fluxkustomize "github.com/fluxcd/pkg/kustomize"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/kustomize/api/provider"
	"sigs.k8s.io/kustomize/api/resource"

	"github.com/go-kure/kure/internal/kustomizeschema"
)

// substituteWith runs Flux's postBuild substitution, offline and in dry-run
// mode as substituteBuild runs it, over a ConfigMap written as the repro of
// go-kure/kure#1176 writes it: an annotation holding ${ANCHOR}${NAME:=a-cm},
// and a name ${ALIAS}. k holds ANCHOR "&id " and ALIAS "*id" inline, so YAML
// reads the name as the annotation's text. Under a substituteFrom,
// extra is added to spec.postBuild.substitute of the copy of k it runs, as the
// probe binds a variable the cluster may set.
func substituteWith(t *testing.T, extra map[string]string) *resource.Resource {
	t.Helper()
	k := &kustv1.Kustomization{}
	k.Name, k.Namespace = "app", "flux-system"
	k.Spec.PostBuild = &kustv1.PostBuild{
		Substitute:     map[string]string{"ANCHOR": "&id ", "ALIAS": "*id"},
		SubstituteFrom: []kustv1.SubstituteReference{{Kind: "ConfigMap", Name: "cluster-vars"}},
	}
	for name, value := range extra {
		k.Spec.PostBuild.Substitute[name] = value
	}
	kust, err := runtime.DefaultUnstructuredConverter.ToUnstructured(k)
	if err != nil {
		t.Fatal(err)
	}
	kustomizeschema.Parse()
	rf := provider.NewDefaultDepProvider().GetResourceFactory()
	res, err := rf.FromMap(map[string]any{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
		"metadata": map[string]any{
			"name":        "${ALIAS}",
			"namespace":   "default",
			"annotations": map[string]any{"example.com/value": "${ANCHOR}${NAME:=a-cm}"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	// A nil client reads no cluster: a substitution that tried would panic.
	out, err := fluxkustomize.SubstituteVariables(context.Background(), nil, unstructured.Unstructured{Object: kust}, res,
		fluxkustomize.SubstituteWithDryRun(true))
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// The sentinel probe of go-kure/kure#1176 binds a variable the cluster may
// set by adding it to spec.postBuild.substitute of a copy of the Kustomization,
// and reads the substituted name again. That holds only if the dry-run
// substitution takes such a binding without reading the cluster, and if the
// alias then carries it into the name, which the unbound run does not show.
func TestSubstituteProbe_BindingThroughPostBuildSubstituteReadsNoCluster(t *testing.T) {
	if got := substituteWith(t, nil).GetName(); got != "a-cm" {
		t.Fatalf("unbound name = %q, want the default a-cm the alias reads", got)
	}
	if got := substituteWith(t, map[string]string{"NAME": "kure-probe-a"}).GetName(); got != "kure-probe-a" {
		t.Fatalf("bound name = %q, want the sentinel the alias carries into it", got)
	}
}
