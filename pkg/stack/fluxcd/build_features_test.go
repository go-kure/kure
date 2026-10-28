package fluxcd_test

import (
	"path"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	kustv1 "github.com/fluxcd/kustomize-controller/api/v1"
	"github.com/fluxcd/pkg/apis/kustomize"
	metaapi "github.com/fluxcd/pkg/apis/meta"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-kure/kure/pkg/stack"
	fluxstack "github.com/go-kure/kure/pkg/stack/fluxcd"
	"github.com/go-kure/kure/pkg/stack/layout"
)

// The reconcile-order checks read what Flux's apply and wait do with a Flux
// Kustomization's spec (go-kure/kure#1094): an object the apply skips leaves
// the wait, and a health check on the checker itself is dropped.

// handWrittenWritingWeb integrates noWaitCluster, web depending on platform,
// in placement with UmbrellasDoNotWait set; platform's application emits
// hand-written, whose spec.path is the directory holding web's Kustomization,
// so web's dependsOn is written as one on hand-written and hand-written's
// apply writes web, as the root Kustomization's does. It keeps the layout and
// re-integrates it after edit, returning the kept layout and the error of the
// re-integration.
func handWrittenWritingWeb(t *testing.T, placement layout.FluxPlacement, edit func(hw, web *kustv1.Kustomization)) (*layout.ManifestLayout, error) {
	t.Helper()
	k := handWritten("hand-written")
	k.Spec.Path = webDirectory(t, placement)
	var o client.Object = k
	app := stack.NewApplication("platform-app", "default", &fakeAppConfig{objs: []*client.Object{&o}})
	cl := noWaitCluster(srBundle("platform", app), true)
	rules := withPlacement(nodeOnly, placement)
	ml, err := fluxstack.NewLayoutIntegrator(noWaitGenerator()).CreateLayoutWithResources(cl, rules)
	if err != nil {
		t.Fatalf("first integration: %v", err)
	}
	assertDependsOn(t, ml, "web", "hand-written")
	hw := findKept(ml, "hand-written")
	if got := builtKustomizations(t, ml, path.Clean(hw.Spec.Path)); !slices.Contains(got, "web") {
		t.Fatalf("hand-written's build holds %v, want web among them", got)
	}
	edit(hw, findKept(ml, "web"))
	return ml, fluxstack.NewLayoutIntegrator(noWaitGenerator()).IntegrateWithLayout(ml, cl, rules)
}

// webPatch is a strategic-merge patch for the Flux Kustomization w that
// merges label, one "key: value" line, into its labels.
func webPatch(w *kustv1.Kustomization, label string) string {
	return "apiVersion: " + kustv1.GroupVersion.String() + "\nkind: " + kustv1.KustomizationKind +
		"\nmetadata:\n  name: " + w.Name + "\n  namespace: " + w.Namespace + "\n  labels:\n    " + label + "\n"
}

// builtKustomizations returns the names of the Flux Kustomizations the build
// of dir, a directory of the tree WriteToDisk writes for ml, holds, as
// kustomize-controller builds it.
func builtKustomizations(t *testing.T, ml *layout.ManifestLayout, dir string) []string {
	t.Helper()
	root := writeAll(t, ml)["WriteToDisk"].root
	built, err := buildLikeFlux(t, root, filepath.Join(root, filepath.FromSlash(dir)))
	if err != nil {
		t.Fatalf("build of %q: %v", dir, err)
	}
	var out []string
	for _, r := range built.Resources() {
		if r.GetKind() == kustv1.KustomizationKind && strings.HasPrefix(r.GetApiVersion(), kustv1.GroupVersion.Group+"/") {
			out = append(out, r.GetName())
		}
	}
	return out
}

