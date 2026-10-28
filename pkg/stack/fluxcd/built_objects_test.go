package fluxcd_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	kustv1 "github.com/fluxcd/kustomize-controller/api/v1"
	fluxkustomize "github.com/fluxcd/pkg/kustomize"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-kure/kure/pkg/stack"
	fluxstack "github.com/go-kure/kure/pkg/stack/fluxcd"
	"github.com/go-kure/kure/pkg/stack/layout"
)

// A health check's writer is read from what each Kustomization's build
// applies (go-kure/kure#1094 E): an object's identity as substituted, and a
// generated object under its hashed name.

// builtNames returns the names of the v1 objects of kind that k's build
// makes on the tree ml writes, built as kustomize-controller builds it.
func builtNames(t *testing.T, ml *layout.ManifestLayout, k *kustv1.Kustomization, kind string) []string {
	t.Helper()
	obj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(k)
	if err != nil {
		t.Fatal(err)
	}
	tree := filepath.Join(t.TempDir(), "repo")
	if err := os.CopyFS(tree, os.DirFS(writeAll(t, ml)["WriteToDisk"].root)); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(tree, filepath.FromSlash(k.Spec.Path))
	if _, err := fluxkustomize.NewGenerator(tree, unstructured.Unstructured{Object: obj}).WriteFile(dir); err != nil {
		t.Fatalf("Flux generator on %q: %v", k.Spec.Path, err)
	}
	built, err := buildLikeFlux(t, tree, dir)
	if err != nil {
		t.Fatalf("build of %q: %v", k.Spec.Path, err)
	}
	var out []string
	for _, r := range built.Resources() {
		if r.GetApiVersion() == "v1" && r.GetKind() == kind {
			out = append(out, r.GetName())
		}
	}
	return out
}

// builtName returns the one name builtNames lists that starts with prefix.
func builtName(t *testing.T, names []string, prefix string) string {
	t.Helper()
	var out []string
	for _, n := range names {
		if strings.HasPrefix(n, prefix) {
			out = append(out, n)
		}
	}
	if len(out) != 1 {
		t.Fatalf("got %v built with prefix %q, want one", out, prefix)
	}
	return out[0]
}

// go-kure/kure#1145: b-longer writes the ConfigMap ${NS}/ready and depends on
// a, which health-checks red/ready. With NS=red set inline, b-longer's apply
// writes red/ready, the edge is certain and the cycle is refused. With NS
// read from the cluster, or set nowhere (so the namespace is empty), the
// writer stays uncertain.
func TestBuiltObjects_SubstitutedIdentityIsRead(t *testing.T) {
	cases := map[string]struct {
		postBuild *stack.PostBuild
		refused   bool
	}{
		"NS set inline":            {postBuild: &stack.PostBuild{Substitute: map[string]string{"NS": "red"}}, refused: true},
		"NS read from the cluster": {postBuild: &stack.PostBuild{SubstituteFrom: []stack.SubstituteRef{{Kind: "ConfigMap", Name: "vars"}}}},
		"NS set nowhere":           {postBuild: &stack.PostBuild{Substitute: map[string]string{"OTHER": "red"}}},
	}
	for name, tc := range cases {
		for _, placement := range append(slices.Clone(integratedPlacements), layout.FluxSeparate) {
			t.Run(name+"/"+string(placement), func(t *testing.T) {
				c := pairCluster(stack.HealthCheck{APIVersion: "v1", Kind: "ConfigMap", Namespace: "red", Name: "ready"},
					[]client.Object{unstructuredObj("v1", "ConfigMap", "default", "a-cm")},
					[]client.Object{unstructuredObj("v1", "ConfigMap", "${NS}", "ready")})
				c.Node.Children[1].Bundle.PostBuild = tc.postBuild
				err := integrateWalked(t, c, placement, nil)
				if tc.refused && (err == nil || !strings.Contains(err.Error(), "can never all become Ready")) {
					t.Fatalf("got %v, want the cycle through a's health check refused", err)
				}
				if !tc.refused && err != nil {
					t.Fatalf("got %v, want the tree admitted", err)
				}
			})
		}
	}
}

