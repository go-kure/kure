package fluxcd_test

import (
	"os"
	"path/filepath"
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

// go-kure/kure#1177: a Kustomization that sets a postBuild but transforms
// nothing else is read as the tree holds it, which leaves out the ConfigMaps
// its configMapGenerator makes, and so does their substitution. Where the
// postBuild substitutes, the writer is built, and its generated objects are
// read as built and substituted.

// cutIdentity is an inline value whose carriage returns Flux's YAML parser
// reads as the name and namespace of the ConfigMap it is substituted into,
// and a document break.
const cutIdentity = "x\r  name: ready\r  namespace: red\r---\r"

// generatedPair integrates pairCluster with a, which writes a-cm, generating
// the ConfigMap settings with the annotation example.com/inject: ${CUT}, and
// health-checking red/ready, which b-longer writes and depends on a. edit
// changes the layout that holds a-cm and a's kept Kustomization first; a
// health check it sets on a replaces the one on red/ready.
func generatedPair(t *testing.T, placement layout.FluxPlacement, edit func(l *layout.ManifestLayout, a *kustv1.Kustomization)) error {
	t.Helper()
	c := pairCluster(stack.HealthCheck{}, []client.Object{unstructuredObj("v1", "ConfigMap", "default", "a-cm")},
		[]client.Object{unstructuredObj("v1", "ConfigMap", "red", "ready")})
	c.Node.Children[0].Bundle.HealthChecks = nil
	rules := withPlacement(nodeOnly, placement)
	ml, err := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).CreateLayoutWithResources(c, rules)
	if err != nil {
		t.Fatalf("CreateLayoutWithResources: %v", err)
	}
	a := kustomizationsByName(ml)["a"]
	l := findLayout(ml, "a-cm")
	if a == nil || l == nil {
		t.Fatalf("no a, or no layout holding a-cm, in %s", describeKustomizations(ml))
	}
	l.ConfigMapGenerators = append(l.ConfigMapGenerators, layout.ConfigMapGeneratorSpec{
		Name:        "settings",
		Files:       []string{"values.yaml"},
		Annotations: map[string]string{"example.com/inject": "${CUT}"},
	})
	l.ExtraFiles = append(l.ExtraFiles, layout.ExtraFile{Name: "values.yaml", Content: []byte("k: v\n")})
	edit(l, a)
	if len(a.Spec.HealthChecks) == 0 {
		checks("v1", "ConfigMap", "red", "ready")(a)
	}
	return fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).IntegrateWithLayout(ml, c, rules)
}

// a generates the ConfigMap settings, whose annotation takes the inline
// value CUT, and health-checks red/ready, which b-longer writes and depends
// on. Where CUT makes the generated ConfigMap red/ready, a writes it too, so
// b-longer is not the only writer, a's wait never depends on b-longer alone,
// and the tree is admitted. A plain CUT leaves the ConfigMap as it was, and
// the cycle through b-longer is refused. A substituteFrom alone is read as it
// is for any built object: the generated name and namespace hold no variable,
// so the identity is as built and the cycle is refused.
func TestPostBuildGenerated_SubstitutedIdentityIsRead(t *testing.T) {
	fromCluster := []stack.SubstituteRef{{Kind: "ConfigMap", Name: "vars"}}
	cases := map[string]struct {
		postBuild *kustv1.PostBuild
		refused   bool
	}{
		"CUT makes the ConfigMap red/ready":   {postBuild: &kustv1.PostBuild{Substitute: map[string]string{"CUT": cutIdentity}}},
		"CUT is a plain string":               {postBuild: &kustv1.PostBuild{Substitute: map[string]string{"CUT": "plain"}}, refused: true},
		"CUT read from the cluster":           {postBuild: postBuildFrom(fromCluster), refused: true},
		"CUT unset, a postBuild substituting": {postBuild: &kustv1.PostBuild{Substitute: map[string]string{"OTHER": "x"}}, refused: true},
	}
	for name, tc := range cases {
		for _, placement := range integratedPlacements {
			t.Run(name+"/"+string(placement), func(t *testing.T) {
				err := generatedPair(t, placement, func(_ *layout.ManifestLayout, a *kustv1.Kustomization) {
					a.Spec.PostBuild = tc.postBuild
				})
				if tc.refused && (err == nil || !strings.Contains(err.Error(), "can never all become Ready")) {
					t.Fatalf("got %v, want the cycle through a's health check refused", err)
				}
				if !tc.refused && err != nil {
					t.Fatalf("got %v, want the tree admitted: a may write red/ready", err)
				}
			})
		}
	}
}

