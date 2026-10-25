package fluxcd_test

import (
	"errors"
	"path"
	"strings"
	"testing"

	kustv1 "github.com/fluxcd/kustomize-controller/api/v1"
	kustv1beta2 "github.com/fluxcd/kustomize-controller/api/v1beta2"
	"github.com/fluxcd/pkg/apis/kustomize"
	metaapi "github.com/fluxcd/pkg/apis/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-kure/kure/pkg/stack"
	fluxstack "github.com/go-kure/kure/pkg/stack/fluxcd"
	"github.com/go-kure/kure/pkg/stack/layout"
)

// The reconcile-order checks read a Flux Kustomization's spec fields in their
// context (go-kure/kure#1062): spec.path is a directory of the tree only on
// the tree's own source, a health check on another cluster names none of the
// tree's Kustomizations, and one a healthCheckExprs predicate decides waits
// for no readiness.

// remoteCluster is a spec.kubeConfig: Flux applies to, and polls, that
// cluster.
func remoteCluster() *metaapi.KubeConfigReference {
	return &metaapi.KubeConfigReference{SecretRef: &metaapi.SecretKeyReference{Name: "remote-kubeconfig"}}
}

// currentExpr is a healthCheckExprs entry for apiVersion and kind that
// evaluates every object it matches as current.
func currentExpr(apiVersion, kind string) kustomize.CustomHealthCheck {
	return kustomize.CustomHealthCheck{APIVersion: apiVersion, Kind: kind, HealthCheckExpressions: kustomize.HealthCheckExpressions{Current: "true"}}
}

// webDirectory is the directory that holds web's Kustomization once
// noWaitCluster is integrated in placement.
func webDirectory(t *testing.T, placement layout.FluxPlacement) string {
	t.Helper()
	off, err := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).CreateLayoutWithResources(noWaitCluster(srBundle("platform", cmApp("platform-app")), true), withPlacement(nodeOnly, placement))
	if err != nil {
		t.Fatalf("without the setting: %v", err)
	}
	return path.Dir(path.Clean(findKept(off, "web").Spec.Path))
}

// integrateWithHandWritten integrates noWaitCluster, web depending on
// platform, whose application emits hand-written as edit leaves it: platform
// applies it, so web's dependsOn is written as one on hand-written.
func integrateWithHandWritten(placement layout.FluxPlacement, edit func(k *kustv1.Kustomization)) (*layout.ManifestLayout, error) {
	k := handWritten("hand-written")
	edit(k)
	var o client.Object = k
	app := stack.NewApplication("platform-app", "default", &fakeAppConfig{objs: []*client.Object{&o}})
	return integrateNoWait(noWaitCluster(srBundle("platform", app), true), nodeOnly, placement)
}

// An application's Kustomization with wait on the tree's own source builds
// the directory its spec.path names: there, web's Kustomization, which
// depends on it once rewritten. Neither becomes Ready, and the rewrite is
// refused, whether or not its sourceRef spells the namespace out.
func TestReconcileContext_SameSourceWaitIsACycle(t *testing.T) {
	for _, placement := range integratedPlacements {
		for _, namespace := range []string{"", "flux-system"} {
			t.Run(string(placement)+"/sourceRef namespace "+namespace, func(t *testing.T) {
				dir := webDirectory(t, placement)
				_, err := integrateWithHandWritten(placement, func(k *kustv1.Kustomization) {
					k.Spec.Wait, k.Spec.Path = true, dir
					k.Spec.SourceRef.Namespace = namespace
				})
				if !errors.Is(err, fluxstack.ErrUmbrellaDependency) || !strings.Contains(err.Error(), "ready flux-system/hand-written -> ready flux-system/web -> apply flux-system/web") {
					t.Fatalf("got %v, want ErrUmbrellaDependency naming the chain through web", err)
				}
			})
		}
	}
}

// The same Kustomization reading another Source, or applying to another
// cluster, builds no directory of this tree: its wait waits for nothing
// here, and the rewrite is accepted. Another Source of the same repository
// (another ref) is another Source.
func TestReconcileContext_ForeignWaitIsNoCycle(t *testing.T) {
	cases := map[string]func(k *kustv1.Kustomization){
		"another Source name": func(k *kustv1.Kustomization) { k.Spec.SourceRef.Name = "flux-system-next" },
		"another Source kind": func(k *kustv1.Kustomization) { k.Spec.SourceRef.Kind = "OCIRepository" },
		"another Source namespace": func(k *kustv1.Kustomization) {
			k.Spec.SourceRef.Namespace = "elsewhere"
		},
		"another cluster": func(k *kustv1.Kustomization) { k.Spec.KubeConfig = remoteCluster() },
	}
	for name, edit := range cases {
		for _, placement := range integratedPlacements {
			t.Run(name+"/"+string(placement), func(t *testing.T) {
				dir := webDirectory(t, placement)
				ml, err := integrateWithHandWritten(placement, func(k *kustv1.Kustomization) {
					k.Spec.Wait, k.Spec.Path = true, dir
					edit(k)
				})
				if err != nil {
					t.Fatalf("CreateLayoutWithResources: %v", err)
				}
				assertDependsOn(t, ml, "web", "hand-written")
			})
		}
	}
}

