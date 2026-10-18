package fluxcd_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"slices"
	"strings"
	"testing"

	kustv1 "github.com/fluxcd/kustomize-controller/api/v1"
	kustv1beta2 "github.com/fluxcd/kustomize-controller/api/v1beta2"
	metaapi "github.com/fluxcd/pkg/apis/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-kure/kure/pkg/stack"
	fluxstack "github.com/go-kure/kure/pkg/stack/fluxcd"
	"github.com/go-kure/kure/pkg/stack/layout"
)

// nodeOnly groups bundles and applications flat and keeps one directory per
// node, so that each node's bundle is a Kustomization of its own.
var nodeOnly = layout.LayoutRules{
	BundleGrouping: layout.GroupFlat, ApplicationGrouping: layout.GroupFlat,
	FluxPlacement: layout.FluxIntegratedPerBundle,
}

// noWaitGenerator is a generator with UmbrellasDoNotWait set.
func noWaitGenerator() *fluxstack.ResourceGenerator {
	g := fluxstack.NewResourceGenerator()
	g.UmbrellasDoNotWait = true
	return g
}

// integrateNoWait renders c in placement with UmbrellasDoNotWait set.
func integrateNoWait(c *stack.Cluster, rules layout.LayoutRules, placement layout.FluxPlacement) (*layout.ManifestLayout, error) {
	rules.FluxPlacement = placement
	return fluxstack.NewLayoutIntegrator(noWaitGenerator()).CreateLayoutWithResources(c, rules)
}

// noWaitCluster is demo -> {apps (bundle u), web (bundle web depending on
// u)}: u is the caller's umbrella, web the Kustomization whose dependsOn R3
// rewrites. Without dependsOnU, web is an unrelated leaf.
func noWaitCluster(u *stack.Bundle, dependsOnU bool) *stack.Cluster {
	web := srBundle("web", cmApp("web-app"))
	if dependsOnU {
		web.DependsOn = []*stack.Bundle{u}
	}
	apps := &stack.Node{Name: "apps", Bundle: u}
	webN := &stack.Node{Name: "web", Bundle: web}
	root := &stack.Node{Name: "demo", Children: []*stack.Node{apps, webN}}
	apps.SetParent(root)
	webN.SetParent(root)
	return &stack.Cluster{Name: "demo", Node: root}
}

// umbrellaOf is a bundle named name with children, no applications.
func umbrellaOf(name string, children ...*stack.Bundle) *stack.Bundle {
	return &stack.Bundle{Name: name, SourceRef: testSR(), Children: children}
}

// describeKustomizations lists every Kustomization of ml with its wait, its
// health checks and its dependsOn, for a failure message.
func describeKustomizations(ml *layout.ManifestLayout) string {
	var out []string
	for _, k := range kustomizations(ml) {
		out = append(out, fmt.Sprintf("%s(path=%s wait=%v hc=%d deps=%v)", k.Name, k.Spec.Path, k.Spec.Wait, len(k.Spec.HealthChecks), dependsOnNames(k)))
	}
	return strings.Join(out, ", ")
}

// assertUmbrella checks R1 on the Kustomization named name: no wait, no
// health checks.
func assertUmbrella(t *testing.T, ml *layout.ManifestLayout, name string) {
	t.Helper()
	k := mustKustomization(t, ml, name)
	if k.Spec.Wait || len(k.Spec.HealthChecks) > 0 {
		t.Errorf("umbrella %s: wait=%v healthChecks=%d, want neither; tree: %s", name, k.Spec.Wait, len(k.Spec.HealthChecks), describeKustomizations(ml))
	}
}

// assertLeafWaits checks R2 on the Kustomization named name: it waits.
func assertLeafWaits(t *testing.T, ml *layout.ManifestLayout, name string) {
	t.Helper()
	if k := mustKustomization(t, ml, name); !k.Spec.Wait {
		t.Errorf("leaf %s does not wait; tree: %s", name, describeKustomizations(ml))
	}
}

func assertDependsOn(t *testing.T, ml *layout.ManifestLayout, name string, want ...string) {
	t.Helper()
	if got := dependsOnNames(mustKustomization(t, ml, name)); !slices.Equal(got, want) {
		t.Errorf("%s dependsOn = %v, want %v", name, got, want)
	}
}

// R1 and R2: the umbrella neither waits nor health-checks its children, and
// each child waits, in every integrated placement and in GenerateForBundle.
func TestUmbrellasDoNotWait_TwoChildren(t *testing.T) {
	for _, placement := range integratedPlacements {
		t.Run(string(placement), func(t *testing.T) {
			u := waitingUmbrella(nil)
			u.HealthChecks = nil
			ml, err := integrateNoWait(noWaitCluster(u, false), nodeOnly, placement)
			if err != nil {
				t.Fatalf("CreateLayoutWithResources: %v", err)
			}
			assertUmbrella(t, ml, "platform")
			assertLeafWaits(t, ml, "y-infra")
			assertLeafWaits(t, ml, "y-services")
			assertLeafWaits(t, ml, "web")
		})
	}
	t.Run("GenerateForBundle", func(t *testing.T) {
		u := waitingUmbrella(nil)
		u.HealthChecks = nil
		objs, err := noWaitGenerator().GenerateForBundle(u, "demo/apps")
		if err != nil {
			t.Fatalf("GenerateForBundle: %v", err)
		}
		k := objs[0].(*kustv1.Kustomization)
		if k.Spec.Wait || len(k.Spec.HealthChecks) > 0 {
			t.Errorf("umbrella: wait=%v healthChecks=%v, want neither", k.Spec.Wait, k.Spec.HealthChecks)
		}
		objs, err = noWaitGenerator().GenerateForBundle(u.Children[0], "demo/apps/y-infra")
		if err != nil {
			t.Fatalf("GenerateForBundle child: %v", err)
		}
		if !objs[0].(*kustv1.Kustomization).Spec.Wait {
			t.Error("child does not wait")
		}
	})
}

// A leaf's explicit Wait false is kept: R2 resolves only an unset wait.
func TestUmbrellasDoNotWait_LeafWaitFalseKept(t *testing.T) {
	u := waitingUmbrella(nil)
	u.HealthChecks = nil
	u.Children[0].Wait = boolPtr(false)
	for _, placement := range integratedPlacements {
		ml, err := integrateNoWait(noWaitCluster(u, false), nodeOnly, placement)
		if err != nil {
			t.Fatalf("%s: %v", placement, err)
		}
		if mustKustomization(t, ml, "y-infra").Spec.Wait {
			t.Errorf("%s: y-infra waits although its bundle sets Wait false", placement)
		}
	}
}

// R1 on nested umbrellas: neither the outer nor the inner umbrella waits; the
// leaves do.
func TestUmbrellasDoNotWait_Nested(t *testing.T) {
	for _, placement := range integratedPlacements {
		t.Run(string(placement), func(t *testing.T) {
			u := umbrellaOf("platform",
				umbrellaOf("mid", srBundle("leaf-a", cmApp("leaf-a-app")), srBundle("leaf-b", cmApp("leaf-b-app"))),
				srBundle("leaf-c", cmApp("leaf-c-app")))
			ml, err := integrateNoWait(noWaitCluster(u, true), nodeOnly, placement)
			if err != nil {
				t.Fatalf("CreateLayoutWithResources: %v", err)
			}
			assertUmbrella(t, ml, "platform")
			assertUmbrella(t, ml, "mid")
			for _, leaf := range []string{"leaf-a", "leaf-b", "leaf-c"} {
				assertLeafWaits(t, ml, leaf)
			}
			// R3 expands the nested umbrella.
			assertDependsOn(t, ml, "web", "leaf-a", "leaf-b", "leaf-c")
		})
	}
}

