package fluxcd_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	kustv1 "github.com/fluxcd/kustomize-controller/api/v1"
	"github.com/fluxcd/pkg/apis/kustomize"
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
// writer stays uncertain. Written ${NS:=red}, the namespace is red where NS is
// set nowhere, and refused; read from the cluster, NS may override the
// default, and the writer stays uncertain.
func TestBuiltObjects_SubstitutedIdentityIsRead(t *testing.T) {
	fromCluster := []stack.SubstituteRef{{Kind: "ConfigMap", Name: "vars"}}
	cases := map[string]struct {
		namespace string
		postBuild *stack.PostBuild
		refused   bool
	}{
		"NS set inline":                       {postBuild: &stack.PostBuild{Substitute: map[string]string{"NS": "red"}}, refused: true},
		"NS read from the cluster":            {postBuild: &stack.PostBuild{SubstituteFrom: fromCluster}},
		"NS set nowhere":                      {postBuild: &stack.PostBuild{Substitute: map[string]string{"OTHER": "red"}}},
		"NS defaulted":                        {namespace: "${NS:=red}", postBuild: &stack.PostBuild{Substitute: map[string]string{"OTHER": "red"}}, refused: true},
		"NS defaulted, read from the cluster": {namespace: "${NS:=red}", postBuild: &stack.PostBuild{Substitute: map[string]string{"OTHER": "red"}, SubstituteFrom: fromCluster}},
	}
	for name, tc := range cases {
		for _, placement := range append(slices.Clone(integratedPlacements), layout.FluxSeparate) {
			t.Run(name+"/"+string(placement), func(t *testing.T) {
				namespace := tc.namespace
				if namespace == "" {
					namespace = "${NS}"
				}
				c := pairCluster(stack.HealthCheck{APIVersion: "v1", Kind: "ConfigMap", Namespace: "red", Name: "ready"},
					[]client.Object{unstructuredObj("v1", "ConfigMap", "default", "a-cm")},
					[]client.Object{unstructuredObj("v1", "ConfigMap", namespace, "ready")})
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

// A Kustomization that is no member, but whose build is the tree's, may be
// the certain writer: a's application emits h, which builds a's directory
// with a component adding default/h-cm, which nothing else applies, and
// b-longer health-checks h-cm. Under UmbrellasDoNotWait, which follows the
// other Kustomizations of the tree, b-longer waits for h's apply; when h
// depends on b-longer the cycle is refused, naming it, and without that
// dependsOn the tree is admitted.
func TestBuiltObjects_NonMemberIsTheCertainWriter(t *testing.T) {
	for _, dependsOnB := range []bool{true, false} {
		for _, placement := range integratedPlacements {
			name := "h independent/"
			if dependsOnB {
				name = "h depends on b-longer/"
			}
			t.Run(name+string(placement), func(t *testing.T) {
				ml, err := integrateNonMemberWriter(t, placement, dependsOnB, nil)
				if dependsOnB && (err == nil || !strings.Contains(err.Error(), "can never all become Ready") || !strings.Contains(err.Error(), "apply flux-system/h")) {
					t.Fatalf("got %v, want the cycle through b-longer's health check on what h writes refused; tree %s", err, describeKustomizations(ml))
				}
				if !dependsOnB && err != nil {
					t.Fatalf("got %v, want the tree admitted", err)
				}
			})
		}
	}
}

// integrateNonMemberWriter integrates, under UmbrellasDoNotWait, the tree of
// TestBuiltObjects_NonMemberIsTheCertainWriter: a emits h, whose build of a's
// directory adds h-cm through a component, and b-longer, without wait,
// health-checks h-cm; with dependsOnB h depends on b-longer. a also emits
// each of others.
func integrateNonMemberWriter(t *testing.T, placement layout.FluxPlacement, dependsOnB bool, others func(dir string) []*kustv1.Kustomization) (*layout.ManifestLayout, error) {
	t.Helper()
	c := pairCluster(stack.HealthCheck{},
		[]client.Object{unstructuredObj("v1", "ConfigMap", "default", "a-cm")},
		[]client.Object{unstructuredObj("v1", "ConfigMap", "default", "b-cm")})
	c.Node.Children[0].Bundle.HealthChecks = nil
	// No dependsOn on a, which the setting would write as one on h:
	// only the existence edge onto h's apply can close the cycle.
	c.Node.Children[1].Bundle.DependsOn = nil
	// A leaf waits under the setting, and Flux ignores health checks
	// under wait.
	c.Node.Children[1].Bundle.Wait = boolPtr(false)
	c.Node.Children[1].Bundle.HealthChecks = []stack.HealthCheck{{APIVersion: "v1", Kind: "ConfigMap", Namespace: "default", Name: "h-cm"}}
	rules := withPlacement(nodeOnly, placement)
	ml, err := layout.WalkCluster(c, rules)
	if err != nil {
		t.Fatalf("WalkCluster: %v", err)
	}
	a := findLayout(ml, "a-cm")
	if a == nil {
		t.Fatal("no layout holds a-cm")
	}
	var h *kustv1.Kustomization
	if dependsOnB {
		h = handWritten("h", "b-longer")
	} else {
		h = handWritten("h")
	}
	h.Spec.Path = "./" + a.FullRepoPath()
	h.Spec.Components = []string{"component"}
	a.Resources = append(a.Resources, h)
	if others != nil {
		for _, o := range others("./" + a.FullRepoPath()) {
			a.Resources = append(a.Resources, o)
		}
	}
	// a's kustomization.yaml lists neither file: only h's build,
	// through its component, holds h-cm.
	a.ExtraFiles = append(a.ExtraFiles,
		layout.ExtraFile{Name: "component/kustomization.yaml", Content: []byte("apiVersion: kustomize.config.k8s.io/v1alpha1\nkind: Component\nresources:\n- h-cm.yaml\n")},
		layout.ExtraFile{Name: "component/h-cm.yaml", Content: []byte("apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: h-cm\n  namespace: default\n")})
	return ml, fluxstack.NewLayoutIntegrator(noWaitGenerator()).IntegrateWithLayout(ml, c, rules)
}

// A Kustomization on another Source that builds a's directory is no writer:
// its build is not the tree's, so it is read as the directory holds it, which
// shows no h-cm. A transform may still make one (spec.components; spec.patches
// over a directory that holds objects; a name prefix over one that holds a
// ConfigMap), so h stops being the certain writer, the existence edge goes
// and the tree is admitted. Without a transform h stays certain and the cycle
// is refused.
func TestBuiltObjects_AnotherSourcesTransformLeavesTheWriterUncertain(t *testing.T) {
	for name, tc := range map[string]struct {
		edit    func(k *kustv1.Kustomization)
		refused bool
	}{
		"components": {edit: func(k *kustv1.Kustomization) { k.Spec.Components = []string{"component"} }},
		"patches": {edit: func(k *kustv1.Kustomization) {
			k.Spec.Patches = []kustomize.Patch{{Patch: "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: a-cm\n", Target: &kustomize.Selector{Kind: "ConfigMap"}}}
		}},
		"name prefix":  {edit: func(k *kustv1.Kustomization) { k.Spec.NamePrefix = "h-" }},
		"no transform": {edit: func(*kustv1.Kustomization) {}, refused: true},
	} {
		for _, placement := range integratedPlacements {
			t.Run(name+"/"+string(placement), func(t *testing.T) {
				ml, err := integrateNonMemberWriter(t, placement, true, func(dir string) []*kustv1.Kustomization {
					o := handWritten("other")
					o.Spec.Path = dir
					o.Spec.SourceRef = kustv1.CrossNamespaceSourceReference{Kind: "GitRepository", Name: "other-repo"}
					tc.edit(o)
					return []*kustv1.Kustomization{o}
				})
				if tc.refused && (err == nil || !strings.Contains(err.Error(), "apply flux-system/h")) {
					t.Fatalf("got %v, want the cycle through h refused; tree %s", err, describeKustomizations(ml))
				}
				if !tc.refused && err != nil {
					t.Fatalf("got %v, want the tree admitted: other may write h-cm", err)
				}
			})
		}
	}
}

// A Kustomization on another Source that builds a's directory with a
// component, other, is read as its first copy an apply does not skip: a's
// copy carries kustomize.toolkit.fluxcd.io/ssa Ignore, so a's apply skips it.
// Where p, which also builds a's directory, removes the mark, other applies
// with its component and h stops being the certain writer: the tree is
// admitted. Where p also gives other a spec.kubeConfig, other applies on
// another cluster, h stays certain and the cycle is refused; so it is where no
// p applies other at all.
func TestBuiltObjects_AnotherSourceIsReadAsItsAppliedCopy(t *testing.T) {
	const unskip = "- op: remove\n  path: /metadata/annotations/kustomize.toolkit.fluxcd.io~1ssa\n"
	for name, c := range map[string]struct {
		patch   string
		refused bool
	}{
		"applied here":           {patch: unskip},
		"applied on another one": {patch: unskip + "- op: add\n  path: /spec/kubeConfig\n  value:\n    secretRef:\n      name: remote-kubeconfig\n", refused: true},
		"never applied":          {refused: true},
	} {
		for _, placement := range integratedPlacements {
			t.Run(name+"/"+string(placement), func(t *testing.T) {
				ml, err := integrateNonMemberWriter(t, placement, true, func(dir string) []*kustv1.Kustomization {
					o := handWritten("other")
					o.Spec.Path, o.Spec.Components = dir, []string{"component"}
					o.Spec.SourceRef = kustv1.CrossNamespaceSourceReference{Kind: "GitRepository", Name: "other-repo"}
					o.Annotations = map[string]string{"kustomize.toolkit.fluxcd.io/ssa": "Ignore"}
					out := []*kustv1.Kustomization{o}
					if c.patch != "" {
						p := handWritten("p")
						p.Spec.Path = dir
						p.Spec.Patches = []kustomize.Patch{{Patch: c.patch, Target: &kustomize.Selector{Kind: "Kustomization", Name: "other"}}}
						out = append(out, p)
					}
					return out
				})
				if c.refused && (err == nil || !strings.Contains(err.Error(), "apply flux-system/h")) {
					t.Fatalf("got %v, want the cycle through h refused; tree %s", err, describeKustomizations(ml))
				}
				if !c.refused && err != nil {
					t.Fatalf("got %v, want the tree admitted: other may write h-cm", err)
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
// the edge certain and the cycle is refused. So does a literal Secret
// b-longer also writes, credsx or creds-foo, whose name is neither creds nor
// creds and a content hash. A name suffix is not read: the merged Secret
// built as credsx under nameSuffix x keeps its writer certain too, and the
// tree is refused.
func TestBuiltObjects_MergedGeneratorSecretStaysUncertain(t *testing.T) {
	creds := unstructuredObj("v1", "Secret", "default", "creds")
	creds.Object["data"] = map[string]any{"base": "dg=="}
	const merge = "- name: creds\n  namespace: default\n  behavior: merge\n  literals:\n  - k=v\n"
	cases := map[string]struct {
		generator, prefix, suffix, literal string
		refused                            bool
	}{
		"merge":                    {generator: merge, prefix: "creds"},
		"merge, name suffix x":     {generator: merge, prefix: "credsx", suffix: "x", refused: true},
		"create":                   {generator: "- name: extra\n  namespace: default\n  literals:\n  - k=v\n", prefix: "extra-", refused: true},
		"merge, literal credsx":    {generator: merge, prefix: "credsx", literal: "credsx", refused: true},
		"merge, literal creds-foo": {generator: merge, prefix: "creds-foo", literal: "creds-foo", refused: true},
	}
	for name, tc := range cases {
		for _, placement := range integratedPlacements {
			t.Run(name+"/"+string(placement), func(t *testing.T) {
				objs := []client.Object{unstructuredObj("v1", "ConfigMap", "default", "b-cm"), creds.DeepCopy()}
				if tc.literal != "" {
					objs = append(objs, unstructuredObj("v1", "Secret", "default", tc.literal))
				}
				err := keptPair(t, placement, objs, "Secret", tc.prefix,
					func(l *layout.ManifestLayout, b *kustv1.Kustomization) {
						l.ExtraFiles = append(l.ExtraFiles, layout.ExtraFile{
							Name:    "component/kustomization.yaml",
							Content: []byte("apiVersion: kustomize.config.k8s.io/v1alpha1\nkind: Component\nsecretGenerator:\n" + tc.generator),
						})
						b.Spec.Components = []string{"component"}
						b.Spec.NameSuffix = tc.suffix
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