// A health check of hand-written on web closes the same cycle as a wait,
// unless it does not wait for web's readiness: on another cluster it names
// another Kustomization, and where a healthCheckExprs entry matches Flux
// Kustomizations (by group and kind, any version; or the whole group) Flux
// evaluates that instead. An entry for another kind leaves the check as it
// was. With wait, the predicate decides what the wait polls the same way.
func TestReconcileContext_HealthCheckContext(t *testing.T) {
	checksWeb := func(k *kustv1.Kustomization) {
		k.Spec.HealthChecks = []metaapi.NamespacedObjectKindReference{{APIVersion: kustv1.GroupVersion.String(), Kind: "Kustomization", Name: "web"}}
	}
	type verdict struct {
		edit    func(t *testing.T, placement layout.FluxPlacement, k *kustv1.Kustomization)
		refused bool
	}
	withExpr := func(e kustomize.CustomHealthCheck) func(*testing.T, layout.FluxPlacement, *kustv1.Kustomization) {
		return func(_ *testing.T, _ layout.FluxPlacement, k *kustv1.Kustomization) {
			checksWeb(k)
			k.Spec.HealthCheckExprs = []kustomize.CustomHealthCheck{e}
		}
	}
	cases := map[string]verdict{
		"a health check on web": {edit: func(_ *testing.T, _ layout.FluxPlacement, k *kustv1.Kustomization) { checksWeb(k) }, refused: true},
		"on another cluster": {edit: func(_ *testing.T, _ layout.FluxPlacement, k *kustv1.Kustomization) {
			checksWeb(k)
			k.Spec.KubeConfig = remoteCluster()
		}},
		"an expression for Kustomization":          {edit: withExpr(currentExpr(kustv1.GroupVersion.String(), "Kustomization"))},
		"an expression at another version":         {edit: withExpr(currentExpr(kustv1beta2.GroupVersion.String(), "Kustomization"))},
		"an expression for the whole group":        {edit: withExpr(currentExpr(kustv1.GroupVersion.String(), ""))},
		"an expression for another kind":           {edit: withExpr(currentExpr("apps/v1", "Deployment")), refused: true},
		"an expression for the group's other kind": {edit: withExpr(currentExpr(kustv1.GroupVersion.String(), "Other")), refused: true},
		"wait with an expression for Kustomization": {edit: func(t *testing.T, placement layout.FluxPlacement, k *kustv1.Kustomization) {
			k.Spec.Wait, k.Spec.Path = true, webDirectory(t, placement)
			k.Spec.HealthCheckExprs = []kustomize.CustomHealthCheck{currentExpr(kustv1.GroupVersion.String(), "Kustomization")}
		}},
	}
	for name, c := range cases {
		for _, placement := range integratedPlacements {
			t.Run(name+"/"+string(placement), func(t *testing.T) {
				ml, err := integrateWithHandWritten(placement, func(k *kustv1.Kustomization) { c.edit(t, placement, k) })
				if c.refused {
					if !errors.Is(err, fluxstack.ErrUmbrellaDependency) || !strings.Contains(err.Error(), "ready flux-system/hand-written -> ready flux-system/web") {
						t.Fatalf("got %v, want ErrUmbrellaDependency naming the chain through web", err)
					}
					return
				}
				if err != nil {
					t.Fatalf("CreateLayoutWithResources: %v", err)
				}
				assertDependsOn(t, ml, "web", "hand-written")
			})
		}
	}
}