// Per-layout placement: an application whose two sub-layouts are chained by
// dependsOn. The Kustomizations applying the sub-layouts' do not wait, both
// sub-layout Kustomizations do, and the chain is kept.
func TestUmbrellasDoNotWait_PerLayoutSubLayouts(t *testing.T) {
	bundle := &stack.Bundle{Name: "apps", SourceRef: testSR(), Applications: []*stack.Application{
		stack.NewApplication("myapp", "default", hookGroupAugmenter{}),
	}}
	cluster := &stack.Cluster{Name: "prod", Node: &stack.Node{Name: "prod", Bundle: bundle}}
	ml, err := integrateNoWait(cluster, layout.LayoutRules{}, layout.FluxIntegratedPerLayout)
	if err != nil {
		t.Fatalf("CreateLayoutWithResources: %v", err)
	}
	var pre, hooks *kustv1.Kustomization
	for _, k := range kustomizations(ml) {
		switch {
		case strings.HasSuffix(k.Spec.Path, "-00-pre-install"):
			pre = k
		case strings.HasSuffix(k.Spec.Path, "-01-hooks"):
			hooks = k
		}
	}
	if pre == nil || hooks == nil {
		t.Fatalf("no sub-layout Kustomizations; the tree holds %s", describeKustomizations(ml))
	}
	if !pre.Spec.Wait || !hooks.Spec.Wait {
		t.Errorf("sub-layouts: pre wait=%v hooks wait=%v, want both; tree: %s", pre.Spec.Wait, hooks.Spec.Wait, describeKustomizations(ml))
	}
	if got := dependsOnNames(hooks); !slices.Equal(got, []string{pre.Name}) {
		t.Errorf("hooks dependsOn = %v, want [%s]", got, pre.Name)
	}
	umbrellas, err := fluxstack.UmbrellaTerminals(ml)
	if err != nil {
		t.Fatalf("UmbrellaTerminals: %v", err)
	}
	if len(umbrellas) == 0 {
		t.Fatalf("no umbrella in the tree: %s", describeKustomizations(ml))
	}
	for key, terms := range umbrellas {
		_, name, _ := strings.Cut(key, "/")
		assertUmbrella(t, ml, name)
		if !slices.Contains(terms, "flux-system/"+hooks.Name) || slices.Contains(terms, "flux-system/"+pre.Name) {
			t.Errorf("terminals of %s = %v, want the hooks layout's and not pre-install's", key, terms)
		}
	}
}

// fluxKustomizationApp is an application whose objects hold one Flux
// Kustomization named name, as an author writes it by hand, depending on the
// Kustomizations named dependsOn.
func fluxKustomizationApp(app, name string, dependsOn ...string) *stack.Application {
	var o client.Object = handWritten(name, dependsOn...)
	return stack.NewApplication(app, "default", &fakeAppConfig{objs: []*client.Object{&o}})
}

// handWritten is the Flux Kustomization fluxKustomizationApp emits.
func handWritten(name string, dependsOn ...string) *kustv1.Kustomization {
	k := &kustv1.Kustomization{
		TypeMeta:   metav1.TypeMeta{APIVersion: kustv1.GroupVersion.String(), Kind: "Kustomization"},
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "flux-system"},
		Spec: kustv1.KustomizationSpec{
			Path:      "./elsewhere",
			SourceRef: kustv1.CrossNamespaceSourceReference{Kind: "GitRepository", Name: "flux-system"},
		},
	}
	for _, d := range dependsOn {
		k.Spec.DependsOn = append(k.Spec.DependsOn, kustv1.DependencyReference{Name: d})
	}
	return k
}

// listApp is an application whose one object is a List holding the Flux
// Kustomization named name, with spec.path specPath.
func listApp(app, name, specPath string) *stack.Application {
	k := handWritten(name)
	k.Spec.Path = specPath
	raw, err := json.Marshal(k)
	if err != nil {
		panic(err)
	}
	list := &metav1.List{TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "List"}, Items: []runtime.RawExtension{{Raw: raw}}}
	u := &unstructured.Unstructured{}
	obj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(list)
	if err != nil {
		panic(err)
	}
	u.Object = obj
	var o client.Object = u
	return stack.NewApplication(app, "default", &fakeAppConfig{objs: []*client.Object{&o}})
}

// R3: a dependsOn on an umbrella is written as one on its terminal leaves.
func TestUmbrellasDoNotWait_DependsOnTerminals(t *testing.T) {
	cases := map[string]struct {
		umbrella func() *stack.Bundle
		want     []string
	}{
		"ordered children keep the last": {
			umbrella: func() *stack.Bundle {
				c0 := srBundle("c0", cmApp("c0-app"))
				c1 := srBundle("c1", cmApp("c1-app"))
				c1.DependsOn = []*stack.Bundle{c0}
				return umbrellaOf("platform", c0, c1)
			},
			want: []string{"c1"},
		},
		"unordered children keep both": {
			umbrella: func() *stack.Bundle {
				return umbrellaOf("platform", srBundle("a", cmApp("a-app")), srBundle("b", cmApp("b-app")))
			},
			want: []string{"a", "b"},
		},
		"nested umbrella expands": {
			umbrella: func() *stack.Bundle {
				x := srBundle("x", cmApp("x-app"))
				y := srBundle("y", cmApp("y-app"))
				y.DependsOn = []*stack.Bundle{x}
				return umbrellaOf("platform", umbrellaOf("mid", x, y), srBundle("z", cmApp("z-app")))
			},
			want: []string{"y", "z"},
		},
		"a Flux Kustomization among the objects is a terminal by its own name": {
			umbrella: func() *stack.Bundle {
				return srBundle("platform", fluxKustomizationApp("platform-app", "hand-written"))
			},
			want: []string{"hand-written"},
		},
	}
	for name, tc := range cases {
		for _, placement := range integratedPlacements {
			t.Run(name+"/"+string(placement), func(t *testing.T) {
				ml, err := integrateNoWait(noWaitCluster(tc.umbrella(), true), nodeOnly, placement)
				if err != nil {
					t.Fatalf("CreateLayoutWithResources: %v", err)
				}
				assertDependsOn(t, ml, "web", tc.want...)
				assertUmbrella(t, ml, "platform")
			})
		}
	}
	t.Run("a dependency on a leaf is untouched", func(t *testing.T) {
		for _, placement := range integratedPlacements {
			ml, err := integrateNoWait(noWaitCluster(srBundle("platform", cmApp("platform-app")), true), nodeOnly, placement)
			if err != nil {
				t.Fatalf("%s: %v", placement, err)
			}
			assertDependsOn(t, ml, "web", "platform")
			assertLeafWaits(t, ml, "platform")
		}
	})
}