// kustomize-controller leaves an object its apply skips (ssa Ignore, or
// reconcile disabled, on a label or annotation, any case) out of what its
// wait waits for. hand-written, with wait, writes web, which depends on
// hand-written: hand-written's wait waits for web, which waits for
// hand-written. With the mark in hand-written's spec.commonMetadata, its
// apply skips web and its wait no longer waits for it; the root
// Kustomization, which does not carry that commonMetadata, still creates
// web, so the tree converges. A mark on web itself skips web in every apply,
// the root's included: web never exists, a stall the checks do not judge,
// and hand-written's wait does not close the cycle either. ssa IfNotPresent
// skips only an object that exists, so hand-written's apply creates web and
// waits for it; a patch of hand-written's build that sets or clears web's
// mark is read as that build applies it; and a health check waits for a
// skipped object too.
func TestBuildFeatures_ExcludedObjectLeavesTheWait(t *testing.T) {
	label := func(key, value string) *kustv1.CommonMetadata {
		return &kustv1.CommonMetadata{Labels: map[string]string{key: value}}
	}
	annotation := func(key, value string) *kustv1.CommonMetadata {
		return &kustv1.CommonMetadata{Annotations: map[string]string{key: value}}
	}
	cases := map[string]struct {
		edit    func(hw, web *kustv1.Kustomization)
		refused bool
	}{
		"unmarked": {func(hw, _ *kustv1.Kustomization) { hw.Spec.Wait = true }, true},
		"commonMetadata label ssa IGNORE": {func(hw, _ *kustv1.Kustomization) {
			hw.Spec.Wait = true
			hw.Spec.CommonMetadata = label("kustomize.toolkit.fluxcd.io/ssa", "IGNORE")
		}, false},
		"commonMetadata annotation reconcile Disabled": {func(hw, _ *kustv1.Kustomization) {
			hw.Spec.Wait = true
			hw.Spec.CommonMetadata = annotation("kustomize.toolkit.fluxcd.io/reconcile", "Disabled")
		}, false},
		"web labelled ssa Ignore": {func(hw, w *kustv1.Kustomization) {
			hw.Spec.Wait = true
			w.Labels = map[string]string{"kustomize.toolkit.fluxcd.io/ssa": "Ignore"}
		}, false},
		"web annotated reconcile disabled": {func(hw, w *kustv1.Kustomization) {
			hw.Spec.Wait = true
			w.Annotations = map[string]string{"kustomize.toolkit.fluxcd.io/reconcile": "disabled"}
		}, false},
		"commonMetadata overrides web's mark": {func(hw, w *kustv1.Kustomization) {
			hw.Spec.Wait = true
			w.Labels = map[string]string{"kustomize.toolkit.fluxcd.io/ssa": "Ignore"}
			hw.Spec.CommonMetadata = label("kustomize.toolkit.fluxcd.io/ssa", "Merge")
		}, true},
		"commonMetadata ssa IfNotPresent": {func(hw, _ *kustv1.Kustomization) {
			hw.Spec.Wait = true
			hw.Spec.CommonMetadata = label("kustomize.toolkit.fluxcd.io/ssa", "IfNotPresent")
		}, true},
		"commonMetadata mark, patched build": {func(hw, w *kustv1.Kustomization) {
			hw.Spec.Wait = true
			hw.Spec.CommonMetadata = label("kustomize.toolkit.fluxcd.io/ssa", "Ignore")
			hw.Spec.Patches = []kustomize.Patch{{Patch: webPatch(w, "patched: \"yes\"")}}
		}, false},
		"a patch labels web ssa Ignore": {func(hw, w *kustv1.Kustomization) {
			hw.Spec.Wait = true
			hw.Spec.Patches = []kustomize.Patch{{Patch: webPatch(w, "kustomize.toolkit.fluxcd.io/ssa: Ignore")}}
		}, false},
		"a patch clears web's ssa Ignore": {func(hw, w *kustv1.Kustomization) {
			hw.Spec.Wait = true
			w.Labels = map[string]string{"kustomize.toolkit.fluxcd.io/ssa": "Ignore"}
			hw.Spec.Patches = []kustomize.Patch{{Patch: webPatch(w, "kustomize.toolkit.fluxcd.io/ssa: null")}}
		}, true},
		"commonMetadata mark, health check without wait": {func(hw, w *kustv1.Kustomization) {
			hw.Spec.CommonMetadata = label("kustomize.toolkit.fluxcd.io/ssa", "Ignore")
			hw.Spec.HealthChecks = []metaapi.NamespacedObjectKindReference{{APIVersion: kustv1.GroupVersion.String(), Kind: kustv1.KustomizationKind, Name: w.Name}}
		}, true},
	}
	for name, c := range cases {
		for _, placement := range integratedPlacements {
			t.Run(name+"/"+string(placement), func(t *testing.T) {
				ml, err := handWrittenWritingWeb(t, placement, c.edit)
				if c.refused {
					if err == nil || !strings.Contains(err.Error(), "can never all become Ready") {
						t.Fatalf("got %v, want a reconcile-order refusal (%s)", err, describeKustomizations(ml))
					}
					return
				}
				if err != nil {
					t.Fatalf("got %v, want the tree admitted (%s)", err, describeKustomizations(ml))
				}
				// The creation route: the root's build holds web too, so the
				// root Kustomization applies it without platform's
				// commonMetadata.
				if got := builtKustomizations(t, ml, ml.FullRepoPath()); !slices.Contains(got, "web") {
					t.Fatalf("the root's build holds %v, want web among them", got)
				}
			})
		}
	}
}

// keptPlatform integrates noWaitCluster, web depending on platform, in
// placement, keeps it, and re-integrates it after edit.
func keptPlatform(t *testing.T, placement layout.FluxPlacement, edit func(ml *layout.ManifestLayout)) (*layout.ManifestLayout, error) {
	t.Helper()
	return keptPlatformIn(t, fluxstack.DefaultNamespace, placement, edit)
}