// A Kustomization a healthCheckExprs predicate decides must still exist for
// Flux to evaluate it: hand-written, checking inner that web's apply writes,
// becomes Ready only once web applied, and web, depending on it once
// rewritten, applies only once it is Ready. Refused, as the Ready edge would
// be; on another cluster, inner is not this one, and it is accepted.
func TestReconcileContext_ExistenceIsAwaited(t *testing.T) {
	for _, remote := range []bool{false, true} {
		for _, placement := range integratedPlacements {
			t.Run(string(placement)+map[bool]string{false: "/local", true: "/remote"}[remote], func(t *testing.T) {
				k := handWritten("hand-written")
				k.Spec.HealthChecks = []metaapi.NamespacedObjectKindReference{{APIVersion: kustv1.GroupVersion.String(), Kind: "Kustomization", Name: "inner"}}
				k.Spec.HealthCheckExprs = []kustomize.CustomHealthCheck{currentExpr(kustv1.GroupVersion.String(), "Kustomization")}
				if remote {
					k.Spec.KubeConfig = remoteCluster()
				}
				var o client.Object = k
				platform := srBundle("platform", stack.NewApplication("platform-app", "default", &fakeAppConfig{objs: []*client.Object{&o}}))
				c := noWaitCluster(platform, true)
				web := c.Node.Children[1].Bundle
				web.Applications = append(web.Applications, fluxKustomizationApp("inner-app", "inner"))
				ml, err := integrateNoWait(c, nodeOnly, placement)
				if remote {
					if err != nil {
						t.Fatalf("CreateLayoutWithResources: %v", err)
					}
					assertDependsOn(t, ml, "web", "hand-written")
					return
				}
				if !errors.Is(err, fluxstack.ErrUmbrellaDependency) || !strings.Contains(err.Error(), "ready flux-system/hand-written -> apply flux-system/web") {
					t.Fatalf("got %v, want ErrUmbrellaDependency naming hand-written's wait for web's apply", err)
				}
			})
		}
	}
}

// A Kustomization kept from the tree in place of a generated one must read
// what the generated one would, on this cluster: its spec.path is a directory
// of this tree only then. One on another Source, or with spec.kubeConfig, is
// a namesake, and refused as one. Its sourceRef namespace may be left out.
//
// Every Kustomization of the first integration is tried: the bundles' (kept
// where the integration adds its objects) and, under FluxIntegratedPerLayout
// with grouping by name, the layouts' (kept where it places them).
func TestReconcileContext_KeptKustomizationSource(t *testing.T) {
	cases := map[string]struct {
		edit func(k *kustv1.Kustomization)
		want string
	}{
		"another Source":    {func(k *kustv1.Kustomization) { k.Spec.SourceRef.Name = "flux-system-next" }, "on GitRepository flux-system/flux-system-next"},
		"another cluster":   {func(k *kustv1.Kustomization) { k.Spec.KubeConfig = remoteCluster() }, "with spec.kubeConfig"},
		"namespace omitted": {func(k *kustv1.Kustomization) { k.Spec.SourceRef.Namespace = "" }, ""},
	}
	ruleSets := map[string]layout.LayoutRules{
		"per bundle": withPlacement(nodeOnly, layout.FluxIntegratedPerBundle),
		"per layout": withPlacement(nodeOnly, layout.FluxIntegratedPerLayout),
		"per layout, grouped by name": {
			BundleGrouping: layout.GroupByName, ApplicationGrouping: layout.GroupByName,
			FluxPlacement: layout.FluxIntegratedPerLayout,
		},
	}
	build := func() *stack.Cluster { return noWaitCluster(srBundle("platform", cmApp("platform-app")), false) }
	for name, c := range cases {
		for rulesName, rules := range ruleSets {
			first, err := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).CreateLayoutWithResources(build(), rules)
			if err != nil {
				t.Fatalf("first integration: %v", err)
			}
			for _, kept := range kustomizations(first) {
				t.Run(name+"/"+rulesName+"/"+kept.Name, func(t *testing.T) {
					c2 := build()
					ml, err := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).CreateLayoutWithResources(c2, rules)
					if err != nil {
						t.Fatalf("first integration: %v", err)
					}
					c.edit(findKept(ml, kept.Name))
					err = fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).IntegrateWithLayout(ml, c2, rules)
					if c.want == "" {
						if err != nil {
							t.Fatalf("got %v, want the Kustomization kept", err)
						}
						return
					}
					if err == nil || !strings.Contains(err.Error(), `already has Flux Kustomization "`+kept.Name+`"`) || !strings.Contains(err.Error(), c.want) {
						t.Fatalf("got %v, want %q refused as a namesake %s", err, kept.Name, c.want)
					}
				})
			}
		}
	}
}