// keptPair integrates pairCluster with no health check, lets edit change the
// layout that holds b-cm and b-longer's kept Kustomization, makes a's kept
// Kustomization health-check the v1 object of kind in default that
// b-longer's build makes under a name starting with prefix, and re-integrates.
func keptPair(t *testing.T, placement layout.FluxPlacement, bObjs []client.Object, kind, prefix string, edit func(l *layout.ManifestLayout, b *kustv1.Kustomization)) error {
	t.Helper()
	c := pairCluster(stack.HealthCheck{}, []client.Object{unstructuredObj("v1", "ConfigMap", "default", "a-cm")}, bObjs)
	c.Node.Children[0].Bundle.HealthChecks = nil
	rules := withPlacement(nodeOnly, placement)
	ml, err := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).CreateLayoutWithResources(c, rules)
	if err != nil {
		t.Fatalf("CreateLayoutWithResources: %v", err)
	}
	byName := kustomizationsByName(ml)
	a, b := byName["a"], byName["b-longer"]
	if a == nil || b == nil {
		t.Fatalf("no a or b-longer in %s", describeKustomizations(ml))
	}
	l := findLayout(ml, "b-cm")
	if l == nil {
		t.Fatal("no layout holds b-cm")
	}
	edit(l, b)
	checks("v1", kind, "default", builtName(t, builtNames(t, ml, b, kind), prefix))(a)
	return fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).IntegrateWithLayout(ml, c, rules)
}

// b-longer generates the ConfigMap settings, which its apply writes as
// settings plus a content hash, and a health-checks that name. Built as
// kustomize-controller builds it (here, for its target namespace) b-longer's
// result holds the hashed name, the edge is certain and the cycle refused. A
// b-longer whose build transforms nothing is read as the tree holds it, which
// computes no hash: a ConfigMap whose name starts with settings- stays
// uncertain.
func TestBuiltObjects_GeneratedNameIsRead(t *testing.T) {
	for _, transformed := range []bool{true, false} {
		for _, placement := range integratedPlacements {
			name := "transforms nothing/"
			if transformed {
				name = "target namespace/"
			}
			t.Run(name+string(placement), func(t *testing.T) {
				err := keptPair(t, placement, []client.Object{unstructuredObj("v1", "ConfigMap", "default", "b-cm")}, "ConfigMap", "settings-",
					func(l *layout.ManifestLayout, b *kustv1.Kustomization) {
						l.ConfigMapGenerators = append(l.ConfigMapGenerators, layout.ConfigMapGeneratorSpec{Name: "settings", Files: []string{"values.yaml"}})
						l.ExtraFiles = append(l.ExtraFiles, layout.ExtraFile{Name: "values.yaml", Content: []byte("k: v\n")})
						if transformed {
							b.Spec.TargetNamespace = "default"
						}
					})
				if transformed && (err == nil || !strings.Contains(err.Error(), "can never all become Ready")) {
					t.Fatalf("got %v, want the cycle through a's health check refused", err)
				}
				if !transformed && err != nil {
					t.Fatalf("got %v, want the tree admitted", err)
				}
			})
		}
	}
}

// b-longer writes the Secret default/creds, and a component of its build
// generates a Secret over it. a health-checks the Secret b-longer's build
// makes. A generator merging into creds (behavior: merge) gives it a content
// hash that depends on the kustomize version (go-kure/kure#1094 §7), so the
// writer stays uncertain; a generator creating another Secret, extra, keeps
// the edge certain and the cycle is refused.
func TestBuiltObjects_MergedGeneratorSecretStaysUncertain(t *testing.T) {
	creds := unstructuredObj("v1", "Secret", "default", "creds")
	creds.Object["data"] = map[string]any{"base": "dg=="}
	cases := map[string]struct {
		generator, prefix string
		refused           bool
	}{
		"merge":  {generator: "- name: creds\n  namespace: default\n  behavior: merge\n  literals:\n  - k=v\n", prefix: "creds"},
		"create": {generator: "- name: extra\n  namespace: default\n  literals:\n  - k=v\n", prefix: "extra-", refused: true},
	}
	for name, tc := range cases {
		for _, placement := range integratedPlacements {
			t.Run(name+"/"+string(placement), func(t *testing.T) {
				err := keptPair(t, placement, []client.Object{unstructuredObj("v1", "ConfigMap", "default", "b-cm"), creds.DeepCopy()}, "Secret", tc.prefix,
					func(l *layout.ManifestLayout, b *kustv1.Kustomization) {
						l.ExtraFiles = append(l.ExtraFiles, layout.ExtraFile{
							Name:    "component/kustomization.yaml",
							Content: []byte("apiVersion: kustomize.config.k8s.io/v1alpha1\nkind: Component\nsecretGenerator:\n" + tc.generator),
						})
						b.Spec.Components = []string{"component"}
					})
				if tc.refused && (err == nil || !strings.Contains(err.Error(), "can never all become Ready")) {
					t.Fatalf("got %v, want the cycle through a's health check refused", err)
				}
				if !tc.refused && err != nil {
					t.Fatalf("got %v, want the tree admitted", err)
				}
			})
		}
	}
}