// R1 and R3 refusals: each wraps its sentinel and names the Kustomization.
func TestUmbrellasDoNotWait_Refusals(t *testing.T) {
	type refusal struct {
		run  func() error
		is   error
		want string
	}
	cases := map[string]refusal{}
	for _, placement := range integratedPlacements {
		p := placement
		cases["Wait true on an umbrella bundle/"+string(p)] = refusal{
			run: func() error {
				u := waitingUmbrella(boolPtr(true))
				u.HealthChecks = nil
				_, err := integrateNoWait(noWaitCluster(u, false), nodeOnly, p)
				return err
			},
			is: fluxstack.ErrUmbrellaWait, want: `bundle "platform" sets Wait true`,
		}
		cases["HealthChecks on an umbrella bundle/"+string(p)] = refusal{
			run: func() error {
				_, err := integrateNoWait(noWaitCluster(waitingUmbrella(nil), false), nodeOnly, p)
				return err
			},
			is: fluxstack.ErrUmbrellaWait, want: `bundle "platform" sets HealthChecks`,
		}
		cases["a dependent inside its target's subtree/"+string(p)] = refusal{
			run: func() error {
				c := waitCycleBelowRoot()
				c.Node.Children[0].Bundle.Wait = nil
				_, err := integrateNoWait(c, propertyGroupings["nodeOnly"], p)
				return err
			},
			is: fluxstack.ErrUmbrellaDependency, want: `"b"`,
		}
		// web depends on platform, whose terminal leaf hand-written depends on
		// web: no cycle before the rewrite, web -> hand-written -> web after.
		// The Kustomizations an application emits are read the same in every
		// form it can hand one over in.
		for _, form := range kustomizationForms {
			cases["a rewrite closing a cycle through an application's Kustomization/"+form+"/"+string(p)] = refusal{
				run: func() error {
					app := formApp("platform-app", handWritten("hand-written", "web"), form)
					_, err := integrateNoWait(noWaitCluster(srBundle("platform", app), true), nodeOnly, p)
					return err
				},
				is: fluxstack.ErrUmbrellaDependency, want: `apply flux-system/hand-written -> ready flux-system/web`,
			}
		}
		// a depends on b, which a creates: refused without the setting. b is an
		// umbrella over the hand-written its application emits, so with the
		// setting the dependsOn becomes one on hand-written, which exists only
		// once b, and so a, has applied.
		cases["a rewrite onto a leaf the dependent creates/"+string(p)] = refusal{
			run: func() error {
				rules := propertyGroupings["nodeOnly"]
				if _, err := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).CreateLayoutWithResources(createdLeafCluster(), withPlacement(rules, p)); err == nil || !strings.Contains(err.Error(), "never") {
					return fmt.Errorf("without the setting: got %w, want a reconcile-order refusal", err)
				}
				_, err := integrateNoWait(createdLeafCluster(), rules, p)
				return err
			},
			is: fluxstack.ErrUmbrellaDependency, want: `apply flux-system/hand-written -> apply flux-system/b -> apply flux-system/a`,
		}
		for _, form := range kustomizationForms {
			// web depends on platform, whose terminal leaf hand-written
			// health-checks web, without wait: it becomes Ready only once web is.
			cases["a rewrite onto an application's Kustomization health-checking the dependent/"+form+"/"+string(p)] = refusal{
				run: func() error {
					_, err := integrateNoWait(noWaitCluster(srBundle("platform", checkingApp("web", form)), true), nodeOnly, p)
					return err
				},
				is: fluxstack.ErrUmbrellaDependency, want: `ready flux-system/hand-written -> ready flux-system/web`,
			}
			// hand-written's dependsOn on umbrella mid is its author's, not
			// rewritten: it would be satisfied before x is Ready.
			cases["an application's Kustomization depending on an umbrella/"+form+"/"+string(p)] = refusal{
				run: func() error {
					_, err := integrateNoWait(noWaitCluster(appCoveredUmbrella(form), true), nodeOnly, p)
					return err
				},
				is: fluxstack.ErrUmbrellaDependency, want: `depends on or health-checks "flux-system/mid"`,
			}
		}
		// web depends on platform, whose terminal leaf a, without wait,
		// health-checks web: a becomes Ready only once web is.
		cases["a rewrite onto a generated leaf health-checking the dependent/"+string(p)] = refusal{
			run: func() error {
				a := srBundle("a", cmApp("a-app"))
				a.Wait = boolPtr(false)
				a.HealthChecks = []stack.HealthCheck{{APIVersion: kustv1.GroupVersion.String(), Kind: "Kustomization", Name: "web"}}
				_, err := integrateNoWait(noWaitCluster(umbrellaOf("platform", a), true), nodeOnly, p)
				return err
			},
			is: fluxstack.ErrUmbrellaDependency, want: `ready flux-system/a -> ready flux-system/web`,
		}
		// A health check is not rewritten: one on umbrella platform would pass
		// once platform applied, before a is Ready.
		cases["a generated leaf health-checking an umbrella/"+string(p)] = refusal{
			run: func() error {
				c := noWaitCluster(umbrellaOf("platform", srBundle("a", cmApp("a-app"))), false)
				web := c.Node.Children[1].Bundle
				web.Wait = boolPtr(false)
				web.HealthChecks = []stack.HealthCheck{{APIVersion: kustv1.GroupVersion.String(), Kind: "Kustomization", Name: "platform"}}
				_, err := integrateNoWait(c, nodeOnly, p)
				return err
			},
			is: fluxstack.ErrUmbrellaDependency, want: `health-checks "flux-system/platform"`,
		}
		cases["a kept leaf health-checking an umbrella/"+string(p)] = refusal{
			run: func() error {
				return integrateAgain(noWaitCluster(umbrellaOf("platform", srBundle("a", cmApp("a-app"))), false), p, noWaitGenerator(), func(ml *layout.ManifestLayout) {
					findKept(ml, "web").Spec.HealthChecks = []metaapi.NamespacedObjectKindReference{{APIVersion: kustv1.GroupVersion.String(), Kind: "Kustomization", Name: "platform"}}
				})
			},
			is: fluxstack.ErrUmbrellaDependency, want: `health-checks "flux-system/platform"`,
		}
		for _, form := range kustomizationForms {
			cases["an application's Kustomization health-checking an umbrella/"+form+"/"+string(p)] = refusal{
				run: func() error {
					u := umbrellaOf("platform", srBundle("a", cmApp("a-app")), srBundle("b", checkingApp("platform", form)))
					_, err := integrateNoWait(noWaitCluster(u, false), nodeOnly, p)
					return err
				},
				is: fluxstack.ErrUmbrellaDependency, want: `depends on or health-checks "flux-system/platform"`,
			}
		}
		cases["a kept umbrella with a health check on another kind/"+string(p)] = refusal{
			run: func() error {
				return integrateAgain(noWaitCluster(umbrellaOf("platform", srBundle("a", cmApp("a-app"))), false), p, noWaitGenerator(), func(ml *layout.ManifestLayout) {
					findKept(ml, "platform").Spec.HealthChecks = []metaapi.NamespacedObjectKindReference{{APIVersion: "apps/v1", Kind: "Deployment", Name: "a", Namespace: "default"}}
				})
			},
			is: fluxstack.ErrUmbrellaWait, want: "sets healthChecks",
		}
		cases["a kept Kustomization depending on an umbrella/"+string(p)] = refusal{
			run: func() error {
				return integrateAgain(noWaitCluster(umbrellaOf("platform", srBundle("a", cmApp("a-app"))), true), p, fluxstack.NewResourceGenerator(), func(ml *layout.ManifestLayout) {
					// Only web's dependsOn is left to refuse.
					k := findKept(ml, "platform")
					k.Spec.Wait, k.Spec.HealthChecks = false, nil
				})
			},
			is: fluxstack.ErrUmbrellaDependency, want: `its dependsOn names "platform"`,
		}
	}
	cases["Wait true on an umbrella layout"] = refusal{
		run: func() error {
			bundle := &stack.Bundle{Name: "apps", SourceRef: testSR(), Applications: []*stack.Application{
				stack.NewApplication("myapp", "default", waitingHookGroups{}),
			}}
			cluster := &stack.Cluster{Name: "prod", Node: &stack.Node{Name: "prod", Bundle: bundle}}
			_, err := integrateNoWait(cluster, layout.LayoutRules{}, layout.FluxIntegratedPerLayout)
			return err
		},
		is: fluxstack.ErrUmbrellaWait, want: `sets Wait true`,
	}
	cases["a layout depending on the application layout above it"] = refusal{
		run: func() error {
			c := &stack.Cluster{Name: "demo", Node: &stack.Node{Name: "platform", Bundle: srBundle("platform",
				stack.NewApplication("chart", "default", aboveDependent{app: "chart"}))}}
			rules := propertyGroupings["GroupByName"]
			if _, err := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).CreateLayoutWithResources(c, withPlacement(rules, layout.FluxIntegratedPerLayout)); err != nil {
				return fmt.Errorf("refused without the setting: %w", err)
			}
			c = &stack.Cluster{Name: "demo", Node: &stack.Node{Name: "platform", Bundle: srBundle("platform",
				stack.NewApplication("chart", "default", aboveDependent{app: "chart"}))}}
			_, err := integrateNoWait(c, rules, layout.FluxIntegratedPerLayout)
			return err
		},
		is: fluxstack.ErrUmbrellaDependency, want: "chart-hooks",
	}
	cases["Wait true on an umbrella bundle/GenerateForBundle"] = refusal{
		run: func() error {
			u := waitingUmbrella(boolPtr(true))
			u.HealthChecks = nil
			_, err := noWaitGenerator().GenerateForBundle(u, "demo/apps")
			return err
		},
		is: fluxstack.ErrUmbrellaWait, want: `Flux Kustomization "platform"`,
	}
	cases["FluxSeparate"] = refusal{
		run: func() error {
			_, err := integrateNoWait(noWaitCluster(srBundle("platform", cmApp("platform-app")), false), nodeOnly, layout.FluxSeparate)
			return err
		},
		is: fluxstack.ErrUmbrellaWait, want: "do not support UmbrellasDoNotWait",
	}
	cases["GenerateFromCluster"] = refusal{
		run: func() error {
			_, err := noWaitGenerator().GenerateFromCluster(noWaitCluster(srBundle("platform", cmApp("platform-app")), false), nodeOnly)
			return err
		},
		is: fluxstack.ErrUmbrellaWait, want: "do not support UmbrellasDoNotWait",
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := tc.run()
			if !errors.Is(err, tc.is) || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("got %v, want an error wrapping %v that contains %s", err, tc.is, tc.want)
			}
		})
	}
}

