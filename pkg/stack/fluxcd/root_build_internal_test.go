package fluxcd

import (
	"fmt"
	"strings"
	"testing"

	kustv1 "github.com/fluxcd/kustomize-controller/api/v1"
	"github.com/fluxcd/pkg/apis/kustomize"
	sourcev1 "github.com/fluxcd/source-controller/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-kure/kure/pkg/stack/layout"
)

// TestCheckRootBuildKeepsHostedSources_FollowsTheBootstrapBuild: the check
// refuses a Kustomization whose build holds the root node's layout and changes
// a Source the pass hosted there (go-kure/kure#908) when the build the Flux
// bootstrap applies, the top of the tree, holds that layout too, and not
// otherwise (go-kure/kure#979).
//
// The check is called on trees built by hand: IntegrateWithLayout sets one
// placement on the whole tree, and under it no Kustomization the pass places
// shares the root node's layout with the bootstrap build. The accepted tree it
// does reach, the layout Kustomization below a FluxIntegratedPerLayout
// wrapper, is run through the integration in
// TestIntegrate_RootBuildChangesHostedSource and
// TestKeptKustomization_RootBuildChangesHostedSource.
func TestCheckRootBuildKeepsHostedSources_FollowsTheBootstrapBuild(t *testing.T) {
	const (
		crName       = "platform-node"
		plainURL     = "https://git.example.com/shared.git"
		templatedURL = "https://${GIT_HOST}/shared.git"
		intervalOp   = "- op: replace\n  path: /spec/interval\n  value: 5m\n"
	)
	patch := func(p kustomize.Patch) func(k *kustv1.Kustomization) {
		return func(k *kustv1.Kustomization) { k.Spec.Patches = []kustomize.Patch{p} }
	}
	postBuild := func(pb kustv1.PostBuild) func(k *kustv1.Kustomization) {
		return func(k *kustv1.Kustomization) { k.Spec.PostBuild = &pb }
	}
	clusterVars := []kustv1.SubstituteReference{{Kind: "ConfigMap", Name: "cluster-vars"}}

	// A tree is its top, the root node's layout, which holds the Source, and
	// the layout that holds the Kustomization of the root node's layout.
	type tree func() (top, root, host *layout.ManifestLayout)
	wrapper := func(placement layout.FluxPlacement) tree {
		return func() (*layout.ManifestLayout, *layout.ManifestLayout, *layout.ManifestLayout) {
			root := &layout.ManifestLayout{Name: "platform", Namespace: "prod", FluxPlacement: placement}
			top := &layout.ManifestLayout{Namespace: "prod", FluxPlacement: placement, Children: []*layout.ManifestLayout{root}}
			return top, root, top
		}
	}
	noWrapper := func(placement layout.FluxPlacement) tree {
		return func() (*layout.ManifestLayout, *layout.ManifestLayout, *layout.ManifestLayout) {
			root := &layout.ManifestLayout{Name: "platform", Namespace: ".", FluxPlacement: placement}
			return root, root, root
		}
	}

	for _, shape := range []struct {
		name string
		tree tree
		// shared: the bootstrap build holds the root node's layout as well.
		shared bool
	}{
		// The wrapper lists the Kustomization and not the directory: the
		// Kustomization is the only one to apply the Source.
		{"wrapper that lists the root node's Kustomization", wrapper(layout.FluxIntegratedPerLayout), false},
		// The wrapper's kustomization.yaml lists the directory, so the
		// bootstrap applies the Source as well.
		{"wrapper that lists the root node's directory", wrapper(layout.FluxIntegratedPerBundle), true},
		// The bootstrap applies the root node's directory itself, whatever
		// the placement is: the build decides, not the placement.
		{"no wrapper", noWrapper(layout.FluxIntegratedPerBundle), true},
		{"no wrapper, per-layout placement", noWrapper(layout.FluxIntegratedPerLayout), true},
	} {
		for _, tc := range []struct {
			name   string
			url    string
			change func(k *kustv1.Kustomization)
			// callers: the Source is not one this pass placed.
			callers bool
			// cause is what the refusal names besides the Kustomization and
			// the Source where the build is shared; empty means accepted.
			cause string
		}{
			{name: "patch selecting the Source by kind", url: plainURL,
				change: patch(kustomize.Patch{Patch: intervalOp, Target: &kustomize.Selector{Kind: "GitRepository"}}),
				cause:  "patch 0 (target {kind: GitRepository}) selects it"},
			{name: "patch selecting the Source by kind regex", url: plainURL,
				change: patch(kustomize.Patch{Patch: intervalOp, Target: &kustomize.Selector{Kind: "Git.*"}}),
				cause:  "patch 0 (target {kind: Git.*}) selects it"},
			{name: "patch selecting another kind", url: plainURL,
				change: patch(kustomize.Patch{Patch: intervalOp, Target: &kustomize.Selector{Kind: "Deployment"}})},
			{name: "patch selecting by a label the Source lacks", url: plainURL,
				change: patch(kustomize.Patch{Patch: intervalOp, Target: &kustomize.Selector{LabelSelector: "app=x"}})},
			{name: "target-less patch naming the Source", url: plainURL,
				change: patch(kustomize.Patch{Patch: "apiVersion: source.toolkit.fluxcd.io/v1\nkind: GitRepository\nmetadata:\n  name: shared\n  namespace: flux-system\nspec:\n  interval: 5m\n"}),
				cause:  "patch 0 (no target) names it"},
			{name: "target-less patch naming another object", url: plainURL,
				change: patch(kustomize.Patch{Patch: "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: cm\n  namespace: default\ndata:\n  patched: \"true\"\n"})},
			{name: "postBuild over a plain Source", url: plainURL,
				change: postBuild(kustv1.PostBuild{Substitute: map[string]string{"GIT_HOST": "git.example.com"}})},
			{name: "postBuild substituting into the Source", url: templatedURL,
				change: postBuild(kustv1.PostBuild{Substitute: map[string]string{"GIT_HOST": "git.example.com"}}),
				cause:  "postBuild substitution changes it"},
			{name: "postBuild substituteFrom only", url: templatedURL,
				change: postBuild(kustv1.PostBuild{SubstituteFrom: clusterVars}),
				cause:  "postBuild substitution reads GIT_HOST, which the inline vars do not set and substituteFrom can"},
			// $$ escapes a $ and reads no var: only running the substitution
			// without inline vars, as Flux does with substituteFrom set, shows
			// that it changes the URL.
			{name: "postBuild substituteFrom only, escaped expression", url: "https://git.example.com/$${REPO}.git",
				change: postBuild(kustv1.PostBuild{SubstituteFrom: clusterVars}),
				cause:  "postBuild substitution changes it"},
			// The inline PREFIX reproduces the expression offline, but a SUFFIX
			// from the cluster's ConfigMap would change the URL.
			{name: "postBuild substituteFrom var the offline result hides", url: "https://git.example.com/${PREFIX}${SUFFIX}.git",
				change: postBuild(kustv1.PostBuild{Substitute: map[string]string{"PREFIX": "${PREFIX}${SUFFIX}"}, SubstituteFrom: clusterVars}),
				cause:  "postBuild substitution reads SUFFIX, which the inline vars do not set and substituteFrom can"},
			// An inline var overrides substituteFrom's, so an expression reading
			// only inline vars is decided offline: here it is unchanged.
			{name: "postBuild substituteFrom with an inline var that keeps the Source", url: "https://git.example.com/${REPO}.git",
				change: postBuild(kustv1.PostBuild{Substitute: map[string]string{"REPO": "${REPO}"}, SubstituteFrom: clusterVars})},
			// What a Kustomization's patches do to a copy the pass did not
			// place is its owner's.
			{name: "patch selecting a Source the pass did not place", url: plainURL, callers: true,
				change: patch(kustomize.Patch{Patch: intervalOp, Target: &kustomize.Selector{Kind: "GitRepository"}})},
		} {
			check := func(changed bool) error {
				top, root, host := shape.tree()
				source := &sourcev1.GitRepository{
					TypeMeta:   metav1.TypeMeta{APIVersion: sourcev1.GroupVersion.String(), Kind: "GitRepository"},
					ObjectMeta: metav1.ObjectMeta{Name: "shared", Namespace: "flux-system"},
					Spec:       sourcev1.GitRepositorySpec{URL: tc.url},
				}
				root.Resources = append(root.Resources, source)
				cr := &kustv1.Kustomization{
					TypeMeta:   metav1.TypeMeta{APIVersion: kustv1.GroupVersion.String(), Kind: "Kustomization"},
					ObjectMeta: metav1.ObjectMeta{Name: crName, Namespace: "flux-system"},
					Spec:       kustv1.KustomizationSpec{Path: root.FullRepoPath()},
				}
				if changed {
					tc.change(cr)
				}
				host.Resources = append(host.Resources, cr)
				p := &integratedPlacement{
					root:      root,
					generated: map[string]bool{crKey(cr.Namespace, cr.Name): true},
					placed:    map[client.Object]bool{source: !tc.callers},
				}
				return p.checkRootBuildKeepsHostedSources(top)
			}
			t.Run(shape.name+"/"+tc.name, func(t *testing.T) {
				if err := check(false); err != nil {
					t.Fatalf("the Kustomization without the patch or postBuild is refused: %v", err)
				}
				err := check(true)
				if !shape.shared || tc.cause == "" {
					if err != nil {
						t.Fatalf("refused, want accepted: %v", err)
					}
					return
				}
				if err == nil {
					t.Fatalf("got no error, want a refusal naming %q: the bootstrap and the Kustomization would apply the Source differently", tc.cause)
				}
				for _, want := range []string{
					fmt.Sprintf("Flux Kustomization %q", crName),
					`GitRepository "shared"`,
					"the root node's layout the Flux bootstrap applies without patches or postBuild",
					tc.cause,
				} {
					if !strings.Contains(err.Error(), want) {
						t.Errorf("refusal %q does not say %q", err, want)
					}
				}
			})
		}
	}
}