// A kept Kustomization read from YAML is unstructured, and there a
// spec.kubeConfig of null is no kubeConfig, as it is once decoded into the
// typed object: the Kustomization is kept. One that names a kubeConfig is
// still a namesake.
func TestReconcileContext_KeptUnstructuredKubeConfig(t *testing.T) {
	cases := map[string]struct {
		kubeConfig any
		want       string
	}{
		"null":            {nil, ""},
		"another cluster": {map[string]any{"secretRef": map[string]any{"name": "remote-kubeconfig"}}, "with spec.kubeConfig"},
	}
	rules := withPlacement(nodeOnly, layout.FluxIntegratedPerBundle)
	build := func() *stack.Cluster { return noWaitCluster(srBundle("platform", cmApp("platform-app")), false) }
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			c2 := build()
			ml, err := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).CreateLayoutWithResources(c2, rules)
			if err != nil {
				t.Fatalf("first integration: %v", err)
			}
			kept := findKept(ml, "platform")
			replaceUnstructured(t, ml, kept, func(u *unstructured.Unstructured) {
				if err := unstructured.SetNestedField(u.Object, runtime.DeepCopyJSONValue(c.kubeConfig), "spec", "kubeConfig"); err != nil {
					t.Fatal(err)
				}
			})
			err = fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).IntegrateWithLayout(ml, c2, rules)
			if c.want == "" {
				if err != nil {
					t.Fatalf("got %v, want the Kustomization kept", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), `already has Flux Kustomization "platform"`) || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("got %v, want platform refused as a namesake %s", err, c.want)
			}
		})
	}
}

// replaceUnstructured replaces obj, wherever ml holds it, with its
// unstructured form after edit.
func replaceUnstructured(t *testing.T, ml *layout.ManifestLayout, obj client.Object, edit func(*unstructured.Unstructured)) {
	t.Helper()
	m, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		t.Fatal(err)
	}
	u := &unstructured.Unstructured{Object: m}
	u.SetGroupVersionKind(kustv1.GroupVersion.WithKind(kustv1.KustomizationKind))
	edit(u)
	var walk func(l *layout.ManifestLayout) bool
	walk = func(l *layout.ManifestLayout) bool {
		for i, r := range l.Resources {
			if r == obj {
				l.Resources[i] = u
				return true
			}
		}
		for _, c := range l.Children {
			if walk(c) {
				return true
			}
		}
		return false
	}
	if !walk(ml) {
		t.Fatalf("%s is not in the layout", obj.GetName())
	}
}

// The reconcile-order check of every integration reads a kept
// Kustomization's health checks the same way. platform, kept without wait,
// health-checks a Kustomization while web depends on platform: on web itself,
// platform's readiness waits for web's, which waits for platform's, unless a
// healthCheckExprs predicate decides web, which then needs only to exist
// (the bootstrap creates it). On sub, which web's apply creates, even the
// predicate waits for web to apply, and so for platform.
func TestReconcileContext_PlacedOrderReadsExpressions(t *testing.T) {
	build := func() *stack.Cluster {
		c := noWaitCluster(srBundle("platform", cmApp("platform-app")), true)
		web := c.Node.Children[1]
		sub := &stack.Node{Name: "sub", Bundle: srBundle("sub", cmApp("sub-app"))}
		web.Children = []*stack.Node{sub}
		sub.SetParent(web)
		return c
	}
	cases := map[string]struct {
		checked string
		expr    bool
		refused bool
	}{
		"web":                 {checked: "web", refused: true},
		"web, by a predicate": {checked: "web", expr: true},
		"sub":                 {checked: "sub", refused: true},
		"sub, by a predicate": {checked: "sub", expr: true, refused: true},
	}
	for name, c := range cases {
		for _, placement := range integratedPlacements {
			t.Run(name+"/"+string(placement), func(t *testing.T) {
				rules := withPlacement(nodeOnly, placement)
				cl := build()
				ml, err := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).CreateLayoutWithResources(cl, rules)
				if err != nil {
					t.Fatalf("first integration: %v", err)
				}
				k := findKept(ml, "platform")
				k.Spec.Wait = false
				k.Spec.HealthChecks = []metaapi.NamespacedObjectKindReference{{APIVersion: kustv1.GroupVersion.String(), Kind: "Kustomization", Name: findKept(ml, c.checked).Name}}
				if c.expr {
					k.Spec.HealthCheckExprs = []kustomize.CustomHealthCheck{currentExpr(kustv1.GroupVersion.String(), "Kustomization")}
				}
				err = fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).IntegrateWithLayout(ml, cl, rules)
				if c.refused {
					if err == nil || !strings.Contains(err.Error(), "can never all become Ready") {
						t.Fatalf("got %v, want a reconcile-order refusal (%s)", err, describeKustomizations(ml))
					}
					return
				}
				if err != nil {
					t.Fatalf("IntegrateWithLayout: %v", err)
				}
			})
		}
	}
}