// checkingApp is an application emitting the Flux Kustomization hand-written,
// in form, without wait and with a Kustomization health check on checked.
func checkingApp(checked, form string) *stack.Application {
	k := handWritten("hand-written")
	k.Spec.HealthChecks = []metaapi.NamespacedObjectKindReference{{APIVersion: kustv1.GroupVersion.String(), Kind: "Kustomization", Name: checked}}
	return formApp("platform-app", k, form)
}

// appCoveredUmbrella is platform -> {c -> mid -> x, and the hand-written its
// application emits in form, depending on mid}.
func appCoveredUmbrella(form string) *stack.Bundle {
	u := umbrellaOf("platform", umbrellaOf("c", umbrellaOf("mid", srBundle("x", cmApp("x-app")))))
	u.Applications = []*stack.Application{formApp("platform-app", handWritten("hand-written", "mid"), form)}
	return u
}

// kustomizationForms are the forms in which an application can hand over a
// Flux Kustomization: the typed v1 object, its own type embedding one, the
// typed v1beta2 object and an unstructured one. Each is read as the same
// Kustomization.
var kustomizationForms = []string{"v1", "embedding", "v1beta2", "unstructured"}

// embeddedKustomization is an application's own type embedding a Flux
// Kustomization: it serializes as the one it embeds.
type embeddedKustomization struct {
	kustv1.Kustomization
}

// formApp is an application whose one object is k in form, one of
// kustomizationForms.
func formApp(app string, k *kustv1.Kustomization, form string) *stack.Application {
	var o client.Object
	switch form {
	case "v1":
		o = k
	case "embedding":
		o = &embeddedKustomization{Kustomization: *k}
	case "v1beta2", "unstructured":
		raw, err := json.Marshal(k)
		if err != nil {
			panic(err)
		}
		if form == "v1beta2" {
			b := &kustv1beta2.Kustomization{}
			if err := json.Unmarshal(raw, b); err != nil {
				panic(err)
			}
			b.APIVersion = kustv1beta2.GroupVersion.String()
			o = b
		} else {
			u := &unstructured.Unstructured{}
			if err := json.Unmarshal(raw, &u.Object); err != nil {
				panic(err)
			}
			o = u
		}
	default:
		panic("unknown form " + form)
	}
	return stack.NewApplication(app, "default", &fakeAppConfig{objs: []*client.Object{&o}})
}

// An application's Kustomization with wait, on another repository, whose
// spec.path names the directory that holds web's Kustomization here: its
// build is that repository's, so the rewrite onto it closes no cycle and is
// accepted (go-kure/kure#1062).
func TestUmbrellasDoNotWait_ForeignWaitIsNoCycle(t *testing.T) {
	for _, placement := range integratedPlacements {
		t.Run(string(placement), func(t *testing.T) {
			off, err := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).CreateLayoutWithResources(noWaitCluster(srBundle("platform", cmApp("platform-app")), true), withPlacement(nodeOnly, placement))
			if err != nil {
				t.Fatalf("without the setting: %v", err)
			}
			k := handWritten("hand-written")
			k.Spec.Wait, k.Spec.Path = true, path.Dir(path.Clean(findKept(off, "web").Spec.Path))
			k.Spec.SourceRef.Name = "unrelated-repository"
			var o client.Object = k
			app := stack.NewApplication("platform-app", "default", &fakeAppConfig{objs: []*client.Object{&o}})
			ml, err := integrateNoWait(noWaitCluster(srBundle("platform", app), true), nodeOnly, placement)
			if err != nil {
				t.Fatalf("CreateLayoutWithResources: %v", err)
			}
			assertDependsOn(t, ml, "web", "hand-written")
		})
	}
}

// A dependsOn of an application's Kustomization is not rewritten, so one on
// an umbrella awaits none of its leaves: hand-written, depending on mid, does
// not stand for c, and platform's terminal leaves are x and hand-written. One
// on a sibling leaf is read: with platform -> {a, hand-written depending on
// a}, a is no terminal leaf. Both hold in every form an application can hand
// a Kustomization over in.
func TestUmbrellaTerminals_ApplicationDependenciesAwaitNoLeaf(t *testing.T) {
	siblingDependent := func(form string) *stack.Bundle {
		u := umbrellaOf("platform", srBundle("a", cmApp("a-app")))
		u.Applications = []*stack.Application{formApp("platform-app", handWritten("hand-written", "a"), form)}
		return u
	}
	for _, form := range kustomizationForms {
		for _, placement := range integratedPlacements {
			t.Run(form+"/"+string(placement), func(t *testing.T) {
				for _, tc := range []struct {
					bundle *stack.Bundle
					want   []string
				}{
					{appCoveredUmbrella(form), []string{"flux-system/x", "flux-system/hand-written"}},
					{siblingDependent(form), []string{"flux-system/hand-written"}},
				} {
					ml, err := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).CreateLayoutWithResources(noWaitCluster(tc.bundle, true), withPlacement(nodeOnly, placement))
					if err != nil {
						t.Fatalf("CreateLayoutWithResources: %v", err)
					}
					terms, err := fluxstack.UmbrellaTerminals(ml)
					if err != nil {
						t.Fatalf("UmbrellaTerminals: %v", err)
					}
					if got := terms["flux-system/platform"]; !slices.Equal(got, tc.want) {
						t.Errorf("terminals of platform = %v, want %v", got, tc.want)
					}
				}
			})
		}
	}
}