func postBuildFrom(refs []stack.SubstituteRef) *kustv1.PostBuild {
	pb := &kustv1.PostBuild{}
	for _, r := range refs {
		pb.SubstituteFrom = append(pb.SubstituteFrom, kustv1.SubstituteReference{Kind: r.Kind, Name: r.Name})
	}
	return pb
}

// The reading above rests on what Flux does with cutIdentity: the generated
// ConfigMap, substituted, is the ConfigMap red/ready (the first document of
// the substituted text; its second is only what the break leaves).
func TestPostBuildGenerated_FluxReadsTheInjectedIdentity(t *testing.T) {
	c := pairCluster(stack.HealthCheck{}, []client.Object{unstructuredObj("v1", "ConfigMap", "default", "a-cm")},
		[]client.Object{unstructuredObj("v1", "ConfigMap", "red", "ready")})
	c.Node.Children[0].Bundle.HealthChecks = nil
	ml, err := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).CreateLayoutWithResources(c, withPlacement(nodeOnly, layout.FluxIntegratedPerBundle))
	if err != nil {
		t.Fatalf("CreateLayoutWithResources: %v", err)
	}
	a := kustomizationsByName(ml)["a"]
	l := findLayout(ml, "a-cm")
	l.ConfigMapGenerators = append(l.ConfigMapGenerators, layout.ConfigMapGeneratorSpec{
		Name: "settings", Files: []string{"values.yaml"}, Annotations: map[string]string{"example.com/inject": "${CUT}"},
	})
	l.ExtraFiles = append(l.ExtraFiles, layout.ExtraFile{Name: "values.yaml", Content: []byte("k: v\n")})
	a.Spec.PostBuild = &kustv1.PostBuild{Substitute: map[string]string{"CUT": cutIdentity}}

	obj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(a)
	if err != nil {
		t.Fatal(err)
	}
	tree := filepath.Join(t.TempDir(), "repo")
	if err := os.CopyFS(tree, os.DirFS(writeAll(t, ml)["WriteToDisk"].root)); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(tree, filepath.FromSlash(a.Spec.Path))
	if _, err := fluxkustomize.NewGenerator(tree, unstructured.Unstructured{Object: obj}).WriteFile(dir); err != nil {
		t.Fatalf("Flux generator on %q: %v", a.Spec.Path, err)
	}
	built, err := buildLikeFlux(t, tree, dir)
	if err != nil {
		t.Fatalf("build of %q: %v", a.Spec.Path, err)
	}
	for _, r := range built.Resources() {
		if r.GetKind() != "ConfigMap" || !strings.HasPrefix(r.GetName(), "settings-") {
			continue
		}
		m, err := r.Map()
		if err != nil {
			t.Fatal(err)
		}
		got := substituted(t, a, &unstructured.Unstructured{Object: m})
		if name, ns := got["metadata"].(map[string]any)["name"], got["metadata"].(map[string]any)["namespace"]; name != "ready" || ns != "red" {
			t.Fatalf("the generated ConfigMap substituted is %v/%v, want red/ready: %v", ns, name, got)
		}
		return
	}
	t.Fatalf("no generated ConfigMap settings-* in the build of %q", a.Spec.Path)
}