// keptPlatformIn is keptPlatform with the generator writing its Flux
// resources into namespace, none for "".
func keptPlatformIn(t *testing.T, namespace string, placement layout.FluxPlacement, edit func(ml *layout.ManifestLayout)) (*layout.ManifestLayout, error) {
	t.Helper()
	rules := withPlacement(nodeOnly, placement)
	cl := noWaitCluster(srBundle("platform", cmApp("platform-app")), true)
	gen := func() *fluxstack.ResourceGenerator {
		g := fluxstack.NewResourceGenerator()
		g.DefaultNamespace = namespace
		return g
	}
	ml, err := fluxstack.NewLayoutIntegrator(gen()).CreateLayoutWithResources(cl, rules)
	if err != nil {
		t.Fatalf("first integration: %v", err)
	}
	edit(ml)
	return ml, fluxstack.NewLayoutIntegrator(gen()).IntegrateWithLayout(ml, cl, rules)
}

// kustomizationCheck is a health check on the Flux Kustomization
// namespace/name.
func kustomizationCheck(namespace, name string) metaapi.NamespacedObjectKindReference {
	return metaapi.NamespacedObjectKindReference{APIVersion: kustv1.GroupVersion.String(), Kind: kustv1.KustomizationKind, Namespace: namespace, Name: name}
}

// Flux drops a health check on the checker itself when its namespace and
// name, as written, are the checker's (kustomize-controller v1.10.1
// internal/controller/kustomization_controller.go:1118-1126): a kept
// platform without wait checking itself waits for nothing. With no
// namespace Flux neither defaults it nor drops it, and its poller rejects
// the check, so platform never becomes Ready: still refused, by name. Two
// Kustomizations checking each other wait for each other. Written without
// a namespace, platform exists on the cluster in "default", where a check
// naming default/platform is the self-check Flux drops.
func TestBuildFeatures_SelfHealthCheck(t *testing.T) {
	cases := map[string]struct {
		namespace string
		edit      func(ml *layout.ManifestLayout)
		want      string
	}{
		"self, with namespace": {fluxstack.DefaultNamespace, func(ml *layout.ManifestLayout) {
			p := findKept(ml, "platform")
			p.Spec.Wait = false
			p.Spec.HealthChecks = []metaapi.NamespacedObjectKindReference{kustomizationCheck(p.Namespace, p.Name)}
		}, ""},
		"self, no namespace": {fluxstack.DefaultNamespace, func(ml *layout.ManifestLayout) {
			p := findKept(ml, "platform")
			p.Spec.Wait = false
			p.Spec.HealthChecks = []metaapi.NamespacedObjectKindReference{kustomizationCheck("", p.Name)}
		}, "health-checks itself with no namespace"},
		"self, default namespace on an omitted one": {"", func(ml *layout.ManifestLayout) {
			p := findKept(ml, "platform")
			p.Spec.Wait = false
			p.Spec.HealthChecks = []metaapi.NamespacedObjectKindReference{kustomizationCheck("default", p.Name)}
		}, ""},
		"each other": {fluxstack.DefaultNamespace, func(ml *layout.ManifestLayout) {
			p, w := findKept(ml, "platform"), findKept(ml, "web")
			w.Spec.DependsOn = nil
			p.Spec.Wait, w.Spec.Wait = false, false
			p.Spec.HealthChecks = []metaapi.NamespacedObjectKindReference{kustomizationCheck(w.Namespace, w.Name)}
			w.Spec.HealthChecks = []metaapi.NamespacedObjectKindReference{kustomizationCheck(p.Namespace, p.Name)}
		}, "can never all become Ready"},
	}
	for name, c := range cases {
		for _, placement := range integratedPlacements {
			t.Run(name+"/"+string(placement), func(t *testing.T) {
				ml, err := keptPlatformIn(t, c.namespace, placement, c.edit)
				if c.want == "" {
					if err != nil {
						t.Fatalf("got %v, want the tree admitted (%s)", err, describeKustomizations(ml))
					}
					return
				}
				if err == nil || !strings.Contains(err.Error(), c.want) {
					t.Fatalf("got %v, want a refusal naming %q (%s)", err, c.want, describeKustomizations(ml))
				}
			})
		}
	}
}