// An application's Flux Kustomization that cannot be read in typed form (a
// spec.dependsOn that is no list) integrates as before without the setting,
// and is refused with it, whose checks read its spec.
func TestUmbrellasDoNotWait_UnreadableApplicationKustomization(t *testing.T) {
	unreadable := func() *stack.Bundle {
		u := &unstructured.Unstructured{Object: map[string]any{
			"apiVersion": kustv1.GroupVersion.String(),
			"kind":       "Kustomization",
			"metadata":   map[string]any{"name": "hand-written", "namespace": "flux-system"},
			"spec":       map[string]any{"path": "./elsewhere", "dependsOn": "web"},
		}}
		var o client.Object = u
		return srBundle("platform", stack.NewApplication("platform-app", "default", &fakeAppConfig{objs: []*client.Object{&o}}))
	}
	for _, placement := range integratedPlacements {
		t.Run(string(placement), func(t *testing.T) {
			if _, err := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).CreateLayoutWithResources(noWaitCluster(unreadable(), true), withPlacement(nodeOnly, placement)); err != nil {
				t.Fatalf("without the setting: %v", err)
			}
			if _, err := integrateNoWait(noWaitCluster(unreadable(), true), nodeOnly, placement); err == nil || !strings.Contains(err.Error(), `read Flux Kustomization "flux-system/hand-written" in typed form`) {
				t.Fatalf("with the setting: got %v, want the typed-form read error", err)
			}
		})
	}
}

// createdLeafCluster is r -> a -> b: a depends on b, whose application emits
// the Flux Kustomization hand-written, and a's apply creates b's CR.
func createdLeafCluster() *stack.Cluster {
	b := &stack.Node{Name: "b", Bundle: srBundle("b", fluxKustomizationApp("b-app", "hand-written"))}
	a := &stack.Node{Name: "a", Bundle: srBundle("a", cmApp("a-app")), Children: []*stack.Node{b}}
	a.Bundle.DependsOn = []*stack.Bundle{b.Bundle}
	r := &stack.Node{Name: "r", Children: []*stack.Node{a}}
	b.SetParent(a)
	a.SetParent(r)
	return &stack.Cluster{Name: "demo", Node: r}
}

func withPlacement(rules layout.LayoutRules, placement layout.FluxPlacement) layout.LayoutRules {
	rules.FluxPlacement = placement
	return rules
}

// aboveDependent gives its application layout a hooks child that depends on
// the application layout above it, which applies the hooks' Kustomization.
type aboveDependent struct{ app string }

func (a aboveDependent) Generate(*stack.Application) ([]*client.Object, error) {
	return []*client.Object{cmObj(a.app + "-cm")}, nil
}

func (a aboveDependent) AugmentLayout(ml *layout.ManifestLayout) error {
	ml.Children = append(ml.Children, &layout.ManifestLayout{
		Name:          a.app + "-hooks",
		Namespace:     ml.FullRepoPath(),
		Resources:     []client.Object{*cmObj(a.app + "-hook")},
		FluxPlacement: ml.FluxPlacement,
		DependsOn:     []string{ml.Name},
	})
	return nil
}

// waitingHookGroups is hookGroupAugmenter with Wait true on the application's
// own layout, which applies its two sub-layouts' Kustomizations.
type waitingHookGroups struct{ hookGroupAugmenter }

func (a waitingHookGroups) AugmentLayout(ml *layout.ManifestLayout) error {
	ml.Wait = boolPtr(true)
	return a.hookGroupAugmenter.AugmentLayout(ml)
}

// No tree whose waits the reconcile-order check refuses without the setting
// compiles with it, so there is no "refused before, compiles now" case to
// pin. Such a cycle runs through a Kustomization U that waits for one it
// applies, C, and back to U through a dependsOn on U. Under the setting U is
// an umbrella: an explicit wait or health check on it is refused (R1), and an
// unset one leaves the auto health checks, whose dependsOn on U is rewritten
// onto U's terminal leaves (R3). A terminal leaf is C or depends on C through
// its siblings, so the rewritten edges close the same cycle; and where the
// dependent sits inside U's subtree, R3 refuses it. The guard instead is the
// property test's assertion that every tree compiling under the setting has
// acyclic dependsOn edges.
//
// This test pins the R1 side: waitCycleBelowRoot, a with Wait true waiting
// for b, which depends on a, is refused for a's wait, not for the order.
// Its R3 side, with a's wait unset, is the "dependent inside its target's
// subtree" refusal.
func TestUmbrellasDoNotWait_ReconcileOrder(t *testing.T) {
	for _, placement := range integratedPlacements {
		_, err := integrateNoWait(waitCycleBelowRoot(), propertyGroupings["nodeOnly"], placement)
		if !errors.Is(err, fluxstack.ErrUmbrellaWait) {
			t.Errorf("%s: got %v, want the R1 refusal of a's Wait true", placement, err)
		}
	}
}

// expected is what the property test derives on its own from a tree: for each
// Kustomization (namespace/name), whether it applies another one. A
// Kustomization's CR is applied by the Kustomization whose spec.path is the
// longest one at or above the directory the CR sits in.
func expectedUmbrellas(ml *layout.ManifestLayout) map[string]bool {
	type placed struct {
		k   *kustv1.Kustomization
		dir string
	}
	var all []placed
	var walk func(l *layout.ManifestLayout)
	walk = func(l *layout.ManifestLayout) {
		for _, r := range l.Resources {
			if k, ok := r.(*kustv1.Kustomization); ok {
				all = append(all, placed{k, path.Clean(l.FullRepoPath())})
			}
		}
		for _, c := range l.Children {
			walk(c)
		}
	}
	walk(ml)
	under := func(dir, p string) bool { return dir == p || strings.HasPrefix(dir, p+"/") }
	out := map[string]bool{}
	for _, cr := range all {
		var applier *kustv1.Kustomization
		best := -1
		for _, k := range all {
			p := path.Clean(strings.TrimPrefix(k.k.Spec.Path, "./"))
			if k.k != cr.k && under(cr.dir, p) && len(p) > best {
				applier, best = k.k, len(p)
			}
		}
		if applier != nil {
			out[applier.Namespace+"/"+applier.Name] = true
		}
	}
	return out
}

// dependsOnShapes add to the package's property shapes trees with a
// dependsOn on an umbrella, so that R3 runs on every grouping too.
var dependsOnShapes = map[string]func() *stack.Cluster{
	"depends on an umbrella": func() *stack.Cluster {
		c0 := srBundle("c0", cmApp("c0-app"))
		c1 := srBundle("c1", cmApp("c1-app"))
		c1.DependsOn = []*stack.Bundle{c0}
		return noWaitCluster(umbrellaOf("platform", c0, c1), true)
	},
	"depends on a nested umbrella": func() *stack.Cluster {
		return noWaitCluster(umbrellaOf("platform",
			umbrellaOf("mid", srBundle("x", cmApp("x-app")), srBundle("y", cmApp("y-app"))),
			srBundle("z", cmApp("z-app"))), true)
	},
	"augmenter in an umbrella": func() *stack.Cluster {
		u := umbrellaOf("platform", &stack.Bundle{Name: "charts", SourceRef: testSR(), Applications: []*stack.Application{
			stack.NewApplication("chart", "default", &hookAugmenter{app: "chart"}),
		}})
		return noWaitCluster(u, true)
	},
}

