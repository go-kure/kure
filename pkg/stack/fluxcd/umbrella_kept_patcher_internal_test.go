package fluxcd

import (
	"strings"
	"testing"

	kustv1 "github.com/fluxcd/kustomize-controller/api/v1"
	"github.com/fluxcd/pkg/apis/kustomize"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-kure/kure/pkg/stack/layout"
)

// The refusal for a local-config Flux Kustomization that a patch keeps, that
// patches a directory of the tree and that no writer reads names it and its
// Source (go-kure/kure#1243). See TestUmbrellaTerminals_KeptLocalConfigPatcher
// for the tree and what UmbrellaTerminals makes of it.
func TestUnbuiltPatcher_NamesAKeptLocalConfigPatcher(t *testing.T) {
	kust := func(name, path, source string, annotations map[string]string, patches ...kustomize.Patch) *kustv1.Kustomization {
		return &kustv1.Kustomization{
			TypeMeta:   metav1.TypeMeta{APIVersion: kustv1.GroupVersion.String(), Kind: "Kustomization"},
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "flux-system", Annotations: annotations},
			Spec: kustv1.KustomizationSpec{
				Path:      path,
				SourceRef: kustv1.CrossNamespaceSourceReference{Kind: "GitRepository", Name: source},
				Patches:   patches,
			},
		}
	}
	root := &layout.ManifestLayout{Name: "demo", Namespace: "clusters"}
	controllers := &layout.ManifestLayout{Name: "controllers", Namespace: root.FullRepoPath()}
	apps := &layout.ManifestLayout{Name: "apps", Namespace: controllers.FullRepoPath()}
	lc := &layout.ManifestLayout{Name: "lc", Namespace: root.FullRepoPath(), UmbrellaChild: true}
	root.Children = []*layout.ManifestLayout{controllers, lc}
	controllers.Children = []*layout.ManifestLayout{apps}
	keep := "apiVersion: kustomize.toolkit.fluxcd.io/v1\nkind: Kustomization\nmetadata:\n  name: p\n  namespace: flux-system\n  annotations:\n    config.kubernetes.io/local-config: \"false\"\n"
	addExpr := kustomize.Patch{
		Patch:  "- op: add\n  path: /spec/dependsOn/0/readyExpr\n  value: \"dep.status.conditions.exists(c, c.type == 'Ready' && c.status == 'True')\"\n",
		Target: &kustomize.Selector{Kind: "Kustomization", Name: "h"},
	}
	h := kust("h", "./elsewhere", "flux-system", nil)
	h.Spec.DependsOn = []kustv1.DependencyReference{{Name: "x"}}
	root.Resources = []client.Object{
		kust("a", controllers.FullRepoPath(), "flux-system", nil),
		kust("c", lc.FullRepoPath(), "other", nil, kustomize.Patch{Patch: keep}),
	}
	controllers.Resources = []client.Object{kust("b", apps.FullRepoPath(), "flux-system", nil)}
	apps.Resources = []client.Object{h, kust("x", "./elsewhere", "flux-system", nil)}
	lc.Resources = []client.Object{kust("p", apps.FullRepoPath(), "flux-system", map[string]string{"config.kubernetes.io/local-config": "true"}, addExpr)}
	root.SetFluxKustomizations([]layout.FluxKustomizationRecord{
		{Namespace: "flux-system", Name: "a", Path: controllers.FullRepoPath(), SourceKind: "GitRepository", SourceName: "flux-system"},
		{Namespace: "flux-system", Name: "b", Path: apps.FullRepoPath(), SourceKind: "GitRepository", SourceName: "flux-system"},
		{Namespace: "flux-system", Name: "c", Path: lc.FullRepoPath(), SourceKind: "GitRepository", SourceName: "other"},
	})
	tree, err := indexKustomizationTree(root, recordedMembers)
	if err != nil {
		t.Fatal(err)
	}
	_, err = newDepSnapshot(tree, false)
	if err == nil {
		t.Fatal("got the tree admitted, want it refused")
	}
	for _, want := range []string{`"flux-system/p"`, `Source "GitRepository flux-system/flux-system"`, "does not read it as a writer"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("got %q, want it to contain %s", err, want)
		}
	}
}