// Guards for what the checks document rather than model (go-kure/kure#1094):
// a health check on web decided by an expression that is always current is
// read as readiness, as one on the Ready condition is (Flux evaluates it
// after its generation guard; the expression is arbitrary CEL), and a cycle
// through a suspended Kustomization stays refused, since a suspended one
// never applies on a fresh install.
func TestBuildFeatures_DocumentedReads(t *testing.T) {
	cases := map[string]func(ml *layout.ManifestLayout){
		"expression current true": func(ml *layout.ManifestLayout) {
			p, w := findKept(ml, "platform"), findKept(ml, "web")
			p.Spec.Wait = false
			p.Spec.HealthChecks = []metaapi.NamespacedObjectKindReference{kustomizationCheck(w.Namespace, w.Name)}
			p.Spec.HealthCheckExprs = []kustomize.CustomHealthCheck{currentExpr(kustv1.GroupVersion.String(), kustv1.KustomizationKind)}
		},
		"suspended": func(ml *layout.ManifestLayout) {
			p, w := findKept(ml, "platform"), findKept(ml, "web")
			p.Spec.Wait, p.Spec.Suspend = false, true
			p.Spec.HealthChecks = []metaapi.NamespacedObjectKindReference{kustomizationCheck(w.Namespace, w.Name)}
		},
	}
	for name, edit := range cases {
		for _, placement := range integratedPlacements {
			t.Run(name+"/"+string(placement), func(t *testing.T) {
				ml, err := keptPlatform(t, placement, edit)
				if err == nil || !strings.Contains(err.Error(), "can never all become Ready") {
					t.Fatalf("got %v, want a reconcile-order refusal (%s)", err, describeKustomizations(ml))
				}
			})
		}
	}
}

// keptUmbrella integrates demo -> apps (umbrella platform with child child)
// in placement, keeps it, and re-integrates it after edit.
func keptUmbrella(t *testing.T, placement layout.FluxPlacement, edit func(platform, child *kustv1.Kustomization)) (*layout.ManifestLayout, error) {
	t.Helper()
	node := &stack.Node{Name: "apps", Bundle: umbrellaOf("platform", srBundle("child", cmApp("child-app")))}
	root := &stack.Node{Name: "demo", Children: []*stack.Node{node}}
	node.SetParent(root)
	cl := &stack.Cluster{Name: "demo", Node: root}
	rules := withPlacement(nodeOnly, placement)
	ml, err := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).CreateLayoutWithResources(cl, rules)
	if err != nil {
		t.Fatalf("first integration: %v", err)
	}
	edit(findKept(ml, "platform"), findKept(ml, "child"))
	return ml, fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).IntegrateWithLayout(ml, cl, rules)
}

// The members' reconcile-order check reads the same exclusion: an umbrella
// with wait waits for the child it applies, and a child depending on its
// umbrella closes the cycle. With the mark in the umbrella's
// spec.commonMetadata, or on the child itself, the umbrella's apply skips
// the child, which then never exists (no other Kustomization applies an
// umbrella's child): a stall the checks do not judge, and no cycle. ssa
// IfNotPresent still creates the child, and the umbrella waits for it.
func TestBuildFeatures_ExcludedChildLeavesTheUmbrellaWait(t *testing.T) {
	label := func(key, value string) *kustv1.CommonMetadata {
		return &kustv1.CommonMetadata{Labels: map[string]string{key: value}}
	}
	cases := map[string]struct {
		edit    func(platform, child *kustv1.Kustomization)
		refused bool
	}{
		"unmarked": {func(*kustv1.Kustomization, *kustv1.Kustomization) {}, true},
		"commonMetadata label ssa Ignore": {func(p, _ *kustv1.Kustomization) {
			p.Spec.CommonMetadata = label("kustomize.toolkit.fluxcd.io/ssa", "Ignore")
		}, false},
		"child annotated reconcile disabled": {func(_, c *kustv1.Kustomization) {
			c.Annotations = map[string]string{"kustomize.toolkit.fluxcd.io/reconcile": "disabled"}
		}, false},
		"commonMetadata ssa IfNotPresent": {func(p, _ *kustv1.Kustomization) {
			p.Spec.CommonMetadata = label("kustomize.toolkit.fluxcd.io/ssa", "IfNotPresent")
		}, true},
	}
	for name, c := range cases {
		for _, placement := range integratedPlacements {
			t.Run(name+"/"+string(placement), func(t *testing.T) {
				ml, err := keptUmbrella(t, placement, func(p, ch *kustv1.Kustomization) {
					p.Spec.Wait = true
					ch.Spec.DependsOn = []kustv1.DependencyReference{{Name: p.Name}}
					c.edit(p, ch)
				})
				if c.refused {
					if err == nil || !strings.Contains(err.Error(), "ready flux-system/platform -> ready flux-system/child -> apply flux-system/child -> ready flux-system/platform") {
						t.Fatalf("got %v, want a reconcile-order refusal through child (%s)", err, describeKustomizations(ml))
					}
					return
				}
				if err != nil {
					t.Fatalf("got %v, want the tree admitted (%s)", err, describeKustomizations(ml))
				}
			})
		}
	}
}