// TestUmbrellasDoNotWait_Property is the regression guard: on every tree the
// package's property tests build, in every grouping and integrated
// placement, with the setting no Kustomization that applies a Flux
// Kustomization waits or health-checks one, every other one waits, no
// dependsOn names an umbrella, the dependsOn edges are acyclic, and the
// Kustomizations are the ones written without the setting.
func TestUmbrellasDoNotWait_Property(t *testing.T) {
	shapes := map[string]func() *stack.Cluster{}
	for name, shape := range propertyShapes {
		shapes[name] = shape
	}
	for name, shape := range dependsOnShapes {
		shapes[name] = shape
	}
	umbrellas := 0
	for shapeName, shape := range shapes {
		for groupingName, rules := range allGroupings() {
			for _, placement := range integratedPlacements {
				t.Run(shapeName+"/"+groupingName+"/"+string(placement), func(t *testing.T) {
					rules.FluxPlacement = placement
					off, errOff := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).CreateLayoutWithResources(shape(), rules)
					on, errOn := integrateNoWait(shape(), rules, placement)
					if errOff != nil {
						t.Skipf("refused without the setting: %v", errOff)
					}
					if errOn != nil {
						t.Fatalf("refused with the setting only: %v", errOn)
					}
					umbrellas += checkUmbrellaProperty(t, off, on)
				})
			}
		}
	}
	// The sweep is not vacuous: its trees hold umbrellas.
	if umbrellas == 0 {
		t.Error("no tree of the sweep holds an umbrella")
	}
}

// checkUmbrellaProperty checks the property on one tree and returns how many
// umbrellas it holds.
func checkUmbrellaProperty(t *testing.T, off, on *layout.ManifestLayout) int {
	t.Helper()
	umbrellas := expectedUmbrellas(on)
	terms, err := fluxstack.UmbrellaTerminals(on)
	if err != nil {
		t.Fatalf("UmbrellaTerminals: %v", err)
	}
	for key := range umbrellas {
		if _, ok := terms[key]; !ok {
			t.Errorf("UmbrellaTerminals misses umbrella %s: %v", key, terms)
		}
	}
	for key := range terms {
		if !umbrellas[key] {
			t.Errorf("UmbrellaTerminals names %s, which applies no Flux Kustomization", key)
		}
	}

	byKey := map[string]*kustv1.Kustomization{}
	var shape []string
	for _, k := range kustomizations(on) {
		key := k.Namespace + "/" + k.Name
		byKey[key] = k
		shape = append(shape, key+"@"+k.Spec.Path)
		if umbrellas[key] {
			if k.Spec.Wait || len(k.Spec.HealthChecks) > 0 {
				t.Errorf("umbrella %s: wait=%v healthChecks=%d", key, k.Spec.Wait, len(k.Spec.HealthChecks))
			}
		} else if !k.Spec.Wait {
			t.Errorf("leaf %s does not wait", key)
		}
		for _, d := range k.Spec.DependsOn {
			ns := d.Namespace
			if ns == "" {
				ns = k.Namespace
			}
			if umbrellas[ns+"/"+d.Name] {
				t.Errorf("%s depends on umbrella %s/%s", key, ns, d.Name)
			}
		}
	}
	var offShape []string
	for _, k := range kustomizations(off) {
		offShape = append(offShape, k.Namespace+"/"+k.Name+"@"+k.Spec.Path)
	}
	slices.Sort(shape)
	slices.Sort(offShape)
	if !slices.Equal(shape, offShape) {
		t.Errorf("Kustomizations with the setting %v, without %v", shape, offShape)
	}

	// The dependsOn edges are acyclic.
	state := map[string]int{}
	var visit func(key string) bool
	visit = func(key string) bool {
		switch state[key] {
		case 1:
			return false
		case 2:
			return true
		}
		state[key] = 1
		if k := byKey[key]; k != nil {
			for _, d := range k.Spec.DependsOn {
				ns := d.Namespace
				if ns == "" {
					ns = k.Namespace
				}
				if !visit(ns + "/" + d.Name) {
					return false
				}
			}
		}
		state[key] = 2
		return true
	}
	for key := range byKey {
		if !visit(key) {
			t.Errorf("dependsOn cycle through %s", key)
			break
		}
	}
	return len(umbrellas)
}

// UmbrellaTerminals reads several trees at once, keyed namespace/name, and
// refuses a Kustomization two trees both hold.
func TestUmbrellaTerminals(t *testing.T) {
	// render is a tree of its own per umbrella: one node holding u, named
	// after it, so that two trees share a Kustomization only where asked.
	render := func(u *stack.Bundle) *layout.ManifestLayout {
		c := &stack.Cluster{Name: u.Name, Node: &stack.Node{Name: u.Name, Bundle: u}}
		ml, err := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).CreateLayoutWithResources(c, nodeOnly)
		if err != nil {
			t.Fatalf("CreateLayoutWithResources: %v", err)
		}
		return ml
	}
	first := render(umbrellaOf("platform", srBundle("a", cmApp("a-app")), srBundle("b", cmApp("b-app"))))
	second := render(umbrellaOf("other", srBundle("c", cmApp("c-app"))))
	got, err := fluxstack.UmbrellaTerminals(first, nil, second)
	if err != nil {
		t.Fatalf("UmbrellaTerminals: %v", err)
	}
	want := map[string][]string{
		"flux-system/platform": {"flux-system/a", "flux-system/b"},
		"flux-system/other":    {"flux-system/c"},
	}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("UmbrellaTerminals = %v, want %v", got, want)
	}

	_, err = fluxstack.UmbrellaTerminals(first, render(umbrellaOf("platform", srBundle("d", cmApp("d-app")))))
	if !errors.Is(err, fluxstack.ErrUmbrellaTerminals) || !strings.Contains(err.Error(), "flux-system/platform") {
		t.Errorf("duplicate Kustomization: got %v, want ErrUmbrellaTerminals naming flux-system/platform", err)
	}
}

// integrateAgain integrates c in placement with first, applies edit to the
// tree, and integrates the tree again with the setting: every Kustomization
// of the first integration is then kept from the tree.
func integrateAgain(c *stack.Cluster, placement layout.FluxPlacement, first *fluxstack.ResourceGenerator, edit func(*layout.ManifestLayout)) error {
	rules := withPlacement(nodeOnly, placement)
	ml, err := fluxstack.NewLayoutIntegrator(first).CreateLayoutWithResources(c, rules)
	if err != nil {
		return fmt.Errorf("first integration: %w", err)
	}
	edit(ml)
	return fluxstack.NewLayoutIntegrator(noWaitGenerator()).IntegrateWithLayout(ml, c, rules)
}

// findKept returns the Kustomization named name in ml, which the test edits
// in place; it panics when there is none, so that a case cannot pass vacuously.
func findKept(ml *layout.ManifestLayout, name string) *kustv1.Kustomization {
	for _, k := range kustomizations(ml) {
		if k.Name == name {
			return k
		}
	}
	panic(fmt.Sprintf("no Kustomization %q in %s", name, describeKustomizations(ml)))
}

// singleFile walks c and writes the application layout named app as one file
// into its parent's directory (AppFileSingle).
func singleFile(t *testing.T, c *stack.Cluster, rules layout.LayoutRules, app string) *layout.ManifestLayout {
	t.Helper()
	ml, err := layout.WalkCluster(c, rules)
	if err != nil {
		t.Fatalf("WalkCluster: %v", err)
	}
	var found *layout.ManifestLayout
	var walk func(l *layout.ManifestLayout)
	walk = func(l *layout.ManifestLayout) {
		if l.Name == app {
			found = l
		}
		for _, child := range l.Children {
			if child != nil {
				walk(child)
			}
		}
	}
	walk(ml)
	if found == nil {
		t.Fatalf("no layout named %q", app)
	}
	found.ApplicationFileMode = layout.AppFileSingle
	return ml
}