// a's generator is named ${GEN:=settings}. Read as the tree holds it, the
// name is unresolved, so every ConfigMap check is uncertain and the tree is
// admitted. substituteStrategy Always substitutes it with no variable set,
// so the build makes settings plus a hash, which is not red/ready: b-longer
// is the certain writer and the cycle is refused. The default strategy has
// nothing to substitute, and stays admitted.
func TestPostBuildGenerated_AlwaysStrategySubstitutesTheName(t *testing.T) {
	for name, tc := range map[string]struct {
		strategy kustv1.SubstituteStrategy
		refused  bool
	}{
		"Always":        {strategy: kustv1.SubstituteStrategyAlways, refused: true},
		"WithVariables": {strategy: kustv1.SubstituteStrategyWithVariables},
	} {
		for _, placement := range integratedPlacements {
			t.Run(name+"/"+string(placement), func(t *testing.T) {
				err := generatedPair(t, placement, func(l *layout.ManifestLayout, a *kustv1.Kustomization) {
					l.ConfigMapGenerators[len(l.ConfigMapGenerators)-1].Name = "${GEN:=settings}"
					a.Spec.PostBuild = &kustv1.PostBuild{SubstituteStrategy: tc.strategy}
				})
				if tc.refused && (err == nil || !strings.Contains(err.Error(), "can never all become Ready")) {
					t.Fatalf("got %v, want the cycle through a's health check refused", err)
				}
				if !tc.refused && err != nil {
					t.Fatalf("got %v, want the tree admitted: the generated name is unresolved", err)
				}
			})
		}
	}
}

// a generates settings from a remote file, which kure does not read, and
// health-checks a ConfigMap nothing writes, so no cycle is in the tree. Where
// a's postBuild substitutes, a's build is built and not read, so the tree is
// refused naming it; where it substitutes nothing, a is read as the tree
// holds it, as before, and the tree is admitted. A tree the held read admitted
// with a generator kure cannot build is now refused at the build.
func TestPostBuildGenerated_UnreadBuildIsRefused(t *testing.T) {
	for name, tc := range map[string]struct {
		postBuild *kustv1.PostBuild
		unread    bool
	}{
		"inline variable":       {postBuild: &kustv1.PostBuild{Substitute: map[string]string{"X": "y"}}, unread: true},
		"substituteFrom":        {postBuild: postBuildFrom([]stack.SubstituteRef{{Kind: "ConfigMap", Name: "vars"}}), unread: true},
		"nothing to substitute": {postBuild: &kustv1.PostBuild{}},
		"no postBuild at all":   {},
	} {
		for _, placement := range integratedPlacements {
			t.Run(name+"/"+string(placement), func(t *testing.T) {
				err := generatedPair(t, placement, func(l *layout.ManifestLayout, a *kustv1.Kustomization) {
					l.ConfigMapGenerators[len(l.ConfigMapGenerators)-1].Files = []string{"https://example.com/values.yaml"}
					a.Spec.PostBuild = tc.postBuild
					checks("v1", "ConfigMap", "default", "nothing-writes-this")(a)
				})
				if tc.unread && (err == nil || !strings.Contains(err.Error(), "is not read")) {
					t.Fatalf("got %v, want a's build not read", err)
				}
				if !tc.unread && err != nil {
					t.Fatalf("got %v, want the tree admitted: a is read as the tree holds it", err)
				}
			})
		}
	}
}

// h, which builds a's directory, writes h-cm that b-longer health-checks.
// other, a Kustomization on another Source, builds a's directory too and is
// no writer. When a generates the ConfigMap settings and other substitutes an
// inline X it never uses, other may not transform a ConfigMap into h-cm:
// only a transform (components, patches, a name prefix) does, so h stays the
// certain writer and the cycle is refused.
func TestPostBuildGenerated_AnotherSourceSubstitutingIsNotATransform(t *testing.T) {
	for _, placement := range integratedPlacements {
		t.Run(string(placement), func(t *testing.T) {
			ml, err := integrateNonMemberWriter(t, placement, true, func(dir string) []*kustv1.Kustomization {
				o := handWritten("other")
				o.Spec.Path = dir
				o.Spec.SourceRef = kustv1.CrossNamespaceSourceReference{Kind: "GitRepository", Name: "other-repo"}
				o.Spec.PostBuild = &kustv1.PostBuild{Substitute: map[string]string{"X": "value"}}
				return []*kustv1.Kustomization{o}
			}, func(a *layout.ManifestLayout) {
				a.ConfigMapGenerators = append(a.ConfigMapGenerators, layout.ConfigMapGeneratorSpec{Name: "settings", Files: []string{"values.yaml"}})
				a.ExtraFiles = append(a.ExtraFiles, layout.ExtraFile{Name: "values.yaml", Content: []byte("k: v\n")})
			})
			if err == nil || !strings.Contains(err.Error(), "apply flux-system/h") {
				t.Fatalf("got %v, want the cycle through h refused; tree %s", err, describeKustomizations(ml))
			}
		})
	}
}