// An application written as one file into its bundle's directory is in that
// directory's build: the Flux Kustomization it emits makes the bundle's
// Kustomization an umbrella (R1), and is its terminal leaf (R3).
func TestUmbrellasDoNotWait_SingleFileApplication(t *testing.T) {
	for _, placement := range integratedPlacements {
		t.Run(string(placement), func(t *testing.T) {
			c := noWaitCluster(srBundle("platform", fluxKustomizationApp("platform-app", "hand-written")), true)
			rules := withPlacement(nodeOnly, placement)
			rules.ApplicationGrouping = layout.GroupByName
			ml := singleFile(t, c, rules, "platform-app")
			if err := fluxstack.NewLayoutIntegrator(noWaitGenerator()).IntegrateWithLayout(ml, c, rules); err != nil {
				t.Fatalf("IntegrateWithLayout: %v", err)
			}
			assertUmbrella(t, ml, "platform")
			assertDependsOn(t, ml, "web", "hand-written")
			terms, err := fluxstack.UmbrellaTerminals(ml)
			if err != nil {
				t.Fatalf("UmbrellaTerminals: %v", err)
			}
			if got := terms["flux-system/platform"]; !slices.Equal(got, []string{"flux-system/hand-written"}) {
				t.Errorf("terminals of platform = %v, want [flux-system/hand-written]", got)
			}
		})
	}
}

// A Flux Kustomization an application emits inside a List is the
// application's, so UmbrellaTerminals reads it as a leaf: its spec.path,
// umbrella web's, whose build applies leaf, does not make it an umbrella, and
// it is platform's terminal.
func TestUmbrellaTerminals_ListItemsAreLeaves(t *testing.T) {
	cluster := func(platform *stack.Bundle) *stack.Cluster {
		apps := &stack.Node{Name: "apps", Bundle: platform}
		web := &stack.Node{Name: "web", Bundle: umbrellaOf("web", srBundle("leaf", cmApp("leaf-app")))}
		root := &stack.Node{Name: "demo", Children: []*stack.Node{apps, web}}
		apps.SetParent(root)
		web.SetParent(root)
		return &stack.Cluster{Name: "demo", Node: root}
	}
	for _, placement := range integratedPlacements {
		t.Run(string(placement), func(t *testing.T) {
			probe, err := integrateNoWait(cluster(srBundle("platform", cmApp("platform-app"))), nodeOnly, placement)
			if err != nil {
				t.Fatalf("probe: %v", err)
			}
			webPath := mustKustomization(t, probe, "web").Spec.Path
			terms, err := fluxstack.UmbrellaTerminals(probe)
			if err != nil {
				t.Fatalf("UmbrellaTerminals: %v", err)
			}
			if got := terms["flux-system/web"]; !slices.Equal(got, []string{"flux-system/leaf"}) {
				t.Fatalf("terminals of web = %v, want [flux-system/leaf]: the case needs a spec.path whose build applies a Kustomization", got)
			}
			ml, err := integrateNoWait(cluster(srBundle("platform", listApp("platform-app", "hand-written", webPath))), nodeOnly, placement)
			if err != nil {
				t.Fatalf("CreateLayoutWithResources: %v", err)
			}
			assertUmbrella(t, ml, "platform")
			if terms, err = fluxstack.UmbrellaTerminals(ml); err != nil {
				t.Fatalf("UmbrellaTerminals: %v", err)
			}
			if got := terms["flux-system/platform"]; !slices.Equal(got, []string{"flux-system/hand-written"}) {
				t.Errorf("terminals of platform = %v, want [flux-system/hand-written]", got)
			}
			if got, ok := terms["flux-system/hand-written"]; ok {
				t.Errorf("UmbrellaTerminals reads the application's hand-written as an umbrella with terminals %v", got)
			}
		})
	}
}

// Bundles merged into one directory share one Kustomization. Where it is an
// umbrella, R1 writes it without a wait, so an unset Wait and an explicit
// false do not conflict; where it is a leaf, the unset one resolves to true
// (R2) and the two do.
func TestUmbrellasDoNotWait_MergedBundleWaits(t *testing.T) {
	for _, placement := range integratedPlacements {
		t.Run("umbrella/"+string(placement), func(t *testing.T) {
			c := mergedCluster(func(rb, b1, _ *stack.Bundle) {
				rb.Children = []*stack.Bundle{srBundle("rb-child", cmApp("rb-child-app"))}
				b1.Wait = boolPtr(false)
			})
			ml, err := integrateNoWait(c, allFlat, placement)
			if err != nil {
				t.Fatalf("CreateLayoutWithResources: %v", err)
			}
			assertUmbrella(t, ml, "rb")
			assertLeafWaits(t, ml, "rb-child")
		})
		t.Run("leaf/"+string(placement), func(t *testing.T) {
			c := mergedCluster(func(_, b1, _ *stack.Bundle) { b1.Wait = boolPtr(false) })
			_, err := integrateNoWait(c, allFlat, placement)
			if err == nil || !strings.Contains(err.Error(), "their wait differ") {
				t.Fatalf("got %v, want the merged bundles' wait conflict", err)
			}
		})
	}
}

// A rewrite onto a leaf that exists only once the dependent has applied is
// refused however the leaf is reached: an application written as one file,
// or a leaf the dependent also names itself. Each tree is refused without the
// setting too, as a dependency on what the dependent creates.
func TestUmbrellasDoNotWait_CreationDeadlocks(t *testing.T) {
	cases := map[string]func(t *testing.T, placement layout.FluxPlacement) (*stack.Cluster, *layout.ManifestLayout, layout.LayoutRules){
		"an application written as one file": func(t *testing.T, placement layout.FluxPlacement) (*stack.Cluster, *layout.ManifestLayout, layout.LayoutRules) {
			c := createdLeafCluster()
			rules := withPlacement(propertyGroupings["nodeOnly"], placement)
			rules.ApplicationGrouping = layout.GroupByName
			return c, singleFile(t, c, rules, "b-app"), rules
		},
		"a leaf the dependent also names": func(t *testing.T, placement layout.FluxPlacement) (*stack.Cluster, *layout.ManifestLayout, layout.LayoutRules) {
			c := createdLeafCluster()
			a := c.Node.Children[0].Bundle
			a.DependsOn, a.NamedDependsOn = nil, []string{"hand-written", "b"}
			rules := withPlacement(propertyGroupings["nodeOnly"], placement)
			ml, err := layout.WalkCluster(c, rules)
			if err != nil {
				t.Fatalf("WalkCluster: %v", err)
			}
			return c, ml, rules
		},
	}
	for name, build := range cases {
		for _, placement := range integratedPlacements {
			t.Run(name+"/"+string(placement), func(t *testing.T) {
				c, ml, rules := build(t, placement)
				if err := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).IntegrateWithLayout(ml, c, rules); err == nil || !strings.Contains(err.Error(), "never") {
					t.Fatalf("without the setting: got %v, want a reconcile-order refusal", err)
				}
				c, ml, rules = build(t, placement)
				err := fluxstack.NewLayoutIntegrator(noWaitGenerator()).IntegrateWithLayout(ml, c, rules)
				if !errors.Is(err, fluxstack.ErrUmbrellaDependency) || !strings.Contains(err.Error(), "-> apply flux-system/a") {
					t.Fatalf("got %v, want ErrUmbrellaDependency naming the chain to a's apply", err)
				}
			})
		}
	}
}

// A dependsOn that names an umbrella and a leaf of another one awaits what R3
// writes it as: s, depending on x and umbrella b, awaits x and y, every
// terminal leaf of its sibling c. So platform's terminal leaf is s, before the
// rewrite and on the tree it wrote.
func TestUmbrellasDoNotWait_MixedNestedDependencies(t *testing.T) {
	cluster := func() *stack.Cluster {
		x := srBundle("x", cmApp("x-app"))
		b := umbrellaOf("b", srBundle("y", cmApp("y-app")))
		s := srBundle("s", cmApp("s-app"))
		s.DependsOn = []*stack.Bundle{x, b}
		return noWaitCluster(umbrellaOf("platform", umbrellaOf("c", x, b), s), true)
	}
	for _, placement := range integratedPlacements {
		t.Run(string(placement), func(t *testing.T) {
			c := cluster()
			rules := withPlacement(nodeOnly, placement)
			ml, err := fluxstack.NewLayoutIntegrator(noWaitGenerator()).CreateLayoutWithResources(c, rules)
			if err != nil {
				t.Fatalf("CreateLayoutWithResources: %v", err)
			}
			assertDependsOn(t, ml, "web", "s")
			assertDependsOn(t, ml, "s", "x", "y")
			terms, err := fluxstack.UmbrellaTerminals(ml)
			if err != nil {
				t.Fatalf("UmbrellaTerminals: %v", err)
			}
			if got := terms["flux-system/platform"]; !slices.Equal(got, []string{"flux-system/s"}) {
				t.Errorf("terminals of platform on the written tree = %v, want [flux-system/s]", got)
			}
			before := describeKustomizations(ml)
			if err := fluxstack.NewLayoutIntegrator(noWaitGenerator()).IntegrateWithLayout(ml, c, rules); err != nil {
				t.Fatalf("second IntegrateWithLayout: %v", err)
			}
			if after := describeKustomizations(ml); after != before {
				t.Errorf("after a second integration %s, want %s", after, before)
			}
		})
	}
}

// A second integration of a tree the setting wrote keeps every Kustomization
// as it is: a node's dependsOn the first wrote onto an umbrella's terminal
// leaves still satisfies what the node asks for. One it lacks is refused.
func TestUmbrellasDoNotWait_IntegrationIsRepeatable(t *testing.T) {
	nodes := func() (*stack.Cluster, layout.LayoutRules) {
		c, platform, apps := groupsCluster()
		apps.KustomizationName = "apps"
		apps.DependsOn = []*stack.Node{platform}
		return c, perLayoutNodeOnly()
	}
	t.Run("node dependsOn on an umbrella", func(t *testing.T) {
		c, rules := nodes()
		ml, err := fluxstack.NewLayoutIntegrator(noWaitGenerator()).CreateLayoutWithResources(c, rules)
		if err != nil {
			t.Fatalf("CreateLayoutWithResources: %v", err)
		}
		assertDependsOn(t, ml, "apps", "cert-manager")
		before := describeKustomizations(ml)
		if err := fluxstack.NewLayoutIntegrator(noWaitGenerator()).IntegrateWithLayout(ml, c, rules); err != nil {
			t.Fatalf("second IntegrateWithLayout: %v", err)
		}
		if after := describeKustomizations(ml); after != before {
			t.Errorf("after a second integration %s, want %s", after, before)
		}
	})
	t.Run("node dependsOn the kept Kustomization lacks", func(t *testing.T) {
		c, rules := nodes()
		ml, err := fluxstack.NewLayoutIntegrator(noWaitGenerator()).CreateLayoutWithResources(c, rules)
		if err != nil {
			t.Fatalf("CreateLayoutWithResources: %v", err)
		}
		findKept(ml, "apps").Spec.DependsOn = nil
		err = fluxstack.NewLayoutIntegrator(noWaitGenerator()).IntegrateWithLayout(ml, c, rules)
		if !errors.Is(err, fluxstack.ErrUmbrellaDependency) || !strings.Contains(err.Error(), "which lacks") {
			t.Fatalf("got %v, want the kept node Kustomization refused for the dependency it lacks, wrapping ErrUmbrellaDependency", err)
		}
	})
	// platform applies umbrellas mid (over x) and z (over z-leaf), and z
	// depends on mid: platform's terminal leaf is z-leaf. The first
	// integration writes apps -> z-leaf and z -> x, and on that tree
	// platform's terminal leaf is still z-leaf.
	t.Run("node dependsOn on an umbrella whose children depended on an umbrella", func(t *testing.T) {
		c := func() *stack.Cluster {
			mid := &stack.Node{Name: "mid", KustomizationName: "mid", Children: []*stack.Node{{Name: "x", Bundle: srBundle("x", cmApp("x-app"))}}}
			z := &stack.Node{Name: "z", KustomizationName: "z", DependsOn: []*stack.Node{mid}, Children: []*stack.Node{
				{Name: "z-leaf", Bundle: srBundle("z-leaf", cmApp("z-app"))},
			}}
			platform := &stack.Node{Name: "platform", KustomizationName: "platform", Children: []*stack.Node{mid, z}}
			apps := &stack.Node{Name: "apps", KustomizationName: "apps", DependsOn: []*stack.Node{platform}, Children: []*stack.Node{
				{Name: "shop", Bundle: srBundle("shop", cmApp("shop-app"))},
			}}
			return &stack.Cluster{Name: "demo", Node: &stack.Node{Name: "prod", Children: []*stack.Node{platform, apps}}}
		}()
		rules := perLayoutNodeOnly()
		ml, err := fluxstack.NewLayoutIntegrator(noWaitGenerator()).CreateLayoutWithResources(c, rules)
		if err != nil {
			t.Fatalf("CreateLayoutWithResources: %v", err)
		}
		assertDependsOn(t, ml, "apps", "z-leaf")
		assertDependsOn(t, ml, "z", "x")
		terms, err := fluxstack.UmbrellaTerminals(ml)
		if err != nil {
			t.Fatalf("UmbrellaTerminals: %v", err)
		}
		if got := terms["flux-system/platform"]; !slices.Equal(got, []string{"flux-system/z-leaf"}) {
			t.Errorf("terminals of platform on the written tree = %v, want [flux-system/z-leaf]", got)
		}
		before := describeKustomizations(ml)
		if err := fluxstack.NewLayoutIntegrator(noWaitGenerator()).IntegrateWithLayout(ml, c, rules); err != nil {
			t.Fatalf("second IntegrateWithLayout: %v", err)
		}
		if after := describeKustomizations(ml); after != before {
			t.Errorf("after a second integration %s, want %s", after, before)
		}
	})
	for _, placement := range integratedPlacements {
		t.Run("bundle units/"+string(placement), func(t *testing.T) {
			c := dependsOnShapes["depends on a nested umbrella"]()
			rules := withPlacement(nodeOnly, placement)
			ml, err := fluxstack.NewLayoutIntegrator(noWaitGenerator()).CreateLayoutWithResources(c, rules)
			if err != nil {
				t.Fatalf("CreateLayoutWithResources: %v", err)
			}
			before := describeKustomizations(ml)
			if err := fluxstack.NewLayoutIntegrator(noWaitGenerator()).IntegrateWithLayout(ml, c, rules); err != nil {
				t.Fatalf("second IntegrateWithLayout: %v", err)
			}
			if after := describeKustomizations(ml); after != before {
				t.Errorf("after a second integration %s, want %s", after, before)
			}
		})
	}
}
